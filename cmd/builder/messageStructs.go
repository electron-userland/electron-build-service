package main

import "github.com/develar/app-builder/pkg/electron"

type BuildRequest struct {
	Targets  []TargetInfo `json:"targets"`
	Platform string       `json:"platform"`

	// if builder will unpack Electron
	ElectronDownload electron.ElectronDownloadOptions `json:"electronDownload"`

	// rename electron executable to
	ExecutableName string `json:"executableName"`
}

type TargetInfo struct {
	Name            string `json:"name"`
	Arch            string `json:"arch"`
	UnpackedDirName string `json:"unpackedDirName"`
}
