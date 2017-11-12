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