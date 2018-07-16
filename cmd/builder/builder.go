package main

import (
    "fmt"
  "io/ioutil"
  "net/http"
  "os"
  "path/filepath"
  "runtime"
  "strings"
  "time"

    "github.com/develar/app-builder/pkg/download"
  "github.com/develar/errors"
    "github.com/didip/tollbooth"
  "github.com/didip/tollbooth/limiter"
  "github.com/electronuserland/electron-build-service/internal"
    "github.com/mitchellh/go-homedir"
    "go.uber.org/zap"
)

func main() {
  logger := internal.CreateLogger()
  defer logger.Sync()

  err := start(logger)
  if err != nil {
    logger.Fatal("cannot start", zap.Error(err))
  }
}

func start(logger *zap.Logger) error {
  builderTmpDir, err := getBuilderTmpDir()
  if err != nil {
    return errors.WithStack(err)
  }

  zstdPath, err := download.DownloadZstd(download.GetCurrentOs())
  if err != nil {
    return errors.WithStack(err)
  }

  buildHandler := &BuildHandler{
    logger:   logger,
    stageDir: string(os.PathSeparator) + "stage",
    tmpDir:   builderTmpDir,
    zstdPath: filepath.Join(zstdPath, "zstd"),
  }

  err = buildHandler.PrepareDirs()
  if err != nil {
    return errors.WithStack(err)
  }

  buildLimit := tollbooth.NewLimiter(1, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
  buildLimit.SetBurst(10)

  // client uses app-builder downloader that does parallel requests, so, limit is soft
  downloadLimit := tollbooth.NewLimiter(10, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
  downloadLimit.SetBurst(100)

  http.Handle("/v2/build", tollbooth.LimitFuncHandler(buildLimit, buildHandler.HandleBuildRequest))
  http.Handle(baseDownloadPath, tollbooth.LimitFuncHandler(downloadLimit, buildHandler.HandleDownloadRequest))

  err = buildHandler.CreateAndStartQueue(runtime.NumCPU() + 1)
  if err != nil {
    return errors.WithStack(err)
  }

  port := internal.GetListenPort("BUILDER_PORT")
  err = buildHandler.RegisterAgent(port)
  if err != nil {
    return errors.WithStack(err)
  }

  logger.Info("started",
    zap.String("port", port),
    zap.String("stage dir", buildHandler.stageDir),
    zap.String("temp dir", buildHandler.tmpDir),
    zap.String("etcdKey", buildHandler.agentEntry.Key),
    zap.String("zstdPath", buildHandler.zstdPath),
  )
  internal.ListenAndServeTLS(port, 4*time.Minute, func() {
    // remove agent entry before server shutdown (as early as possible)
    internal.Close(buildHandler.agentEntry, logger)
  }, logger)

  // wait until all tasks are completed (do not abort)
  buildHandler.WaitTasksAreComplete()
  return nil
}

func getBuilderTmpDir() (string, error) {
  builderTmpDir := os.Getenv("ELECTRON_BUILDER_TMP_DIR")
  if builderTmpDir == "" {
    builderTmpDir = string(os.PathSeparator) + "builder-tmp"
  } else {
    homeDir, err := homedir.Dir()
    if err != nil {
      return "", errors.WithStack(err)
    }

    if builderTmpDir == os.TempDir() || strings.HasPrefix(homeDir, builderTmpDir) || builderTmpDir == "/" {
      return "", fmt.Errorf("%s cannot be used as ELECTRON_BUILDER_TMP_DIR because this dir will be emptied", builderTmpDir)
    }
  }

  return builderTmpDir, nil
}

func getAgentKey(port string, logger *zap.Logger) (string, error) {
  ip, err := getExternalPublicIp(logger)
  if err != nil {
    return "", errors.WithStack(err)
  }
  return ip + ":" + port, nil
}

func getExternalPublicIp(logger *zap.Logger) (string, error) {
  explicit := os.Getenv("BUILDER_HOST")
  if explicit != "" {
    logger.Debug("host specified explicitly via env", zap.String("host", explicit))
    return explicit, nil
  }

  ipType := ""
  preferredIpVersion := os.Getenv("PREFERRED_IP_VERSION")
  if len(preferredIpVersion) == 1 {
    ipType = "ipv" + preferredIpVersion + "."
  }

  //noinspection SpellCheckingInspection
  url := "https://" + ipType + "myexternalip.com/raw"
  logger.Debug("get external ip", zap.String("url", url))
  response, err := http.Get(url)
  if err != nil {
    return "", errors.WithStack(err)
  }

  responseBytes, err := ioutil.ReadAll(response.Body)
  if err != nil {
    return "", errors.WithStack(err)
  }

  responseText := string(responseBytes)
  if response.StatusCode != http.StatusOK {
    return "", fmt.Errorf("cannot get external public ip - status: %d, response: %s", response.StatusCode, responseText)
  }
  return strings.TrimSpace(responseText), nil
}
