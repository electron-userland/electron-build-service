import { exec } from "builder-util"
import { Job } from "bull"
import { Packager, PackagerOptions, PublishOptions } from "electron-builder-lib"
import { emptyDir, readJson, unlink } from "fs-extra-p"
import * as path from "path"
import { ArtifactInfo, BuildTask, BuildTaskResult, getArchivePath, getBuildDir } from "./buildJobApi"
import { removeFiles, Timer } from "./util"

export default async function processor(job: Job): Promise<BuildTaskResult> {
  const data: BuildTask = job.data
  const archiveName = data.archiveName
  if (archiveName == null) {
    throw new Error("Archive path not specified")
  }

  if (process.env.ELECTRON_BUILDER_TMP_DIR == null) {
    throw new Error("Env ELECTRON_BUILDER_TMP_DIR must be set for builder process")
  }

  // must be called before we set ELECTRON_BUILDER_TMP_DIR
  const projectDir = getBuildDir(archiveName)
  await emptyDir(projectDir)

  // env can be changed globally because worker is sandboxed (separate process)
  // we do cleanup in any case, no need to waste nodejs worker time
  process.env.TMP_DIR_MANAGER_ENSURE_REMOVED_ON_EXIT = "false"
  // where electron-builder creates temp files
  process.env.ELECTRON_BUILDER_TMP_DIR = projectDir + path.sep + "tmp"

  const targets = data.targets
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

  const archiveFile = getArchivePath(archiveName)
  const unarchiveTimer = new Timer(`Unarchive (${job.id})`)
  await exec(tarPath, ["-I", "zstd", "-xf", archiveFile, "-C", projectDir])
  const unarchiveTime = unarchiveTimer.end()
  const buildTimer = new Timer(`Build (${job.id})`)
  // remove archive, not needed anymore
  const info = (await Promise.all([unlink(archiveFile), readJson(projectDir + path.sep + "info.json")]))[1]

  const prepackaged = path.join(projectDir, "linux-unpacked")
  // do not use build function because we don't need to publish artifacts
  const options: PackagerOptions & PublishOptions = {
    prepackaged,
    projectDir,
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
    // cleanup early because prepackaged files can be on a RAM disk
    removeFiles([prepackaged, process.env.ELECTRON_BUILDER_TMP_DIR!!])

    return {
      artifacts,
      relativePathOffset: projectDir.length + "dist".length + 1,
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