# Agent Instructions

## Project Summary

Cross-platform system tray app for controlling mitmproxy and system proxy settings. Built with Go + systray. Works on macOS (status menu) and Windows (system tray).

## Tech Stack

- **Language**: Go 1.23+
- **UI**: getlantern/systray (cross-platform system tray)
- **Platforms**: macOS (status menu) + Windows (system tray)

## Folder Layout

```
├── main.go             # Shared systray UI and menu handling
├── mitm.go             # Shared mitmproxy process control + log management
├── mitm_darwin.go      # macOS process utilities (pgrep, signal)
├── mitm_windows.go     # Windows process utilities (tasklist, taskkill)
├── proxy_darwin.go     # macOS proxy config (networksetup)
├── proxy_windows.go    # Windows proxy config (registry + WinINet)
├── cert_darwin.go      # macOS CA cert install (Keychain + security CLI)
├── cert_windows.go     # Windows CA cert install (certutil)
├── simulator_darwin.go # macOS booted iOS simulator CA install (simctl keychain)
├── simulator_windows.go # No-op simulator stubs on Windows
├── emulator.go         # Cross-platform Android emulator support (adb-based)
├── emulator_test.go    # Unit tests for subject_hash_old and DisplayName
├── open_darwin.go      # macOS URL/file opening (open command)
├── open_windows.go     # Windows URL/file opening (rundll32/explorer)
├── go.mod              # Go module definition
├── go.sum              # Go dependencies lock
└── README.md
```

## Commands

```bash
# Build
go build -o mitmproxy-controller

# Run
./mitmproxy-controller

# Test
go test ./...

# Tidy dependencies
go mod tidy
```

## Key Files

- `main.go` - Shared UI logic:
  - `onReady()` - Sets up systray menu items
  - `startMitmproxy()` / `stopMitmproxy()` - Process control wrappers
  - `enableProxy()` / `disableProxy()` - System proxy wrappers
  - `updateStatus()` - Updates tray icon and status text

- `mitm.go` - Shared process control:
  - `startMitm()` / `stopMitm()` - Start/stop mitmweb/mitmdump process
  - `isMitmproxyRunning()` - Check if mitmproxy is running
  - `isWebUIAvailable()` / `getWebUIURL()` - Web UI availability and URL
  - `getLogsDirectory()` / `getCurrentLogPath()` - Log file management
  - Constants: `proxyHost` (127.0.0.1), `proxyPort` (8899), `webUIPort` (8898)

- `cert_darwin.go` / `cert_windows.go` - CA Certificate:
  - `isCertInstalled()` - Check if certificate is installed
  - `isCertTrusted()` - Check if certificate is trusted by system
  - `installCACertificate()` - Install and trust the CA cert
  - `trustCACertificate()` - Trust an already-installed cert (macOS)
  - `removeCACertificate()` - Remove cert from system trust store
  - `getMitmproxyCertPath()` - Path to mitmproxy CA cert
  - `getCertThumbprint()` - Get SHA1 thumbprint (Windows only)

- `open_darwin.go` / `open_windows.go` - Platform utilities:
  - `openURL()` - Open URL in default browser
  - `revealInFileManager()` - Open folder in Finder/Explorer

- `emulator.go` - Cross-platform Android emulator support (runs on macOS and Windows; `adb` does the cross-platform work):
  - `listBootedEmulators()` - Returns emulator serials/AVDs by parsing `adb devices -l` + `adb -s <serial> emu avd name`
  - `installAndTrustCACertificateOnEmulator(serial)` - `adb root` → `remount` → push cert to `/system/etc/security/cacerts/<hash>.0` → `chmod 644`
  - `enableEmulatorProxy(serial)` / `disableEmulatorProxy(serial)` - `adb shell settings put global http_proxy 10.0.2.2:<port>` and `:0` to clear
  - `isEmulatorProxyEnabled(serial)` - Polls `settings get global http_proxy` for live menu state
  - `rebootEmulator(serial)` - `adb -s <serial> reboot`; also clears the in-session "cert installed" cache for that serial
  - `subjectHashOld(certPEM)` - Pure-Go reimplementation of OpenSSL's `-subject_hash_old` (MD5 of DER subject, first 4 bytes LE, hex) — matches Android's cacerts filename convention. Tested in `emulator_test.go`.
  - `adbPath()` - Finds `adb` from PATH or `ANDROID_HOME`/`ANDROID_SDK_ROOT` or standard SDK install paths
  - Constant: `androidEmulatorHostAlias` = `10.0.2.2` (special emulator → host loopback alias)

- `proxy_darwin.go` - macOS proxy:
  - `enableSystemProxy()` / `disableSystemProxy()` - via networksetup
  - `isProxyEnabled()` - Check proxy state
  - `getActiveNetworkService()` - Detects active network interface

- `proxy_windows.go` - Windows proxy:
  - `enableSystemProxy()` / `disableSystemProxy()` - via registry
  - `isProxyEnabled()` - Check registry ProxyEnable value
  - `notifyProxyChange()` - Calls WinINet API to refresh

## Status Icons

- 🟢 mitmproxy running + proxy enabled
- 🟡 mitmproxy running + proxy disabled  
- 🟠 mitmproxy stopped + proxy enabled
- ⚫ both off

## Important Implementation Details

- **Port**: Uses `8899` (not 8080, which conflicts with Jenkins and other services)
- **Headless mode**: Uses `mitmdump` instead of `mitmproxy` - the TUI version fails without a TTY in background/systray apps
- **Network detection**: Uses `route get default` to find active interface, then maps device (e.g., `en0`) to service name (e.g., `Wi-Fi`) via `networksetup -listallhardwareports`
- **Proxy state**: Must explicitly call `-setwebproxystate on` after setting proxy host/port
- **Process lifecycle**: Uses goroutine with `cmd.Wait()` to track when mitmdump exits and clear `mitmProcess`
- **Thread safety**: Single goroutine handles both ticker and menu clicks to ensure thread-safe UI access
- **Polling**: Status refreshes every 5 seconds to detect external changes
- **Smart menu**: Actions disabled during operations and when not applicable (prevents double-clicks)

## Conventions

- Multi-file app with platform-specific code via Go build tags
- System commands use `exec.Command` with proper error handling
- App runs as a background process in the macOS status menu
