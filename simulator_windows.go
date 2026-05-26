//go:build windows

package main

import "fmt"

type BootedSimulator struct {
	Name    string
	UDID    string
	Runtime string
}

func (s BootedSimulator) DisplayName() string {
	if s.Runtime == "" {
		return s.Name
	}
	return fmt.Sprintf("%s (%s)", s.Name, s.Runtime)
}

func listBootedSimulators() ([]BootedSimulator, error) {
	return nil, nil
}

func installAndTrustCACertificateOnSimulator(udid string) string {
	return "iOS simulator certificates are only supported on macOS."
}

func isSimulatorCACertificateKnownInstalled(udid string) bool {
	return false
}
