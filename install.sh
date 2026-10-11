#!/bin/sh
# Installs the OpenSysML command-line tools from a GitHub release.
#
# Linux and macOS (and Git Bash or MSYS2 on Windows; install.ps1 is the native
# Windows installer):
#
#   curl -fsSL https://opensysml.org/install.sh | sh
#   curl -fsSL https://opensysml.org/install.sh | sh -s -- --version v0.9.1 --tools sysml
#
# The release's bundle archive (opensysml-<os>-<arch>.tar.gz, .zip on Windows) is
# downloaded with its SHA256SUMS.txt manifest, every download is checked against
# the manifest before anything is installed, and the installed binaries are run
# once to confirm they report the release they came from. Downloads go through
# curl or wget, which never set macOS's quarantine attribute, so Gatekeeper does
# not object (docs/guide/01-install.md, "macOS: Gatekeeper").
#
# Run with --help for the options.
#
# Needs: curl or wget; tar (unzip or a zip-capable tar for the Windows archive);
# sha256sum, shasum or openssl.
set -eu

REPO_URL="https://github.com/Open-MBEE/OpenSysML"
DEFAULT_BASE_URL="$REPO_URL/releases"

# The identities the checksum manifests are signed with: releases by the CircleCI
# release pipeline, nightly snapshots by the nightly workflow on develop.
RELEASE_OIDC_ISSUER="https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62"
RELEASE_IDENTITY_REGEXP='^https://circleci\.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d/pipeline-definitions/[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$'
NIGHTLY_OIDC_ISSUER="https://token.actions.githubusercontent.com"
NIGHTLY_IDENTITY="https://github.com/Open-MBEE/OpenSysML/.github/workflows/nightly.yml@refs/heads/develop"

version="${OPENSYSML_VERSION:-latest}"
tools="${OPENSYSML_TOOLS:-sysml,sysml-lsp}"
prefix="${OPENSYSML_PREFIX:-}"
bin_dir="${OPENSYSML_BIN_DIR:-}"
base_url="${OPENSYSML_DOWNLOAD_BASE:-$DEFAULT_BASE_URL}"
target_os=""
target_arch=""
verify_signature=false
dry_run=false

usage() {
	# A literal, not read back from $0: piped through `sh` there is no file to read.
	cat <<'EOF'
usage: install.sh [--version <tag>] [--tools <list>] [--prefix <dir>|--bin-dir <dir>]
                  [--os <os> --arch <arch>] [--base-url <url>] [--verify-signature] [--dry-run]

Installs the OpenSysML command-line tools (sysml, sysml-lsp, sysml-grpc,
sysml-jupyter-kernel) from a GitHub release, checking every download against
the release's SHA256SUMS.txt.

Options (each has an environment variable, so a piped `sh` can be configured):

  --version <tag>      release tag (v0.9.1), `latest` (default) or `nightly`
                       [OPENSYSML_VERSION]
  --tools <list>       comma-separated subset of sysml,sysml-lsp,sysml-grpc,
                       sysml-jupyter-kernel, or `all`; default `sysml,sysml-lsp`
                                                             [OPENSYSML_TOOLS]
  --prefix <dir>       install under <dir>/bin and <dir>/share/man/man1; default
                       /usr/local when writable, else ~/.local [OPENSYSML_PREFIX]
  --bin-dir <dir>      install the binaries into <dir> and no manual pages
                       (overrides --prefix)                  [OPENSYSML_BIN_DIR]
  --os <os>, --arch <arch>
                       stage another platform's build instead of this machine's
                       (linux|darwin|windows, amd64|arm64); it is not run
  --base-url <url>     where the releases are, for a mirror; default
                       https://github.com/Open-MBEE/OpenSysML/releases
                       [OPENSYSML_DOWNLOAD_BASE]
  --verify-signature   also verify SHA256SUMS.txt's cosign bundle against the
                       identity that signs the releases (needs cosign on PATH)
  --dry-run            print what would be installed and from where, then exit
  -h, --help           this text
EOF
}

info() { printf '%s\n' "$*"; }
warn() { printf 'warning: %s\n' "$*" >&2; }
fail() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

need_value() {
	[ "$#" -ge 2 ] || fail "$1 needs a value"
}

while [ "$#" -gt 0 ]; do
	case "$1" in
	--version)
		need_value "$@"
		version="$2"
		shift 2
		;;
	--version=*) version="${1#*=}"; shift ;;
	--tools)
		need_value "$@"
		tools="$2"
		shift 2
		;;
	--tools=*) tools="${1#*=}"; shift ;;
	--prefix)
		need_value "$@"
		prefix="$2"
		shift 2
		;;
	--prefix=*) prefix="${1#*=}"; shift ;;
	--bin-dir)
		need_value "$@"
		bin_dir="$2"
		shift 2
		;;
	--bin-dir=*) bin_dir="${1#*=}"; shift ;;
	--os)
		need_value "$@"
		target_os="$2"
		shift 2
		;;
	--os=*) target_os="${1#*=}"; shift ;;
	--arch)
		need_value "$@"
		target_arch="$2"
		shift 2
		;;
	--arch=*) target_arch="${1#*=}"; shift ;;
	--base-url)
		need_value "$@"
		base_url="$2"
		shift 2
		;;
	--base-url=*) base_url="${1#*=}"; shift ;;
	--verify-signature) verify_signature=true; shift ;;
	--dry-run) dry_run=true; shift ;;
	-h | --help)
		usage
		exit 0
		;;
	*) fail "unknown option '$1' (try --help)" ;;
	esac
done

# --- what is being installed ------------------------------------------------

case "$version" in
latest | nightly) ;;
v[0-9]*.[0-9]*.[0-9]*) ;;
[0-9]*.[0-9]*.[0-9]*) version="v$version" ;;
*) fail "--version must be a release tag like v0.9.1, 'latest' or 'nightly', not '$version'" ;;
esac

[ "$tools" != all ] || tools="sysml,sysml-lsp,sysml-grpc,sysml-jupyter-kernel"
tool_list=$(printf '%s' "$tools" | tr ',' ' ')
[ -n "$(printf '%s' "$tool_list" | tr -d ' ')" ] || fail "--tools names nothing to install"
for tool in $tool_list; do
	case "$tool" in
	sysml | sysml-lsp | sysml-grpc | sysml-jupyter-kernel) ;;
	*) fail "unknown tool '$tool'; the released tools are sysml, sysml-lsp, sysml-grpc and sysml-jupyter-kernel" ;;
	esac
done
want() { # <tool>: whether it was asked for
	case " $tool_list " in *" $1 "*) return 0 ;; *) return 1 ;; esac
}

base_url="${base_url%/}"

# --- the platform -----------------------------------------------------------

host_os=""
host_arch=""
uname_s=$(uname -s 2>/dev/null || echo unknown)
uname_m=$(uname -m 2>/dev/null || echo unknown)
case "$uname_s" in
Linux) host_os=linux ;;
Darwin) host_os=darwin ;;
MINGW* | MSYS* | CYGWIN* | Windows_NT) host_os=windows ;;
esac
case "$uname_m" in
x86_64 | amd64) host_arch=amd64 ;;
aarch64 | arm64) host_arch=arm64 ;;
esac
# A shell under Rosetta reports x86_64; the native build is the one to install.
if [ "$host_os" = darwin ] && [ "$host_arch" = amd64 ] &&
	[ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = 1 ]; then
	host_arch=arm64
fi

if [ -z "$target_os" ]; then
	target_os="$host_os"
	[ -n "$target_os" ] || fail "unsupported operating system '$uname_s': releases are built for Linux, macOS and Windows; see $REPO_URL#install for building from source"
fi
if [ -z "$target_arch" ]; then
	target_arch="$host_arch"
	[ -n "$target_arch" ] || fail "unsupported architecture '$uname_m': releases are built for amd64 and arm64; 'go install github.com/Open-MBEE/OpenSysML/cmd/sysml@latest' builds one for this machine"
fi
case "$target_os" in
linux | darwin | windows) ;;
*) fail "--os must be linux, darwin or windows, not '$target_os'" ;;
esac
case "$target_arch" in
amd64 | arm64) ;;
*) fail "--arch must be amd64 or arm64, not '$target_arch'" ;;
esac
[ "$target_os" != windows ] || [ "$target_arch" = amd64 ] ||
	fail "no windows/$target_arch build is published; 'go install github.com/Open-MBEE/OpenSysML/cmd/sysml@latest' builds one"
platform="$target_os-$target_arch"
is_host_platform=false
[ "$target_os" != "$host_os" ] || [ "$target_arch" != "$host_arch" ] || is_host_platform=true

exe=""
[ "$target_os" != windows ] || exe=".exe"
bundle="opensysml-$platform.tar.gz"
[ "$target_os" != windows ] || bundle="opensysml-$platform.zip"
grpc_asset="sysml-grpc-$platform$exe"
kernel_asset="sysml-jupyter-kernel-$platform$exe"

# --- where it goes ----------------------------------------------------------

man_dir=""
if [ -z "$bin_dir" ]; then
	if [ -z "$prefix" ]; then
		if [ -w /usr/local/bin ] && { [ -w /usr/local/share/man/man1 ] || { [ ! -e /usr/local/share/man/man1 ] && [ -w /usr/local ]; }; }; then
			prefix=/usr/local
		else
			prefix="$HOME/.local"
		fi
	fi
	bin_dir="$prefix/bin"
	[ "$target_os" = windows ] || man_dir="$prefix/share/man/man1"
fi

# --- tools on this machine --------------------------------------------------

have() { command -v "$1" >/dev/null 2>&1; }

downloader=""
if have curl; then
	downloader=curl
elif have wget; then
	downloader=wget
else
	fail "curl or wget is needed to download the release"
fi

# Only https is followed when the releases are fetched over https; a plain-http
# mirror is left to its own policy.
curl_proto=""
case "$base_url" in https://*) curl_proto="--proto =https --proto-redir =https" ;; esac

download() { # <url> <destination>
	if [ "$downloader" = curl ]; then
		# shellcheck disable=SC2086  # curl_proto is a list of options
		curl -fsSL $curl_proto --retry 3 -o "$2" "$1"
	else
		wget -q -O "$2" "$1"
	fi
}

sha256_of() { # <file>: the hex digest
	if have sha256sum; then
		sha256sum "$1" | cut -d' ' -f1
	elif have shasum; then
		shasum -a 256 "$1" | cut -d' ' -f1
	elif have openssl; then
		openssl dgst -sha256 "$1" | sed 's/.*= //'
	else
		fail "sha256sum, shasum or openssl is needed to verify the download"
	fi
}
have sha256sum || have shasum || have openssl || fail "sha256sum, shasum or openssl is needed to verify the download"

if [ "$target_os" = windows ]; then
	have unzip || have tar || fail "unzip or tar is needed to extract $bundle"
else
	have tar || fail "tar is needed to extract $bundle"
fi
if [ "$verify_signature" = true ]; then
	have cosign || fail "--verify-signature needs cosign on PATH (https://docs.sigstore.dev/cosign/system_config/installation/)"
fi

# --- which release ----------------------------------------------------------

# Resolves `latest` to its tag through the redirect GitHub serves for it, so the
# release is named before anything is downloaded; a mirror without the redirect
# still works through the `latest/download/` path.
resolve_latest() {
	location=""
	if [ "$downloader" = curl ]; then
		# shellcheck disable=SC2086
		location=$(curl -fsSLI $curl_proto -o /dev/null -w '%{url_effective}' "$base_url/latest" 2>/dev/null) || location=""
	else
		location=$(wget -q --spider --server-response "$base_url/latest" 2>&1 | sed -n 's/^ *[Ll]ocation: *//p' | tail -n 1 | tr -d '\r') || location=""
	fi
	case "$location" in
	*/tag/*) printf '%s\n' "${location##*/tag/}" ;;
	*) return 1 ;;
	esac
}

tag="$version"
asset_dir=""
if [ "$version" = latest ]; then
	if tag=$(resolve_latest); then
		asset_dir="$base_url/download/$tag"
	else
		tag=""
		asset_dir="$base_url/latest/download"
	fi
else
	asset_dir="$base_url/download/$version"
fi
asset_url() { printf '%s/%s\n' "$asset_dir" "$1"; }

release_name="${tag:-the latest release}"
[ "$version" != nightly ] || release_name="the nightly snapshot"

info "Installing OpenSysML ($release_name) for $platform"
info "  tools:    $tool_list"
info "  binaries: $bin_dir"
[ -z "$man_dir" ] || info "  manuals:  $man_dir"
info "  from:     $(asset_url "$bundle")"
! want sysml-grpc || info "            $(asset_url "$grpc_asset")"
! want sysml-jupyter-kernel || info "            $(asset_url "$kernel_asset")"
if [ "$dry_run" = true ]; then
	info "Dry run: nothing downloaded or installed."
	exit 0
fi

# --- download and verify ----------------------------------------------------

work=$(mktemp -d 2>/dev/null || mktemp -d -t opensysml)
trap 'rm -rf "$work"' EXIT INT TERM

fetch() { # <asset>: into the work directory
	download "$(asset_url "$1")" "$work/$1" ||
		fail "could not download $(asset_url "$1"); is $release_name a published release with a $platform build? Bundles and SHA256SUMS.txt exist from v0.0.4. See $base_url"
}

info "Downloading..."
fetch SHA256SUMS.txt
if [ "$verify_signature" = true ]; then
	fetch SHA256SUMS.txt.bundle
	if [ "$version" = nightly ]; then
		cosign verify-blob "$work/SHA256SUMS.txt" --bundle "$work/SHA256SUMS.txt.bundle" \
			--certificate-oidc-issuer "$NIGHTLY_OIDC_ISSUER" \
			--certificate-identity "$NIGHTLY_IDENTITY" >/dev/null 2>&1 ||
			fail "SHA256SUMS.txt is not signed by the nightly workflow ($NIGHTLY_IDENTITY)"
	else
		cosign verify-blob "$work/SHA256SUMS.txt" --bundle "$work/SHA256SUMS.txt.bundle" \
			--certificate-oidc-issuer "$RELEASE_OIDC_ISSUER" \
			--certificate-identity-regexp "$RELEASE_IDENTITY_REGEXP" >/dev/null 2>&1 ||
			fail "SHA256SUMS.txt is not signed by the OpenSysML release pipeline"
	fi
	info "  signature of SHA256SUMS.txt verified"
fi

verify() { # <asset>: against the manifest
	expected=$(awk -v name="$1" '{ n = $2; sub(/^\*/, "", n); if (n == name) print $1 }' "$work/SHA256SUMS.txt")
	[ -n "$expected" ] || fail "SHA256SUMS.txt of $release_name does not list $1; releases before v0.0.4 have no bundle, sysml-grpc is published from v0.9.0 and sysml-jupyter-kernel from v0.10.0"
	actual=$(sha256_of "$work/$1")
	[ "$actual" = "$expected" ] || fail "$1 does not match SHA256SUMS.txt (expected $expected, got $actual); the download is corrupt or tampered with"
	info "  $1 verified"
}

fetch "$bundle"
verify "$bundle"
if want sysml-grpc; then
	fetch "$grpc_asset"
	verify "$grpc_asset"
fi
if want sysml-jupyter-kernel; then
	fetch "$kernel_asset"
	verify "$kernel_asset"
fi

# --- install ----------------------------------------------------------------

mkdir -p "$work/bundle"
if [ "$target_os" = windows ]; then
	if have unzip; then
		unzip -q "$work/$bundle" -d "$work/bundle"
	else
		tar -xf "$work/$bundle" -C "$work/bundle"
	fi
else
	tar -xzf "$work/$bundle" -C "$work/bundle"
fi

install_file() { # <source> <destination> <mode>
	cp "$1" "$2.tmp.$$" && chmod "$3" "$2.tmp.$$" && mv -f "$2.tmp.$$" "$2"
}

mkdir -p "$bin_dir" 2>/dev/null || fail "cannot create $bin_dir; pass --prefix or --bin-dir for a writable location, or run with sudo"
[ -w "$bin_dir" ] || fail "cannot write to $bin_dir; pass --prefix or --bin-dir for a writable location, or run with sudo"
if [ -n "$man_dir" ]; then
	mkdir -p "$man_dir" 2>/dev/null && [ -w "$man_dir" ] || fail "cannot write to $man_dir; pass --bin-dir to install the binaries alone"
fi

installed=""
for tool in $tool_list; do
	if [ "$tool" = sysml-grpc ]; then
		source="$work/$grpc_asset"
	elif [ "$tool" = sysml-jupyter-kernel ]; then
		source="$work/$kernel_asset"
	else
		source="$work/bundle/$tool$exe"
		[ -f "$source" ] || fail "$bundle has no $tool$exe"
	fi
	install_file "$source" "$bin_dir/$tool$exe" 0755
	installed="$installed $bin_dir/$tool$exe"
	if [ -n "$man_dir" ] && [ -f "$work/bundle/share/man/man1/$tool.1" ]; then
		install_file "$work/bundle/share/man/man1/$tool.1" "$man_dir/$tool.1" 0644
	fi
done

# --- check ------------------------------------------------------------------

if [ "$is_host_platform" = true ]; then
	for tool in $tool_list; do
		reported=$("$bin_dir/$tool$exe" --version 2>&1 | head -n 1) ||
			fail "$bin_dir/$tool$exe does not run: $reported"
		if [ -n "$tag" ] && [ "$version" != nightly ]; then
			case " $reported " in
			*" $tag "*) ;;
			*) fail "$bin_dir/$tool$exe reports '$reported', not $tag" ;;
			esac
		fi
		info "  $reported"
	done
else
	info "  staged for $platform, not run on this $host_os-$host_arch machine"
fi

info "Installed:$installed"
case ":$PATH:" in
*":$bin_dir:"*) ;;
*)
	warn "$bin_dir is not on PATH; add it, e.g. in your shell profile:"
	warn "  export PATH=\"$bin_dir:\$PATH\""
	;;
esac
for tool in $tool_list; do
	found=$(command -v "$tool" 2>/dev/null || true)
	if [ -n "$found" ] && [ "$found" != "$bin_dir/$tool$exe" ] && [ "$found" != "$bin_dir/$tool" ]; then
		warn "'$tool' on PATH is $found, which comes before $bin_dir/$tool$exe"
	fi
done
