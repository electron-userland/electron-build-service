const {safeLoad, safeDump} = require("js-yaml")
const fs = require("fs")
const path = require("path")

const resultFile = path.join(__dirname, "private/builder.stack.yml")
const data = safeLoad(fs.readFileSync(resultFile, "utf-8"))
const correctData = safeLoad(fs.readFileSync(path.join(__dirname, "../docker-compose.prod.yml"), "utf-8"))

data.services.builder.volumes = correctData.services.builder.volumes
fs.writeFileSync(resultFile, safeDump(data, {lineWidth: 8000}))