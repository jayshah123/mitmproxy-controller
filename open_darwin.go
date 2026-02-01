//go:build darwin

package main

import (
	"os/exec"
)

func openURL(url string) error {
	return exec.Command("open", url).Start()
}

func revealInFileManager(path string) error {
	return exec.Command("open", path).Start()
}
