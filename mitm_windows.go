//go:build windows

package main

import (
	"os"
	"os/exec"
	"strings"
)

func configureMitmCmd(cmd *exec.Cmd) {
	// No special configuration needed for Windows
}

func isProcessAlive(p *os.Process) bool {
	// On Windows, FindProcess always succeeds, so we try to signal
	// But Signal(0) doesn't work the same way on Windows
	// Instead, we rely on our Wait() goroutine to clear mitmProcess
	return p != nil
}

func killExistingMitmproxy() bool {
	killed := false
	processes := []string{"mitmdump.exe", "mitmproxy.exe", "mitmweb.exe"}

	for _, proc := range processes {
		out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq "+proc, "/FO", "CSV", "/NH").Output()
		if err != nil {
			continue
		}
		if strings.Contains(string(out), proc) {
			exec.Command("taskkill", "/F", "/IM", proc).Run()
			killed = true
		}
	}

	return killed
}

func checkExistingMitmproxy() bool {
	processes := []string{"mitmdump.exe", "mitmproxy.exe", "mitmweb.exe"}

	for _, proc := range processes {
		out, _ := exec.Command("tasklist", "/FI", "IMAGENAME eq "+proc, "/FO", "CSV", "/NH").Output()
		if strings.Contains(string(out), proc) {
			return true
		}
	}

	return false
}
