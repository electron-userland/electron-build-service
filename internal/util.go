package internal

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/develar/app-builder/pkg/util"
	"go.uber.org/zap"
)

func GetListenPort(envName string) string {
	port := os.Getenv(envName)
	// k8s sets envs like BUILDER_SERVICE_PORT=tcp://10.43.216.215:443, so, to avoid issues, do not use such values
	if port == "" || strings.HasPrefix(port, "tcp://") {
		return "443"
	} else {
		return port
	}
}

func createHttpServerOptions(port string) *http.Server {
	return &http.Server{
		Addr: ":" + port,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
}

type BeforeServerShutdown func()

func ListenAndServe(port string, logger *zap.Logger) *http.Server {
	http.HandleFunc("/health-check", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
	})

	server := createHttpServerOptions(port)

	go func() {
		var err error
		if os.Getenv("USE_SSL") == "false" {
			err = server.ListenAndServe()
		} else {
			err = server.ListenAndServeTLS("/etc/secrets/tls.cert", "/etc/secrets/tls.key")
		}
		if err == http.ErrServerClosed {
			logger.Debug("server closed")
		} else {
			logger.Fatal("cannot serve", zap.Error(err), zap.String("port", port))
		}
	}()

	return server
}

func WaitUntilTerminated(server *http.Server, shutdownTimeout time.Duration, beforeServerShutdown BeforeServerShutdown, logger *zap.Logger) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals

	if beforeServerShutdown != nil {
		beforeServerShutdown()
	}

	shutdownHttpServer(server, shutdownTimeout, logger)
}

func shutdownHttpServer(server *http.Server, shutdownTimeout time.Duration, logger *zap.Logger) {
	if server == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	logger.Info("shutdown server", zap.Duration("timeout", shutdownTimeout))
	start := time.Now()
	err := server.Shutdown(ctx)
	if err != nil {
		logger.Error("cannot shutdown server", zap.Error(err))
		return
	}

	logger.Info("server is shutdown", zap.Duration("duration", time.Since(start)))
}

func CreateLogger() *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.Encoding = util.GetEnvOrDefault("LOG_ENCODING", "console")
	config.DisableCaller = true
	logger, err := config.Build()
	if err != nil {
		log.Fatal(err)
	}
	return logger
}

func Close(c io.Closer, logger *zap.Logger) {
	err := c.Close()
	if err != nil && err != os.ErrClosed {
		logger.Error("cannot close", zap.Error(err))
	}
}
