package main

import (
  "net/http"
  "path/filepath"
  "strings"
)

const baseDownloadPath = "/v2/download/"

func (t BuildHandler) HandleDownloadRequest(w http.ResponseWriter, r *http.Request) {
  jobId := r.URL.Path[len(baseDownloadPath):]
  jobId = jobId[0:strings.Index(jobId, "/")]

  file := filepath.Join(t.stageDir, jobId, outDirName, r.URL.Path[(len(baseDownloadPath) + len(jobId) + 1 /* slash */):])
  http.ServeFile(w, r, file)
}
