import { Queue } from "bull"
import { Stats } from "fs"
import { close, createWriteStream, fstat, open } from "fs-extra-p"
import { constants, IncomingHttpHeaders, ServerHttp2Stream } from "http2"
import { BuildTask, BuildTaskResult, getArchivePath, getBuildDir } from "./buildJobApi"
import { BuildServerConfiguration } from "./main"
import { removeFiles, Timer } from "./util"

const {
  HTTP2_HEADER_PATH,
  HTTP2_HEADER_STATUS,
  HTTP_STATUS_OK,
  HTTP_STATUS_INTERNAL_SERVER_ERROR,
} = constants

let tmpFileCounter = 0

export function handleBuildRequest(stream: ServerHttp2Stream, headers: IncomingHttpHeaders, configuration: BuildServerConfiguration, buildQueue: Queue) {
  const targets = headers["x-targets"]
  const platform = headers["x-platform"] as string

  function headerNotSpecified(name: string) {
    stream.respond({[HTTP2_HEADER_STATUS]: constants.HTTP_STATUS_BAD_REQUEST})
    stream.end(JSON.stringify({error: `Header ${name} is not specified`}))
  }

  if (targets == null) {
    headerNotSpecified("x-targets")
    return
  }
  if (platform == null) {
    headerNotSpecified("x-platform")
    return
  }

  const archiveName = `${(tmpFileCounter++).toString(16)}.zst`
  doHandleBuildRequest(stream, {
    archiveName,
    platform,
    targets: Array.isArray(targets) ? targets : [targets],
    zstdCompression: parseInt((headers["x-zstd-compression-level"] as string | undefined) || "-1", 10)
  }, buildQueue)
    .catch(error => {
      console.error(error)
      stream.respond({[HTTP2_HEADER_STATUS]: HTTP_STATUS_INTERNAL_SERVER_ERROR}, {endStream: true})
    })
}

async function doHandleBuildRequest(stream: ServerHttp2Stream, jobData: BuildTask, buildQueue: Queue) {
  const fd = await open(getArchivePath(jobData.archiveName),  "wx")

  // save to temp file
  const errorHandler = (error: Error) => {
    close(fd)
      .catch(error => console.error(error))
    console.error(error)
    if (!stream.destroyed) {
      stream.respond({[HTTP2_HEADER_STATUS]: HTTP_STATUS_INTERNAL_SERVER_ERROR}, {endStream: true})
    }
  }

  const fileStream = createWriteStream("", {
    fd,
    autoClose: false,
  })
  fileStream.on("error", errorHandler)

  if (process.env.KEEP_TMP_DIR_AFTER_BUILD == null) {
    stream.once("streamClosed", () => {
      // normally, archive file is already deleted, but -f flag is used for rm, so, it is ok
      removeFiles([getBuildDir(jobData.archiveName), getArchivePath(jobData.archiveName)])
    })
  }

  stream.on("error", errorHandler)
  const uploadTimer = new Timer(`Upload`)
  fileStream.on("finish", () => {
    fileUploaded(fd, uploadTimer, stream, jobData, buildQueue, errorHandler)
      .catch(errorHandler)
  })
  stream.pipe(fileStream)
}

async function fileUploaded(fd: number, uploadTimer: Timer, stream: ServerHttp2Stream, jobData: BuildTask, buildQueue: Queue, errorHandler: (error: Error) => void): Promise<void> {
  let stats: Stats | null = null
  try {
    stats = await fstat(fd)
  }
  catch (e) {
    console.error(e)
  }

  // it is ok to include stat time into upload time, stat call time is negligible
  jobData.archiveSize = stats === null ? -1 : stats.size
  jobData.uploadTime = uploadTimer.end(`Upload (size: ${jobData.archiveSize})`)

  if (stream.destroyed) {
    console.log("Client stream destroyed unexpectedly, job not added to queue")
    return
  }

  const job = await buildQueue.add(jobData)
  if (stream.destroyed) {
    console.log(`Client stream destroyed unexpectedly, discard job ${job.id}`)
    job.discard()
    return
  }

  let isBuilt = false
  const streamClosedHandler = () => {
    if (!isBuilt) {
      job.discard()
    }
  }
  stream.once("streamClosed", streamClosedHandler)

  const data: BuildTaskResult = await job.finished()
  isBuilt = true
  stream.removeListener("streamClosed", streamClosedHandler)

  if (stream.destroyed) {
    console.log("Client stream destroyed unexpectedly, do not send artifacts")
    return
  }

  if (data.error != null) {
    stream.respond({[HTTP2_HEADER_STATUS]: HTTP_STATUS_OK})
    stream.end(JSON.stringify(data))
    return
  }

  const artifacts = data.artifacts!!
  const artifactNames: Array<string> = []
  for (const artifact of artifacts) {
    const file = artifact.file
    const relativePath = file.substring(data.relativePathOffset!!)
    artifactNames.push(relativePath)
    // path must start with / otherwise stream will be not pushed
    const headers: any = {
      [HTTP2_HEADER_PATH]: relativePath,
    }
    if (artifact.target != null) {
      headers["x-target"] = artifact.target
    }
    if (artifact.arch != null) {
      headers["x-arch"] = artifact.arch
    }
    if (artifact.safeArtifactName != null) {
      // noinspection SpellCheckingInspection
      headers["x-safeartifactname"] = artifact.safeArtifactName
    }
    if (artifact.isWriteUpdateInfo) {
      // noinspection SpellCheckingInspection
      headers["x-iswriteupdateinfo"] = "1"
    }
    if (artifact.updateInfo != null) {
      // noinspection SpellCheckingInspection
      headers["x-updateinfo"] = JSON.stringify(artifact.updateInfo)
    }

    if (stream.destroyed) {
      console.log("Client stream destroyed unexpectedly, do not send artifacts")
      return
    }

    stream.pushStream(headers, pushStream => {
      pushStream.respondWithFile(file, undefined, {
        onError: errorHandler,
      })
    })
  }

  stream.respond({":status": 200})
  stream.end(JSON.stringify({files: artifactNames}))
}