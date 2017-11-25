import { createServer, IncomingMessage, ServerResponse } from "http"
import { createRedisClient, ServiceCatalog } from "service-registry-redis"

async function main() {
  const redisClient = createRedisClient()
  const catalog = new ServiceCatalog(redisClient)

  const server = createServer(((request, response) => {
    const url = request.url
    if (url === "/" || url == null || url.length === 0 || url.startsWith("/find-build-agent")) {
      handleRequest(response, request, catalog)
        .catch(error => {
          console.error(error.stack || error.toString())
          response.statusCode = 500
          response.end()
        })
    }
    else {
      console.error(`Unsupported route: ${url}`)
      response.statusCode = 404
      response.end()
    }
  }))

  require("async-exit-hook")((callback: (() => void) | null) => {
    console.log("Exit signal received, stopping server and queue")

    let serverClosed = false
    const closed = (label: string) => {
      console.log(label)
      if (serverClosed && callback != null) {
        callback()
      }
    }

    server.close(() => {
      serverClosed = true
      closed("Server stopped")
    })
  })

  await new Promise((resolve, reject) => {
    server.on("error", reject)
    const explicitPort = process.env.ELECTRON_BUILD_SERVICE_ROUTER_PORT
    const port = explicitPort ? parseInt(explicitPort!, 10) : 80
    server.listen(port, () => {
      console.log(`Server listening on ${server.address().address}:${server.address().port}`)
      resolve()
    })
  })
}

async function handleRequest(response: ServerResponse, request: IncomingMessage, catalog: ServiceCatalog) {
  const list = await catalog.getServices()
  if (list.length === 0) {
    console.error("No running build agents")
    response.statusCode = 503
    response.end('{"error: "No running build agents"}')
    return
  }

  // todo take geo position in account
  const service = list[Math.floor(Math.random() * list.length)]
  return `{"endpoint": "https://${service.ip}:${service.port || 443}"}`
}

main()
  .catch(error => {
    console.error(error.stack || error)
    process.exit(1)
  })