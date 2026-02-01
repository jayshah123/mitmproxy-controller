# Future Plans

## Run as Background Service / Auto-Start

Currently the app blocks the terminal when run. Here's how to run it as a background auto-start process.

### Key Insight

Systray apps need GUI session access, so traditional "services/daemons" won't work—they run in isolated sessions without UI access.

---

### macOS: LaunchAgent (not LaunchDaemon)

1. Build and install binary to a stable location (e.g., `/usr/local/bin/mitmproxy-controller`)

2. Create `~/Library/LaunchAgents/com.yourorg.mitmproxy-controller.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.yourorg.mitmproxy-controller</string>

  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/mitmproxy-controller</string>
  </array>

  <key>RunAtLoad</key>
  <true/>

  <key>KeepAlive</key>
  <true/>

  <!-- Required for tray apps: must be in GUI session -->
  <key>LimitLoadToSessionType</key>
  <string>Aqua</string>

  <key>StandardOutPath</key>
  <string>/tmp/mitmproxy-controller.out.log</string>
  <key>StandardErrorPath</key>
  <string>/tmp/mitmproxy-controller.err.log</string>
</dict>
</plist>
```

3. Load and start:
```bash
launchctl load ~/Library/LaunchAgents/com.yourorg.mitmproxy-controller.plist
launchctl start com.yourorg.mitmproxy-controller
```

4. Verify:
```bash
launchctl list | grep mitmproxy-controller
```

---

### Windows: Startup Folder or Task Scheduler

#### Option A: Startup Folder (simplest)

1. Build without console window:
   ```bash
   go build -ldflags="-H=windowsgui" -o mitmproxy-controller.exe
   ```

2. Create shortcut in startup folder:
   - Win+R → `shell:startup`
   - Drop a shortcut to the exe

#### Option B: Task Scheduler (more robust)

1. Create a scheduled task with:
   - Trigger: **At log on**
   - Action: Start your exe
   - Check: "Run only when user is logged on" (required for tray UI)
   - Optional: "Run with highest privileges" if proxy toggling needs admin

---

### Why Not a True Service?

- **Windows Services** run in Session 0—no tray icon visible to users
- **macOS LaunchDaemons** have no GUI access

---

### Advanced: Split Architecture (if needed later)

If you need privileged operations without prompts, split into:

1. **UI tray app** (LaunchAgent / Startup) - current app
2. **Privileged helper service**:
   - macOS: LaunchDaemon (root) with IPC via local socket
   - Windows: Windows Service with IPC via named pipe

The UI calls the helper for privileged actions like "start/stop mitmproxy" or "toggle system proxy".

**Effort estimate:** 1-3 hours to 1-2 days depending on complexity.

---

## Homebrew Distribution

### Setup a Tap Repository

1. Create repo: `github.com/<you>/homebrew-tap`
2. Add `Formula/mitmproxy-controller.rb`:

```ruby
class MitmproxyController < Formula
  desc "System tray controller for mitmproxy"
  homepage "https://github.com/jayshah123/mitmproxy-controller"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jayshah123/mitmproxy-controller/releases/download/v0.1.0/mitmproxy-controller_darwin_arm64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA256"
    else
      url "https://github.com/jayshah123/mitmproxy-controller/releases/download/v0.1.0/mitmproxy-controller_darwin_amd64.tar.gz"
      sha256 "REPLACE_WITH_ACTUAL_SHA256"
    end
  end

  def install
    bin.install Dir["mitmproxy-controller*"].first => "mitmproxy-controller"
  end

  test do
    system "#{bin}/mitmproxy-controller", "--help"
  end
end
```

3. Users install via:
```bash
brew tap jayshah123/tap
brew install mitmproxy-controller
```

### Compute SHA256 for releases
```bash
shasum -a 256 mitmproxy-controller_darwin_arm64.tar.gz
```

### Future: macOS .app Bundle
If you ship a `.app` bundle later, use a **Homebrew Cask** instead of a formula for proper `/Applications` install.

---

## winget Distribution (Windows)

### Option A: Portable Package (simplest)

1. Ensure stable release URL exists, e.g.:
   `https://github.com/jayshah123/mitmproxy-controller/releases/download/v0.1.0/mitmproxy-controller_windows_amd64.zip`

2. Install wingetcreate:
   ```powershell
   winget install wingetcreate
   ```

3. Generate manifest:
   ```powershell
   wingetcreate new https://github.com/jayshah123/mitmproxy-controller/releases/download/v0.1.0/mitmproxy-controller_windows_amd64.zip
   ```

4. Edit manifest fields:
   - `PackageIdentifier`: `Jayshah123.MitmproxyController`
   - `PackageVersion`: `0.1.0`
   - `InstallerType`: `portable`
   - `Commands`: `mitmproxy-controller`

5. Submit PR to: https://github.com/microsoft/winget-pkgs

### Option B: MSI/EXE Installer (better UX)

Use Inno Setup or WiX to create a proper installer with:
- Start Menu shortcuts
- Uninstall support
- Optional auto-start configuration

---

## Code Signing (Future Polish)

### macOS
- Sign with Apple Developer ID
- Notarize with `xcrun notarytool`
- Eliminates Gatekeeper warnings

### Windows
- Sign with code signing certificate
- Eliminates SmartScreen warnings
- Required for enterprise deployment
