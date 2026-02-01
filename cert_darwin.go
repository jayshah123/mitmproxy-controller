//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func getMitmproxyCertPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mitmproxy", "mitmproxy-ca-cert.pem")
}

func isCertInstalled() bool {
	out, err := exec.Command("security", "find-certificate", "-c", "mitmproxy", "/Library/Keychains/System.keychain").Output()
	if err == nil && len(out) > 0 {
		return true
	}
	out, _ = exec.Command("security", "find-certificate", "-c", "mitmproxy").Output()
	return len(out) > 0
}

func isCertTrusted() bool {
	cmd := exec.Command("security", "find-certificate", "-c", "mitmproxy", "-p", "/Library/Keychains/System.keychain")
	certPem, err := cmd.Output()
	if err != nil || len(certPem) == 0 {
		return false
	}

	tmpFile, err := os.CreateTemp("", "mitmproxy-cert-*.pem")
	if err != nil {
		return false
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Write(certPem)
	tmpFile.Close()

	verifyCmd := exec.Command("security", "verify-cert", "-c", tmpFile.Name())
	return verifyCmd.Run() == nil
}

func installCACertificate() string {
	certPath := getMitmproxyCertPath()

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return "CA cert not found. Start mitmproxy first to generate it."
	}

	// Two-step process like Proxyman:
	// 1. Delete all existing mitmproxy certs from System keychain
	// 2. Import cert to System keychain
	// 3. Set trust settings in admin domain
	script := fmt.Sprintf(`do shell script "
		# Delete all existing mitmproxy certs
		while security delete-certificate -c mitmproxy /Library/Keychains/System.keychain 2>/dev/null; do :; done
		
		# Import the certificate to System keychain
		security import '%s' -k /Library/Keychains/System.keychain -t cert
		
		# Set trust settings (this is the key step for trusting)
		security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain '%s'
		
		# Refresh trust daemon
		killall -HUP trustd 2>/dev/null || true
	" with administrator privileges`, certPath, certPath)

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("Failed to install certificate: %v", err)
	}

	return "CA certificate installed & trusted. Restart your browser."
}
