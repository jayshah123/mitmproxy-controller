package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	proxyHost   = "127.0.0.1"
	proxyPort   = "8899"
	webUIPort   = "8898"
	webPassword = "mitmcontroller"
	maxLogFiles = 10
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
	if err := loadProfilesFromDisk(); err != nil {
		return fmt.Sprintf("Failed to load profiles: %v", err)
	}
	activeProfile, ok := getSelectedProfile()
	if !ok {
		return "No active profile found"
	}

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
		args, buildErr := buildMitmArgs(true, currentLogPath, activeProfile)
		if buildErr != nil {
			return fmt.Sprintf("Failed to build mitmweb command: %v", buildErr)
		}
		cmd = exec.Command("mitmweb", args...)
		usingMitmweb = true
	} else {
		args, buildErr := buildMitmArgs(false, currentLogPath, activeProfile)
		if buildErr != nil {
			return fmt.Sprintf("Failed to build mitmdump command: %v", buildErr)
		}
		cmd = exec.Command("mitmdump", args...)
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
	return fmt.Sprintf("%s started (PID: %d) | profile: %s", mode, cmd.Process.Pid, activeProfile.Name)
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

func getMitmHomeDirectory() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.TempDir()
	}
	return filepath.Join(home, ".mitmproxy")
}

func ensureMitmHomeDirectoryExists() (string, error) {
	mitmHomeDir := getMitmHomeDirectory()
	absPath, err := filepath.Abs(mitmHomeDir)
	if err == nil {
		mitmHomeDir = absPath
	}

	if err := os.MkdirAll(mitmHomeDir, 0755); err != nil {
		return "", err
	}
	return mitmHomeDir, nil
}

func getMitmConfigPath() string {
	return filepath.Join(getMitmHomeDirectory(), "config.yaml")
}

func ensureMitmConfigExists() (string, error) {
	mitmHomeDir, err := ensureMitmHomeDirectoryExists()
	if err != nil {
		return "", err
	}
	configPath := filepath.Join(mitmHomeDir, "config.yaml")

	f, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err == nil {
		if closeErr := f.Close(); closeErr != nil {
			return "", closeErr
		}
		return configPath, nil
	}
	if os.IsExist(err) {
		return configPath, nil
	}

	return "", err
}

func buildMitmArgs(useWebUI bool, logPath string, profile ServiceProfile) ([]string, error) {
	args := []string{
		"--set", "confdir=" + getMitmHomeDirectory(),
		"--set", "listen_host=" + proxyHost,
		"--set", "listen_port=" + proxyPort,
	}

	if useWebUI {
		args = append(args,
			"--set", "web_host="+proxyHost,
			"--set", "web_port="+webUIPort,
			"--set", "web_password="+webPassword,
			"--no-web-open-browser",
		)
	}

	if profile.Mode != "" {
		args = append(args, "--mode", profile.Mode)
	}

	for _, scriptPath := range profile.ScriptPaths {
		if _, err := os.Stat(scriptPath); err != nil {
			return nil, fmt.Errorf("profile %q has missing script %q", profile.Name, scriptPath)
		}
		args = append(args, "-s", scriptPath)
	}

	keys := make([]string, 0, len(profile.SetOptions))
	for key := range profile.SetOptions {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if strings.EqualFold(strings.TrimSpace(key), "confdir") {
			continue
		}
		value := profile.SetOptions[key]
		args = append(args, "--set", fmt.Sprintf("%s=%s", key, value))
	}

	args = append(args, "-w", logPath)
	return args, nil
}
