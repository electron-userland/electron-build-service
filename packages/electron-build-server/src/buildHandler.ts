import { Job, Queue } from "bull"
import { CancellationToken } from "electron-builder-lib"
import { stat, unlink } from "fs-extra-p"
import { IncomingMessage, request, RequestOptions, ServerResponse } from "http"
import * as path from "path"
import { BuildTask, BuildTaskResult } from "./buildJobApi"
import { removeFiles } from "./util"

const nanoid = require("nanoid")

class Task {
  readonly cancellationToken = new CancellationToken()

  constructor(readonly id: string) {
  }
}

// 60 minutes (quite enough for user to download file)
// const ORPHAN_DIR_TTL = 60 * 60 * 1000
const ORPHAN_DIR_TTL = 20 * 1000
// 1 minute
// const ORPHAN_DIR_CHECK_TTL = 60 * 1000
const ORPHAN_DIR_CHECK_TTL = 1000

let requestIdCounter = 0

export class BuildHandler {
  // delete local dir after 30 minutes to ensure that even if something goes wrong will be no orphan files
  private readonly createdIds = new Map<string, number>()
  private lastOrphanCheck = 0

  constructor(private readonly buildQueue: Queue, private readonly builderTmpDir: string) {
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
        toDelete.push(this.builderTmpDir + path.sep + id)
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
    const targets = headers["x-targets"]
    const platform = headers["x-platform"] as string
    let archiveFile = headers["x-file"] as string

    // required for development (when node is not managed by docker and cannot access to docker volume)
    const projectArchiveDir = process.env.PROJECT_ARCHIVE_DIR_PARENT
    if (projectArchiveDir != null) {
      archiveFile = projectArchiveDir + archiveFile
    }

    function headerNotSpecified(name: string) {
      response.statusCode = 400
      if (archiveFile != null) {
        unlink(archiveFile)
          .catch(error => {
            console.error(`Cannot delete archiveFile on incorrect header: ${error}`)
          })
      }
      response.end(JSON.stringify({error: `Header ${name} is not specified`}))
    }

    if (targets == null) {
      headerNotSpecified("x-targets")
      return
    }
    if (platform == null) {
      headerNotSpecified("x-platform")
      return
    }

    if (archiveFile == null) {
      response.statusCode = 500
      response.end(JSON.stringify({error: `Internal error: header x-file is not specified`}))
      return
    }

    // UUID uses 128-bit, to be sure, we use 144-bit
    // base64 is not safe for fs / query / path (also, we use this id for file name - avoid uppercase vs lowercase)
    // counter makes id unique, random 4 bytes secure
    const requestId = `${(requestIdCounter++).toString(36)}-${nanoid(8)}`
    const task = new Task(requestId)
    request.on("aborted", () => {
      console.log(`Request ${requestId} aborted`)
      task.cancellationToken.cancel()
    })

    createdIds.set(requestId, Date.now())

    doHandleBuildRequest(response, {
      archiveFile,
      platform,
      targets: Array.isArray(targets) ? targets : [targets],
      zstdCompression: parseInt((headers["x-zstd-compression-level"] as string | undefined) || "-1", 10)
    }, this.buildQueue, task)
      .then(job => {
        if (job == null) {
          return
        }

        response.statusCode = 200
        response.end(`{"id": "${requestId}"}`)

        pushResult(task, job)
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

async function pushResult(task: Task, job: Job) {
  const eventRequestOptions: RequestOptions = {
    host: process.env.NGINX_ADDRESS || "nginx",
    port: 8001,
    method: "POST",
    path: `/publish-build-event?id=${task.id}`
  }

  function publishEvent(data: string) {
    const eventRequest = request(eventRequestOptions)
    eventRequest.on("error", error => {
      console.error(`Cannot publish event: ${error.stack || error}`)
    })
    eventRequest.write(data.length.toString(10))
    eventRequest.write(data)
    eventRequest.end()
  }

  // create channel, push_stream_authorized_channels_only is set to "on", so, channel must be created prior to client connect
  publishEvent("job added")

  let data: BuildTaskResult
  try {
    data = await job.finished()
  }
  catch (error) {
    console.error(`Job ${job.id} error: ${error.stack || error}`)
    publishEvent('{"error": "internal server error"}')
    return
  }

  if (data.error != null) {
    publishEvent(JSON.stringify(data))
    return
  }

  publishEvent(JSON.stringify({files: data.artifacts}))
}

async function doHandleBuildRequest(response: ServerResponse, jobData: BuildTask, buildQueue: Queue, task: Task): Promise<Job | null> {
  const stats = await stat(jobData.archiveFile)
  jobData.archiveSize = stats.size

  if (task.cancellationToken.cancelled) {
    console.log(`Task ${task.id} cancelled unexpectedly - not added to queue`)
    return null
  }

  const job = await buildQueue.add(jobData, {jobId: task.id})
  if (task.cancellationToken.cancelled) {
    console.log(`Task ${task.id} cancelled unexpectedly - job discarded`)
    job.discard()
    return null
  }

  return job
}