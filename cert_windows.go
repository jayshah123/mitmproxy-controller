//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getMitmproxyCertPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mitmproxy", "mitmproxy-ca-cert.cer")
}

func isCertInstalled() bool {
	out, err := exec.Command("certutil", "-store", "-user", "Root").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "mitmproxy")
}

func isCertTrusted() bool {
	// On Windows, if cert is in Root store, it's trusted
	return isCertInstalled()
}

func installCACertificate() string {
	certPath := getMitmproxyCertPath()

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return "CA cert not found. Start mitmproxy first to generate it."
	}

	if isCertInstalled() {
		return "CA certificate is already installed"
	}

	cmd := exec.Command("certutil", "-addstore", "-user", "Root", certPath)
	if err := cmd.Run(); err != nil {
		return "Failed to install certificate: " + err.Error()
	}

	return "CA certificate installed successfully. Restart your browser."
}
