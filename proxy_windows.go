//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

const (
	internetOptionSettingsChanged = 39
	internetOptionRefresh         = 37
)

var (
	wininet                = syscall.NewLazyDLL("wininet.dll")
	internetSetOptionProc  = wininet.NewProc("InternetSetOptionW")
)

func enableSystemProxy() error {
	proxyServer := fmt.Sprintf("%s:%s", proxyHost, proxyPort)

	// Set ProxyEnable = 1
	if err := exec.Command("reg", "add",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		"/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "1", "/f").Run(); err != nil {
		return fmt.Errorf("failed to enable proxy: %w", err)
	}

	// Set ProxyServer
	if err := exec.Command("reg", "add",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		"/v", "ProxyServer", "/t", "REG_SZ", "/d", proxyServer, "/f").Run(); err != nil {
		return fmt.Errorf("failed to set proxy server: %w", err)
	}

	// Notify applications of the change
	notifyProxyChange()

	return nil
}

func disableSystemProxy() error {
	// Set ProxyEnable = 0
	if err := exec.Command("reg", "add",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		"/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "0", "/f").Run(); err != nil {
		return fmt.Errorf("failed to disable proxy: %w", err)
	}

	// Notify applications of the change
	notifyProxyChange()

	return nil
}

func isProxyEnabled() bool {
	out, err := exec.Command("reg", "query",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		"/v", "ProxyEnable").Output()
	if err != nil {
		return false
	}

	// Check if ProxyEnable is set to 1
	// Output format: "    ProxyEnable    REG_DWORD    0x1"
	return strings.Contains(string(out), "0x1")
}

func notifyProxyChange() {
	// Call InternetSetOption to notify applications of proxy settings change
	internetSetOptionProc.Call(
		0,
		uintptr(internetOptionSettingsChanged),
		0,
		0,
	)
	internetSetOptionProc.Call(
		0,
		uintptr(internetOptionRefresh),
		0,
		0,
	)
}

// Suppress unused warning for unsafe import (used for potential future extensions)
var _ = unsafe.Sizeof(0)
