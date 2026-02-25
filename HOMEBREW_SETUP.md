# Homebrew Distribution Setup

This document describes how to set up and maintain Homebrew distribution for mitmproxy-controller.

---

## Overview

Homebrew uses "taps" (third-party repositories) to distribute formulas not in the official homebrew-core. Our tap repository is `jayshah123/homebrew-tap`.

**User installation command:**
```bash
brew tap jayshah123/tap
brew install mitmproxy-controller
```

---

## Architecture

```
jayshah123/mitmproxy-controller     jayshah123/homebrew-tap
┌─────────────────────────────┐     ┌─────────────────────────┐
│  Source code                │     │  Formula/               │
│  publish-packages workflow  │────▶│    mitmproxy-controller.rb
│  Release artifacts          │     │  README.md              │
└─────────────────────────────┘     └─────────────────────────┘
        │                                     ▲
        │  On GitHub release publish           │
        └─────────────────────────────────────┘
              Auto-updates formula
```

---

## Initial Setup (One-Time)

### Step 1: Create the Tap Repository

1. Go to https://github.com/new
2. Repository name: `homebrew-tap` (the `homebrew-` prefix is required)
3. Set to **Public** (required for Homebrew)
4. Initialize with a README
5. Create repository

### Step 2: Add Formula Directory Structure

Clone the tap repo and add the formula:

```bash
git clone https://github.com/jayshah123/homebrew-tap.git
cd homebrew-tap
mkdir -p Formula
```

Create `Formula/mitmproxy-controller.rb`:

```ruby
class MitmproxyController < Formula
  desc "System tray controller for mitmproxy"
  homepage "https://github.com/jayshah123/mitmproxy-controller"
  version "0.0.1"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/jayshah123/mitmproxy-controller/releases/download/v0.0.1/mitmproxy-controller_darwin_arm64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  def install
    bin.install "mitmproxy-controller_darwin_arm64" => "mitmproxy-controller"
  end

  test do
    assert_predicate bin/"mitmproxy-controller", :exist?
  end
end
```

Commit and push:

```bash
git add .
git commit -m "Add mitmproxy-controller formula"
git push
```

### Step 3: Create Personal Access Token

1. Go to https://github.com/settings/tokens
2. Click **"Generate new token (classic)"**
3. Name: `homebrew-tap-access`
4. Expiration: Set as needed (or "No expiration" for convenience)
5. Select scopes:
   - ✅ `repo` (full control of private repositories)
   - ✅ `workflow` (update GitHub Action workflows)
6. Click **"Generate token"**
7. **Copy the token immediately** (you won't see it again)

### Step 4: Add Token to Main Repository

1. Go to https://github.com/jayshah123/mitmproxy-controller/settings/secrets/actions
2. Click **"New repository secret"**
3. Name: `HOMEBREW_TAP_TOKEN`
4. Value: Paste the token from Step 3
5. Click **"Add secret"**

---

## How It Works

When you push a version tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The release pipeline:
1. `ci.yml` builds binaries for macOS ARM64 and Windows
2. `ci.yml` creates a GitHub Release with artifacts
3. `publish-packages.yml` **automatically updates** `Formula/mitmproxy-controller.rb` in the tap repo:
   - Updates `version` to match the tag
   - Updates `url` to point to new release artifact
   - Updates `sha256` checksum

Note: Homebrew publishing runs only for stable tags (no `-beta` / `-rc` suffix).

---

## Manual Formula Update (if needed)

If auto-update fails, manually update the formula:

```bash
# Get the SHA256 of the release artifact
curl -sL https://github.com/jayshah123/mitmproxy-controller/releases/download/v0.1.0/mitmproxy-controller_darwin_arm64.tar.gz | shasum -a 256

# Update Formula/mitmproxy-controller.rb with new version, url, and sha256
```

---

## Testing the Formula Locally

```bash
# Install from local tap
brew tap jayshah123/tap
brew install mitmproxy-controller

# Or test formula directly
brew install --build-from-source ./Formula/mitmproxy-controller.rb

# Uninstall
brew uninstall mitmproxy-controller
brew untap jayshah123/tap
```

---

## Troubleshooting

### "Token doesn't have required permissions"
- Ensure PAT has `repo` and `workflow` scopes
- Regenerate token if needed

### "Formula syntax error"
- Run `brew audit --strict Formula/mitmproxy-controller.rb`
- Run `brew style Formula/mitmproxy-controller.rb`

### "SHA256 mismatch"
- The auto-updater calculates SHA256 automatically
- If manual update needed, re-download artifact and recalculate

---

## Files Reference

| File | Location | Purpose |
|------|----------|---------|
| Publish Workflow | `.github/workflows/publish-packages.yml` | Triggers formula update on release publish |
| Formula | `jayshah123/homebrew-tap/Formula/mitmproxy-controller.rb` | Homebrew formula definition |
| Token Secret | Repository Settings → Secrets | `HOMEBREW_TAP_TOKEN` for tap access |
