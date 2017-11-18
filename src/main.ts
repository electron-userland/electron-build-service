import * as Queue from "bull"
import { emptyDir } from "fs-extra-p"
import { constants, createSecureServer } from "http2"
import * as os from "os"
import * as path from "path"
import { handleBuildRequest } from "./buildHandler"
import { prepareBuildTools } from "./download-required-tools"
import { getSslOptions } from "./sslKeyAndCert"

const {
  HTTP2_HEADER_PATH,
  HTTP2_HEADER_STATUS,
  HTTP_STATUS_NOT_FOUND
} = constants

export interface BuildServerConfiguration {
  readonly queueName: string
}

// clean queue (wait and delayed jobs) on restart since in any case client task is cancelled on abort
async function cancelOldJobs(queue: any) {
  const waitingJobs = await queue.getWaiting()
  for (const job of waitingJobs) {
    job.discard()
  }

  const delayedJobs = await queue.getDelayed()
  for (const job of delayedJobs) {
    job.discard()
  }

  console.log(`Discarded jobs: waiting ${waitingJobs.length}, delayed: ${delayedJobs.length}`)
}

async function main() {
  const redisEndpoint = process.env.REDIS_ENDPOINT
  if (redisEndpoint == null || redisEndpoint.length === 0) {
    throw new Error(`Env REDIS_ENDPOINT must be set to Redis database endpoint. Free plan on https://redislabs.com is suitable.`)
  }

  const port = process.env.ELECTRON_BUILD_SERVICE_PORT ? parseInt(process.env.ELECTRON_BUILD_SERVICE_PORT!!, 10) : 443
  // if port < 1024 it means that we are in the docker container / special server for service, and so, no need to use path qualifier in the tmp dir
  const isDockerOrServer = port < 1024
  const stageDir = isDockerOrServer ? os.tmpdir() : path.join(os.tmpdir(), "electron-build-server")
  process.env.STAGE_DIR = stageDir
  if (process.env.ELECTRON_BUILDER_TMP_DIR == null) {
    process.env.ELECTRON_BUILDER_TMP_DIR = os.tmpdir()
  }

  const configuration: BuildServerConfiguration = {
    queueName: `build-${os.hostname()}`
  }

  const buildQueue = new Queue(configuration.queueName, redisEndpoint.startsWith("redis://") ? redisEndpoint : `redis://${redisEndpoint}`)
  buildQueue.on("error", error => {
    console.error(error)
  })

  await Promise.all([
    cancelOldJobs(buildQueue),
    prepareBuildTools(),
    isDockerOrServer ? Promise.resolve() : emptyDir(stageDir!),
  ])

  const isSandboxed = process.env.SANDBOXED_BUILD_PROCESS !== "false"
  const concurrency = isSandboxed ? (os.cpus().length + 1) : 1
  const builderPath = path.join(__dirname, "builder.js")
  // noinspection JSIgnoredPromiseFromCall
  buildQueue.process(concurrency, isSandboxed ? builderPath : require(builderPath).default)

  const server = createSecureServer(await getSslOptions())
  server.on("stream", (stream, headers) => {
    const requestPath = headers[HTTP2_HEADER_PATH]
    if (requestPath !== "/v1/build") {
      stream.respond({ [HTTP2_HEADER_STATUS]: HTTP_STATUS_NOT_FOUND }, {endStream: true})
      return
    }

    handleBuildRequest(stream, headers, configuration, buildQueue)
  })

  // callback null if sync exit
  require("async-exit-hook")((callback: (() => void) | null) => {
    console.log("Exit signal received, stopping server and queue")

    let serverClosed = false
    let queueStopped = false
    const closed = (label: string) => {
      console.log(label)
      if (serverClosed && queueStopped && callback != null) {
        callback()
      }
    }

    server.close(() => {
      serverClosed = true
      closed("Server stopped")
    })
    buildQueue.close()
      .then(() => {
        queueStopped = true
        closed("Build queue closed")
      })
      .catch(error => {
        queueStopped = true
        closed(`Build queue closed (with error: ${error.stack || error})`)
      })
  })

  return new Promise((resolve, reject) => {
    server.on("error", reject)
    // LISTEN_FDS - systemd socket
    server.listen(process.env.LISTEN_FDS == null ? (port) : {fd: 3}, () => {
      console.log(`Server listening on ${server.address().address}:${server.address().port}, concurrency: ${concurrency}, tmpfs: ${process.env.ELECTRON_BUILDER_TMP_DIR || "no"}, ${JSON.stringify(configuration, null, 2)}`)
      resolve()
    })
  })
}

main()
  .catch(error => {
    console.error(error.stack || error)
    process.exit(1)
  })