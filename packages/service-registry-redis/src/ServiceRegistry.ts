import * as redis from "ioredis"
import { DEFAULT_SERVICE_ENTRY_TTL, KEY_PREFIX, ServiceChannels, ServiceEntry, ServiceInfo } from "./service-discovery"

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
      const callback: (error: Error | null, result: 0 | 1) => void = (error, result) => {
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

  join(name: string, serviceInfo: ServiceInfo): Promise<ServiceEntry> {
    const key = `${KEY_PREFIX}${name}/${serviceInfo.ip}/${serviceInfo.port}`
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