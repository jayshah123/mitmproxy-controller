# Winget Distribution Setup

This document describes how to distribute mitmproxy-controller via Windows Package Manager (winget).

---

## Overview

Winget uses a central community repository (`microsoft/winget-pkgs`) where package manifests are submitted via Pull Request. Once merged, users can install via:

```powershell
winget install Jayshah123.MitmproxyController
```

---

## Architecture

```
jayshah123/mitmproxy-controller          microsoft/winget-pkgs
┌─────────────────────────────┐          ┌──────────────────────────────────┐
│  Source code                │          │  manifests/j/Jayshah123/         │
│  CI/CD workflow             │─────────▶│    MitmproxyController/          │
│  Release artifacts (.zip)   │   PR     │      0.1.0/                      │
└─────────────────────────────┘          │        *.installer.yaml          │
                                         │        *.locale.en-US.yaml       │
                                         │        *.yaml (version)          │
                                         └──────────────────────────────────┘
```

---

## Package Type: ZIP + Portable

Since we distribute a ZIP containing a single `.exe`, we use:
- `InstallerType: zip`
- `NestedInstallerType: portable`

This requires no installer tooling (MSI/MSIX) while still supporting `winget install`.

---

## Manifest Structure

Winget requires a **multi-file manifest** with 3 YAML files:

```
manifests/j/Jayshah123/MitmproxyController/0.1.0/
├── Jayshah123.MitmproxyController.yaml              # Version manifest
├── Jayshah123.MitmproxyController.locale.en-US.yaml # Locale manifest  
└── Jayshah123.MitmproxyController.installer.yaml    # Installer manifest
```

### 1. Version Manifest (`Jayshah123.MitmproxyController.yaml`)

```yaml
PackageIdentifier: Jayshah123.MitmproxyController
PackageVersion: 0.1.0
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.6.0
```

### 2. Locale Manifest (`Jayshah123.MitmproxyController.locale.en-US.yaml`)

```yaml
PackageIdentifier: Jayshah123.MitmproxyController
PackageVersion: 0.1.0
PackageLocale: en-US
Publisher: jayshah123
PublisherUrl: https://github.com/jayshah123
PackageName: mitmproxy-controller
PackageUrl: https://github.com/jayshah123/mitmproxy-controller
License: MIT
LicenseUrl: https://github.com/jayshah123/mitmproxy-controller/blob/main/LICENSE
ShortDescription: System tray controller for mitmproxy
Description: A cross-platform system tray application for controlling mitmproxy and system proxy settings.
Tags:
  - mitmproxy
  - proxy
  - network
  - developer-tools
ManifestType: defaultLocale
ManifestVersion: 1.6.0
```

### 3. Installer Manifest (`Jayshah123.MitmproxyController.installer.yaml`)

```yaml
PackageIdentifier: Jayshah123.MitmproxyController
PackageVersion: 0.1.0
Installers:
  - Architecture: x64
    InstallerType: zip
    NestedInstallerType: portable
    NestedInstallerFiles:
      - RelativeFilePath: mitmproxy-controller_windows_amd64.exe
        PortableCommandAlias: mitmproxy-controller
    InstallerUrl: https://github.com/jayshah123/mitmproxy-controller/releases/download/v0.1.0/mitmproxy-controller_windows_amd64.zip
    InstallerSha256: REPLACE_WITH_ACTUAL_SHA256
ManifestType: installer
ManifestVersion: 1.6.0
```

---

## Initial Submission (One-Time)

### Prerequisites

1. Install winget (comes with Windows 11, or install via Microsoft Store)
2. Install wingetcreate:
   ```powershell
   winget install wingetcreate
   ```

### Step 1: Create a Release

First, create a release in your repo with the Windows artifact:
```bash
git tag v0.1.0
git push origin v0.1.0
```

Wait for CI to create the GitHub Release with `mitmproxy-controller_windows_amd64.zip`.

### Step 2: Generate Manifest with wingetcreate

```powershell
# Generate new manifest interactively
wingetcreate new https://github.com/jayshah123/mitmproxy-controller/releases/download/v0.1.0/mitmproxy-controller_windows_amd64.zip

# Or update existing manifest
wingetcreate update Jayshah123.MitmproxyController --version 0.1.0 --urls https://github.com/jayshah123/mitmproxy-controller/releases/download/v0.1.0/mitmproxy-controller_windows_amd64.zip
```

### Step 3: Validate Manifest Locally

```powershell
winget validate --manifest <path-to-manifest-folder>
```

### Step 4: Test in Windows Sandbox (Recommended)

Clone winget-pkgs and use the sandbox test script:

```powershell
git clone https://github.com/microsoft/winget-pkgs.git
cd winget-pkgs
.\Tools\SandboxTest.ps1 <path-to-manifest-folder>
```

### Step 5: Submit PR

**Option A: Using wingetcreate**
```powershell
wingetcreate submit <path-to-manifest-folder>
```

**Option B: Manual PR**
1. Fork `microsoft/winget-pkgs`
2. Add manifest files to `manifests/j/Jayshah123/MitmproxyController/<version>/`
3. Create PR to `microsoft/winget-pkgs`

### Step 6: Wait for Review

- Automated validation runs (hash check, URL validation, binary scan)
- Microsoft moderators review
- Once merged, package becomes available via `winget search`

---

## Automating Updates (GitHub Actions)

After the first submission is merged, automate future updates.

Prerequisites for automation:
- At least one package version already exists in `microsoft/winget-pkgs`
- A fork of `microsoft/winget-pkgs` exists under `jayshah123/winget-pkgs`

### Create GitHub PAT

1. Go to https://github.com/settings/tokens
2. Generate new token (classic)
3. Select scope: `public_repo`
4. Add as secret `WINGET_TOKEN` in your repo

### Add to CI Workflow

```yaml
name: publish-packages

on:
  release:
    types: [published]

jobs:
  winget:
    name: Update Winget Manifest
    if: secrets.WINGET_TOKEN != '' && !contains(github.event.release.tag_name, '-')
    runs-on: windows-latest
    steps:
      - name: Submit Winget manifest update
        uses: vedantmgoyal9/winget-releaser@v2
        with:
          identifier: Jayshah123.MitmproxyController
          release-tag: ${{ github.event.release.tag_name }}
          installers-regex: mitmproxy-controller_windows_amd64\\.zip$
          token: ${{ secrets.WINGET_TOKEN }}
```

This job runs only when a release is published, skips automatically if `WINGET_TOKEN` is not configured, and ignores prerelease tags (`-beta`, `-rc`).

---

## Validation Checklist

Before submitting, ensure:

- [ ] Manifest files validate: `winget validate --manifest <folder>`
- [ ] SHA256 hash matches the actual artifact
- [ ] InstallerUrl points to GitHub Release asset (stable, direct URL)
- [ ] Version follows SemVer (e.g., `1.2.3`)
- [ ] All required metadata fields are filled
- [ ] Tested in Windows Sandbox

---

## Computing SHA256

```powershell
# PowerShell
(Get-FileHash -Algorithm SHA256 .\mitmproxy-controller_windows_amd64.zip).Hash

# Or using certutil
certutil -hashfile mitmproxy-controller_windows_amd64.zip SHA256
```

---

## Common Validation Errors

| Error | Cause | Fix |
|-------|-------|-----|
| Hash mismatch | ZIP was re-uploaded after manifest created | Recompute SHA256 |
| URL validation failed | Asset not publicly accessible | Ensure release is public |
| Binary validation failed | SmartScreen/reputation issue | May need Microsoft review |
| Manifest path error | Wrong folder structure | Check `manifests/j/Jayshah123/MitmproxyController/<version>/` |

---

## Portable ZIP Limitations

Using `InstallerType: zip` + `NestedInstallerType: portable`:

| Feature | Supported |
|---------|-----------|
| `winget install` | ✅ |
| `winget upgrade` | ✅ |
| `winget uninstall` | ✅ |
| Start Menu shortcut | ❌ |
| Add/Remove Programs entry | Limited |
| Auto-start on login | ❌ (manual setup required) |

For full Windows integration, consider MSI/MSIX installer in the future.

---

## Future: MSI/MSIX Installer

If you later need:
- Start Menu shortcuts
- Add/Remove Programs entries
- Auto-start configuration
- Enterprise deployment

Consider:
- **Inno Setup**: Easy EXE installer creation
- **WiX Toolset**: MSI creation
- **MSIX**: Modern Windows packaging (requires code signing)

---

## References

- [Winget Package Submission Guide](https://learn.microsoft.com/en-us/windows/package-manager/package/)
- [Manifest Schema](https://learn.microsoft.com/en-us/windows/package-manager/package/manifest)
- [winget-pkgs Repository](https://github.com/microsoft/winget-pkgs)
- [wingetcreate Tool](https://github.com/microsoft/winget-create)
- [Submission Policies](https://learn.microsoft.com/en-us/windows/package-manager/package/windows-package-manager-policies)
