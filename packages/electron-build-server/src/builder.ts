import { exec } from "builder-util"
import { Job } from "bull"
import { Packager, PackagerOptions, PublishOptions } from "electron-builder-lib"
import { emptyDir, ensureDir, readJson, unlink } from "fs-extra-p"
import * as path from "path"
import { ArtifactInfo, BuildTask, BuildTaskResult, getBuildDir } from "./buildJobApi"
import { removeFiles, Timer } from "./util"

process.env.ORIGINAL_ELECTRON_BUILDER_TMP_DIR = process.env.ELECTRON_BUILDER_TMP_DIR
process.env.ELECTRON_BUILDER_REMOVE_STAGE_EVEN_IF_DEBUG = "true"

export default async function processor(job: Job): Promise<BuildTaskResult> {
  const data: BuildTask = job.data
  const archiveFile = data.archiveFile
  if (archiveFile == null) {
    throw new Error("Archive path not specified")
  }

  if (process.env.ELECTRON_BUILDER_TMP_DIR == null) {
    throw new Error("Env ELECTRON_BUILDER_TMP_DIR must be set for builder process")
  }

  const projectDir = getBuildDir(job.id as string)
  await emptyDir(projectDir)

  // env can be changed globally because worker is sandboxed (separate process)
  // we do cleanup in any case, no need to waste nodejs worker time
  process.env.TMP_DIR_MANAGER_ENSURE_REMOVED_ON_EXIT = "false"
  // where electron-builder creates temp files
  const projectTempDir = projectDir + path.sep + "tmp"
  process.env.ELECTRON_BUILDER_TMP_DIR = projectTempDir
  // nodejs mkdtemp doesn't create intermediate dirs
  await ensureDir(projectTempDir)

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

  const unarchiveTimer = new Timer(`Unarchive (${job.id})`)
  await exec(tarPath, ["-I", "zstd", "-xf", archiveFile, "-C", projectDir])
  const unarchiveTime = unarchiveTimer.end()
  const buildTimer = new Timer(`Build (${job.id})`)
  const infoFile = projectDir + path.sep + "info.json"
  // remove archive, not needed anymore
  const info = (await Promise.all([unlink(archiveFile), readJson(infoFile)]))[1]

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
  const relativePathOffset = projectDir.length + "dist".length + 2
  packager.artifactCreated(event => {
    if (event.file != null) {
      artifacts.push({
        file: event.file.substring(relativePathOffset),
        target: event.target == null ? null : event.target.name,
        arch: event.arch,
        safeArtifactName: event.safeArtifactName,
        isWriteUpdateInfo: event.isWriteUpdateInfo === true,
        updateInfo: event.updateInfo,
      })
    }
  })

  function cleanup() {
    // cleanup early because prepackaged files can be on a RAM disk
    removeFiles([prepackaged, projectTempDir, infoFile])
  }

  try {
    await packager._build(info.configuration, info.metadata, info.devMetadata, info.repositoryInfo)
    cleanup()

    return {
      artifacts,
      unarchiveTime,
      buildTime: buildTimer.end(),
    }
  }
  catch (e) {
    cleanup()
    return {
      error: e.stack,
      unarchiveTime,
      buildTime: buildTimer.end(),
    }
  }
}

// https://github.com/OptimalBits/bull/issues/786
module.exports = processor