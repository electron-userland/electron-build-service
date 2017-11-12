import { computeEnv, getLinuxToolsPath } from "builder-util/out/bundledTool"
import * as path from "path"

// on macOS GNU tar is required
export async function prepareBuildTools() {
  if (process.platform === "darwin") {
    const linuxToolsPath = await getLinuxToolsPath()
    process.env.PATH = computeEnv(process.env.PATH, [path.join(linuxToolsPath, "bin")])
    process.env.DYLD_LIBRARY_PATH = computeEnv(process.env.DYLD_LIBRARY_PATH, [path.join(linuxToolsPath, "lib")])
    process.env.TAR_PATH = path.join(linuxToolsPath, "bin", "gtar")
  }
  else {
    process.env.TAR_PATH = "tar"
  }
}

if (process.mainModule === module) {
  prepareBuildTools()
    .catch(error => {
      console.error(error.stack || error.toString())
      process.exitCode = 1
    })
}