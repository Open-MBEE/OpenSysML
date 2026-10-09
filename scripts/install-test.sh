#!/usr/bin/env bash
# Checks install.sh (and install.ps1 when pwsh is on PATH) against a throwaway
# release served from localhost, so the platform choice, the checksum gate, the
# latest-tag redirect and the failure messages are the only things under test and
# no network is needed. Run: scripts/install-test.sh
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
server_pid=""
trap '[[ -z "$server_pid" ]] || kill "$server_pid" 2>/dev/null; chmod -R u+w "$work"; rm -rf "$work"' EXIT

# --- the host platform, named the way the release pipeline names it ----------

case "$(uname -s)" in
Linux) host_os=linux ;;
Darwin) host_os=darwin ;;
*) echo "install-test: unsupported host $(uname -s)" >&2 && exit 1 ;;
esac
case "$(uname -m)" in
x86_64 | amd64) host_arch=amd64 ;;
aarch64 | arm64) host_arch=arm64 ;;
*) echo "install-test: unsupported host $(uname -m)" >&2 && exit 1 ;;
esac
host="$host_os-$host_arch"
# A foreign Unix platform the host can stage but must not run.
other=$([[ "$host" = linux-arm64 ]] && echo darwin-arm64 || echo linux-arm64)

good=v9.9.9-test       # a complete release
signed=v9.9.8-test     # one that also carries the signed Windows build
tampered=v9.9.7-test   # one whose manifest lies about the bundle
unlisted=v9.9.6-test   # one whose manifest omits sysml-grpc
mismatch=v9.9.5-test   # one whose binaries report another version

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then sha256sum "$@"; else shasum -a 256 "$@"; fi
}

# fake_tool <path> <tool> <version>: a stand-in binary that only knows --version.
fake_tool() {
	printf '#!/bin/sh\necho "%s"\n' "$2 $3" >"$1"
	chmod +x "$1"
}

# bundle <release dir> <platform> <version>: the archive, grpc and kernel
# binaries and manpages the release pipeline would publish for one platform.
bundle() {
	local dir=$1 platform=$2 version=$3 stage
	stage="$work/stage-$platform"
	rm -rf "$stage" && mkdir -p "$stage/share/man/man1"
	case "$platform" in
	windows-*)
		echo "sysml $version for windows" >"$stage/sysml.exe"
		echo "sysml-lsp $version for windows" >"$stage/sysml-lsp.exe"
		(cd "$stage" && python3 -m zipfile -c "$dir/opensysml-$platform.zip" sysml.exe sysml-lsp.exe)
		echo "sysml-grpc $version for windows" >"$dir/sysml-grpc-$platform.exe"
		echo "sysml-jupyter-kernel $version for windows" >"$dir/sysml-jupyter-kernel-$platform.exe"
		;;
	*)
		fake_tool "$stage/sysml" sysml "v$version"
		fake_tool "$stage/sysml-lsp" sysml-lsp "v$version"
		echo ".TH SYSML 1" >"$stage/share/man/man1/sysml.1"
		echo ".TH SYSML-LSP 1" >"$stage/share/man/man1/sysml-lsp.1"
		tar -czf "$dir/opensysml-$platform.tar.gz" -C "$stage" sysml sysml-lsp share
		fake_tool "$dir/sysml-grpc-$platform" "sysml-grpc version" "v$version"
		fake_tool "$dir/sysml-jupyter-kernel-$platform" "sysml-jupyter-kernel version" "v$version"
		;;
	esac
}

# release <tag> <reported version>: a release directory with the host, the other
# Unix platform and Windows, and a manifest over everything in it.
release() {
	local tag=$1 version=$2 dir
	dir="$work/site/releases/download/$tag"
	mkdir -p "$dir"
	bundle "$dir" "$host" "$version"
	bundle "$dir" "$other" "$version"
	bundle "$dir" windows-amd64 "$version"
	(cd "$dir" && sha256 -- * >SHA256SUMS.txt)
}

release "$good" "${good#v}"
release nightly "${good#v}-5-gabcdef0"
release "$mismatch" "${good#v}"

release "$signed" "${signed#v}"
signed_dir="$work/site/releases/download/$signed"
mkdir -p "$work/stage-signed"
echo "sysml ${signed#v} for windows, signed" >"$work/stage-signed/sysml.exe"
echo "sysml-lsp ${signed#v} for windows, signed" >"$work/stage-signed/sysml-lsp.exe"
(cd "$work/stage-signed" && python3 -m zipfile -c "$signed_dir/opensysml-windows-amd64-signed.zip" sysml.exe sysml-lsp.exe)
(cd "$signed_dir" && sha256 opensysml-windows-amd64-signed.zip >SHA256SUMS-windows-signed.txt)

release "$tampered" "${tampered#v}"
sed -i.bak "s/^[0-9a-f]\{64\}\(  opensysml-$host\.tar\.gz\)$/$(printf '0%.0s' $(seq 64))\1/" \
	"$work/site/releases/download/$tampered/SHA256SUMS.txt"
rm "$work/site/releases/download/$tampered/SHA256SUMS.txt.bak"

release "$unlisted" "${unlisted#v}"
sed -i.bak "/ sysml-grpc-/d" "$work/site/releases/download/$unlisted/SHA256SUMS.txt"
rm "$work/site/releases/download/$unlisted/SHA256SUMS.txt.bak"

# A mirror that serves latest/download/ but has no /latest redirect to a tag.
mkdir -p "$work/site/mirror/latest"
cp -R "$work/site/releases/download/$good" "$work/site/mirror/latest/download"

# --- the server: GitHub's /releases/latest redirect over a static tree --------

cat >"$work/server.py" <<'PY'
import functools, http.server, sys
site, tag = sys.argv[1], sys.argv[2]

class Handler(http.server.SimpleHTTPRequestHandler):
    def redirect(self):
        if self.path == "/releases/latest":
            self.send_response(302)
            self.send_header("Location", "/releases/tag/" + tag)
            self.end_headers()
            return True
        if self.path == "/releases/tag/" + tag:
            self.send_response(200)
            self.send_header("Content-Length", "0")
            self.end_headers()
            return True
        return False

    def do_HEAD(self):
        self.redirect() or super().do_HEAD()

    def do_GET(self):
        self.redirect() or super().do_GET()

    def log_message(self, *args):
        pass

server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), functools.partial(Handler, directory=site))
print(server.server_address[1], flush=True)
server.serve_forever()
PY
python3 "$work/server.py" "$work/site" "$good" >"$work/port" &
server_pid=$!
for _ in $(seq 50); do
	[[ -s "$work/port" ]] && break
	sleep 0.1
done
[[ -s "$work/port" ]] || { echo "install-test: the release server did not start" >&2; exit 1; }
base="http://127.0.0.1:$(cat "$work/port")/releases"
mirror="http://127.0.0.1:$(cat "$work/port")/mirror"

# --- the cases ---------------------------------------------------------------

failures=0
n=0
out=""

# run <args...>: install.sh with a fresh prefix, capturing its output; @P@ in
# the arguments stands for that prefix, which the case then finds in $prefix.
run() {
	local arg args=()
	n=$((n + 1))
	prefix="$work/install-$n"
	for arg in "$@"; do args+=("${arg//@P@/$prefix}"); done
	set +e
	out=$(sh "$root/install.sh" --base-url "$base" "${args[@]}" 2>&1)
	status=$?
	set -e
}

# ok <name> <args...>: the install must succeed.
ok() {
	local name=$1
	shift
	run "$@"
	if [[ "$status" -ne 0 ]]; then
		echo "FAIL $name: exit $status" >&2 && printf '%s\n' "$out" >&2
		failures=$((failures + 1))
		return 1
	fi
	echo "ok   $name"
}

# fails <name> <message> <args...>: the install must fail, saying <message>.
fails() {
	local name=$1 message=$2
	shift 2
	run "$@"
	if [[ "$status" -eq 0 ]] || [[ "$out" != *"$message"* ]]; then
		echo "FAIL $name: exit $status, expected a failure mentioning '$message'" >&2 && printf '%s\n' "$out" >&2
		failures=$((failures + 1))
		return 1
	fi
	echo "ok   $name"
}

# said <text>: the last run's output must mention it.
said() {
	[[ "$out" == *"$1"* ]] || { echo "FAIL: expected output to mention '$1'" >&2; printf '%s\n' "$out" >&2; failures=$((failures + 1)); }
}

# installed <paths...>: files the last run must have left executable under $prefix.
installed() {
	local path
	for path in "$@"; do
		[[ -x "$prefix/$path" ]] || { echo "FAIL: $prefix/$path missing or not executable" >&2; failures=$((failures + 1)); }
	done
}

# absent <paths...>: files the last run must not have left under $prefix.
absent() {
	local path
	for path in "$@"; do
		[[ ! -e "$prefix/$path" ]] || { echo "FAIL: $prefix/$path should not exist" >&2; failures=$((failures + 1)); }
	done
}

if ok "release by tag" --version "$good" --prefix @P@; then
	installed bin/sysml bin/sysml-lsp
	absent bin/sysml-grpc bin/sysml-jupyter-kernel
	[[ -f "$prefix/share/man/man1/sysml.1" ]] && [[ -f "$prefix/share/man/man1/sysml-lsp.1" ]] ||
		{ echo "FAIL: man pages not installed" >&2; failures=$((failures + 1)); }
	said "Installing OpenSysML ($good) for $host"
	said "opensysml-$host.tar.gz verified"
	said "sysml $good"
	said "sysml-lsp $good"
	said "Installed: $prefix/bin/sysml $prefix/bin/sysml-lsp"
	said "$prefix/bin is not on PATH"
fi

if ok "tag without the v" --version "${good#v}" --prefix @P@; then
	said "Installing OpenSysML ($good)"
fi

if ok "all tools" --version "$good" --tools all --prefix @P@; then
	installed bin/sysml bin/sysml-lsp bin/sysml-grpc bin/sysml-jupyter-kernel
	said "sysml-grpc-$host verified"
	said "sysml-grpc version $good"
	said "sysml-jupyter-kernel-$host verified"
	said "sysml-jupyter-kernel version $good"
fi

if ok "the kernel alone" --version "$good" --tools sysml-jupyter-kernel --bin-dir @P@/k; then
	[[ -x "$prefix/k/sysml-jupyter-kernel" ]] || { echo "FAIL: $prefix/k/sysml-jupyter-kernel missing" >&2; failures=$((failures + 1)); }
	absent k/sysml k/sysml-lsp k/sysml-grpc
fi

if ok "one tool into a bin dir" --version "$good" --tools sysml --bin-dir @P@/b; then
	[[ -x "$prefix/b/sysml" ]] || { echo "FAIL: $prefix/b/sysml missing" >&2; failures=$((failures + 1)); }
	absent b/sysml-lsp share
	said "Installed: $prefix/b/sysml"
fi

if ok "latest through the redirect" --prefix @P@; then
	said "Installing OpenSysML ($good)"
	said "download/$good/opensysml-$host.tar.gz"
	installed bin/sysml bin/sysml-lsp
fi

if ok "latest from a mirror without the redirect" --base-url "$mirror" --prefix @P@; then
	said "Installing OpenSysML (the latest release)"
	said "$mirror/latest/download/opensysml-$host.tar.gz"
	installed bin/sysml bin/sysml-lsp
fi

if ok "nightly" --version nightly --prefix @P@; then
	said "Installing OpenSysML (the nightly snapshot)"
	said "sysml ${good}-5-gabcdef0"
	installed bin/sysml bin/sysml-lsp
fi

if ok "staged for windows" --version "$good" --os windows --prefix @P@; then
	[[ -f "$prefix/bin/sysml.exe" ]] && [[ -f "$prefix/bin/sysml-lsp.exe" ]] ||
		{ echo "FAIL: windows binaries not staged" >&2; failures=$((failures + 1)); }
	absent bin/sysml share
	said "opensysml-windows-amd64.zip verified"
	said "staged for windows-amd64, not run on this $host machine"
fi

if ok "staged for another unix platform" --version "$good" --os "${other%-*}" --arch "${other#*-}" --tools all --prefix @P@; then
	installed bin/sysml bin/sysml-lsp bin/sysml-grpc bin/sysml-jupyter-kernel
	said "staged for $other, not run on this $host machine"
fi

# --help must be complete when the script arrives on stdin, as the one-liner has it.
for how in file stdin; do
	n=$((n + 1))
	set +e
	case $how in
	file) out=$(sh "$root/install.sh" --help 2>&1) ;;
	stdin) out=$(cat "$root/install.sh" | sh -s -- --help 2>&1) ;;
	esac
	status=$?
	set -e
	if [[ "$status" -ne 0 ]] || [[ "$out" != *"--verify-signature   also verify"* ]] || [[ "$out" != *"[OPENSYSML_TOOLS]"* ]]; then
		echo "FAIL --help from $how: exit $status" >&2 && printf '%s\n' "$out" >&2
		failures=$((failures + 1))
	else
		echo "ok   --help from $how"
	fi
done

if ok "dry run" --version "$good" --tools all --dry-run --prefix @P@; then
	said "Dry run: nothing downloaded or installed."
	said "download/$good/sysml-grpc-$host"
	said "download/$good/sysml-jupyter-kernel-$host"
	[[ ! -e "$prefix" ]] || { echo "FAIL: dry run created $prefix" >&2; failures=$((failures + 1)); }
fi

if fails "tampered bundle" "opensysml-$host.tar.gz does not match SHA256SUMS.txt" --version "$tampered" --prefix @P@; then
	[[ ! -e "$prefix" ]] || { echo "FAIL: a tampered release left files under $prefix" >&2; failures=$((failures + 1)); }
fi
fails "asset the manifest omits" "does not list sysml-grpc-$host" --version "$unlisted" --tools sysml-grpc --prefix @P@
if fails "binary reporting another version" "reports 'sysml $good', not $mismatch" --version "$mismatch" --prefix @P@; then
	installed bin/sysml
fi
fails "unpublished release" "could not download $base/download/v0.0.0/SHA256SUMS.txt" --version v0.0.0 --prefix @P@
fails "unpublished windows arm64" "no windows/arm64 build is published" --os windows --arch arm64 --prefix @P@
fails "bad version" "--version must be a release tag" --version main --prefix @P@
fails "bad tool" "unknown tool 'sysml-repl'" --tools sysml-repl --prefix @P@
fails "no tool" "--tools names nothing to install" --tools , --prefix @P@
fails "bad os" "--os must be linux, darwin or windows" --os freebsd --prefix @P@
fails "bad option" "unknown option '--prefix-dir'" --prefix-dir @P@
fails "option without a value" "--prefix needs a value" --prefix

if [[ "$(id -u)" -ne 0 ]]; then
	mkdir -p "$work/readonly" && chmod 555 "$work/readonly"
	fails "unwritable destination" "cannot create $work/readonly/bin" --version "$good" --prefix "$work/readonly"
fi

# shim <tools...>: a PATH holding only the named commands, so the installer's
# fallbacks run without the tool it prefers.
shim() {
	local dir="$work/shim-$n" tool path
	rm -rf "$dir" && mkdir -p "$dir"
	for tool in "$@"; do
		path=$(command -v "$tool" 2>/dev/null) || return 1
		ln -s "$path" "$dir/$tool"
	done
	printf '%s\n' "$dir"
}
essentials=(sh uname mktemp mkdir rm cp mv chmod cat head tail tr sed awk cut tar gzip id dirname basename)

if command -v wget >/dev/null 2>&1 && shim_path=$(shim "${essentials[@]}" wget sha256sum); then
	n=$((n + 1)) && prefix="$work/install-$n"
	set +e
	out=$(PATH="$shim_path" sh "$root/install.sh" --base-url "$base" --prefix "$prefix" 2>&1)
	status=$?
	set -e
	if [[ "$status" -eq 0 ]]; then
		echo "ok   wget without curl" && said "Installing OpenSysML ($good)" && installed bin/sysml bin/sysml-lsp
	else
		echo "FAIL wget without curl: exit $status" >&2 && printf '%s\n' "$out" >&2 && failures=$((failures + 1))
	fi
else
	echo "skip wget without curl: wget or sha256sum not on PATH"
fi

if shim_path=$(shim "${essentials[@]}" curl sha256sum); then
	n=$((n + 1)) && prefix="$work/install-$n"
	set +e
	out=$(PATH="$shim_path" sh "$root/install.sh" --base-url "$base" --verify-signature --prefix "$prefix" 2>&1)
	status=$?
	set -e
	if [[ "$status" -ne 0 ]] && [[ "$out" == *"--verify-signature needs cosign on PATH"* ]]; then
		echo "ok   signature verification without cosign"
	else
		echo "FAIL signature verification without cosign: exit $status" >&2 && printf '%s\n' "$out" >&2 && failures=$((failures + 1))
	fi
else
	echo "skip signature verification without cosign: curl or sha256sum not on PATH"
fi

if command -v pwsh >/dev/null 2>&1; then
	pwsh -NoProfile -File "$root/scripts/install-test.ps1" -Script "$root/install.ps1" -BaseUrl "$base" -MirrorUrl "$mirror" \
		-HostPlatform "$host" -Good "$good" -Signed "$signed" -Tampered "$tampered" || failures=$((failures + 1))
else
	echo "skip install.ps1: pwsh not on PATH"
fi

if [[ "$failures" -ne 0 ]]; then
	echo "install-test: $failures failure(s)" >&2
	exit 1
fi
echo "install-test: all cases passed"
