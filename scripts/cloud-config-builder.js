const {safeLoad, safeDump} = require("js-yaml")
const fs = require("fs")
const path = require("path")
const os = require("os")

const data = safeLoad(fs.readFileSync(path.join(__dirname, "cloud-config.yml"), "utf-8"))
const files = data.write_files

data.ssh_authorized_keys = [fs.readFileSync(path.join(os.homedir(), ".ssh", "develar_rsa.pub"), "utf-8")]

const mappings = {
  "/etc/ssh/sshd_config": path.join(__dirname, "sshd_config"),
  "/etc/rc.local": path.join(__dirname, "private", "rc.local"),
  "/opt/rancher/bin/start.sh": path.join(__dirname, "rancher-start.sh"),
  "/etc/app-certs/bundle.crt": "/Volumes/data/electron-build-ca/certs/bundle.crt",
  "/etc/app-certs/node.key": "/Volumes/data/electron-build-ca/certs/node.key",
  "/etc/nginx-conf/nginx.conf": path.join(__dirname, "../nginx-conf/nginx.conf"),
  "/etc/nginx-conf/builder-upstream.prod.conf": path.join(__dirname, "../nginx-conf/builder-upstream.prod.conf"),
  "/etc/docker-compose.stack.yml": path.join(__dirname, "private", "docker-compose.stack.yml"),
}

for (const file of files) {
  const localFile = mappings[file.path]
  if (localFile == null) {
    throw new Error(`Unknown path ${file.path}`)
  }
  file.content = fs.readFileSync(localFile, "utf-8")
}

fs.writeFileSync(path.join(__dirname, "private/cloud-config.yml"), safeDump(data, {lineWidth: 8000}))