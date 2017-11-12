import { Queue } from "bull"
import { close, createWriteStream, fstat, open } from "fs-extra-p"
import { constants, IncomingHttpHeaders, ServerHttp2Stream } from "http2"
import * as path from "path"
import { BuildTask, BuildTaskResult } from "./builder"
import { BuildServerConfiguration } from "./main"
import { Timer } from "./util"

const {
  HTTP2_HEADER_PATH,
  HTTP2_HEADER_STATUS,
  HTTP_STATUS_OK,
  HTTP_STATUS_INTERNAL_SERVER_ERROR,
} = constants

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

  const archiveFile = path.join(configuration.pendingAppArchiveDir, `${process.pid.toString(16)}-${Date.now().toString(16)}-${Math.floor(Math.random() * 1024 * 1024).toString(16)}.zst`)
  doHandleBuildRequest({
    app: archiveFile,
    platform,
    targets: Array.isArray(targets) ? targets : [targets],
    zstdCompression: parseInt((headers["x-zstd-compression-level"] as string | undefined) || "-1", 10)
  }, stream, buildQueue)
    .catch(error => {
      console.error(error)
      stream.respond({[HTTP2_HEADER_STATUS]: HTTP_STATUS_INTERNAL_SERVER_ERROR}, {endStream: true})
    })
}

async function doHandleBuildRequest(jobData: BuildTask, stream: ServerHttp2Stream, buildQueue: Queue) {
  const fd = await open(jobData.app, "wx")

  // save to temp file
  const errorHandler = (error: Error) => {
    close(fd)
      .catch(error => console.error(error))
    console.error(error)
    stream.respond({[HTTP2_HEADER_STATUS]: HTTP_STATUS_INTERNAL_SERVER_ERROR}, {endStream: true})
  }

  const fileStream = createWriteStream("", {
    fd,
    autoClose: false,
  })
  fileStream.on("error", errorHandler)

  stream.on("error", errorHandler)
  const uploadTimer = new Timer(`Upload`)
  fileStream.on("finish", () => {
    fstat(fd)
      .catch(error => {
        // ignore
        console.error(error)
        return null
      })
      .then(stats => {
        // it is ok to include stat time into upload time, stat call time is negligible
        jobData.archiveSize = stats == null ? -1 : stats.size
        jobData.uploadTime = uploadTimer.end(`Upload (size: ${jobData.archiveSize})`)
        return buildQueue.add(jobData)
      })
      .then(job => job.finished() as any as Promise<BuildTaskResult>)
      .then((data: BuildTaskResult) => {
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
          stream.pushStream(headers, pushStream => {
            pushStream.respondWithFile(file, undefined, {
              onError: errorHandler,
            })
          })
        }

        stream.respond({":status": 200})
        stream.end(JSON.stringify({files: artifactNames}))
      })
      .catch(errorHandler)
  })
  stream.pipe(fileStream)
}