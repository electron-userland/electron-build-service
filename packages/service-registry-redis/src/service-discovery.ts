import * as redis from "ioredis"
import * as needle from "needle"
import * as os from "os"
import { promisify } from "util"
import { ServiceRegistry } from "./ServiceRegistry"
import Timer = NodeJS.Timer

/** @internal */
export const KEY_PREFIX = "services/"
/** @internal */
export const DEFAULT_SERVICE_ENTRY_TTL = 30

// we use ioredis instead of node-redis because bull uses ioredis and we need to reuse connection
export function createRedisClient() {
  const redisEndpoint = process.env.REDIS_ENDPOINT
  if (redisEndpoint == null || redisEndpoint.length === 0) {
    throw new Error(`Env REDIS_ENDPOINT must be set to Redis database endpoint. Free plan on https://redislabs.com is suitable.`)
  }
  console.log(`Redis endpoint (last 3 symbols): ${redisEndpoint.substring(redisEndpoint.length - 3)}`)
  return redis(redisEndpoint.startsWith("redis://") ? redisEndpoint : `redis://${redisEndpoint}`)
}

export async function createServiceInfo(port: string) {
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

  return new ServiceInfo(data.ip, port, data.latitude, data.longitude)
}

export class ServiceInfo {
  readonly hostname: string = os.hostname()
  readonly cpuCount = os.cpus().length

  jobCount = 0

  constructor(readonly ip: string, readonly port: string, readonly latitude: number, readonly longitude: number) {
  }
}

export class ServiceEntry {
  constructor(readonly info: ServiceInfo, readonly key: string, private readonly registry: ServiceRegistry, public destroyed = false, public timeoutHandle: Timer | null = null) {
  }

  leave() {
    return this.registry.leave(this)
  }
}

/** @internal */
export class ServiceChannels {
  static readonly JOIN = "service-join"
  static readonly LEAVE = "service-leave"
}
