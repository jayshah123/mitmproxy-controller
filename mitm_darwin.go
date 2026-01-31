//go:build darwin

package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func configureMitmCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func isProcessAlive(p *os.Process) bool {
	return p.Signal(syscall.Signal(0)) == nil
}

func killExistingMitmproxy() bool {
	out, err := exec.Command("pgrep", "-f", "mitmdump|mitmproxy").Output()
	if err != nil {
		return false
	}

	pids := strings.Fields(string(out))
	killed := false
	for _, pidStr := range pids {
		pid, _ := strconv.Atoi(pidStr)
		if p, err := os.FindProcess(pid); err == nil {
			p.Kill()
			killed = true
		}
	}
	return killed
}

func checkExistingMitmproxy() bool {
	out, _ := exec.Command("pgrep", "-f", "mitmdump|mitmproxy").Output()
	return len(strings.TrimSpace(string(out))) > 0
}
