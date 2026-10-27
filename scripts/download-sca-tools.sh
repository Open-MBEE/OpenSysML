#!/usr/bin/env bash
# Provision the dependency-audit tools `make audit` runs into build/sca/: cargo-audit
# (client/rust), osv-scanner (client/java's Maven trees) and pip-audit (the Python
# packages, in a venv). govulncheck and npm audit need nothing from here: `make vulncheck`
# runs govulncheck through `go run` at a pinned version, and npm ships `npm audit`.
# Every download is pinned and SHA-256 verified; a tool already present at the pinned
# version is left alone. Pass tool names to provision a subset (cargo-audit, osv-scanner,
# pip-audit); with no arguments all three are provisioned.
# Needs: curl, tar, sha256sum, and python3 with venv for pip-audit.
set -euo pipefail
HTTPS_ONLY="=https"

CARGO_AUDIT_VERSION="0.22.2"
CARGO_AUDIT_SHA256_LINUX_X86_64="7fb9497f8594b389e5fce5ef9b92db08432996895b2e0c5a0167a69ed445c428"
CARGO_AUDIT_SHA256_LINUX_AARCH64="c6603814ddaa45e51263dafd31c0ac98808f688d26f7395804f9670b0fd599dd"
CARGO_AUDIT_SHA256_DARWIN_X86_64="847831323de932155b226ab60ee4a180e13e5d007a019f0d4b7b4d89a6de2ab2"
CARGO_AUDIT_SHA256_DARWIN_AARCH64="ec7ca4263769593df4d909be85b94a6b79efa2897be5d2bb8ebd516e823175af"
OSV_SCANNER_VERSION="2.7.0"
OSV_SCANNER_SHA256_LINUX_X86_64="160c6253e2922e9e0a6a9771c5c4c40a4b3c6090b34a3cdd5e80c96cc2e4218b"
OSV_SCANNER_SHA256_LINUX_AARCH64="4061ef4c021ed3c47f203b818f337e351336cc5cd67c0274fc36bd1dfc728dd7"
OSV_SCANNER_SHA256_DARWIN_X86_64="e82c5df7ab0a42dff29a1cd0660711447b56b4c0483ff66ad8634df10fd81a45"
OSV_SCANNER_SHA256_DARWIN_AARCH64="d4926ef8da134332543be276dff157e4c170f99ad91fc48e89448a8816bdf70b"
PIP_AUDIT_VERSION="2.10.1"

root=$(cd "$(dirname "$0")/.." && pwd)
dest="$root/build/sca"
bin="$dest/bin"
venv="$dest/venv"
mkdir -p "$bin"

os=$(uname -s)
arch=$(uname -m)
case "$os/$arch" in
	Linux/x86_64) platform=LINUX_X86_64; cargo_target=x86_64-unknown-linux-musl; osv_asset=linux_amd64 ;;
	Linux/aarch64 | Linux/arm64) platform=LINUX_AARCH64; cargo_target=aarch64-unknown-linux-gnu; osv_asset=linux_arm64 ;;
	Darwin/x86_64) platform=DARWIN_X86_64; cargo_target=x86_64-apple-darwin; osv_asset=darwin_amd64 ;;
	Darwin/arm64) platform=DARWIN_AARCH64; cargo_target=aarch64-apple-darwin; osv_asset=darwin_arm64 ;;
	*) echo "download-sca-tools.sh: no pinned cargo-audit/osv-scanner build for $os/$arch" >&2; exit 1 ;;
esac

fetch() { # fetch <url> <sha256> <out>
	curl -fsSL --proto "$HTTPS_ONLY" --proto-redir "$HTTPS_ONLY" -o "$3" "$1"
	echo "$2  $3" | sha256sum --check --strict --quiet
}

install_cargo_audit() {
	if [[ -x "$bin/cargo-audit" ]] && [[ "$("$bin/cargo-audit" --version 2>/dev/null)" == "cargo-audit $CARGO_AUDIT_VERSION" ]]; then
		echo "cargo-audit $CARGO_AUDIT_VERSION already installed"
		return
	fi
	echo "Installing cargo-audit $CARGO_AUDIT_VERSION ..."
	local sha asset tmp
	sha=CARGO_AUDIT_SHA256_$platform
	asset="cargo-audit-$cargo_target-v$CARGO_AUDIT_VERSION"
	tmp=$(mktemp -d)
	fetch "https://github.com/rustsec/rustsec/releases/download/cargo-audit/v$CARGO_AUDIT_VERSION/$asset.tgz" "${!sha}" "$tmp/$asset.tgz"
	tar -xzf "$tmp/$asset.tgz" -C "$tmp" "$asset/cargo-audit"
	install -m 0755 "$tmp/$asset/cargo-audit" "$bin/cargo-audit"
	rm -rf "$tmp"
	[[ "$("$bin/cargo-audit" --version)" == "cargo-audit $CARGO_AUDIT_VERSION" ]]
}

install_osv_scanner() {
	if [[ -x "$bin/osv-scanner" ]] && "$bin/osv-scanner" --version 2>/dev/null | grep -qx "osv-scanner version: $OSV_SCANNER_VERSION"; then
		echo "osv-scanner $OSV_SCANNER_VERSION already installed"
		return
	fi
	echo "Installing osv-scanner $OSV_SCANNER_VERSION ..."
	local sha tmp
	sha=OSV_SCANNER_SHA256_$platform
	tmp=$(mktemp)
	fetch "https://github.com/google/osv-scanner/releases/download/v$OSV_SCANNER_VERSION/osv-scanner_$osv_asset" "${!sha}" "$tmp"
	install -m 0755 "$tmp" "$bin/osv-scanner"
	rm -f "$tmp"
	"$bin/osv-scanner" --version | grep -qx "osv-scanner version: $OSV_SCANNER_VERSION"
}

install_pip_audit() {
	if [[ -x "$venv/bin/pip-audit" ]] && [[ "$("$venv/bin/pip-audit" --version 2>/dev/null)" == "pip-audit $PIP_AUDIT_VERSION" ]]; then
		echo "pip-audit $PIP_AUDIT_VERSION already installed"
		return
	fi
	echo "Installing pip-audit $PIP_AUDIT_VERSION ..."
	python3 -m venv "$venv"
	"$venv/bin/python" -m pip install --quiet --only-binary :all: "pip-audit==$PIP_AUDIT_VERSION"
	[[ "$("$venv/bin/pip-audit" --version)" == "pip-audit $PIP_AUDIT_VERSION" ]]
}

tools=("$@")
[[ ${#tools[@]} -eq 0 ]] && tools=(cargo-audit osv-scanner pip-audit)
for tool in "${tools[@]}"; do
	case "$tool" in
		cargo-audit) install_cargo_audit ;;
		osv-scanner) install_osv_scanner ;;
		pip-audit) install_pip_audit ;;
		*) echo "download-sca-tools.sh: unknown tool $tool (cargo-audit, osv-scanner, pip-audit)" >&2; exit 2 ;;
	esac
done
echo "SCA tools under $dest"
