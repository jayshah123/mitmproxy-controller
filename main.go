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
	mSimulators   *systray.MenuItem
	mEmulators    *systray.MenuItem
)

var (
	profileItems      = map[string]*systray.MenuItem{}
	profileSelectionC = make(chan string, 32)
)

var (
	simulatorItems      = map[string]*systray.MenuItem{}
	simulatorSelectionC = make(chan string, 32)
	mNoSimulators       *systray.MenuItem
)

type emulatorMenu struct {
	root         *systray.MenuItem
	installCert  *systray.MenuItem
	enableProxy  *systray.MenuItem
	disableProxy *systray.MenuItem
	reboot       *systray.MenuItem
}

type emulatorAction struct {
	Serial string
	Kind   string // "install_cert", "enable_proxy", "disable_proxy"
}

var (
	emulatorItems      = map[string]*emulatorMenu{}
	emulatorActionC    = make(chan emulatorAction, 32)
	mNoEmulators       *systray.MenuItem
)

var (
	transientStatus      string
	transientStatusUntil time.Time
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
	mSimulators = systray.AddMenuItem("Booted Simulators", "Install and trust mitmproxy CA cert on a booted simulator")
	syncBootedSimulatorSubmenu()
	mEmulators = systray.AddMenuItem("Booted Emulators", "Install CA cert and toggle proxy on a booted Android emulator")
	syncBootedEmulatorSubmenu()

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

			case simulatorUDID := <-simulatorSelectionC:
				disableAllActions()
				setTransientStatus(installAndTrustCACertificateOnSimulator(simulatorUDID))
				updateStatus()

			case action := <-emulatorActionC:
				disableAllActions()
				var msg string
				switch action.Kind {
				case "install_cert":
					msg = installAndTrustCACertificateOnEmulator(action.Serial)
				case "enable_proxy":
					msg = enableEmulatorProxy(action.Serial)
				case "disable_proxy":
					msg = disableEmulatorProxy(action.Serial)
				case "reboot":
					msg = rebootEmulator(action.Serial)
				default:
					msg = fmt.Sprintf("Unknown emulator action: %s", action.Kind)
				}
				setTransientStatus(msg)
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

func syncBootedSimulatorSubmenu() {
	if mSimulators == nil {
		return
	}

	simulators, err := listBootedSimulators()
	if err != nil {
		mSimulators.SetTitle("Booted Simulators: Unavailable")
		mSimulators.Enable()
		hideSimulatorItems()
		if mNoSimulators == nil {
			mNoSimulators = mSimulators.AddSubMenuItem("Unable to list simulators", err.Error())
		} else {
			mNoSimulators.SetTitle("Unable to list simulators")
			mNoSimulators.SetTooltip(err.Error())
			mNoSimulators.Show()
		}
		mNoSimulators.Disable()
		return
	}

	mSimulators.SetTitle(fmt.Sprintf("Booted Simulators (%d)", len(simulators)))
	mSimulators.Enable()

	if len(simulators) == 0 {
		hideSimulatorItems()
		if mNoSimulators == nil {
			mNoSimulators = mSimulators.AddSubMenuItem("No booted simulators", "Start an iOS simulator to install the CA certificate")
		} else {
			mNoSimulators.SetTitle("No booted simulators")
			mNoSimulators.SetTooltip("Start an iOS simulator to install the CA certificate")
			mNoSimulators.Show()
		}
		mNoSimulators.Disable()
		return
	}

	if mNoSimulators != nil {
		mNoSimulators.Hide()
	}

	visibleUDIDs := make(map[string]bool, len(simulators))
	for _, simulator := range simulators {
		s := simulator
		visibleUDIDs[s.UDID] = true
		title := fmt.Sprintf("Install & Trust CA: %s", s.DisplayName())
		if isSimulatorCACertificateKnownInstalled(s.UDID) {
			title = fmt.Sprintf("CA Certificate ✓ Installed: %s", s.DisplayName())
		}
		tooltip := fmt.Sprintf("Install mitmproxy CA certificate on simulator %s", s.UDID)

		item, ok := simulatorItems[s.UDID]
		if !ok {
			item = mSimulators.AddSubMenuItem(title, tooltip)
			simulatorItems[s.UDID] = item
			wireSimulatorSelection(s.UDID, item)
		} else {
			item.SetTitle(title)
			item.SetTooltip(tooltip)
			item.Show()
			item.Enable()
		}
	}

	for udid, item := range simulatorItems {
		if !visibleUDIDs[udid] {
			item.Hide()
		}
	}
}

func hideSimulatorItems() {
	for _, item := range simulatorItems {
		item.Hide()
	}
}

func wireSimulatorSelection(udid string, menuItem *systray.MenuItem) {
	go func() {
		for range menuItem.ClickedCh {
			select {
			case simulatorSelectionC <- udid:
			default:
			}
		}
	}()
}

func syncBootedEmulatorSubmenu() {
	if mEmulators == nil {
		return
	}

	emulators, err := listBootedEmulators()
	if err != nil {
		mEmulators.SetTitle("Booted Emulators: Unavailable")
		mEmulators.Enable()
		hideEmulatorItems()
		if mNoEmulators == nil {
			mNoEmulators = mEmulators.AddSubMenuItem("Unable to list emulators", err.Error())
		} else {
			mNoEmulators.SetTitle("Unable to list emulators")
			mNoEmulators.SetTooltip(err.Error())
			mNoEmulators.Show()
		}
		mNoEmulators.Disable()
		return
	}

	mEmulators.SetTitle(fmt.Sprintf("Booted Emulators (%d)", len(emulators)))
	mEmulators.Enable()

	if len(emulators) == 0 {
		hideEmulatorItems()
		msg := "No booted Android emulators"
		tip := "Start an Android emulator (with -writable-system for cert install) to enable these actions"
		if adbPath() == "" {
			msg = "adb not found"
			tip = "Install Android platform-tools or set ANDROID_HOME"
		}
		if mNoEmulators == nil {
			mNoEmulators = mEmulators.AddSubMenuItem(msg, tip)
		} else {
			mNoEmulators.SetTitle(msg)
			mNoEmulators.SetTooltip(tip)
			mNoEmulators.Show()
		}
		mNoEmulators.Disable()
		return
	}

	if mNoEmulators != nil {
		mNoEmulators.Hide()
	}

	visibleSerials := make(map[string]bool, len(emulators))
	for _, emulator := range emulators {
		e := emulator
		visibleSerials[e.Serial] = true

		menu, ok := emulatorItems[e.Serial]
		if !ok {
			menu = &emulatorMenu{
				root: mEmulators.AddSubMenuItem(e.DisplayName(), fmt.Sprintf("Actions for emulator %s", e.Serial)),
			}
			menu.installCert = menu.root.AddSubMenuItem("Install & Trust CA Certificate", "Install mitmproxy CA cert into the emulator's system trust store")
			menu.enableProxy = menu.root.AddSubMenuItem("Enable Emulator Proxy", "Point this emulator's HTTP proxy at the host mitmproxy")
			menu.disableProxy = menu.root.AddSubMenuItem("Disable Emulator Proxy", "Clear this emulator's HTTP proxy setting")
			menu.reboot = menu.root.AddSubMenuItem("Reboot Emulator", "Soft-reboot this emulator via adb reboot")
			emulatorItems[e.Serial] = menu
			wireEmulatorAction(e.Serial, "install_cert", menu.installCert)
			wireEmulatorAction(e.Serial, "enable_proxy", menu.enableProxy)
			wireEmulatorAction(e.Serial, "disable_proxy", menu.disableProxy)
			wireEmulatorAction(e.Serial, "reboot", menu.reboot)
		} else {
			menu.root.SetTitle(e.DisplayName())
			menu.root.Show()
		}

		if isEmulatorCACertificateKnownInstalled(e.Serial) {
			menu.installCert.SetTitle("CA Certificate ✓ Installed")
		} else {
			menu.installCert.SetTitle("Install & Trust CA Certificate")
		}

		proxyOn := isEmulatorProxyEnabled(e.Serial)
		if proxyOn {
			menu.enableProxy.SetTitle("Proxy ✓ Enabled")
			menu.enableProxy.Disable()
			menu.disableProxy.SetTitle("Disable Emulator Proxy")
			menu.disableProxy.Enable()
		} else {
			menu.enableProxy.SetTitle("Enable Emulator Proxy")
			menu.enableProxy.Enable()
			menu.disableProxy.SetTitle("Disable Emulator Proxy")
			menu.disableProxy.Disable()
		}
		menu.installCert.Enable()
		menu.reboot.Enable()
	}

	for serial, menu := range emulatorItems {
		if !visibleSerials[serial] {
			menu.root.Hide()
		}
	}
}

func hideEmulatorItems() {
	for _, menu := range emulatorItems {
		menu.root.Hide()
	}
}

func wireEmulatorAction(serial, kind string, menuItem *systray.MenuItem) {
	go func() {
		for range menuItem.ClickedCh {
			select {
			case emulatorActionC <- emulatorAction{Serial: serial, Kind: kind}:
			default:
			}
		}
	}()
}

func setTransientStatus(message string) {
	transientStatus = message
	transientStatusUntil = time.Now().Add(15 * time.Second)
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
	if mSimulators != nil {
		mSimulators.Disable()
	}
	if mEmulators != nil {
		mEmulators.Disable()
	}
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
	if transientStatus != "" {
		if time.Now().Before(transientStatusUntil) {
			statusText = transientStatus
		} else {
			transientStatus = ""
		}
	}
	mStatus.SetTitle(statusText)
	mProfiles.SetTitle(fmt.Sprintf("Service Profile: %s", profileName))
	syncBootedSimulatorSubmenu()
	syncBootedEmulatorSubmenu()

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
