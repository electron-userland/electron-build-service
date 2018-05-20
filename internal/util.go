package internal

import (
  "crypto/tls"
  "io"
  "log"
  "net/http"
  "os"

  "go.uber.org/zap"
)

func GetListenPort(envName string) string {
  port := os.Getenv(envName)
  if port == "" {
    return "443"
  }
  return port
}

func CreateHttpServerOptions(port string) *http.Server {
  return &http.Server{
    Addr: ":" + port,
    TLSConfig: &tls.Config{
      MinVersion: tls.VersionTLS12,
    },
  }
}

func CreateLogger() *zap.Logger {
  config := zap.NewDevelopmentConfig()
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
