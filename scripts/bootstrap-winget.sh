#!/usr/bin/env bash

set -euo pipefail

owner="jayshah123"
version=""
workdir=""

usage() {
  cat <<'EOF'
Usage:
  ./scripts/bootstrap-winget.sh [--version 0.1.2] [--owner jayshah123] [--workdir /tmp/winget-pkgs]

Description:
  Creates/updates the initial winget-pkgs manifest files for
  Jayshah123.MitmproxyController, commits them to your winget-pkgs fork branch,
  pushes, and opens a PR to microsoft/winget-pkgs.

Notes:
  - Requires: gh, git, jq
  - If --version is omitted, latest release tag from <owner>/mitmproxy-controller is used.
  - If forking fails with SAML enforcement, authorize and rerun.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      version="${2:-}"
      shift 2
      ;;
    --owner)
      owner="${2:-}"
      shift 2
      ;;
    --workdir)
      workdir="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

for cmd in gh git jq; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing required command: $cmd" >&2
    exit 1
  fi
done

app_repo="${owner}/mitmproxy-controller"
fork_repo="${owner}/winget-pkgs"
pkg_id="Jayshah123.MitmproxyController"

if [ -z "$version" ]; then
  latest_tag="$(gh release list --repo "$app_repo" --limit 1 --json tagName --jq '.[0].tagName')"
  if [ -z "$latest_tag" ]; then
    echo "failed to resolve latest release tag for $app_repo" >&2
    exit 1
  fi
  version="${latest_tag#v}"
fi

version="${version#v}"
tag="v${version}"

release_json="$(gh api "repos/${app_repo}/releases/tags/${tag}")"
asset_name="mitmproxy-controller_windows_amd64.zip"
asset_url="$(jq -r --arg name "$asset_name" '.assets[] | select(.name == $name) | .browser_download_url' <<<"$release_json")"
asset_digest="$(jq -r --arg name "$asset_name" '.assets[] | select(.name == $name) | .digest // empty' <<<"$release_json")"
release_date="$(jq -r '.published_at // .created_at' <<<"$release_json" | cut -c1-10)"

if [ -z "$asset_url" ]; then
  echo "could not find release asset: $asset_name on tag $tag" >&2
  exit 1
fi

if [ -n "$asset_digest" ]; then
  sha256="$(echo "${asset_digest#sha256:}" | tr '[:lower:]' '[:upper:]')"
else
  tmp_zip="$(mktemp)"
  curl -fsSL "$asset_url" -o "$tmp_zip"
  sha256="$(shasum -a 256 "$tmp_zip" | awk '{print toupper($1)}')"
  rm -f "$tmp_zip"
fi

if ! gh repo view "$fork_repo" >/dev/null 2>&1; then
  echo "fork not found: $fork_repo" >&2
  echo "attempting to fork microsoft/winget-pkgs..." >&2
  if ! gh repo fork microsoft/winget-pkgs --clone=false; then
    echo "failed to fork winget-pkgs. If prompted with SAML auth URL, authorize and rerun." >&2
    exit 1
  fi
fi

if [ -z "$workdir" ]; then
  workdir="${TMPDIR:-/tmp}/winget-pkgs-${owner}"
fi

if [ -d "$workdir/.git" ]; then
  git -C "$workdir" remote set-url origin "https://github.com/${fork_repo}.git"
  git -C "$workdir" fetch --all --prune
else
  git clone "https://github.com/${fork_repo}.git" "$workdir"
fi

git -C "$workdir" checkout master
git -C "$workdir" pull --ff-only origin master

branch="add-mitmproxy-controller-${version}"
git -C "$workdir" checkout -B "$branch"

manifest_dir="$workdir/manifests/j/Jayshah123/MitmproxyController/${version}"
mkdir -p "$manifest_dir"

cat > "${manifest_dir}/${pkg_id}.yaml" <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.version.1.10.0.schema.json

PackageIdentifier: ${pkg_id}
PackageVersion: ${version}
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.10.0
EOF

cat > "${manifest_dir}/${pkg_id}.locale.en-US.yaml" <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.defaultLocale.1.10.0.schema.json

PackageIdentifier: ${pkg_id}
PackageVersion: ${version}
PackageLocale: en-US
Publisher: jayshah123
PublisherUrl: https://github.com/jayshah123
PublisherSupportUrl: https://github.com/jayshah123/mitmproxy-controller/issues
Author: jayshah123
PackageName: mitmproxy-controller
PackageUrl: https://github.com/jayshah123/mitmproxy-controller
License: MIT
LicenseUrl: https://github.com/jayshah123/mitmproxy-controller/blob/main/LICENSE
ShortDescription: System tray controller for mitmproxy
Description: Cross-platform system tray app for controlling mitmproxy and system proxy settings.
Tags:
- mitmproxy
- proxy
- network
- developer-tools
ReleaseNotesUrl: https://github.com/jayshah123/mitmproxy-controller/releases/tag/${tag}
ManifestType: defaultLocale
ManifestVersion: 1.10.0
EOF

cat > "${manifest_dir}/${pkg_id}.installer.yaml" <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.installer.1.10.0.schema.json

PackageIdentifier: ${pkg_id}
PackageVersion: ${version}
InstallerType: zip
NestedInstallerType: portable
ReleaseDate: ${release_date}
Installers:
- Architecture: x64
  NestedInstallerFiles:
  - RelativeFilePath: mitmproxy-controller_windows_amd64.exe
    PortableCommandAlias: mitmproxy-controller
  InstallerUrl: ${asset_url}
  InstallerSha256: ${sha256}
ManifestType: installer
ManifestVersion: 1.10.0
EOF

git -C "$workdir" add "$manifest_dir"

if git -C "$workdir" diff --cached --quiet; then
  echo "no manifest changes to commit."
else
  git -C "$workdir" commit -m "Add ${pkg_id} version ${version}"
fi

git -C "$workdir" push -u origin "$branch"

pr_title="Add ${pkg_id} version ${version}"
pr_body="Initial package submission for mitmproxy-controller ${version}."

if gh pr view --repo microsoft/winget-pkgs "${owner}:${branch}" >/dev/null 2>&1; then
  echo "PR already exists for branch ${owner}:${branch}"
else
  gh pr create \
    --repo microsoft/winget-pkgs \
    --head "${owner}:${branch}" \
    --base master \
    --title "$pr_title" \
    --body "$pr_body"
fi

echo "Bootstrap flow completed for ${pkg_id} ${version}."
