//go:build darwin

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func enableSystemProxy() error {
	service, err := getActiveNetworkService()
	if err != nil {
		return err
	}

	if err := exec.Command("networksetup", "-setwebproxy", service, proxyHost, proxyPort).Run(); err != nil {
		return fmt.Errorf("failed to set HTTP proxy: %w", err)
	}

	if err := exec.Command("networksetup", "-setsecurewebproxy", service, proxyHost, proxyPort).Run(); err != nil {
		return fmt.Errorf("failed to set HTTPS proxy: %w", err)
	}

	// Explicitly enable the proxy state
	exec.Command("networksetup", "-setwebproxystate", service, "on").Run()
	exec.Command("networksetup", "-setsecurewebproxystate", service, "on").Run()

	return nil
}

func disableSystemProxy() error {
	service, err := getActiveNetworkService()
	if err != nil {
		return err
	}

	exec.Command("networksetup", "-setwebproxystate", service, "off").Run()
	exec.Command("networksetup", "-setsecurewebproxystate", service, "off").Run()

	return nil
}

func isProxyEnabled() bool {
	service, err := getActiveNetworkService()
	if err != nil {
		return false
	}

	out, _ := exec.Command("networksetup", "-getwebproxy", service).Output()
	return strings.Contains(string(out), "Enabled: Yes")
}

func getActiveNetworkService() (string, error) {
	// Get the default route interface (e.g., en0)
	routeOut, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return "Wi-Fi", nil
	}

	var activeInterface string
	for _, line := range strings.Split(string(routeOut), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "interface:") {
			activeInterface = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
			break
		}
	}

	if activeInterface == "" {
		return "Wi-Fi", nil
	}

	// Map interface (en0) to service name (Wi-Fi) using hardware ports
	hwOut, err := exec.Command("networksetup", "-listallhardwareports").Output()
	if err != nil {
		return "Wi-Fi", nil
	}

	lines := strings.Split(string(hwOut), "\n")
	for i, line := range lines {
		if strings.Contains(line, "Device: "+activeInterface) && i > 0 {
			// Look for "Hardware Port:" in previous lines
			for j := i - 1; j >= 0; j-- {
				if strings.HasPrefix(lines[j], "Hardware Port:") {
					service := strings.TrimSpace(strings.TrimPrefix(lines[j], "Hardware Port:"))
					return service, nil
				}
			}
		}
	}

	return "Wi-Fi", nil
}
