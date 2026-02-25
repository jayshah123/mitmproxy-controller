# Deployment & Release Guide

This document describes the CI/CD pipeline, build process, and release workflow.

---

## CI/CD Overview

The project uses GitHub Actions for automated builds, releases, and package manager publishing.

**Workflow files:**
- `.github/workflows/ci.yml` (build + GitHub Release artifacts)
- `.github/workflows/publish-packages.yml` (Homebrew + Winget publishing)
- `.github/workflows/auto-version-tag.yml` (automatic semver tagging from conventional commits)
- `.github/workflows/commit-lint.yml` (enforces conventional commit messages)

---

## Build Triggers

| Trigger | What Happens |
|---------|--------------|
| Push to any branch | Build all platforms, upload as workflow artifacts |
| Push to any branch | `commit-lint.yml` validates commit messages |
| Pull request | Build all platforms, upload as workflow artifacts |
| Pull request | `commit-lint.yml` validates commits in PR |
| Push to `main` | `auto-version-tag.yml` may create next semver tag |
| Push tag `v*` (e.g., `v0.1.0`) | Build all platforms + create GitHub Release with artifacts |
| Release published/released | Publish package updates to Homebrew/Winget for stable tags (if secrets configured) |

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
1. Build 2 platform binaries
2. Package them (tar.gz for macOS, zip for Windows)
3. Create a GitHub Release with auto-generated release notes
4. Attach all artifacts to the release
5. Trigger package publishing workflow on release publish event

### 3. Verify

- Go to **Releases** page on GitHub
- Confirm both artifacts are attached
- Edit release notes if needed

### 4. Verify package publishing

- Check **Actions → publish-packages** run for the same release tag
- Confirm Homebrew tap formula PR/commit was created
- Confirm Winget PR was submitted to `microsoft/winget-pkgs`

---

## Required Secrets

Configure these in `Repository Settings → Secrets and variables → Actions`:

| Secret | Required For | Notes |
|--------|--------------|-------|
| `RELEASE_TAG_TOKEN` | Auto version tagging | Classic PAT (`repo`, `workflow`) used to push tags that trigger release workflows |
| `HOMEBREW_TAP_TOKEN` | Homebrew publishing | PAT with access to `jayshah123/homebrew-tap` |
| `WINGET_TOKEN` | Winget publishing | GitHub token used by winget releaser to submit PRs |

If a secret is missing, that package job is skipped.
If the tag contains a suffix like `-beta` or `-rc`, package jobs are skipped.
Winget updates are also skipped until the initial package bootstrap exists in `microsoft/winget-pkgs`.

---

## Version Tagging Convention

| Tag Format | Example | Description |
|------------|---------|-------------|
| `v<major>.<minor>.<patch>` | `v1.2.3` | Stable release |
| `v<major>.<minor>.<patch>-beta.<n>` | `v1.0.0-beta.1` | Pre-release |
| `v<major>.<minor>.<patch>-rc.<n>` | `v1.0.0-rc.1` | Release candidate |

---

## Automatic Version Tagging

Workflow: `.github/workflows/auto-version-tag.yml`

- Runs on pushes to `main`
- Uses conventional commits since the last `v*` tag
- Creates and pushes one new annotated semver tag when needed
- Uses `RELEASE_TAG_TOKEN` so pushed tags trigger `build-and-release`

Auto bump rules:

| Commit Pattern | Version Bump |
|----------------|--------------|
| `BREAKING CHANGE` or `type(scope)!:` | major |
| `feat:` | minor |
| `fix:` or `perf:` | patch |
| No matching commits | no tag |

You can also run it manually from Actions with a forced bump (`patch`/`minor`/`major`).

---

## Conventional Commit Enforcement

Workflow: `.github/workflows/commit-lint.yml`

- Runs on push and pull request
- Fails CI for non-conventional commit subjects
- Allowed merge/revert auto-generated Git messages are exempted

For local enforcement before commit:

```bash
./scripts/setup-git-hooks.sh
```

Hook/lint scripts:
- `.githooks/prepare-commit-msg`
- `.githooks/commit-msg`
- `scripts/lint-commit-msg.sh`

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

- [ ] Verify release page has both artifacts
- [ ] Download and test each binary
- [ ] Confirm Homebrew workflow updated formula (or update manually if needed)
- [ ] Confirm Winget workflow submitted update PR (or submit manually if needed)
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

### Homebrew or Winget job did not run
- Ensure `publish-packages.yml` ran on the `release` event (`published`/`released`)
- Verify corresponding secret exists (`HOMEBREW_TAP_TOKEN` / `WINGET_TOKEN`)
