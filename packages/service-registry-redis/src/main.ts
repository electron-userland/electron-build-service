import * as redis from "ioredis"
import * as needle from "needle"
import * as os from "os"
import { promisify } from "util"
import Timer = NodeJS.Timer

// we use ioredis instead of node-redis because bull uses ioredis and we need to reuse connection

export function createRedisClient() {
  const redisEndpoint = process.env.REDIS_ENDPOINT
  if (redisEndpoint == null || redisEndpoint.length === 0) {
    throw new Error(`Env REDIS_ENDPOINT must be set to Redis database endpoint. Free plan on https://redislabs.com is suitable.`)
  }
  return redis(redisEndpoint.startsWith("redis://") ? redisEndpoint : `redis://${redisEndpoint}`)
}

export async function createServiceInfo(name: string, port: string) {
  const setTimeoutPromise = promisify(setTimeout)
  let data: any = null
  for (let attempt = 0; attempt < 3; attempt++) {
    try {
      data = (await needle("get", "https://ipapi.co/json/")).body
      break
    }
    catch (e) {
      console.error(`Cannot get IP info: ${e.stack || e}`)
      if (attempt === 2) {
        throw e
      }

      await setTimeoutPromise(1000 * attempt, "wait")
    }
  }

  return new ServiceInfo(name, data.ip, port, data.latitude, data.longitude)
}

export class ServiceInfo {
  readonly hostname: string = os.hostname()

  constructor(readonly name: string, readonly ip: string, readonly port: string, readonly latitude: number, readonly longitude: number) {
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
    const key = `${KEY_PREFIX}${serviceInfo.name}/${serviceInfo.ip}/${serviceInfo.port}`
    const value = JSON.stringify(serviceInfo)
    const entry = new ServiceEntry(serviceInfo, key, this)
    return this.store.multi()
      .setex(key, this.expire, value)
      .publish(ServiceChannels.JOIN, value)
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
}
