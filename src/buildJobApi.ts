import { UploadTask } from "electron-builder-lib"
import * as path from "path"

export interface ArtifactInfo extends UploadTask {
  target: string | null

  readonly isWriteUpdateInfo?: boolean
  readonly updateInfo?: any
}

export interface BuildTaskResult {
  artifacts?: Array<ArtifactInfo>
  relativePathOffset?: number

  error?: Error

  unarchiveTime: number
  buildTime: number
}

export interface BuildTask {
  app: string
  platform: string
  targets: Array<string>

  // only for stats purpose, not required for build
  uploadTime?: number
  zstdCompression: number
  archiveSize?: number
}

export function getBuildDir(archiveFile: string): string {
  const explicitElectronBuilderTmpDir = process.env.ELECTRON_BUILDER_TMP_DIR
  // for now we use env ELECTRON_BUILDER_TMP_DIR (can be set to docker tmpfs) only for electron-builder, but not to store uploaded app archive (because build job is queued, but upload is not - can be quite a lot downloaded, but not processed uploaded files)
  return explicitElectronBuilderTmpDir == null ? archiveFile.substring(0, archiveFile.lastIndexOf(".")) : (explicitElectronBuilderTmpDir + path.sep + path.basename(archiveFile, ".zst"))
}