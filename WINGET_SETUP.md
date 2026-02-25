# Winget Distribution Setup

This document defines the publishing model for `mitmproxy-controller` on Windows Package Manager (`winget`).

Quick runbook: [WINGET_FIRST_PUBLISH.md](WINGET_FIRST_PUBLISH.md)

## Publishing Model (First Principles)

Winget is manifest-based.

- We do **not** upload binaries to Microsoft.
- We publish installer artifacts on GitHub Releases.
- We submit YAML manifests to `microsoft/winget-pkgs` that point to those public artifacts.
- Winget clients download directly from the release URL and verify SHA256.

Implications:

- Installer URLs must be public and stable.
- Release artifacts must be immutable once published.
- First package submission is a PR to `microsoft/winget-pkgs`.

## Package Contract for This Repo

- Package identifier: `Jayshah123.MitmproxyController`
- Installer type: `zip` with nested `portable`
- Windows asset name: `mitmproxy-controller_windows_amd64.zip`
- Nested binary path inside ZIP: `mitmproxy-controller_windows_amd64.exe`
- Release URL pattern:
  - `https://github.com/jayshah123/mitmproxy-controller/releases/download/vX.Y.Z/mitmproxy-controller_windows_amd64.zip`

## Canonical First Submission (Recommended)

Use `wingetcreate` on a Windows machine for the first submission.

## First Submission Checklist (Do This Now)

Run this on a Windows machine, in order:

1. Ensure release `v0.1.2` exists with `mitmproxy-controller_windows_amd64.zip`.
2. Create a classic GitHub PAT with `public_repo`.
3. Start/refresh enterprise SSO session in browser.
4. Install WingetCreate.
5. Generate manifest with `wingetcreate new <release-asset-url>`.
6. Validate with `winget validate --manifest <folder>`.
7. Submit with `wingetcreate submit --token <PAT> <folder>`.
8. Wait for merge in `microsoft/winget-pkgs`.
9. Trigger next release; GitHub Action will auto-submit update PRs.

### 1. Ensure release artifact exists

Create and publish a stable GitHub release first (example: `v0.1.2`) and verify the Windows ZIP is attached.

### 2. Install WingetCreate

```powershell
winget install wingetcreate
```

### 3. Create the manifest (new package)

```powershell
wingetcreate new https://github.com/jayshah123/mitmproxy-controller/releases/download/v0.1.2/mitmproxy-controller_windows_amd64.zip
```

Use these key values in prompts:

- `PackageIdentifier`: `Jayshah123.MitmproxyController`
- `PackageVersion`: `0.1.2` (no leading `v`)
- `InstallerType`: `zip` (auto-detected)

### 4. Validate locally

```powershell
winget validate --manifest <path-to-generated-manifest-folder>
```

Optional but recommended:

```powershell
winget settings --enable LocalManifestFiles
winget install --manifest <path-to-generated-manifest-folder>
```

### 5. Submit PR to winget-pkgs

```powershell
wingetcreate submit --token <CLASSIC_PAT_WITH_public_repo> <path-to-generated-manifest-folder>
```

Expected result: a PR is opened against `microsoft/winget-pkgs` for one package version.

## Auth and 404 Troubleshooting (Important)

Winget submission commonly fails due to GitHub org SSO/SAML authorization.

- Use a **classic PAT** with `public_repo` scope.
- If your org enforces SSO/SAML, authorize that token for org access.
- If browser links from old failures return `404`, regenerate a fresh auth flow by rerunning the command (`wingetcreate submit` or `gh repo fork ...`).
- Verify auth status:

```bash
gh auth status -h github.com
```

If needed, reauthorize in GitHub:

- `Settings -> Developer settings -> Personal access tokens (classic)`
- Open your token and use **Configure SSO** (if shown)

## After First Merge: Automated Updates

After the initial package is merged into `microsoft/winget-pkgs`, this repo uses CI automation for updates.

Workflow:

- `.github/workflows/publish-packages.yml`
- Job: `winget`
- Action: `vedantmgoyal9/winget-releaser@v2`

Required repo secret:

- `WINGET_TOKEN` (classic PAT with `public_repo`)

Behavior:

- Runs on release events for stable tags.
- Skips prerelease tags (contains `-`).
- Skips until package bootstrap exists in `winget-pkgs`.

## Legacy Fallback (Not Primary)

If `wingetcreate` is unavailable, the repo helper script can still be used:

```bash
./scripts/bootstrap-winget.sh --owner jayshah123 --version 0.1.2
```

What it does:

- Reads release asset metadata and SHA256.
- Creates the 3 Winget manifest files in winget-pkgs layout.
- Pushes a branch to your `winget-pkgs` fork.
- Opens a PR to `microsoft/winget-pkgs`.

Treat this as fallback/advanced only. The default path for this repo is WingetCreate.

## Manifest Layout Reference

Winget manifests are stored under:

```text
manifests/j/Jayshah123/MitmproxyController/<version>/
  Jayshah123.MitmproxyController.yaml
  Jayshah123.MitmproxyController.locale.en-US.yaml
  Jayshah123.MitmproxyController.installer.yaml
```

## Update Command (Manual Alternative)

For later versions, if not using CI automation:

```powershell
wingetcreate update Jayshah123.MitmproxyController --version 0.1.3 --urls https://github.com/jayshah123/mitmproxy-controller/releases/download/v0.1.3/mitmproxy-controller_windows_amd64.zip --submit --token <CLASSIC_PAT_WITH_public_repo>
```

## References

- https://learn.microsoft.com/en-us/windows/package-manager/package/
- https://learn.microsoft.com/en-us/windows/package-manager/package/manifest
- https://github.com/microsoft/winget-pkgs
- https://github.com/microsoft/winget-create
