import { Job, Queue } from "bull"
import { CancellationToken } from "electron-builder-lib"
import { stat, unlink } from "fs-extra-p"
import { IncomingMessage, request, RequestOptions, ServerResponse } from "http"
import * as path from "path"
import { ServiceEntry } from "service-registry-redis"
import { BuildTask, BuildTaskResult, getStageDir, TargetInfo } from "./buildJobApi"
import { removeFiles } from "./util"

const nanoid = require("nanoid")

class Task {
  readonly cancellationToken = new CancellationToken()

  constructor(readonly id: string) {
  }
}

// 30 minutes (quite enough for user to download file)
const ORPHAN_DIR_TTL = 30 * 60 * 1000
// const ORPHAN_DIR_TTL = 20 * 1000
// 1 minute
const ORPHAN_DIR_CHECK_TTL = 60 * 1000
// const ORPHAN_DIR_CHECK_TTL = 1000

let requestIdCounter = 0

export class BuildHandler {
  // delete local dir after 30 minutes to ensure that even if something goes wrong will be no orphan files
  private readonly createdIds = new Map<string, number>()
  private lastOrphanCheck = 0

  private _jobCount = 0

  serviceEntry: ServiceEntry | null = null

  get jobCount() {
    return this._jobCount
  }

  clientDownloadedAllFiles(id: string) {
    console.log(`Client completed: ${id}`)
    this.createdIds.delete(id)
    removeFiles(this.getJobDirs(id))
  }

  private readonly jobFinishedHandler = () => {
    this._jobCount--
    const serviceEntry = this.serviceEntry
    if (serviceEntry != null) {
      serviceEntry.info.jobCount = this._jobCount
    }
  }

  private getJobDirs(id: string) {
    return [this.builderTmpDir + path.sep + id, getStageDir() + path.sep + id]
  }

  constructor(private readonly buildQueue: Queue, private readonly builderTmpDir: string) {
    buildQueue.on("active",  job => {
      publishEvent('{"state": "started"}', createPublishEventRequestOptions(job))
    })
  }

  private checkOrphanDirs(createdIds: Map<string, number>) {
    if (createdIds.size <= 0) {
      return
    }

    const now = Date.now()
    if ((now - this.lastOrphanCheck) < ORPHAN_DIR_CHECK_TTL) {
      return
    }

    const toDelete: Array<string> = []
    for (const id of Array.from(createdIds.keys())) {
      const createdAt = createdIds.get(id)!!
      if ((now - createdAt) > ORPHAN_DIR_TTL) {
        createdIds.delete(id)
        toDelete.push(...this.getJobDirs(id))
      }
    }
    if (toDelete.length > 0) {
      console.log(`Delete orphan files: ${toDelete.join(", ")}`)
      removeFiles(toDelete)
    }

    this.lastOrphanCheck = now
  }

  handleBuildRequest(response: ServerResponse, request: IncomingMessage) {
    const createdIds = this.createdIds
    this.checkOrphanDirs(createdIds)

    const headers = request.headers
    const rawTargets = headers["x-targets"]
    const platform = headers["x-platform"] as string
    let archiveFile = headers["x-file"] as string

    // required for development (when node is not managed by docker and cannot access to docker volume)
    const projectArchiveDir = process.env.PROJECT_ARCHIVE_DIR_PARENT
    if (projectArchiveDir != null) {
      archiveFile = projectArchiveDir + archiveFile
    }

    function headerNotSpecified(name: string, message: string = "is not specified") {
      response.statusCode = 400
      if (archiveFile != null) {
        unlink(archiveFile)
          .catch(error => {
            console.error(`Cannot delete archiveFile on incorrect header: ${error}`)
          })
      }
      response.end(JSON.stringify({error: `Header ${name} ${message}`}))
    }

    if (rawTargets == null) {
      headerNotSpecified("x-targets")
      return
    }

    let targets: Array<TargetInfo>
    try {
      targets = JSON.parse(rawTargets as string)
    }
    catch (e) {
      headerNotSpecified(e, "is incorrect, valid JSON is expected: " + e.message)
      return
    }

    if (platform == null) {
      headerNotSpecified("x-platform")
      return
    }

    if (archiveFile == null) {
      const message = `Internal error: header x-file is not specified`
      console.error(message)
      response.statusCode = 500
      response.end(JSON.stringify({error: message}))
      return
    }

    // UUID uses 128-bit, to be sure, we use 144-bit
    // base64 is not safe for fs / query / path (also, we use this id for file name - avoid uppercase vs lowercase)
    // counter makes id unique, random 8 bytes secure
    const requestId = `${(requestIdCounter++).toString(36)}-${nanoid(6)}`
    const task = new Task(requestId)
    request.on("aborted", () => {
      console.log(`Request ${requestId} aborted`)
      task.cancellationToken.cancel()
    })

    createdIds.set(requestId, Date.now())

    addJob(response, {
      archiveFile,
      platform,
      targets,
      zstdCompression: parseInt((headers["x-zstd-compression-level"] as string | undefined) || "-1", 10)
    }, this.buildQueue, task)
      .then(job => {
        if (job == null) {
          return
        }

        const jobFinished = job.finished()
        this._jobCount++
        jobFinished
          .then(this.jobFinishedHandler)
          .catch(this.jobFinishedHandler)

        const eventRequestOptions = createPublishEventRequestOptions(job)
        // create channel before send response to upload, push_stream_authorized_channels_only is set to "on", so, channel must be created prior to client connect
        publishEvent('{"state": "added}', eventRequestOptions)

        response.statusCode = 200
        response.end(`{"id": "${requestId}"}`)

        pushResult(job, jobFinished, eventRequestOptions)
          .catch(error => {
            removeFiles([archiveFile])
            console.error(`Unexpected error on pushResult: ${error.stack || error}`)
          })
      })
      .catch(error => {
        unlink(archiveFile)
          .catch(error => {
            console.error(`Cannot delete archiveFile on error: ${error}`)
          })

        console.error(error)
        response.statusCode = 500
        response.end()
      })
  }
}

async function addJob(response: ServerResponse, jobData: BuildTask, buildQueue: Queue, task: Task): Promise<Job | null> {
  const stats = await stat(jobData.archiveFile)
  jobData.archiveSize = stats.size

  if (task.cancellationToken.cancelled) {
    console.log(`Task ${task.id} cancelled unexpectedly - not added to queue`)
    return null
  }

  // in any case we need to pass requestId to builder (used as name of the build dir), so, instead of additional field, just use job id
  const job = await buildQueue.add(jobData, {jobId: task.id})
  if (task.cancellationToken.cancelled) {
    console.log(`Task ${task.id} cancelled unexpectedly - job discarded`)
    job.discard()
    return null
  }

  return job
}

const isDebugEnabled = process.env.DEBUG != null && process.env.DEBUG!!.includes("electron-builder")

function publishEvent(data: string, eventRequestOptions: RequestOptions) {
  const eventRequest = request(eventRequestOptions)
  eventRequest.on("error", error => {
    console.error(`Cannot publish event: ${error.stack || error}`)
  })
  eventRequest.write(data.length.toString(10))
  if (isDebugEnabled) {
    console.log(`Publish event: ${data}`)
  }
  eventRequest.write(data)
  eventRequest.end()
}

function createPublishEventRequestOptions(job: Job): RequestOptions {
  const result: RequestOptions = {
    method: "POST",
    path: `/publish-build-event?id=${job.id}`
  }
  if (process.env.NGINX_ADDRESS) {
    result.host = process.env.NGINX_ADDRESS!!
    result.port = 8001
  }
  else {
    result.socketPath = "/socket/nginx.socket"
  }
  return result
}

async function pushResult(job: Job, jobFinished: Promise<BuildTaskResult>, eventRequestOptions: RequestOptions) {
  let data: BuildTaskResult
  try {
    data = await jobFinished
  }
  catch (error) {
    console.error(`Job ${job.id} error: ${error.stack || error}`)
    publishEvent('{"error": "internal server error"}', eventRequestOptions)
    return
  }

  if (data.error != null) {
    console.error(`Job ${job.id} error: ${data.error}`)
    publishEvent(JSON.stringify(data), eventRequestOptions)
    return
  }

  publishEvent(JSON.stringify({files: data.artifacts}), eventRequestOptions)
}