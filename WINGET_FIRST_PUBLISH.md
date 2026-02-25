# Winget First Publish Runbook

This is the exact command sequence for the first Winget submission from Windows.

## What to Run Now on Windows (First-Time Publish)

1. Install WingetCreate:

```powershell
winget install wingetcreate
```

2. Create a new manifest from the released installer asset:

```powershell
wingetcreate new https://github.com/jayshah123/mitmproxy-controller/releases/download/v0.1.2/mitmproxy-controller_windows_amd64.zip
```

3. In prompts use:

- `PackageIdentifier`: `Jayshah123.MitmproxyController`
- `PackageVersion`: `0.1.2`

4. Validate the generated manifest:

```powershell
winget validate --manifest <path-to-generated-manifest-folder>
```

5. Submit manifest PR to `microsoft/winget-pkgs`:

```powershell
wingetcreate submit --token <CLASSIC_PAT_WITH_public_repo> <path-to-generated-manifest-folder>
```

## After PR Merge

1. Create the next release (for example `v0.1.3`).
2. Let `.github/workflows/publish-packages.yml` auto-submit Winget updates.
3. Verify install:

```powershell
winget source update
winget search Jayshah123.MitmproxyController
winget install -e --id Jayshah123.MitmproxyController
```
