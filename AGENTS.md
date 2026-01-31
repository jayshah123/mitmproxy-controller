# Agent Instructions

## Project Summary

macOS status menu (menu bar) app for controlling mitmproxy and system proxy settings. Built with Go + systray.

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
  - `startMitmproxy()` / `stopMitmproxy()` - Process control
  - `enableProxy()` / `disableProxy()` - System proxy via networksetup
  - `getStatus()` - Checks mitmproxy process and proxy state
  - `getActiveNetworkService()` - Detects active network interface

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

- Single-file app - all logic in main.go
- System commands use `exec.Command` with proper error handling
- App runs as a background process in the macOS status menu
