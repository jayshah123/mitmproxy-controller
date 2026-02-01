//go:build windows

package main

import (
	"os/exec"
)

func openURL(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func revealInFileManager(path string) error {
	return exec.Command("explorer.exe", path).Start()
}
