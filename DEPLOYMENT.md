# Deployment & Release Guide

This document describes the CI/CD pipeline, build process, and release workflow.

---

## CI/CD Overview

The project uses GitHub Actions for automated builds and releases.

**Workflow file:** `.github/workflows/ci.yml`

---

## Build Triggers

| Trigger | What Happens |
|---------|--------------|
| Push to any branch | Build all platforms, upload as workflow artifacts |
| Pull request | Build all platforms, upload as workflow artifacts |
| Push tag `v*` (e.g., `v0.1.0`) | Build all platforms + create GitHub Release with artifacts |

---

## Build Matrix

Builds run on **native runners** (not cross-compiled) because `getlantern/systray` requires CGO.

| Runner | OS | Architecture | Artifact Name |
|--------|-----|--------------|---------------|
| `macos-latest` | macOS | arm64 (Apple Silicon) | `mitmproxy-controller_darwin_arm64` |
| `windows-latest` | Windows | amd64 | `mitmproxy-controller_windows_amd64` |

---

## Build Configuration

### Compiler Flags

| Platform | Flags | Purpose |
|----------|-------|---------|
| All | `-trimpath` | Reproducible builds, removes local paths |
| All | `-ldflags "-s -w"` | Strip debug info, reduce binary size |
| Windows | `-ldflags "-H=windowsgui"` | Build as GUI app (no console window) |

### Environment

| Variable | Value | Purpose |
|----------|-------|---------|
| `CGO_ENABLED` | `1` | Required for systray library |
| `GOOS` | `darwin` / `windows` | Target OS |
| `GOARCH` | `amd64` / `arm64` | Target architecture |

---

## Artifacts

### Workflow Artifacts (every build)

Available in GitHub Actions → Run → Artifacts section for 90 days.

| Artifact | Contents |
|----------|----------|
| `mitmproxy-controller_darwin_arm64` | `mitmproxy-controller_darwin_arm64.tar.gz` |
| `mitmproxy-controller_windows_amd64` | `mitmproxy-controller_windows_amd64.zip` |

### Release Artifacts (on tags)

Attached to GitHub Releases page.

| File | Platform | Extract Command |
|------|----------|-----------------|
| `mitmproxy-controller_darwin_arm64.tar.gz` | macOS Apple Silicon | `tar -xzf <file>` |
| `mitmproxy-controller_windows_amd64.zip` | Windows x64 | Extract with Explorer or `7z x <file>` |

---

## Creating a Release

### 1. Tag the commit

```bash
# Ensure you're on the commit you want to release
git tag v0.1.0
git push origin v0.1.0
```

### 2. Wait for CI

The workflow will:
1. Build all 3 platform binaries
2. Package them (tar.gz for macOS, zip for Windows)
3. Create a GitHub Release with auto-generated release notes
4. Attach all artifacts to the release

### 3. Verify

- Go to **Releases** page on GitHub
- Confirm all 3 artifacts are attached
- Edit release notes if needed

---

## Version Tagging Convention

| Tag Format | Example | Description |
|------------|---------|-------------|
| `v<major>.<minor>.<patch>` | `v1.2.3` | Stable release |
| `v<major>.<minor>.<patch>-beta.<n>` | `v1.0.0-beta.1` | Pre-release |
| `v<major>.<minor>.<patch>-rc.<n>` | `v1.0.0-rc.1` | Release candidate |

---

## Manual Local Build

If you need to build locally:

### macOS
```bash
CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o mitmproxy-controller .
```

### Windows (from Windows machine)
```powershell
$env:CGO_ENABLED=1
go build -trimpath -ldflags "-s -w -H=windowsgui" -o mitmproxy-controller.exe .
```

### Windows (cross-compile from macOS/Linux)
**Not recommended** due to CGO dependencies. Use the CI workflow instead.

---

## Post-Release Checklist

- [ ] Verify release page has all 3 artifacts
- [ ] Download and test each binary
- [ ] Update Homebrew formula (if using tap) with new version and SHA256
- [ ] Update winget manifest (if published) with new version
- [ ] Announce release if applicable

---

## Troubleshooting

### Build fails with CGO errors
- Ensure you're building on a native runner (not cross-compiling)
- Check that Xcode Command Line Tools are installed (macOS)

### Windows binary shows console window
- Ensure `-H=windowsgui` is in ldflags
- Rebuild with correct flags

### Release not created
- Verify the tag matches pattern `v*` (e.g., `v0.1.0`)
- Check workflow permissions include `contents: write`

### Artifacts missing from release
- Check the `release` job completed successfully
- Verify `needs: [build]` ensures all builds finished first
