# Config Profiles Design

Decision record for adding mitmproxy custom config profile support to the tray controller.

## Problem Statement and Goals

The controller currently starts `mitmweb` or `mitmdump` with fixed arguments and no profile selection support. We need to support mitmproxy configuration profiles while preserving the existing simple tray experience and avoiding behavior regressions in proxy toggle and Web UI actions.

Goals:

1. Support selecting a mitmproxy config profile based on `confdir`.
2. Surface profile selection in tray UI using nested submenu entries.
3. Persist selected profile across app restarts.
4. Apply profile changes immediately when mitmproxy is already running.
5. Keep behavior predictable when profile endpoints conflict with controller assumptions.

## Scope and Non-Goals

In scope:

1. Confdir-based profile identity and selection semantics.
2. Profile discovery convention and persistence contract.
3. Runtime compatibility checks and action gating policy.
4. UX behavior for status, warnings, and profile switching.

Out of scope:

1. Implementing runtime code changes in this document.
2. Arbitrary file-path profile support (non-confdir selection).
3. Automatic adaptation of system proxy/Web UI actions to arbitrary profile endpoints.
4. Dynamic profile editing UI.

## Official Mitmproxy Behavior (Source-Verified)

### `config.yaml` in `confdir`

mitmproxy uses a configuration directory (`confdir`) and reads `config.yaml` from there. The options system exposes `confdir` as an option with default `~/.mitmproxy`.

- Docs: [Mitmproxy Options](https://docs.mitmproxy.org/stable/concepts/options/)
- Source: [mitmproxy/options.py](https://github.com/mitmproxy/mitmproxy/blob/main/mitmproxy/options.py)

### `--set confdir=...` usage

Current mitmdump/mitmweb CLI does not expose `--confdir` as a dedicated top-level flag. `confdir` is set via option injection: `--set confdir=<path>`.

- Docs: [Mitmproxy Options](https://docs.mitmproxy.org/stable/concepts/options/)
- Source: [mitmproxy/tools/main.py](https://github.com/mitmproxy/mitmproxy/blob/main/mitmproxy/tools/main.py)
- Local verification command:
  - `mitmdump --confdir /tmp` => unrecognized argument
  - `mitmdump --set confdir=/tmp --options` => valid invocation

### CLI vs config precedence

mitmproxy option loading applies config-based values and then applies CLI-set values, making explicit CLI values authoritative.

- Source: [mitmproxy/optmanager.py](https://github.com/mitmproxy/mitmproxy/blob/main/mitmproxy/optmanager.py)
- Source: [mitmproxy/tools/main.py](https://github.com/mitmproxy/mitmproxy/blob/main/mitmproxy/tools/main.py)

## Decision Log (Locked)

### D1: Confdir-only profiles (not raw file paths)

1. Decision
   Use `confdir` as the profile identity. Each profile is a directory that may contain `config.yaml`.
2. Status
   Accepted.
3. Context
   mitmproxy natively resolves config through `confdir/config.yaml`.
   Locked default (user-selected): `confdir` only.
4. Alternatives considered
   - Arbitrary YAML file path only.
   - Support both confdir and file-path entries.
5. Rationale
   Confdir-only aligns with mitmproxy’s native model and avoids adapter/copy logic.
6. Consequences
   Users structure profiles as directories. Simpler validation and startup args.
7. Verification notes
   Confirmed via mitmproxy docs/options and local CLI behavior.

### D2: Discovery locations = default confdir + `profiles/*`

1. Decision
   Auto-discover profiles from:
   - Default confdir (`~/.mitmproxy` on macOS/Linux, `%USERPROFILE%\mitmproxy` on Windows).
   - One-level child directories under `<default_confdir>/profiles/*` that contain `config.yaml`.
2. Status
   Accepted.
3. Context
   Need sensible convention without requiring manual list configuration.
   Locked default (user-selected): default confdir + `profiles/*`.
4. Alternatives considered
   - Only default confdir.
   - App-managed directory only under `os.UserConfigDir()`.
5. Rationale
   Supports both baseline and multiple profiles while staying predictable and low-complexity.
6. Consequences
   A clear filesystem convention becomes part of user-facing behavior.
7. Verification notes
   Discovery convention is controller-defined and compatible with mitmproxy confdir semantics.

### D3: Persist selected profile across restarts

1. Decision
   Persist the selected profile confdir in controller state.
2. Status
   Accepted.
3. Context
   Users expect stable selection across app relaunches.
   Locked default (user-selected): persist selection.
4. Alternatives considered
   - Session-only selection reset on app restart.
   - Manual static profile list without persisted choice.
5. Rationale
   Persistence improves usability and avoids repeated menu actions.
6. Consequences
   Introduces a state-file contract (`state.json` with `selected_confdir`).
7. Verification notes
   No conflict with existing runtime design; controller already uses user-scoped config/log storage conventions.

### D4: Profile switch while running performs immediate stop+restart

1. Decision
   Selecting a different profile while mitmproxy is running triggers immediate stop then start with the new profile.
2. Status
   Accepted.
3. Context
   Profile changes should be effective immediately and visible.
   Locked default (user-selected): stop+restart immediately.
4. Alternatives considered
   - Require manual stop before switching.
   - Defer application until next explicit start.
5. Rationale
   Immediate application is operationally clear and minimizes hidden pending state.
6. Consequences
   Profile selection is a disruptive action when running; status messaging must be explicit.
7. Verification notes
   Existing controller already has start/stop primitives required for this flow.

### D5: Mixed-with-warnings policy for endpoint mismatches

1. Decision
   Allow profile selection even when profile options conflict with controller endpoint assumptions, but surface warnings and degrade incompatible actions.
2. Status
   Accepted.
3. Context
   Profiles may intentionally change listen/web behavior.
   Locked default (user-selected): mixed with warnings.
4. Alternatives considered
   - Controller always overrides config endpoint options.
   - Config always overrides with no warnings.
5. Rationale
   Preserves user flexibility while avoiding silent breakage.
6. Consequences
   Compatibility evaluation and warning presentation become required behavior.
7. Verification notes
   Consistent with CLI-overrides precedence and controller safety requirements.

### D6: Disable incompatible actions instead of auto-adapting endpoints

1. Decision
   If profile endpoints are incompatible with controller assumptions, disable affected actions (proxy toggle and/or Web UI access) rather than dynamically adapting to profile values.
2. Status
   Accepted.
3. Context
   Dynamic adaptation increases parsing/state complexity and potential errors.
   Locked default (user-selected): disable incompatible actions.
4. Alternatives considered
   - Fully adapt actions to profile-derived endpoints.
   - Warn only but keep all actions enabled.
5. Rationale
   Safe-by-default behavior with explicit, deterministic constraints.
6. Consequences
   Some valid profile setups may have reduced controller functionality until aligned.
7. Verification notes
   Compatible with current fixed constants in `/Users/jayshah/Documents/programming/mitmproxy-controller/mitm.go`.

### D7: Nested submenu UX for profile selection

1. Decision
   Represent profiles as a nested submenu under a parent item (for example, `Config Profile`).
2. Status
   Accepted.
3. Context
   Profile list should not clutter top-level tray menu.
4. Alternatives considered
   - Flat menu list.
   - External config-only selection with no UI.
5. Rationale
   Nested hierarchy keeps top-level menu concise and scales to multiple profiles.
6. Consequences
   Requires dynamic submenu items and checkmark state management.
7. Verification notes
   Supported by systray API: `AddSubMenuItem` in v1.2.2.

### D8: Keep fixed controller endpoint assumptions for core UX unless overridden

1. Decision
   Controller remains designed around its known endpoint assumptions (`proxyHost=127.0.0.1`, `proxyPort=8899`, `webUIPort=8898`, fixed web token behavior), and compatibility is evaluated against these assumptions.
2. Status
   Accepted.
3. Context
   Existing system-proxy and Web UI flows assume fixed endpoints.
4. Alternatives considered
   - Fully dynamic endpoint model from profile values.
   - Hard reject any profile that changes these values.
5. Rationale
   Preserves existing behavior and reduces risk while still allowing profile selection.
6. Consequences
   Profiles that diverge may run but controller features may be disabled.
7. Verification notes
   Constants and behavior currently live in `/Users/jayshah/Documents/programming/mitmproxy-controller/mitm.go` and `/Users/jayshah/Documents/programming/mitmproxy-controller/main.go`.

### D9: Parse selected config for compatibility-relevant keys only

1. Decision
   Parse only keys needed for controller compatibility checks:
   - `listen_host`, `listen_port`, `web_host`, `web_port`, `web_password`.
2. Status
   Accepted.
3. Context
   Full mitmproxy config parsing is unnecessary for controller decisions.
4. Alternatives considered
   - Parse entire schema.
   - Do not parse at all; infer only from runtime failures.
5. Rationale
   Minimal parser surface keeps implementation pragmatic and robust.
6. Consequences
   Unknown/unrelated keys are ignored by controller logic.
7. Verification notes
   Keys align with current controller feature dependencies.

### D10: Invalid/missing profile config yields warnings, not hard failure

1. Decision
   Invalid YAML or missing `config.yaml` does not block profile visibility/selection. Surface warning status and proceed with safe defaults where possible.
2. Status
   Accepted.
3. Context
   Discovery should be resilient to partial or broken profile directories.
4. Alternatives considered
   - Exclude invalid profiles from menu.
   - Treat invalid profile as fatal error.
5. Rationale
   Avoids brittle UX and makes debugging transparent.
6. Consequences
   Additional warning text and status-state handling are required.
7. Verification notes
   Aligns with mixed-with-warnings product direction and non-disruptive tray behavior.

## UX and Menu Behavior Specification

1. Add a top-level parent menu item for profile selection, with nested child entries for discovered profiles.
2. Exactly one profile is checked at a time to indicate active selection.
3. Selecting a profile:
   - Persists `selected_confdir`.
   - If mitmproxy is running, performs stop+restart immediately.
   - Updates status message with selection/apply result.
4. Warning behavior:
   - If proxy compatibility fails, disable `Enable System Proxy` and `Disable System Proxy`.
   - If web compatibility fails, disable `View Flows (Web UI)`.
5. Status text includes active profile identity and warning summary when incompatibilities exist.

## Runtime Behavior Specification

### Startup argument assembly

1. Build mitm command as today (`mitmweb` preferred, fallback to `mitmdump`).
2. Always include `--set confdir=<selected_confdir>`.
3. Continue passing controller-known runtime arguments for proxy and web behavior.
4. Evaluate compatibility warnings from selected profile config and adjust tray actions accordingly.

### Precedence matrix

| Concern | Source | Effective authority | Controller behavior |
|---|---|---|---|
| Profile location | Controller-selected `confdir` | CLI (`--set confdir=...`) | Always explicit |
| Generic mitm options | `config.yaml` and CLI | CLI overrides config | Keep explicit controller args for core behavior |
| Proxy endpoint compatibility | Parsed selected config vs controller assumptions | Controller policy layer | Warn + disable incompatible proxy actions |
| Web UI compatibility | Parsed selected config vs controller assumptions | Controller policy layer | Warn + disable incompatible Web UI action |

## Data Model and Persistence

### Profile model (documented internal contract)

- `name`: Display label for submenu entry.
- `confdir`: Absolute profile directory path.
- `config_path`: Derived `confdir/config.yaml`.
- `is_default`: Whether this is the default profile directory.
- `overrides`: Parsed compatibility-relevant keys.
- `compatibility`: Computed feature-compatibility booleans and warnings.

### Compatibility model (documented internal contract)

- `proxy_compatible` (bool)
- `web_compatible` (bool)
- `warnings` ([]string)

### Persistence schema (documented internal contract)

File:
- `<os.UserConfigDir()>/mitmproxy-controller/state.json`

Schema:

```json
{
  "selected_confdir": "/absolute/path/to/profile/confdir"
}
```

Behavior:

1. Load on startup.
2. If missing/invalid/not found, fallback to default profile.
3. Save on explicit profile selection.

## Risks and Tradeoffs

1. Endpoint mismatch can reduce functionality:
   - Tradeoff: safety and predictability over dynamic adaptation complexity.
2. Discovery convention may not match every user’s directory layout:
   - Tradeoff: pragmatic convention over fully user-configurable discovery in v1.
3. Immediate restart on profile switch is disruptive:
   - Tradeoff: explicit apply semantics over hidden deferred state.
4. Partial parsing of config keys may miss advanced edge cases:
   - Tradeoff: smaller implementation surface and lower regression risk.

## Validation Plan

Decision-record completeness checks:

1. D1..D10 each appears exactly once in the decision log.
2. Every decision includes at least one rejected alternative and rationale.
3. All user-selected preferences are explicitly marked as locked defaults.
4. Official behavior claims include source links.
5. No unresolved placeholders remain.
6. Document is standalone and understandable without chat history.

Behavioral verification scenarios (for follow-up implementation/testing):

1. Discovery includes default confdir and `profiles/*` entries with deterministic ordering.
2. Selection persistence restores active profile after restart.
3. Switching profile while running performs stop+restart.
4. Compatibility mismatch disables only affected actions and surfaces warnings.
5. Invalid/missing config generates warnings without crashing or removing profile entry.

## Future Enhancements Deferred

1. User-defined additional discovery roots.
2. Full dynamic endpoint adaptation (proxy and Web UI) instead of action gating.
3. Runtime menu refresh/re-scan command for profiles.
4. Profile creation/editing UX.
5. Rich diagnostics pane for config parse and compatibility errors.

## References

Official mitmproxy:

1. Options concept docs: [https://docs.mitmproxy.org/stable/concepts/options/](https://docs.mitmproxy.org/stable/concepts/options/)
2. CLI startup path: [https://github.com/mitmproxy/mitmproxy/blob/main/mitmproxy/tools/main.py](https://github.com/mitmproxy/mitmproxy/blob/main/mitmproxy/tools/main.py)
3. Option loading/precedence internals: [https://github.com/mitmproxy/mitmproxy/blob/main/mitmproxy/optmanager.py](https://github.com/mitmproxy/mitmproxy/blob/main/mitmproxy/optmanager.py)
4. Option definitions (`confdir`): [https://github.com/mitmproxy/mitmproxy/blob/main/mitmproxy/options.py](https://github.com/mitmproxy/mitmproxy/blob/main/mitmproxy/options.py)

Systray submenu API:

1. `AddSubMenuItem` reference (v1.2.2): [https://github.com/getlantern/systray/blob/v1.2.2/systray.go](https://github.com/getlantern/systray/blob/v1.2.2/systray.go)

Local controller integration points:

1. `/Users/jayshah/Documents/programming/mitmproxy-controller/main.go`
2. `/Users/jayshah/Documents/programming/mitmproxy-controller/mitm.go`
