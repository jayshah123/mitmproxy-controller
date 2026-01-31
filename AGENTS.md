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
├── mitm.go             # Shared mitmproxy process control
├── mitm_darwin.go      # macOS process utilities (pgrep, signal)
├── mitm_windows.go     # Windows process utilities (tasklist, taskkill)
├── proxy_darwin.go     # macOS proxy config (networksetup)
├── proxy_windows.go    # Windows proxy config (registry + WinINet)
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
  - `startMitm()` / `stopMitm()` - Start/stop mitmdump process
  - `isMitmproxyRunning()` - Check if mitmproxy is running
  - Constants: `proxyHost` (127.0.0.1), `proxyPort` (8899)

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

## Conventions

- Multi-file app with platform-specific code via Go build tags
- System commands use `exec.Command` with proper error handling
- App runs as a background process in the macOS status menu
