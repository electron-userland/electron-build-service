package main

import (
  "bytes"
  "io"
  "net/http"
  "os"
  "os/exec"
  "path/filepath"
  "time"

    "github.com/develar/errors"
  "github.com/electronuserland/electron-build-service/internal"
  "github.com/electronuserland/electron-build-service/internal/agentRegistry"
  "github.com/json-iterator/go"
  "github.com/mongodb/amboy"
  "github.com/mongodb/amboy/job"
  "github.com/segmentio/ksuid"
  "github.com/tomasen/realip"
  "go.uber.org/zap"
)

type BuildHandler struct {
  agentEntry  *agentRegistry.AgentEntry
  logger *zap.Logger

  queue amboy.Queue

  stageDir string
  tmpDir   string

  zstdPath string
}

func (t *BuildHandler) HandleBuildRequest(w http.ResponseWriter, r *http.Request) {
  logger := t.logger
  if r.Method != "POST" {
    errorMessage := "only POST supported"
    logger.Warn(errorMessage, zap.String("ip", realip.FromRequest(r)))
    http.Error(w, errorMessage, http.StatusMethodNotAllowed)
    return
  }

  rawRequest := r.Header.Get("x-build-request")
  if rawRequest == "" {
    errorMessage := "header x-build-request is not specified"
    logger.Warn(errorMessage, zap.String("ip", realip.FromRequest(r)))
    http.Error(w, errorMessage, http.StatusBadRequest)
    return
  }

  var buildRequest BuildRequest
  err := jsoniter.UnmarshalFromString(rawRequest, &buildRequest)
  if err != nil {
    errorMessage := "cannot parse build request"
    logger.Warn(errorMessage, zap.Error(err), zap.String("ip", realip.FromRequest(r)))
    http.Error(w, errorMessage+": "+err.Error(), http.StatusBadRequest)
    return
  }

  jobId := ksuid.New().String()
  buildJob := &BuildJob{
    Base: job.Base{
      TaskID:       jobId,
      TaskTimeInfo: amboy.JobTimeInfo{MaxTime: 30 * time.Minute},
    },

    buildRequest: &buildRequest,
    rawBuildRequest: &rawRequest,

    projectDir:   filepath.Join(t.stageDir, jobId),
    handler:      t,

    messages: make(chan string),
    complete: make(chan BuildJobResult),

    logger: logger.With(zap.String("jobId", jobId)),
  }

  err = t.doBuild(w, r, buildJob)
  if err != nil {
    logger.Error("error", zap.Error(err))
    http.Error(w, "internal server error", http.StatusInternalServerError)
    return
  }
}

func (t *BuildHandler) cleanUpAfterJobComplete(projectDir string, logger *zap.Logger) {
  t.updateAgentInfo(0)
  removeFileAndLog(logger, projectDir)
}

func (t *BuildHandler) doBuild(w http.ResponseWriter, r *http.Request, buildJob *BuildJob) error {
  projectDir := buildJob.projectDir
  err := os.Mkdir(projectDir, 0700)
  if err != nil {
    return errors.WithStack(err)
  }

  logger := buildJob.logger

  t.updateAgentInfo(1)
  defer func() {
    go t.cleanUpAfterJobComplete(projectDir, logger)
  }()

  start := time.Now()
  err = t.unpackTarZstd(http.MaxBytesReader(w, r.Body, 768*1024*1024), projectDir, logger)
  if err != nil {
    return err
  }

  elapsed := time.Since(start)
  jobId := buildJob.Base.TaskID
  logger.Info("uploaded and unpacked",
    zap.Duration("elapsed", elapsed),
    zap.String("compression level", r.Header.Get("x-zstd-compression-level")),
  )

  buildJob.queueAddTime = time.Now()
  err = t.queue.Put(buildJob)
  if err != nil {
    return errors.WithStack(err)
  }

  closeNotifier, ok := w.(http.CloseNotifier)
  if !ok {
    return errors.New("cannot cast to CloseNotifier")
  }
  closeChannel := closeNotifier.CloseNotify()

  flusher, ok := w.(http.Flusher)
  if !ok {
    return errors.New("cannot cast to Flusher")
  }

  runner, ok := t.queue.Runner().(amboy.AbortableRunner)
  if !ok {
    return errors.New("cannot cast to AbortableRunner")
  }

  jsonWriter := jsoniter.NewStream(jsoniter.ConfigDefault, w, 16*1024)

  flushJsonWriter := func() error {
    err := jsonWriter.Flush()
    if err != nil {
      logger.Error("abort job on message write error")
      abortErr := runner.Abort(jobId)
      if abortErr != nil {
        logger.Error("cannot abort job", zap.Error(abortErr))
      }
      return err
    }

    flusher.Flush()
    return nil
  }

  isCompleted := false
  for {
    select {
    case <-closeChannel:
      logger.Info("client closed connection")
      if !isCompleted {
        err = runner.Abort(jobId)
        if err != nil {
          return errors.WithStack(err)
        }
      }
      return nil

    case message := <-buildJob.messages:
      jsonWriter.Reset(w)
      jsonWriter.WriteObjectStart()
      jsonWriter.WriteObjectField("status")
      jsonWriter.WriteString(message)
      jsonWriter.WriteObjectEnd()
      err = flushJsonWriter()
      if err != nil {
        return err
      }

    case result := <-buildJob.complete:
      isCompleted = true
      logger.Debug("complete received", zap.Error(result.error))
      if result.error != nil {
        return err
      }

      jsonWriter.Reset(w)
      t.writeResultInfo(&result, jobId, jsonWriter)
      err = flushJsonWriter()
      if err != nil {
        return err
      }
      // do not return - wait until client download artifacts and close connection
    }
  }
}

func (t *BuildHandler) updateAgentInfo(relativeValue int) {
  queueStats := t.queue.Stats()
  t.agentEntry.Update(queueStats.Pending + queueStats.Running + relativeValue /* our job */)
}

func (t *BuildHandler) writeResultInfo(result *BuildJobResult, jobId string, jsonWriter *jsoniter.Stream) {
  jsonWriter.WriteObjectStart()

  jsonWriter.WriteObjectField("baseUrl")
  jsonWriter.WriteString(baseDownloadPath + jobId)

  jsonWriter.WriteMore()
  if len(result.rawResult) > 0 && result.rawResult[0] == '[' {
    jsonWriter.WriteObjectField("files")
    jsonWriter.WriteRaw(result.rawResult)
  } else {
    jsonWriter.WriteObjectField("error")
    if len(result.rawResult) == 0 || result.rawResult[0] != '{' {
      jsonWriter.WriteString(result.rawResult)
    } else {
      jsonWriter.WriteRaw(result.rawResult)
    }
  }

  if result.fileSizes != nil && len(result.fileSizes) > 0 {
    jsonWriter.WriteMore()
    jsonWriter.WriteObjectField("fileSizes")
    jsonWriter.WriteArrayStart()

    isFirst := true
    for _, value := range result.fileSizes {
      if !isFirst {
        jsonWriter.WriteMore()
        isFirst = false
      }

      jsonWriter.WriteInt64(value)
    }
    jsonWriter.WriteArrayEnd()
  }

  jsonWriter.WriteObjectEnd()
}

func (t *BuildHandler) unpackTarZstd(reader io.ReadCloser, unpackDir string, logger *zap.Logger) error {
  defer internal.Close(reader, logger)
  command := exec.Command("tar", "--use-compress-program=" + t.zstdPath, "-x", "-C", unpackDir)
  command.Stdin = reader

  var errorOutput bytes.Buffer
  command.Stderr = &errorOutput

  err := command.Run()
  if err != nil {
    logger.Error("tar error", zap.ByteString("errorOutput", errorOutput.Bytes()))
    return errors.WithStack(err)
  }

  return nil
}
