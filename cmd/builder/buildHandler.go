package main

import (
  "bytes"
  "context"
  "io"
  "net/http"
  "os"
  "os/exec"
  "path/filepath"
  "time"

  "github.com/apex/log"
  "github.com/develar/errors"
  "github.com/develar/go-fs-util"
    "github.com/electronuserland/electron-build-service/internal/agentRegistry"
  "github.com/electronuserland/electron-build-service/internal/gopool"
  "github.com/json-iterator/go"
  "github.com/segmentio/ksuid"
  "github.com/tomasen/realip"
  "go.uber.org/zap"
)

const queueCompleteTimeOut = 5 * time.Minute
const maxRequestBody = 768*1024*1024
const jobMaxTime = 30 * time.Minute

type BuildHandler struct {
  agentEntry  *agentRegistry.AgentEntry
  logger *zap.Logger

  queue *gopool.ManagedSource
  queueContextCancel context.CancelFunc
  pool gopool.GoPool

  stageDir string
  tmpDir   string

  zstdPath string
}

func (t *BuildHandler) CreateAndStartQueue(numWorkers int) error {
  ctx, cancel := context.WithCancel(context.Background())
  t.queueContextCancel = cancel
  logger := t.logger.Named("queue")
  t.queue = gopool.NewManagedSource(gopool.NewPriorityQueue(), ctx, logger)
  jobPool := gopool.New(numWorkers, ctx, t.queue.Source, logger)
  jobPool.JobMaxTime = jobMaxTime
  return nil
}

func (t *BuildHandler) WaitTasksAreComplete() {
  t.logger.Info("wait until all tasks are completed", zap.Duration("timeout", queueCompleteTimeOut))

  // defer context cancelling
  defer t.queueContextCancel()

  start := time.Now()
  // close queue to not accept new jobs
  t.queue.Close()
  // close pool to to ask workers to exit as soon as possible
  t.pool.Close()

  stopTimer := time.NewTimer(queueCompleteTimeOut)
  defer stopTimer.Stop()

  select {
  case <-t.pool.Done():
    t.logger.Info("tasks completed", zap.Duration("duration", time.Since(start)))
  case <-stopTimer.C:
    log.Warn("cannot wait all uncompleted tasks, abort all")
  }
}

func (t *BuildHandler) PrepareDirs() error {
  err := fsutil.EnsureEmptyDir(t.stageDir)
  if err != nil {
    return errors.WithStack(err)
  }

  err = fsutil.EnsureEmptyDir(t.tmpDir)
  if err != nil {
    return errors.WithStack(err)
  }

  return nil
}

func (t *BuildHandler) RegisterAgent(port string) (error) {
  agentKey, err := getAgentKey(port, t.logger)
  if err != nil {
    return errors.WithStack(err)
  }

  agentEntry, err := agentRegistry.NewAgentEntry("/builders/"+agentKey, t.logger)
  if err != nil {
    return errors.WithStack(err)
  }

  t.agentEntry = agentEntry
  return nil
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
    id:              jobId,
    buildRequest:    &buildRequest,
    rawBuildRequest: &rawRequest,

    projectDir: filepath.Join(t.stageDir, jobId),
    handler:    t,

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

func (t *BuildHandler) executeUnpackTarZstd(w http.ResponseWriter, r *http.Request, buildJob *BuildJob, projectDir string, ctx context.Context) (error) {
  start := time.Now()
  err := t.unpackTarZstd(http.MaxBytesReader(w, r.Body, maxRequestBody), projectDir, ctx, buildJob.logger)
  if err != nil {
    // do not wrap error, stack is clear
    return err
  }

  elapsed := time.Since(start)
  buildJob.logger.Info("uploaded and unpacked",
    zap.Duration("elapsed", elapsed),
    zap.String("compressionLevel", r.Header.Get("x-zstd-compression-level")),
  )

  return nil
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
    go t.updateAgentInfo(0)
    go removeFileAndLog(logger, projectDir)
  }()

  closeNotifier, ok := w.(http.CloseNotifier)
  if !ok {
    return errors.New("cannot cast to CloseNotifier")
  }
  closeChannel := closeNotifier.CloseNotify()

  unpackContext, cancelUnpack := context.WithCancel(context.Background())
  err = t.executeUnpackTarZstd(w, r, buildJob, projectDir, unpackContext)
  cancelUnpack()
  cancelUnpack = nil
  if err != nil {
    select {
    case <-closeChannel:
      logger.Debug("ignore unpack error because client closed connection")
      return nil
    default:
      return err
    }
  }

  buildJob.queueAddTime = time.Now()
  jobEntry := gopool.NewJob(buildJob, 0)
  t.queue.Add <- jobEntry

  flusher, ok := w.(http.Flusher)
  if !ok {
    return errors.New("cannot cast to Flusher")
  }

  jobId := buildJob.id

  jsonWriter := jsoniter.NewStream(jsoniter.ConfigFastest, w, 16*1024)

  flushJsonWriter := func() error {
    jsonWriter.WriteRaw("\n")
    err := jsonWriter.Flush()
    if err != nil {
      logger.Error("abort job on message write error")
      jobEntry.Cancel()
      return err
    }

    flusher.Flush()
    return nil
  }

  ticker := time.NewTicker(20 * time.Second)
  defer ticker.Stop()

  isCompleted := false
  for {
    select {
    case <-closeChannel:
      logger.Debug("client closed connection")
      if !isCompleted {
        isCompleted = true
        if cancelUnpack != nil {
          cancelUnpack()
        }

        jobEntry.Cancel()
      }
      return nil

    case <-ticker.C:
      // as ping messages to ensure that connection will be not closed
      jsonWriter.Reset(w)
      var message string
      if isCompleted {
        message = "download in progress..."
      } else {
        message = "build in progress..."
      }
      writeStatus(message, jsonWriter)
      err = flushJsonWriter()
      if err != nil {
        logger.Error("cannot write ping message", zap.Error(err))
      }

    case message := <-buildJob.messages:
      jsonWriter.Reset(w)
      writeStatus(message, jsonWriter)
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
      writeResultInfo(&result, jobId, jsonWriter)
      err = flushJsonWriter()
      if err != nil {
        return err
      }
      // do not return - wait until client download artifacts and close connection
    }
  }
}

func (t *BuildHandler) updateAgentInfo(relativeValue uint32) {
  pending := t.queue.PendingJobCount.Load()
  running := t.pool.RunningJobCount.Load()
  t.logger.Debug("queue stat", zap.Uint32("pending", pending), zap.Uint32("running", running))
  count := pending + running + relativeValue
  t.agentEntry.Update(int(count) /* our job */)
}

func writeStatus(message string, jsonWriter *jsoniter.Stream) {
  jsonWriter.WriteObjectStart()
  jsonWriter.WriteObjectField("status")
  jsonWriter.WriteString(message)
  jsonWriter.WriteObjectEnd()
}

func writeResultInfo(result *BuildJobResult, jobId string, jsonWriter *jsoniter.Stream) {
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

func (t *BuildHandler) unpackTarZstd(reader io.ReadCloser, unpackDir string, ctx context.Context, logger *zap.Logger) error {
  defer func() {
    err := reader.Close()
    // do not log ErrUnexpectedEOF - it means that client closed connection during upload
    if err != nil && err != os.ErrClosed && err != io.ErrUnexpectedEOF {
      logger.Error("cannot close", zap.Error(err))
    }
  }()

  command := exec.CommandContext(ctx, "tar", "--use-compress-program=" + t.zstdPath, "-x", "-C", unpackDir)
  command.Stdin = reader

  var errorOutput bytes.Buffer
  command.Stderr = &errorOutput

  err := command.Run()
  if err != nil && ctx.Err() == nil {
    return errors.Wrapf(err, "errorOutput: " + errorOutput.String())
  }

  return nil
}
