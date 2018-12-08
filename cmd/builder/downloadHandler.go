package main

import (
	"net/http"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

const baseDownloadPath = "/v2/download/"

func (t *BuildHandler) HandleDownloadRequest(w http.ResponseWriter, r *http.Request) {
	jobId := r.URL.Path[len(baseDownloadPath):]
	jobId = jobId[0:strings.Index(jobId, "/")]

	t.logger.Debug("download", zap.String("file", r.URL.Path), zap.String("range", r.Header.Get("Range")))
	file := filepath.Join(t.stageDir, jobId, outDirName, r.URL.Path[(len(baseDownloadPath)+len(jobId)+1 /* slash */):])
	http.ServeFile(w, r, file)
}
