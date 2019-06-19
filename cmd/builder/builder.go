package main

import (
	"fmt"
	"github.com/coreos/etcd/embed"
	"github.com/develar/app-builder/pkg/util"
	"io/ioutil"
	"log"
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
	defer func() {
		err := logger.Sync()
		if err != nil {
			log.Printf("cannot sync logger: %s", err)
		}
	}()

	if util.IsEnvTrue("USE_EMBEDDED_ETCD") {
		err := os.Setenv("ETCD_ENDPOINT", embed.DefaultListenClientURLs)
		if err != nil {
			logger.Fatal("cannot set env ETCD_ENDPOINT", zap.Error(err))
		}

		serverEtcd, err := internal.StartEmbeddedServer(logger)
		if err != nil {
			logger.Fatal("cannot start embedded etcd server", zap.Error(err))
		}

		defer serverEtcd.Close()
	}

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

	zstdPath, err := download.DownloadZstd(util.GetCurrentOs())
	if err != nil {
		return errors.WithStack(err)
	}

	scriptPath := os.Getenv("BUILDER_NODE_MODULES")
	if scriptPath == "" {
		executableFile, err := os.Executable()
		if err != nil {
			return errors.WithStack(err)
		}
		scriptPath = filepath.Join(filepath.Dir(executableFile), "../..")
	}

	buildHandler := &BuildHandler{
		logger:     logger,
		stageDir:   internal.GetBuilderDirectory("stage"),
		tempDir:    builderTmpDir,
		zstdPath:   filepath.Join(zstdPath, "zstd"),
		scriptPath: filepath.Join(scriptPath, "node_modules/app-builder-lib/out/remoteBuilder/builder-cli.js"),
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
	server := internal.ListenAndServe(port, logger)

	disposer := NewDisposer()
	defer disposer.Dispose()

	err = buildHandler.RegisterAgent(port, disposer)
	if err != nil {
		return errors.WithStack(err)
	}

	err = configureRouter(logger, disposer)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("started",
		zap.String("port", port),
		zap.String("stage dir", buildHandler.stageDir),
		zap.String("temp dir", buildHandler.tempDir),
		zap.String("etcdKey", buildHandler.agentEntry.Key),
		zap.String("zstdPath", buildHandler.zstdPath),
		zap.String("scriptPath", buildHandler.scriptPath),
		zap.Strings("env", os.Environ()),
	)

	internal.WaitUntilTerminated(server, 4*time.Minute, func() {
		// remove agent entry before server shutdown (as early as possible)
		disposer.Dispose()
	}, logger)

	// wait until all tasks are completed (do not abort)
	buildHandler.WaitTasksAreComplete()
	return nil
}

func getBuilderTmpDir() (string, error) {
	builderTmpDir := os.Getenv("APP_BUILDER_TMP_DIR")

	if builderTmpDir == "" {
		builderTmpDir = internal.GetBuilderDirectory("tmp")
	} else {
		homeDir, err := homedir.Dir()
		if err != nil {
			return "", errors.WithStack(err)
		}

		if builderTmpDir == os.TempDir() || strings.HasPrefix(homeDir, builderTmpDir) || builderTmpDir == "/" {
			return "", fmt.Errorf("%s cannot be used as APP_BUILDER_TMP_DIR because this dir will be emptied", builderTmpDir)
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
		explicit = strings.TrimSpace(explicit)
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
