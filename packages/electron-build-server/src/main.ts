import * as Queue from "bull"
import { emptyDir, unlink } from "fs-extra-p"
import { createServer } from "http"
import * as redis from "ioredis"
import * as os from "os"
import * as path from "path"
import { createRedisClient, createServiceInfo } from "service-registry-redis"
import { ServiceRegistry } from "service-registry-redis/out/ServiceRegistry"
import { BuildHandler } from "./buildHandler"
import { getStageDir } from "./buildJobApi"
import { prepareBuildTools } from "./download-required-tools"

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

function getQueueName() {
  let name = os.hostname()
  const prefix = "bs-"
  if (name.startsWith(prefix)) {
    name = name.substring(prefix.length)
  }
  return `build-${name}`
}

function setupBuilderTmpDir() {
  let builderTmpDir = process.env.ELECTRON_BUILDER_TMP_DIR
  if (builderTmpDir == null) {
    builderTmpDir = os.tmpdir() + path.sep + "builder-tmp"
    process.env.ELECTRON_BUILDER_TMP_DIR = builderTmpDir
  }
  else if (builderTmpDir === os.tmpdir() || os.homedir().startsWith(builderTmpDir) || builderTmpDir === "/") {
    throw new Error(`${builderTmpDir} cannot be used as ELECTRON_BUILDER_TMP_DIR because this dir will be emptied`)
  }
  return builderTmpDir
}

async function main() {
  const builderTmpDir = setupBuilderTmpDir()
  const redisClient = await createRedisClient()
  let subscriber: redis.Redis | null = null
  const queueName = getQueueName()
  const buildQueue = new Queue(queueName, {
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

  const stageDir = getStageDir()
  await Promise.all([
    cancelOldJobs(buildQueue),
    prepareBuildTools(),
    emptyDir(stageDir),
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
      const localFile = stageDir + request.headers["x-file"]
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

    const serviceEntry = buildHandler.serviceEntry
    if (serviceEntry != null) {
      serviceEntry.leave()
        .catch(error => {
          console.warn(`Service unregistered (with error: ${error.stack || error})`)
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

  await new Promise((resolve, reject) => {
    server.on("error", reject)
    const port = process.env.ELECTRON_BUILD_SERVICE_PORT ? parseInt(process.env.ELECTRON_BUILD_SERVICE_PORT!!, 10) : 80
    server.listen(port, () => {
      console.log(`Server listening on ${server.address().address}:${server.address().port}, concurrency: ${concurrency}, temp dir: ${process.env.ELECTRON_BUILDER_TMP_DIR || "no"}, queueName: ${queueName}`)
      resolve()
    })
  })

  const serviceRegistry = new ServiceRegistry(redisClient)
  const serviceInfo = await createServiceInfo("443")
  console.log(JSON.stringify(serviceInfo, null, 2))
  buildHandler.serviceEntry = await serviceRegistry.join("builder", serviceInfo)
}

main()
  .catch(error => {
    console.error(error.stack || error)
    process.exit(1)
  })