#!/usr/bin/env bash
# Builds the downloadable package of a release:
#   dist/gofm-<version>-windows.zip   gofm-<version>/ with gofm.exe, gofm-arm64.exe,
#                                      README.md, CHANGELOG.md, LICENSE,
#                                      THIRD_PARTY_NOTICES.md, windows-terminal/
#   dist/gofm-<version>-windows.zip.sha256
#   dist/release-notes.md             the CHANGELOG section of the version
# The version is read from main.go. With a tag argument (e.g. v1.10, as in the GitHub
# workflow) the script stops if the tag and the version do not match.
# Needs Go, zip and sha256sum (Linux, macOS, Git Bash on Windows).
set -euo pipefail
cd "$(dirname "$0")/.."

version=$(sed -n 's/^const version = "\(.*\)"$/\1/p' main.go)
[ -n "$version" ] || { echo "version not found in main.go" >&2; exit 1; }
if [ $# -gt 0 ] && [ "$1" != "v$version" ]; then
	echo "tag $1 does not match the version in main.go ($version): expected v$version" >&2
	exit 1
fi
grep -q "^## $version " CHANGELOG.md || { echo "CHANGELOG.md has no section for $version" >&2; exit 1; }

name="gofm-$version"
rm -rf dist
mkdir -p "dist/$name"
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "dist/$name/gofm.exe" .
GOOS=windows GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o "dist/$name/gofm-arm64.exe" .
cp README.md CHANGELOG.md LICENSE THIRD_PARTY_NOTICES.md "dist/$name/"
cp -r windows-terminal "dist/$name/"
(cd dist && zip -qr -X "$name-windows.zip" "$name" && sha256sum "$name-windows.zip" > "$name-windows.zip.sha256")

# Release notes: the CHANGELOG section of this version, without its heading.
awk -v v="## $version " 'index($0, v) == 1 {on = 1; next} /^## / {on = 0} on' CHANGELOG.md > dist/release-notes.md
cat >> dist/release-notes.md <<NOTES

---
**Download:** \`$name-windows.zip\` (64-bit Windows: \`gofm.exe\` for x64, \`gofm-arm64.exe\` for ARM).
Unzip, then run \`windows-terminal\\install-gofm.ps1\` or start \`gofm.exe\` directly — see the README.
The icons need the *Cascadia Mono NF* font (README, "Font with icons"); without it press Alt+I.
NOTES
echo "dist/$name-windows.zip"
