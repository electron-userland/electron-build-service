import { doSpawn } from "builder-util"

export class Timer {
  private start = process.hrtime()

  constructor(private readonly label: string) {
  }

  end(label?: string): number {
    const duration = process.hrtime(this.start)
    console.info(`${label || this.label}: %ds %dms`, duration[0], Math.round(duration[1] / 1000000))
    return Math.round((duration[0] * 1000) + (duration[1] / 1e6))
  }
}

// cleanup using a detached rm command
export function removeFiles(files: Array<string>) {
  const args = ["-rf"]
  args.push(...files)
  const rmProcess = doSpawn("rm", args, {
    detached: true,
    stdio: "ignore",
  })
  rmProcess.on("error", error => {
    console.error(`Cleanup error: ${error.stack || error}`)
  })
}