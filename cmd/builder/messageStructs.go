package main

type BuildRequest struct {
  Targets  []TargetInfo `json:"targets"`
  Platform string       `json:"platform"`
}

type TargetInfo struct {
  Name            string `json:"name"`
  Arch            string `json:"arch"`
  UnpackedDirName string `json:"unpackedDirName"`
}
