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
	// Use tasklist to find and taskkill to kill mitmproxy processes
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq mitmdump.exe", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}

	if strings.Contains(string(out), "mitmdump.exe") {
		exec.Command("taskkill", "/F", "/IM", "mitmdump.exe").Run()
		return true
	}

	// Also check for mitmproxy.exe
	out, err = exec.Command("tasklist", "/FI", "IMAGENAME eq mitmproxy.exe", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}

	if strings.Contains(string(out), "mitmproxy.exe") {
		exec.Command("taskkill", "/F", "/IM", "mitmproxy.exe").Run()
		return true
	}

	return false
}

func checkExistingMitmproxy() bool {
	out, _ := exec.Command("tasklist", "/FI", "IMAGENAME eq mitmdump.exe", "/FO", "CSV", "/NH").Output()
	if strings.Contains(string(out), "mitmdump.exe") {
		return true
	}

	out, _ = exec.Command("tasklist", "/FI", "IMAGENAME eq mitmproxy.exe", "/FO", "CSV", "/NH").Output()
	return strings.Contains(string(out), "mitmproxy.exe")
}
