import { UploadTask } from "electron-builder-lib"
import * as path from "path"

export interface TargetInfo {
  name: string
  arch: string
  unpackedDirName: string
}

export interface ArtifactInfo extends UploadTask {
  target: string | null

  readonly isWriteUpdateInfo?: boolean
  readonly updateInfo?: any
}

export interface BuildTaskResult {
  artifacts?: Array<ArtifactInfo>

  error?: Error

  unarchiveTime: number
  buildTime: number
}

export interface BuildTask {
  archiveFile: string
  platform: string
  targets: Array<TargetInfo>

  // only for stats purpose, not required for build
  uploadTime?: number
  zstdCompression: number
  archiveSize?: number
}

// electron-builder project dir
export function getBuildDir(jobId: string): string {
  // for now we use env ELECTRON_BUILDER_TMP_DIR (can be set to docker tmpfs) only for electron-builder, but not to store uploaded app archive (because build job is queued, but upload is not - can be quite a lot downloaded, but not processed uploaded files)
  return process.env.ORIGINAL_ELECTRON_BUILDER_TMP_DIR + path.sep + jobId
}

export function getStageDir() {
  return (process.env.PROJECT_ARCHIVE_DIR_PARENT || "") + "/stage"
}