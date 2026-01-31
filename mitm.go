package main

import (
	"fmt"
	"os"
	"os/exec"
)

const (
	proxyHost = "127.0.0.1"
	proxyPort = "8899"
)

var (
	mitmProcess *os.Process
	mitmCmd     *exec.Cmd
)

func startMitm() string {
	if mitmProcess != nil {
		if isProcessAlive(mitmProcess) {
			return "mitmproxy is already running"
		}
		mitmProcess = nil
	}

	cmd := exec.Command("mitmdump", "--listen-host", proxyHost, "--listen-port", proxyPort)
	configureMitmCmd(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("Failed to start mitmdump: %v", err)
	}

	mitmProcess = cmd.Process
	mitmCmd = cmd

	go func() {
		_ = cmd.Wait()
		mitmProcess = nil
		mitmCmd = nil
	}()

	return fmt.Sprintf("mitmdump started (PID: %d)", cmd.Process.Pid)
}

func stopMitm() string {
	if mitmProcess != nil {
		if err := mitmProcess.Kill(); err != nil {
			return fmt.Sprintf("Failed to kill mitmproxy: %v", err)
		}
		mitmProcess = nil
		mitmCmd = nil
		return "mitmproxy stopped"
	}

	// Try to find and kill any running mitmproxy using OS-specific method
	if killExistingMitmproxy() {
		return "mitmproxy stopped"
	}

	return "No mitmproxy process found"
}

func isMitmproxyRunning() bool {
	if mitmProcess != nil && isProcessAlive(mitmProcess) {
		return true
	}
	return checkExistingMitmproxy()
}
