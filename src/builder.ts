import { doSpawn, exec } from "builder-util"
import { Job } from "bull"
import { Packager, PackagerOptions, PublishOptions } from "electron-builder-lib"
import { emptyDir, readJson, unlink } from "fs-extra-p"
import * as path from "path"
import { ArtifactInfo, BuildTask, BuildTaskResult, getBuildDir } from "./buildJobApi"
import { Timer } from "./util"

// we do cleanup in any case, no need to waste nodejs worker time
//process.env.TMP_DIR_MANAGER_ENSURE_REMOVED_ON_EXIT = "false"

export default async function processor(job: Job): Promise<BuildTaskResult> {
  const data: BuildTask = job.data
  const archivePath = data.app
  const targets = data.targets
  if (archivePath == null) {
    throw new Error("Archive path not specified")
  }
  if (data.platform == null) {
    throw new Error("platform not specified")
  }
  if (targets == null) {
    throw new Error("targets path not specified")
  }
  if (!Array.isArray(targets)) {
    throw new Error("targets must be array of target name")
  }

  const tarPath = process.env.TAR_PATH
  if (tarPath == null) {
    throw new Error("Archive path not specified")
  }

  const targetDirectory = getBuildDir(archivePath)
  await emptyDir(targetDirectory)
  const unarchiveTimer = new Timer(`Unarchive (${job.id})`)
  await exec(tarPath, ["-I", "zstd", "-xf", archivePath, "-C", targetDirectory])
  const unarchiveTime = unarchiveTimer.end()
  const buildTimer = new Timer(`Build (${job.id})`)
  // remove archive, not needed anymore
  const info = (await Promise.all([unlink(archivePath), readJson(targetDirectory + path.sep + "info.json")]))[1]

  const prepackaged = path.join(targetDirectory, "linux-unpacked")
  // do not use build function because we don't need to publish artifacts
  const options: PackagerOptions & PublishOptions = {
    prepackaged,
    projectDir: targetDirectory,
    [data.platform]: targets,
    publish: "never",
    config: {
      publish: null,
    },
  }
  const packager = new Packager(options)

  const artifacts: Array<ArtifactInfo> = []
  packager.artifactCreated(event => {
    if (event.file != null) {
      artifacts.push({
        file: event.file,
        target: event.target == null ? null : event.target.name,
        arch: event.arch,
        safeArtifactName: event.safeArtifactName,
        isWriteUpdateInfo: event.isWriteUpdateInfo === true,
        updateInfo: event.updateInfo,
      })
    }
  })

  try {
    await packager._build(info.configuration, info.metadata, info.devMetadata, info.repositoryInfo)
    // cleanup using a detached rm command
    const rmProcess = doSpawn("rm", ["-rf", prepackaged], {
      detached: true,
      stdio: "ignore",
    })
    rmProcess.on("error", error => {
      console.error(`Cleanup error: ${error.stack || error}`)
    })

    return {
      artifacts,
      relativePathOffset: targetDirectory.length + "dist".length + 1,
      unarchiveTime,
      buildTime: buildTimer.end(),
    }
  }
  catch (e) {
    return {
      error: e.stack,
      unarchiveTime,
      buildTime: buildTimer.end(),
    }
  }
}

// https://github.com/OptimalBits/bull/issues/786
module.exports = processor