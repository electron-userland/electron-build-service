import * as redis from "ioredis"
import * as os from "os"
import { Error } from "tslint/lib/error"
import Timer = NodeJS.Timer

// we use ioredis instead of node-redis because bull uses ioredis and we need to reuse connection

const address = require("network-address")

export class ServiceInfo {
  readonly hostname: string = os.hostname()
  readonly ipv4: string = address.ipv4()
  readonly ipv6: string = address.ipv6()

  constructor(readonly name: string, readonly port: string) {
  }
}

export class ServiceEntry {
  constructor(readonly info: ServiceInfo, readonly key: string, private readonly registry: ServiceRegistry, public destroyed = false, public timeoutHandle: Timer | null = null) {
  }

  leave() {
    return this.registry.leave(this)
  }
}

const KEY_PREFIX = "services/"

class ServiceChannels {
  static readonly JOIN = "service-join"
  static readonly LEAVE = "service-leave"
}

const DEFAULT_SERVICE_ENTRY_TTL = 30
// well, it is ok if client will get dead node address and will check health - but we will not DDoS our redis server and request new list on each client request
const DEFAULT_SERVICE_LIST_TTL = DEFAULT_SERVICE_ENTRY_TTL * 2

export class ServiceCatalog {
  private serviceListPromise: Promise<Array<ServiceInfo>> | null = null
  private lastUpdate = Date.now()

  constructor(private readonly store: redis.Redis) {
  }

  getServices(): Promise<Array<ServiceInfo>> {
    // refresh if last result is more than 60 seconds old ()
    if ((Date.now() - this.lastUpdate) > (DEFAULT_SERVICE_LIST_TTL * 1000)) {
      this.serviceListPromise = null
    }

    let result = this.serviceListPromise
    if (result != null) {
      return result
    }

    this.lastUpdate = Number.MAX_SAFE_INTEGER
    result = new Promise((resolve, reject) => {
      return this.store.keys(`${KEY_PREFIX}/*`, (error, result) => {
        if (error == null) {
          this.lastUpdate = Date.now()
          resolve(result.map(it => JSON.parse(it) as ServiceInfo))
        }
        else {
          this.lastUpdate = -1
          reject(error)
        }
      })
    })
    this.serviceListPromise = result
    return result
  }

  listen() {
    const subClient = this.store.duplicate()
    return new Promise((resolve, reject) => {
      (subClient.subscribe as (channel1: string, channel2: string, callback: (error: Error | null) => void) => void)(ServiceChannels.JOIN, ServiceChannels.LEAVE, error => {
        if (error != null) {
          reject(error)
          return
        }

        this.store.on("message", (channel, message) => {
          if (channel === ServiceChannels.JOIN || channel === ServiceChannels.LEAVE) {
            this.serviceListPromise = null
          }
          else {
            console.warn(`Unknown message from channel ${channel}: ${message}`)
          }
        })

        resolve()
      })
    })
  }
}

export class ServiceRegistry {
  // in seconds
  private expire = DEFAULT_SERVICE_ENTRY_TTL

  constructor(private readonly store: redis.Redis) {
    this.store.on("error", error => {
      console.log(error)
    })
  }

  private scheduleUpdateServiceEntryTtl(entry: ServiceEntry): void {
    const timeout = (this.expire - 2 /* 2 seconds earlier to ensure that redis will not remove key */) * 1000
    const updateTtl = () => {
      const callback: (erroe: Error | null, result: 0 | 1) => void = (error, result) => {
        if (entry.destroyed) {
          return
        }

        if (error != null) {
          console.error(error.stack || error.toString())
        }

        if (error == null && result === 1) {
          entry.timeoutHandle = setTimeout(updateTtl, timeout)
          return
        }

        entry.timeoutHandle = setTimeout(() => {
          this.store.setex(entry.key, this.expire, JSON.stringify(entry.info), callback)
        }, 1000 /* on error or if key key does not exist (removed by redis on expire), set after 1 second */)
      }
      this.store.expire(entry.key, this.expire, callback)
    }

    setTimeout(updateTtl, timeout)
  }

  join(serviceInfo: ServiceInfo): Promise<ServiceEntry> {
    const key = `${KEY_PREFIX}${serviceInfo.name}/${serviceInfo.ipv4}/${serviceInfo.port}`
    const value = JSON.stringify(serviceInfo)
    const entry = new ServiceEntry(serviceInfo, key, this)
    return this.store.multi()
      .setex(key, this.expire, value)
      .publish(ServiceChannels.JOIN, JSON.stringify(serviceInfo))
      .exec()
      .then(() => {
        this.scheduleUpdateServiceEntryTtl(entry)
        return entry
      })
  }

  leave(serviceEntry: ServiceEntry): Promise<void> {
    serviceEntry.destroyed = true
    return this.store
      .multi()
      .del(serviceEntry.key)
      .publish(ServiceChannels.LEAVE, serviceEntry.key)
      .exec()
  }

  // private _leave(list: Array<Entry>, cb) {
  //   const loop = () => {
  //     const next = list.shift()
  //     if (next == null) {
  //       return cb()
  //     }
  //
  //     clearTimeout(next.timeoutHandle)
  //     next.destroyed = true
  //
  //     const index = this.services.indexOf(next)
  //     if (index > -1) {
  //       this.services.splice(index, 1)
  //     }
  //     this.store.del(next.key, loop)
  //   }
  //
  //   loop()
  // }

  // lookup(name: string, cb) {
  //   this.list(name, (err, list) => {
  //     if (err) {
  //       return cb(err)
  //     }
  //     if (!list.length) {
  //       return cb(null, null)
  //     }
  //     cb(null, list[(Math.random() * list.length) | 0])
  //   })
  // }

  // list(name: string, cb) {
  //   return new Promise((resolve, reject) => {
  //     this.store.keys(`${prefix(name || "")}*`, (err, reply) => {
  //       if (err) {
  //         return cb(err)
  //       }
  //
  //       if (reply == null || reply.length === 0) {
  //         return cb(null, [])
  //       }
  //
  //       this.store.mget(reply, (err, replies) => {
  //         if (err) {
  //           return cb(err)
  //         }
  //         if (!replies || replies.length === 0) {
  //           return cb(null, [])
  //         }
  //
  //         const list = replies
  //           .map(node => {
  //             try {
  //               return JSON.parse(node)
  //             }
  //             catch (err) {
  //               return null
  //             }
  //           })
  //           .filter(val => val)
  //
  //         cb(null, list)
  //       })
  //     })
  //   })
  // }
}
