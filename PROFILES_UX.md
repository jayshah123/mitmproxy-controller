# Service Profiles UX

This document explains how service profiles work in `mitmproxy-controller`.

## Quick Mental Model

1. `~/.mitmproxy/config.yaml` is your global mitmproxy base config.
2. A **service profile** is a named overlay:
   - addon scripts (`-s ...`)
   - extra mitmproxy options (`--set key=value`)
3. Selecting a profile in tray applies that overlay when mitmproxy starts.
4. If mitmproxy is already running, selecting a different profile restarts it.

## Where Profiles Are Saved

Profiles are stored in the app config directory (not in repo by default):

1. macOS:
   - `~/Library/Application Support/mitmproxy-controller/profiles/`
   - `~/Library/Application Support/mitmproxy-controller/state.json`
2. Windows:
   - `%APPDATA%\mitmproxy-controller\profiles\`
   - `%APPDATA%\mitmproxy-controller\state.json`

Notes:

1. One profile file per service (`.yaml` or `.yml`).
2. `state.json` stores `selected_profile_id`.
3. The app auto-creates `profiles/default.yaml` on first run.

## Recommended Folder Layout

```text
~/.mitmproxy/
  config.yaml

~/Library/Application Support/mitmproxy-controller/
  state.json
  profiles/
    default.yaml
    stripe.yaml
    slack.yaml
```

You can keep scripts anywhere. Relative script paths in a profile are resolved relative to that profile file's directory.

## Profile Schema (YAML)

```yaml
id: stripe
name: Stripe
scripts:
  - ../../work/mitm-scripts/stripe/auth_rewrite.py
  - ../../work/mitm-scripts/common/log_errors.py
set_options:
  ignore_hosts: "^ocsp\\..*"
  block_global: "false"
mode: regular
```

Fields:

1. `id` (required) unique stable identifier.
2. `name` (optional) UI label. Defaults to `id`.
3. `scripts` (optional) list of addon script paths.
4. `set_options` (optional) map of mitmproxy options passed as `--set key=value`.
5. `mode` (optional) passed as `--mode`.

## How Command Assembly Works

When starting mitmproxy, controller builds command args in this order:

1. Base options (including `confdir=~/.mitmproxy` and default listen/web settings).
2. Selected profile scripts (`-s <absolute path>`).
3. Selected profile `set_options` (`--set key=value`).
4. Capture log output (`-w <path>`).

This means profile options can intentionally override base options.

## CLI vs config.yaml Precedence

`config.yaml` is loaded first, CLI args are applied after.  
So profile overlays provided through CLI are authoritative when keys overlap.

## Tray UX

1. `Service Profile: <Name>` submenu lets you pick active profile.
2. `Edit Active Profile` opens the active profile YAML file.
3. `Open Active Scripts Folder` opens:
   - folder of first script (if scripts exist), else
   - profile file folder.
4. `Edit mitmproxy Config` still opens `~/.mitmproxy/config.yaml`.

## What Happens When You Edit a Profile

1. Save changes in profile YAML.
2. If mitmproxy is stopped, changes apply on next Start.
3. If you switch profiles while running, controller does stop+start immediately.
4. If scripts are missing, start fails with a clear status message.

## Compatibility Warnings

This app's proxy/web actions assume:

1. `listen_host=127.0.0.1`
2. `listen_port=8899`
3. `web_host=127.0.0.1`
4. `web_port=8898`
5. `web_password=mitmcontroller`

If active profile overrides these, the controller shows warnings and disables incompatible actions:

1. Proxy toggles disabled for listen host/port mismatch.
2. Web UI action disabled for web host/port/password mismatch.

## Example Profiles

`default.yaml`

```yaml
id: default
name: Default
scripts: []
set_options: {}
```

`stripe.yaml`

```yaml
id: stripe
name: Stripe Sandbox
scripts:
  - ../scripts/stripe/auth_rewrite.py
  - ../scripts/stripe/webhook_log.py
set_options:
  ignore_hosts: "^ocsp\\..*"
```
