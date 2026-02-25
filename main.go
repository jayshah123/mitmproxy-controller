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
	mProfiles     *systray.MenuItem
	mEditProfile  *systray.MenuItem
	mOpenScripts  *systray.MenuItem
	mViewFlows    *systray.MenuItem
	mRevealLogs   *systray.MenuItem
	mOpenMitmHome *systray.MenuItem
	mEditConfig   *systray.MenuItem
	mInstallCert  *systray.MenuItem
	mRemoveCert   *systray.MenuItem
)

var (
	profileItems      = map[string]*systray.MenuItem{}
	profileSelectionC = make(chan string, 32)
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

	if err := initProfiles(); err != nil {
		fmt.Printf("Failed to initialize profiles: %v\n", err)
	}

	mStatus = systray.AddMenuItem("Status: Checking...", "Current status")
	mStatus.Disable()

	systray.AddSeparator()

	mStartMitm = systray.AddMenuItem("Start mitmproxy", "Start mitmproxy process")
	mStopMitm = systray.AddMenuItem("Stop mitmproxy", "Stop mitmproxy process")

	systray.AddSeparator()

	mEnableProxy = systray.AddMenuItem("Enable System Proxy", "Route traffic through mitmproxy")
	mDisableProxy = systray.AddMenuItem("Disable System Proxy", "Disable system proxy")

	systray.AddSeparator()

	mProfiles = systray.AddMenuItem("Service Profile", "Select active service profile")
	syncProfileSubmenu()
	mEditProfile = systray.AddMenuItem("Edit Active Profile", "Open active service profile file")
	mOpenScripts = systray.AddMenuItem("Open Active Scripts Folder", "Open folder for active profile scripts")

	systray.AddSeparator()

	mViewFlows = systray.AddMenuItem("View Flows (Web UI)", "Open mitmweb interface in browser")
	mRevealLogs = systray.AddMenuItem("Reveal Logs Folder", "Open logs folder in file manager")
	mOpenMitmHome = systray.AddMenuItem("Open mitmproxy Home Folder", "Open ~/.mitmproxy folder in file manager")
	mEditConfig = systray.AddMenuItem("Edit mitmproxy Config", "Open ~/.mitmproxy/config.yaml in your default editor")

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

			case profileID := <-profileSelectionC:
				mStatus.SetTitle(applyProfileSelection(profileID))
				updateStatus()

			case <-mEditProfile.ClickedCh:
				profilePath := selectedProfilePath()
				if profilePath == "" {
					mStatus.SetTitle("No active profile file found")
					continue
				}
				if err := openFile(profilePath); err != nil {
					mStatus.SetTitle(fmt.Sprintf("Failed to open profile: %v", err))
					continue
				}
				mStatus.SetTitle("Opened active profile")

			case <-mOpenScripts.ClickedCh:
				scriptsDir, err := ensureSelectedProfileScriptsFolder()
				if err != nil {
					mStatus.SetTitle(fmt.Sprintf("Failed to prepare scripts folder: %v", err))
					continue
				}
				if err := revealInFileManager(scriptsDir); err != nil {
					mStatus.SetTitle(fmt.Sprintf("Failed to open scripts folder: %v", err))
					continue
				}
				mStatus.SetTitle("Opened scripts folder")

			case <-mViewFlows.ClickedCh:
				if isWebUIAvailable() {
					openURL(getWebUIURL())
				}

			case <-mRevealLogs.ClickedCh:
				revealInFileManager(getLogsDirectory())

			case <-mOpenMitmHome.ClickedCh:
				mitmHomeDir, err := ensureMitmHomeDirectoryExists()
				if err != nil {
					mStatus.SetTitle(fmt.Sprintf("Failed to prepare mitmproxy home: %v", err))
					continue
				}
				if err := revealInFileManager(mitmHomeDir); err != nil {
					mStatus.SetTitle(fmt.Sprintf("Failed to open mitmproxy home: %v", err))
					continue
				}
				mStatus.SetTitle("Opened ~/.mitmproxy")

			case <-mEditConfig.ClickedCh:
				configPath, err := ensureMitmConfigExists()
				if err != nil {
					mStatus.SetTitle(fmt.Sprintf("Failed to prepare config: %v", err))
					continue
				}
				if err := openFile(configPath); err != nil {
					mStatus.SetTitle(fmt.Sprintf("Failed to open config: %v", err))
					continue
				}
				mStatus.SetTitle("Opened config.yaml")

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
				if err := loadProfilesFromDisk(); err != nil {
					mStatus.SetTitle(fmt.Sprintf("Failed to refresh profiles: %v", err))
				} else {
					syncProfileSubmenu()
				}
				updateStatus()

			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func syncProfileSubmenu() {
	profiles := listProfiles()
	visibleIDs := make(map[string]bool, len(profiles))

	for _, profile := range profiles {
		p := profile
		visibleIDs[p.ID] = true
		item, ok := profileItems[p.ID]
		if !ok {
			item = mProfiles.AddSubMenuItemCheckbox(p.Name, p.ID, p.ID == selectedProfileID)
			profileItems[p.ID] = item
			wireProfileSelection(p.ID, item)
		} else {
			item.SetTitle(p.Name)
			item.Show()
		}

		if p.ID == selectedProfileID {
			item.Check()
		} else {
			item.Uncheck()
		}
	}

	for id, item := range profileItems {
		if !visibleIDs[id] {
			item.Hide()
		}
	}
}

func wireProfileSelection(id string, menuItem *systray.MenuItem) {
	go func() {
		for range menuItem.ClickedCh {
			select {
			case profileSelectionC <- id:
			default:
			}
		}
	}()
}

func applyProfileSelection(profileID string) string {
	if profileID == selectedProfileID {
		return fmt.Sprintf("Service profile already selected: %s", selectedProfileName())
	}

	if err := setSelectedProfile(profileID); err != nil {
		return fmt.Sprintf("Failed to select profile: %v", err)
	}

	for id, item := range profileItems {
		if id == selectedProfileID {
			item.Check()
		} else {
			item.Uncheck()
		}
	}

	name := selectedProfileName()
	if isMitmproxyRunning() {
		stopResult := stopMitmproxy()
		startResult := startMitmproxy()
		return fmt.Sprintf("Profile %s applied (%s, %s)", name, stopResult, startResult)
	}

	return fmt.Sprintf("Selected profile: %s", name)
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
	profileName := selectedProfileName()
	proxyCompatible, webCompatible := selectedProfileCompatibility()
	warnings := selectedProfileWarnings()
	loadWarnings := profileLoadWarnings()

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
	statusText = fmt.Sprintf("%s | Profile: %s", statusText, profileName)
	if len(warnings) > 0 {
		statusText = fmt.Sprintf("%s | Warnings: %d", statusText, len(warnings))
	}
	if len(loadWarnings) > 0 {
		statusText = fmt.Sprintf("%s | Profile load warnings: %d", statusText, len(loadWarnings))
	}
	mStatus.SetTitle(statusText)
	mProfiles.SetTitle(fmt.Sprintf("Service Profile: %s", profileName))

	// Enable/disable menu items based on current state
	if mitmRunning {
		mStartMitm.Disable()
		mStopMitm.Enable()
	} else {
		mStartMitm.Enable()
		mStopMitm.Disable()
	}

	if !proxyCompatible {
		mEnableProxy.Disable()
		mDisableProxy.Disable()
	} else if proxyEnabled {
		mEnableProxy.Disable()
		mDisableProxy.Enable()
	} else {
		mEnableProxy.Enable()
		mDisableProxy.Disable()
	}

	// View Flows only available when mitmweb is running
	if isWebUIAvailable() && webCompatible {
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
