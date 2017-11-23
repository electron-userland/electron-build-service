import * as Queue from "bull"
import { emptyDir, unlink } from "fs-extra-p"
import { createServer } from "http"
import * as redis from "ioredis"
import * as os from "os"
import * as path from "path"
import { ServiceEntry, ServiceInfo, ServiceRegistry } from "service-registry-redis"
import { BuildHandler } from "./buildHandler"
import { prepareBuildTools } from "./download-required-tools"

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

  const port = process.env.ELECTRON_BUILD_SERVICE_PORT ? parseInt(process.env.ELECTRON_BUILD_SERVICE_PORT!!, 10) : 80
  let builderTmpDir = process.env.ELECTRON_BUILDER_TMP_DIR
  if (builderTmpDir == null) {
    builderTmpDir = os.tmpdir() + path.sep + "builder-tmp"
    process.env.ELECTRON_BUILDER_TMP_DIR = builderTmpDir
  }
  else if (builderTmpDir === os.tmpdir() || os.homedir().startsWith(builderTmpDir) || builderTmpDir === "/") {
    throw new Error(`${builderTmpDir} cannot be used as ELECTRON_BUILDER_TMP_DIR because this dir will be emptied`)
  }

  const configuration: BuildServerConfiguration = {
    queueName: `build-${os.hostname()}`
  }

  const redisClient = redis(redisEndpoint.startsWith("redis://") ? redisEndpoint : `redis://${redisEndpoint}`)
  let subscriber: redis.Redis | null = null
  const buildQueue = new Queue(configuration.queueName, {
    createClient: type => {
      switch (type) {
        case "client":
          return redisClient
        case "subscriber":
          if (subscriber == null) {
            subscriber = redisClient.duplicate()
          }
          return subscriber
        default:
          return redisClient.duplicate()
      }
    }
  })
  buildQueue.on("error", error => {
    console.error(error)
  })

  await Promise.all([
    cancelOldJobs(buildQueue),
    prepareBuildTools(),
    emptyDir((process.env.PROJECT_ARCHIVE_DIR_PARENT || "") + "/uploaded-projects"),
    emptyDir(builderTmpDir),
  ])

  const isSandboxed = process.env.SANDBOXED_BUILD_PROCESS !== "false"
  const concurrency = isSandboxed ? (os.cpus().length + 1) : 1
  const builderPath = path.join(__dirname, "builder.js")
  // noinspection JSIgnoredPromiseFromCall
  buildQueue.process(concurrency, isSandboxed ? builderPath : require(builderPath))

  const buildHandler = new BuildHandler(buildQueue, builderTmpDir)
  const server = createServer(((request, response) => {
    const url = request.url
    if (url === "/v1/upload") {
      buildHandler.handleBuildRequest(response, request)
    }
    else if (url != null && url.startsWith("/downloaded")) {
      const localFile = builderTmpDir!! + request.headers["x-file"]
      console.log(`Delete downloaded file: ${localFile}`)
      unlink(localFile)
        .catch(error => {
          console.error(`Cannot delete file ${localFile}: ${error.stack || error}`)
        })
    }
    else {
      console.error(`Unsupported route: ${url}`)
      response.statusCode = 404
      response.end()
    }
  }))
  let serviceEntry: ServiceEntry | null = null

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

    if (serviceEntry != null) {
      serviceEntry.leave()
        .catch(error => {
          console.warn(`Build queue closed (with error: ${error.stack || error})`)
        })
    }

    server.close(() => {
      serverClosed = true
      closed("Server stopped")
    })
    buildQueue.close()
      .then(() => {
        redisClient.disconnect()
        if (subscriber != null) {
          subscriber.disconnect()
        }
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
    server.listen(port, () => {
      console.log(`Server listening on ${server.address().address}:${server.address().port}, concurrency: ${concurrency}, tmpfs: ${process.env.ELECTRON_BUILDER_TMP_DIR || "no"}, ${JSON.stringify(configuration, null, 2)}`)

      const serviceRegistry = new ServiceRegistry(redisClient)
      serviceRegistry.join(new ServiceInfo("builder", "443"))
        .then(it => {
          serviceEntry = it
          resolve()
        })
        .catch(reject)
    })
  })
}

main()
  .catch(error => {
    console.error(error.stack || error)
    process.exit(1)
  })