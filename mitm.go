package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

const (
	proxyHost    = "127.0.0.1"
	proxyPort    = "8899"
	webUIPort    = "8898"
	webPassword  = "mitmcontroller"
	maxLogFiles  = 10
)

var (
	mitmProcess    *os.Process
	mitmCmd        *exec.Cmd
	currentLogPath string
	logsDir        string
	usingMitmweb   bool
)

func init() {
	logsDir = getLogsDir()
}

func getLogsDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	return filepath.Join(configDir, "mitmproxy-controller", "logs")
}

func ensureLogsDir() error {
	return os.MkdirAll(logsDir, 0755)
}

func generateLogFilename() string {
	timestamp := time.Now().Format("20060102-150405")
	return filepath.Join(logsDir, fmt.Sprintf("flows-%s.mitm", timestamp))
}

func cleanupOldLogs() {
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return
	}

	var logFiles []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".mitm" {
			logFiles = append(logFiles, e)
		}
	}

	if len(logFiles) <= maxLogFiles {
		return
	}

	sort.Slice(logFiles, func(i, j int) bool {
		infoI, _ := logFiles[i].Info()
		infoJ, _ := logFiles[j].Info()
		return infoI.ModTime().After(infoJ.ModTime())
	})

	for _, f := range logFiles[maxLogFiles:] {
		os.Remove(filepath.Join(logsDir, f.Name()))
	}
}

func startMitm() string {
	if mitmProcess != nil {
		if isProcessAlive(mitmProcess) {
			return "mitmproxy is already running"
		}
		mitmProcess = nil
	}

	if err := ensureLogsDir(); err != nil {
		return fmt.Sprintf("Failed to create logs directory: %v", err)
	}

	cleanupOldLogs()

	currentLogPath = generateLogFilename()

	var cmd *exec.Cmd
	if _, err := exec.LookPath("mitmweb"); err == nil {
		cmd = exec.Command("mitmweb",
			"--listen-host", proxyHost,
			"--listen-port", proxyPort,
			"--web-host", proxyHost,
			"--web-port", webUIPort,
			"--no-web-open-browser",
			"--set", "web_password="+webPassword,
			"-w", currentLogPath,
		)
		usingMitmweb = true
	} else {
		cmd = exec.Command("mitmdump",
			"--listen-host", proxyHost,
			"--listen-port", proxyPort,
			"-w", currentLogPath,
		)
		usingMitmweb = false
	}

	configureMitmCmd(cmd)

	if err := cmd.Start(); err != nil {
		currentLogPath = ""
		return fmt.Sprintf("Failed to start mitmproxy: %v", err)
	}

	mitmProcess = cmd.Process
	mitmCmd = cmd

	go func() {
		_ = cmd.Wait()
		mitmProcess = nil
		mitmCmd = nil
	}()

	mode := "mitmdump"
	if usingMitmweb {
		mode = "mitmweb"
	}
	return fmt.Sprintf("%s started (PID: %d)", mode, cmd.Process.Pid)
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

func getWebUIURL() string {
	return fmt.Sprintf("http://%s:%s/?token=%s", proxyHost, webUIPort, webPassword)
}

func isWebUIAvailable() bool {
	return usingMitmweb && isMitmproxyRunning()
}

func getLogsDirectory() string {
	return logsDir
}

func getCurrentLogPath() string {
	return currentLogPath
}
