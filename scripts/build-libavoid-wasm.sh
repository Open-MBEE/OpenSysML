#!/usr/bin/env bash
# Build internal/ir/view/route/libavoid.wasm, the libavoid router the view
# writers route edges with, from the libavoid-js release the diagram panel
# bundles. Emscripten built that release with the legacy exception-handling
# opcodes, which the Go WebAssembly runtime does not run, so binaryen's
# wasm-opt translates them to the standard exnref form; nothing else changes.
# Both packages are pinned by their npm integrity digests, and the result is
# pinned by sha256 in internal/ir/view/route/libavoid_test.go.
#
# Needs: node/npm, tar, sha256sum. Writes under build/libavoid/ and copies the
# translated module over internal/ir/view/route/libavoid.wasm.
set -euo pipefail

LIBAVOID_JS_VERSION="0.4.5"
LIBAVOID_JS_INTEGRITY="sha512-9BrYRXAQ+nmLuHZSqf4z52YN8TroBPxyqo6A6h6Pj03j5UYNx/Hhnd/rg+kiLVrE76wzBeBVO3OW7kaEpzYC9Q=="
LIBAVOID_WASM_SHA256="d17aa83a84896e65b15f4453913dac00953f4f698c7c271d43c5e93ea9ac16ac"
BINARYEN_VERSION="132.0.0"
BINARYEN_INTEGRITY="sha512-4Usg7yEvwO3FMso3Oov6qkOOHSod1IEv+boKkh77kTljM+eNBouqCTbqWswGzbG+vDNO/b/v2C4gfEh1WgB9Lg=="

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out="$root/internal/ir/view/route/libavoid.wasm"
work="$root/build/libavoid"
mkdir -p "$work"
cd "$work"

# sri_check verifies a file against an npm subresource-integrity digest.
sri_check() {
	local file="$1" integrity="$2"
	local algorithm="${integrity%%-*}" expected="${integrity#*-}"
	local actual
	actual="$(openssl dgst "-$algorithm" -binary "$file" | base64 -w0)"
	if [[ "$actual" != "$expected" ]]; then
		echo "$file: integrity $algorithm-$actual, want $integrity" >&2
		exit 1
	fi
}

fetch() {
	local name="$1" version="$2" integrity="$3"
	local tarball
	tarball="$(npm pack "$name@$version" --silent --pack-destination "$work" 2>/dev/null | tail -n 1)"
	sri_check "$work/$tarball" "$integrity"
	rm -rf "${work:?}/${name:?}"
	mkdir -p "$work/$name"
	tar -xzf "$work/$tarball" -C "$work/$name" --strip-components=1
}

fetch libavoid-js "$LIBAVOID_JS_VERSION" "$LIBAVOID_JS_INTEGRITY"
echo "$LIBAVOID_WASM_SHA256  libavoid-js/dist/libavoid.wasm" | sha256sum -c -
fetch binaryen "$BINARYEN_VERSION" "$BINARYEN_INTEGRITY"

node binaryen/bin/wasm-opt \
	--translate-to-exnref \
	--enable-exception-handling --enable-reference-types --enable-multivalue \
	--enable-bulk-memory --enable-mutable-globals --enable-sign-ext \
	--enable-nontrapping-float-to-int \
	-O0 \
	libavoid-js/dist/libavoid.wasm -o libavoid-exnref.wasm
features="--enable-exception-handling --enable-reference-types --enable-multivalue \
	--enable-bulk-memory --enable-mutable-globals --enable-sign-ext \
	--enable-nontrapping-float-to-int"
# shellcheck disable=SC2086 # the feature flags are meant to split
node binaryen/bin/wasm-as $features "$root/scripts/libavoid-throw.wat" -o throw.wasm
# shellcheck disable=SC2086
node binaryen/bin/wasm-merge $features \
	libavoid-exnref.wasm libavoid throw.wasm env -o libavoid.wasm
cp libavoid.wasm "$out"
sha256sum "$out"
