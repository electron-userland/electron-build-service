import { readFile } from "fs-extra-p"
import * as path from "path"

export async function getSslOptions() {
  const certDir = path.join(__dirname, "..", "certs")
  const data = await Promise.all([readFile(path.join(certDir, "ca.crt")), readFile(path.join(certDir, "node.key")), readFile(path.join(certDir, "node.crt"))])
  // default key and cert for localhost
  return {
    ca: data[0],
    key: data[1],
    cert: data[2],
  }
}