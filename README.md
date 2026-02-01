# mitmproxy-controller

A cross-platform **system tray** app for controlling [mitmproxy](https://mitmproxy.org/) and system proxy settings. Works on macOS (status menu) and Windows (system tray).

## Features

- **Start/Stop mitmproxy** - Launch or kill the mitmdump process (headless mode)
- **Enable/Disable System Proxy** - Configure system proxy to route traffic through mitmproxy (127.0.0.1:8899)
- **Smart Menu Items** - Actions are disabled when not applicable (e.g., can't start if already running)
- **Auto-Refresh** - Status updates every 5 seconds via background polling
- **Manual Refresh** - "Refresh Status" menu item for immediate update
- **Status Indicator** - Tray icon shows current state:
  - 🟢 mitmproxy running + proxy enabled
  - 🟡 mitmproxy running + proxy disabled
  - 🟠 mitmproxy stopped + proxy enabled
  - ⚫ both off

## Prerequisites

- **macOS** or **Windows**
- [Go 1.23+](https://go.dev/dl/)
- [mitmproxy](https://mitmproxy.org/) installed and available in PATH

## Installation

```bash
# macOS
brew install mitmproxy

# Windows (using winget)
winget install -e --id mitmproxy.mitmproxy
```

## Build

```bash
go build -o mitmproxy-controller
```

## Run

```bash
# macOS
./mitmproxy-controller

# Windows
mitmproxy-controller.exe
```

The app runs in the system tray (macOS: top-right, Windows: bottom-right).

## Folder Layout

```
mitmproxy-controller/
├── main.go              # Shared systray UI and menu handling
├── mitm.go              # Shared mitmproxy process control
├── mitm_darwin.go       # macOS-specific process utilities
├── mitm_windows.go      # Windows-specific process utilities
├── proxy_darwin.go      # macOS proxy config (networksetup)
├── proxy_windows.go     # Windows proxy config (registry)
├── go.mod               # Go module definition
├── go.sum               # Go dependencies lock
└── README.md
```

## How It Works

- Uses **mitmdump** (headless) instead of mitmproxy (TUI) for background operation
- Listens on port **8899** (avoids conflict with common services on 8080)
- Uses Go build tags for platform-specific code

### macOS
- Auto-detects active network interface via `route get default` → maps to service name
- Sets both HTTP and HTTPS proxy via `networksetup`

### Windows
- Configures proxy via Windows Registry (`HKCU\...\Internet Settings`)
- Calls WinINet API to notify applications of proxy changes
- App appears in the system tray (bottom-right)

## Notes

- **macOS**: Proxy configuration uses `networksetup` which may require admin privileges
- **Windows**: No admin required for per-user proxy settings
- Visit `mitm.it` in browser to verify traffic is routing through mitmproxy and install certificates

## Cross-Compilation

```bash
# Build for Windows (x64) from macOS/Linux
GOOS=windows GOARCH=amd64 go build -o mitmproxy-controller.exe

# Build for macOS (Apple Silicon) from Windows/Linux
GOOS=darwin GOARCH=arm64 go build -o mitmproxy-controller
```

## License

MIT
