//go:build darwin

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

var simulatorCACertificateInstalled = map[string]bool{}

type BootedSimulator struct {
	Name    string
	UDID    string
	Runtime string
}

func (s BootedSimulator) DisplayName() string {
	runtime := simulatorRuntimeName(s.Runtime)
	if runtime == "" {
		return s.Name
	}
	return fmt.Sprintf("%s (%s)", s.Name, runtime)
}

type simctlDeviceList struct {
	Devices map[string][]simctlDevice `json:"devices"`
}

type simctlDevice struct {
	Name        string `json:"name"`
	UDID        string `json:"udid"`
	State       string `json:"state"`
	IsAvailable bool   `json:"isAvailable"`
}

func listBootedSimulators() ([]BootedSimulator, error) {
	out, err := exec.Command("xcrun", "simctl", "list", "devices", "booted", "-j").Output()
	if err != nil {
		return nil, fmt.Errorf("xcrun simctl list failed: %w", err)
	}

	var deviceList simctlDeviceList
	if err := json.Unmarshal(out, &deviceList); err != nil {
		return nil, fmt.Errorf("parse simctl output: %w", err)
	}

	var simulators []BootedSimulator
	for runtime, devices := range deviceList.Devices {
		for _, device := range devices {
			if device.State != "Booted" || device.UDID == "" {
				continue
			}
			simulators = append(simulators, BootedSimulator{
				Name:    device.Name,
				UDID:    device.UDID,
				Runtime: runtime,
			})
		}
	}

	sort.Slice(simulators, func(i, j int) bool {
		if simulators[i].Name == simulators[j].Name {
			return simulators[i].Runtime < simulators[j].Runtime
		}
		return simulators[i].Name < simulators[j].Name
	})

	return simulators, nil
}

func installAndTrustCACertificateOnSimulator(udid string) string {
	if simulatorCACertificateInstalled[udid] {
		return "CA certificate is already installed & trusted on simulator."
	}

	certPath := getMitmproxyCertPath()
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return "CA cert not found. Start mitmproxy first to generate it."
	}

	cmd := exec.Command("xcrun", "simctl", "keychain", udid, "add-root-cert", certPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		detail := strings.TrimSpace(string(out))
		if simulatorCertAlreadyExists(detail) {
			simulatorCACertificateInstalled[udid] = true
			return "CA certificate is already installed & trusted on simulator."
		}
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Sprintf("Failed to install simulator CA cert: %s", detail)
	}

	simulatorCACertificateInstalled[udid] = true
	return "CA certificate installed & trusted on simulator."
}

func isSimulatorCACertificateKnownInstalled(udid string) bool {
	return simulatorCACertificateInstalled[udid]
}

func simulatorCertAlreadyExists(output string) bool {
	output = strings.ToLower(output)
	return strings.Contains(output, "already") ||
		strings.Contains(output, "exists") ||
		strings.Contains(output, "duplicate")
}

func simulatorRuntimeName(runtimeID string) string {
	const prefix = "com.apple.CoreSimulator.SimRuntime."
	runtimeID = strings.TrimPrefix(runtimeID, prefix)
	runtimeID = strings.ReplaceAll(runtimeID, "-", " ")

	parts := strings.Fields(runtimeID)
	if len(parts) == 0 {
		return ""
	}

	version := ""
	if len(parts) > 1 {
		version = strings.Join(parts[1:], ".")
	}

	switch parts[0] {
	case "iOS":
		if version == "" {
			return "iOS"
		}
		return "iOS " + version
	case "tvOS":
		if version == "" {
			return "tvOS"
		}
		return "tvOS " + version
	case "watchOS":
		if version == "" {
			return "watchOS"
		}
		return "watchOS " + version
	case "visionOS":
		if version == "" {
			return "visionOS"
		}
		return "visionOS " + version
	default:
		if version == "" {
			return parts[0]
		}
		return parts[0] + " " + version
	}
}
