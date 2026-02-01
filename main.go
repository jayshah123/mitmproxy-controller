package main

import (
	"fmt"
	"time"

	"github.com/getlantern/systray"
)

// Menu items (global for access in updateStatus)
var (
	mStatus       *systray.MenuItem
	mStartMitm    *systray.MenuItem
	mStopMitm     *systray.MenuItem
	mEnableProxy  *systray.MenuItem
	mDisableProxy *systray.MenuItem
	mViewFlows    *systray.MenuItem
	mRevealLogs   *systray.MenuItem
	mInstallCert  *systray.MenuItem
	mRemoveCert   *systray.MenuItem
)

// Track cert state for click handler
var (
	certInstalled bool
	certTrusted   bool
)

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTitle("⚡")
	systray.SetTooltip("mitmproxy Controller")

	mStatus = systray.AddMenuItem("Status: Checking...", "Current status")
	mStatus.Disable()

	systray.AddSeparator()

	mStartMitm = systray.AddMenuItem("Start mitmproxy", "Start mitmproxy process")
	mStopMitm = systray.AddMenuItem("Stop mitmproxy", "Stop mitmproxy process")

	systray.AddSeparator()

	mEnableProxy = systray.AddMenuItem("Enable System Proxy", "Route traffic through mitmproxy")
	mDisableProxy = systray.AddMenuItem("Disable System Proxy", "Disable system proxy")

	systray.AddSeparator()

	mViewFlows = systray.AddMenuItem("View Flows (Web UI)", "Open mitmweb interface in browser")
	mRevealLogs = systray.AddMenuItem("Reveal Logs Folder", "Open logs folder in file manager")

	systray.AddSeparator()

	mInstallCert = systray.AddMenuItem("Install CA Certificate", "Install mitmproxy CA cert for HTTPS interception")
	mRemoveCert = systray.AddMenuItem("Remove CA Certificate", "Remove mitmproxy CA cert from system")

	systray.AddSeparator()

	mRefresh := systray.AddMenuItem("Refresh Status", "Refresh current status")

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit", "Quit the app")

	// Update status initially
	updateStatus()

	// Single goroutine handles both periodic polling and menu clicks
	// This ensures thread-safe access to systray UI
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				updateStatus()

			case <-mStartMitm.ClickedCh:
				disableAllActions()
				mStatus.SetTitle(startMitmproxy())
				updateStatus()

			case <-mStopMitm.ClickedCh:
				disableAllActions()
				mStatus.SetTitle(stopMitmproxy())
				updateStatus()

			case <-mEnableProxy.ClickedCh:
				disableAllActions()
				mStatus.SetTitle(enableProxy())
				updateStatus()

			case <-mDisableProxy.ClickedCh:
				disableAllActions()
				mStatus.SetTitle(disableProxy())
				updateStatus()

			case <-mViewFlows.ClickedCh:
				if isWebUIAvailable() {
					openURL(getWebUIURL())
				}

			case <-mRevealLogs.ClickedCh:
				revealInFileManager(getLogsDirectory())

			case <-mInstallCert.ClickedCh:
				if certInstalled && !certTrusted {
					mStatus.SetTitle(trustCACertificate())
				} else {
					mStatus.SetTitle(installCACertificate())
				}
				updateStatus()

			case <-mRemoveCert.ClickedCh:
				mStatus.SetTitle(removeCACertificate())
				updateStatus()

			case <-mRefresh.ClickedCh:
				updateStatus()

			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	// Cleanup if needed
}

func disableAllActions() {
	mStartMitm.Disable()
	mStopMitm.Disable()
	mEnableProxy.Disable()
	mDisableProxy.Disable()
}

func updateStatus() {
	mitmRunning := isMitmproxyRunning()
	proxyEnabled := isProxyEnabled()

	// Update status text and icon
	var statusText string
	if mitmRunning && proxyEnabled {
		systray.SetTitle("🟢")
		statusText = "mitmproxy: Running | Proxy: Enabled"
	} else if mitmRunning {
		systray.SetTitle("🟡")
		statusText = "mitmproxy: Running | Proxy: Disabled"
	} else if proxyEnabled {
		systray.SetTitle("🟠")
		statusText = "mitmproxy: Stopped | Proxy: Enabled"
	} else {
		systray.SetTitle("⚫")
		statusText = "mitmproxy: Stopped | Proxy: Disabled"
	}
	mStatus.SetTitle(statusText)

	// Enable/disable menu items based on current state
	if mitmRunning {
		mStartMitm.Disable()
		mStopMitm.Enable()
	} else {
		mStartMitm.Enable()
		mStopMitm.Disable()
	}

	if proxyEnabled {
		mEnableProxy.Disable()
		mDisableProxy.Enable()
	} else {
		mEnableProxy.Enable()
		mDisableProxy.Disable()
	}

	// View Flows only available when mitmweb is running
	if isWebUIAvailable() {
		mViewFlows.Enable()
	} else {
		mViewFlows.Disable()
	}

	// Update cert menu items based on installation and trust status
	certInstalled = isCertInstalled()
	certTrusted = isCertTrusted()

	if certTrusted {
		mInstallCert.SetTitle("CA Certificate ✓ Trusted")
		mInstallCert.Disable()
		mRemoveCert.Enable()
	} else if certInstalled {
		mInstallCert.SetTitle("Trust CA Certificate")
		mInstallCert.Enable()
		mRemoveCert.Enable()
	} else {
		mInstallCert.SetTitle("Install CA Certificate")
		mInstallCert.Enable()
		mRemoveCert.Disable()
	}
}

func startMitmproxy() string {
	return startMitm()
}

func stopMitmproxy() string {
	return stopMitm()
}

func enableProxy() string {
	err := enableSystemProxy()
	if err != nil {
		return fmt.Sprintf("Failed to enable proxy: %v", err)
	}
	return "Proxy enabled"
}

func disableProxy() string {
	err := disableSystemProxy()
	if err != nil {
		return fmt.Sprintf("Failed to disable proxy: %v", err)
	}
	return "Proxy disabled"
}
