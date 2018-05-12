package main

import (
    "context"
  "fmt"
  "io/ioutil"
  "os"
  "os/exec"
  "path/filepath"
  "time"

  "github.com/develar/errors"
  "github.com/develar/go-fs-util"
  "github.com/electronuserland/electron-build-service/internal"
  "github.com/json-iterator/go"
  "github.com/mongodb/amboy/job"
  "go.uber.org/zap"
)

type BuildJob struct {
  projectDir   string
  queueAddTime time.Time

  buildRequest    *BuildRequest
  rawBuildRequest *string

  handler *BuildHandler

  // channel for status messages
  messages chan string
  // channel for complete (with any result - success or failure or timeout)
  // error - only internal error, not from electron-builder
  complete chan BuildJobResult

  job.Base
}

type BuildJobResult struct {
  error     error
  rawResult string
  fileSizes []int64
}

const outDirName = "out"

func (t *BuildJob) Run(ctx context.Context) {
  defer func() {
    t.Base.MarkComplete()
  }()

  jobStartTime := time.Now()
  waitTime := jobStartTime.Sub(t.queueAddTime)
  t.handler.logger.Info("job started", zap.Duration("waitTime", waitTime))
  t.messages <- fmt.Sprintf("job started (queue time: %s)", waitTime.Round(time.Millisecond))

  err := t.doBuild(ctx, jobStartTime)
  if err != nil {
    t.complete <- BuildJobResult{error: errors.WithStack(err)}
  }
}

func (t *BuildJob) doBuild(ctx context.Context, jobStartTime time.Time) error {
  defer func() {
    if r := recover(); r != nil {
      t.complete <- BuildJobResult{error: errors.Errorf("recovered", r)}
    }
  }()

  // where electron-builder creates temp files
  projectTempDir := filepath.Join(t.handler.tmpDir, t.ID())
  // t.handler.tmpDir should be already created,
  err := os.Mkdir(projectTempDir, 0700)
  if err != nil {
    return errors.WithStack(err)
  }

  projectOutDir := filepath.Join(t.projectDir, outDirName)
  err = fsutil.EnsureEmptyDir(projectOutDir)
  if err != nil {
    return errors.WithStack(err)
  }

  command := exec.CommandContext(ctx, "node", "/node_modules/electron-builder-lib/out/remoteBuilder/builder-cli.js", *t.rawBuildRequest)
  command.Env = append(os.Environ(),
    "PROJECT_DIR="+t.projectDir,
    "PROJECT_OUT_DIR="+projectOutDir,
    "ELECTRON_BUILDER_TMP_DIR="+projectTempDir,
    // we do cleanup in any case, no need to waste nodejs worker time
    "TMP_DIR_MANAGER_ENSURE_REMOVED_ON_EXIT=false",
    // don't want to deal with "The platform "darwin" is incompatible with this module." (because we do yarn install on macOS (developer machine, not inside docker))
    "USE_SYSTEM_APP_BUILDER=true",
    "USE_SYSTEM_7ZA=true",
  )
  command.Dir = t.projectDir

  output, err := command.CombinedOutput()
  if len(output) != 0 {
    t.messages <- string(output)
  }

  if err != nil {
    return errors.WithStack(err)
  }

  // reliable way to get result (since we cannot use out/err output)
  rawResult, err := ioutil.ReadFile(filepath.Join(projectTempDir, "__build-result.json"))
  if err != nil {
    return errors.WithStack(err)
  }

  result := BuildJobResult{
    rawResult: string(rawResult),
  }
  if len(rawResult) > 0 && rawResult[0] == '[' {
    var partialArtifactInfo []PartialArtifactInfo
    err = jsoniter.Unmarshal(rawResult, &partialArtifactInfo)
    if err != nil {
      return errors.WithStack(err)
    }

    result.fileSizes, err = t.computeFileSizes(partialArtifactInfo, projectOutDir)
    if err != nil {
      return errors.WithStack(err)
    }
  }

  t.handler.logger.Info("job completed",
    zap.Duration("duration", time.Since(jobStartTime)),
    zap.ByteString("result", rawResult),
    zap.Int64s("fileSizes", result.fileSizes),
  )

  t.complete <- result

  go t.removeAllFilesExceptArtifacts(projectTempDir)

  return nil
}

func (t *BuildJob) computeFileSizes(partialArtifactInfo []PartialArtifactInfo, projectOutDir string) ([]int64, error) {
  fileSizes := make([]int64, len(partialArtifactInfo))
  err := internal.MapAsync(len(partialArtifactInfo), t.handler.logger, func(taskIndex int) (func() error, error) {
    file := filepath.Join(projectOutDir, partialArtifactInfo[taskIndex].File)
    return func() error {
      info, err := os.Stat(file)
      if err != nil {
        return err
      }

      fileSizes[taskIndex] = info.Size()
      return nil
    }, nil
  })
  return fileSizes, err
}

type PartialArtifactInfo struct {
  File string `json:"file"`
}

func (t *BuildJob) removeAllFilesExceptArtifacts(projectTempDir string) {
  removeFileAndLog(t.handler.logger, projectTempDir)

  // on complete all project dir will be removed, but temp files are not required since now, so cleanup early because files can be on a RAM disk
  // yes, for now we expect the only target
  files, err := fsutil.ReadDirContent(t.projectDir)
  if err != nil {
    t.handler.logger.Error("cannot remove", zap.Error(errors.WithStack(err)))
  }

  for _, file := range files {
    if file == outDirName {
      continue
    }

    removeFileAndLog(t.handler.logger, file)
  }
}

func removeFileAndLog(logger *zap.Logger, file string) {
  logger.Debug("remove", zap.String("file", file))
  err := os.RemoveAll(file)
  if err != nil {
    logger.Error("cannot remove", zap.Error(errors.WithStack(err)))
  }
}