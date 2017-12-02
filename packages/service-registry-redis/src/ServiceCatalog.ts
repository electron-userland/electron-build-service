import * as redis from "ioredis"
import { DEFAULT_SERVICE_ENTRY_TTL, KEY_PREFIX, ServiceChannels, ServiceInfo } from "./service-discovery"

// well, it is ok if client will get dead node address and will check health - but we will not DDoS our redis server and request new list on each client request
const DEFAULT_SERVICE_LIST_TTL_MS = DEFAULT_SERVICE_ENTRY_TTL * 2 * 1000

export class ServiceCatalog {
  private serviceListPromise: Promise<Array<ServiceInfo>> | null = null
  private lastUpdate = Date.now()

  constructor(private readonly store: redis.Redis) {
  }

  getServices(): Promise<Array<ServiceInfo>> {
    // refresh if last result is more than n seconds old
    if ((Date.now() - this.lastUpdate) > DEFAULT_SERVICE_LIST_TTL_MS) {
      this.serviceListPromise = null
    }

    let result = this.serviceListPromise
    if (result != null) {
      return result
    }

    this.lastUpdate = Number.MAX_SAFE_INTEGER
    result = this.store.keys(`${KEY_PREFIX}*`)
      .then(serviceKeys => serviceKeys == null || serviceKeys.length === 0 ? [] : this.store.mget(serviceKeys as any))
      .then((result: Array<string>) => {
        this.lastUpdate = Date.now()
        return result
          .filter(it => it != null && it.length !== 0)
          .map(it => JSON.parse(it) as ServiceInfo)
      })
      .catch(e => {
        this.lastUpdate = -1
        throw e
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