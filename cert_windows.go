//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func getMitmproxyCertPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mitmproxy", "mitmproxy-ca-cert.cer")
}

func getCertThumbprint() string {
	certPath := getMitmproxyCertPath()
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return ""
	}

	// Get SHA1 thumbprint of the certificate file
	out, err := exec.Command("certutil", "-hashfile", certPath, "SHA1").Output()
	if err != nil {
		return ""
	}

	// Parse output - thumbprint is on second line
	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return ""
	}

	// Clean up the thumbprint (remove spaces, lowercase)
	thumbprint := strings.TrimSpace(lines[1])
	thumbprint = strings.ReplaceAll(thumbprint, " ", "")
	return strings.ToLower(thumbprint)
}

func isCertInstalled() bool {
	thumbprint := getCertThumbprint()
	if thumbprint == "" {
		return false
	}

	out, err := exec.Command("certutil", "-store", "-user", "Root").Output()
	if err != nil {
		return false
	}

	// Look for the thumbprint in the store output
	// Format: "Cert Hash(sha1): xx xx xx xx..."
	output := strings.ToLower(string(out))
	// Normalize the store output thumbprints (remove spaces)
	re := regexp.MustCompile(`cert hash\(sha1\):\s*([a-f0-9 ]+)`)
	matches := re.FindAllStringSubmatch(output, -1)

	for _, match := range matches {
		if len(match) > 1 {
			storeThumbprint := strings.ReplaceAll(match[1], " ", "")
			storeThumbprint = strings.TrimSpace(storeThumbprint)
			if storeThumbprint == thumbprint {
				return true
			}
		}
	}

	return false
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

func trustCACertificate() string {
	// On Windows, installed = trusted, so just call install
	return installCACertificate()
}

func removeCACertificate() string {
	thumbprint := getCertThumbprint()
	if thumbprint == "" {
		return "CA cert file not found"
	}

	if !isCertInstalled() {
		return "CA certificate is not installed"
	}

	// Delete by thumbprint for precise targeting
	cmd := exec.Command("certutil", "-delstore", "-user", "Root", thumbprint)
	if err := cmd.Run(); err != nil {
		return "Failed to remove certificate: " + err.Error()
	}

	return "CA certificate removed. Restart your browser."
}
