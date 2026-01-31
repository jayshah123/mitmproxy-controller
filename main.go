package main

import (
	"fmt"

	"github.com/getlantern/systray"
)

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTitle("⚡")
	systray.SetTooltip("mitmproxy Controller")

	mStatus := systray.AddMenuItem("Status: Checking...", "Current status")
	mStatus.Disable()

	systray.AddSeparator()

	mStartMitm := systray.AddMenuItem("Start mitmproxy", "Start mitmproxy process")
	mStopMitm := systray.AddMenuItem("Stop mitmproxy", "Stop mitmproxy process")

	systray.AddSeparator()

	mEnableProxy := systray.AddMenuItem("Enable System Proxy", "Route traffic through mitmproxy")
	mDisableProxy := systray.AddMenuItem("Disable System Proxy", "Disable system proxy")

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit", "Quit the app")

	// Update status initially
	go updateStatus(mStatus)

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-mStartMitm.ClickedCh:
				result := startMitmproxy()
				mStatus.SetTitle(result)
				go updateStatus(mStatus)

			case <-mStopMitm.ClickedCh:
				result := stopMitmproxy()
				mStatus.SetTitle(result)
				go updateStatus(mStatus)

			case <-mEnableProxy.ClickedCh:
				result := enableProxy()
				mStatus.SetTitle(result)
				go updateStatus(mStatus)

			case <-mDisableProxy.ClickedCh:
				result := disableProxy()
				mStatus.SetTitle(result)
				go updateStatus(mStatus)

			case <-mQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()
}

func onExit() {
	// Cleanup if needed
}

func updateStatus(mStatus *systray.MenuItem) {
	mitmRunning := isMitmproxyRunning()
	proxyEnabled := isProxyEnabled()

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
