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
	return err == nil && len(out) > 0
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

	verifyCmd := exec.Command("security", "verify-cert", "-c", tmpFile.Name(), "-p", "ssl")
	return verifyCmd.Run() == nil
}

func installCACertificate() string {
	certPath := getMitmproxyCertPath()

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return "CA cert not found. Start mitmproxy first to generate it."
	}

	// Three-step process:
	// 1. Delete all existing mitmproxy certs from System keychain
	// 2. Import cert to System keychain
	// 3. Set trust settings with SSL policy
	script := fmt.Sprintf(`do shell script "
		# Delete all existing mitmproxy certs
		while security delete-certificate -c mitmproxy /Library/Keychains/System.keychain 2>/dev/null; do :; done
		
		# Import the certificate to System keychain
		security import '%s' -k /Library/Keychains/System.keychain -t cert
		
		# Set trust settings with SSL policy (like devcert)
		security add-trusted-cert -d -r trustRoot -p ssl -p basic -k /Library/Keychains/System.keychain '%s'
		
		# Refresh trust daemon
		killall -HUP trustd 2>/dev/null || true
	" with administrator privileges`, certPath, certPath)

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("Failed to install certificate: %v", err)
	}

	return "CA certificate installed & trusted. Restart your browser."
}

func trustCACertificate() string {
	certPath := getMitmproxyCertPath()

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return "CA cert not found. Start mitmproxy first to generate it."
	}

	// Apply trust settings to already-installed certificate
	script := fmt.Sprintf(`do shell script "
		# Set trust settings with SSL policy
		security add-trusted-cert -d -r trustRoot -p ssl -p basic -k /Library/Keychains/System.keychain '%s'
		
		# Refresh trust daemon
		killall -HUP trustd 2>/dev/null || true
	" with administrator privileges`, certPath)

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("Failed to trust certificate: %v", err)
	}

	return "CA certificate is now trusted. Restart your browser."
}

func removeCACertificate() string {
	// Remove trust settings and delete certificate from System keychain
	script := `do shell script "
		# Remove trust settings
		security remove-trusted-cert -d /Library/Keychains/System.keychain 2>/dev/null || true
		
		# Delete all mitmproxy certs from System keychain
		while security delete-certificate -c mitmproxy /Library/Keychains/System.keychain 2>/dev/null; do :; done
		
		# Refresh trust daemon
		killall -HUP trustd 2>/dev/null || true
	" with administrator privileges`

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("Failed to remove certificate: %v", err)
	}

	return "CA certificate removed. Restart your browser."
}
