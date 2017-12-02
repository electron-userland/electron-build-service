import { chmod } from "fs"
import { Server } from "net"

export interface ListenOptions {
  readonly socketName: string
  readonly explicitPort: string | undefined
  readonly extraMessage?: string
}

export function listen(server: Server, options: ListenOptions) {
  return new Promise((resolve, reject) => {
    server.on("error", reject)
    const  explicitPort = options.explicitPort
    if (explicitPort != null && explicitPort.length !== 0) {
      server.listen(parseInt(explicitPort!, 10), () => {
        console.log(`Server listening on ${server.address().address}:${server.address().port}${options.extraMessage}`)
        resolve()
      })
      return
    }

    const socketPath = `/socket/${options.socketName}.socket`
    server.listen(socketPath, () => {
      chmod(socketPath, 0o777, error => {
        if (error == null) {
          console.log(`Server listening on ${socketPath}${options.extraMessage}`)
          resolve()
        }
        else {
          reject(new Error(`Cannot set socket mode to 777: ${error.stack || error}`))
        }
      })
    })
  })
}