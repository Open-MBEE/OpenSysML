#!/usr/bin/env bash
# Build Sensmetry SysIDE (sysml-2ls) into build/syside/, for the optional third
# column of the differential harness in tools/referee/diff. See F7 in
# docs/project/pilot-differential.md.
#
# SysIDE is an independent SysML v2/KerML implementation in TypeScript. It is a
# static checker only — it parses, resolves names and runs the KerML/SysML
# validation rules; it executes nothing — and the pinned release ships the
# 2024-12 standard library, one release behind the pilot pin in
# scripts/pilot-pin.sh. The harness records both facts in its report.
#
# Everything is pinned: the SysIDE tag, the standard library release SysIDE
# itself declares, and the version the checkout reports. A pin that does not
# match exits non-zero before anything is written to build/syside.
set -euo pipefail

SYSIDE_REPO="${SYSIDE_REPO:-https://github.com/sensmetry/sysml-2ls.git}"
SYSIDE_TAG="${SYSIDE_TAG:-0.9.1}"
# The SysML v2/KerML release SysIDE's bundled standard library is taken from,
# and the fork it is fetched from, both as declared by SYSIDE_TAG itself.
SYSIDE_SPEC="${SYSIDE_SPEC:-2024-12}"
SYSIDE_STDLIB_REPO="${SYSIDE_STDLIB_REPO:-https://github.com/daumantas-kavolis-sensmetry/SysML-v2-Release.git}"
SYSIDE_STDLIB_BRANCH="${SYSIDE_STDLIB_BRANCH:-release/$SYSIDE_SPEC}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source="$repo_root/scripts/syside-validator/validate-syside.cjs"
target="$repo_root/build/syside"
checkout="$target/sysml-2ls"
library="$target/sysml.library"
launcher="$target/validate-syside"
pin="$target/syside-pin.txt"
force=0

case "${1:-}" in
	--force)
		force=1
		shift
		;;
	"") ;;
	*)
		echo "error: unknown option: $1 (only --force is supported)" >&2
		exit 1
		;;
esac

if [[ "$force" -eq 0 ]] && [[ -x "$launcher" ]] && [[ -f "$pin" ]] && [[ -d "$library" ]]; then
	echo "SysIDE already built at $target"
	echo "Remove that directory, or pass --force, to re-provision."
	exit 0
fi

if [[ ! -f "$source" ]]; then
	echo "error: SysIDE validator driver not found at $source" >&2
	exit 1
fi

for tool in git node pnpm; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "error: $tool is required to build SysIDE" >&2
		echo "       SysIDE is a TypeScript implementation: install Node 18+ and pnpm" >&2
		exit 1
	fi
done

node_major="$(node --version | sed -n 's/^v\([0-9][0-9]*\).*/\1/p')"
if [[ -z "$node_major" ]] || [[ "$node_major" -lt 18 ]]; then
	echo "error: SysIDE requires Node 18+, found: $(node --version)" >&2
	exit 1
fi

# Staged outside the target so a failed pin check leaves no artifact behind.
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "Cloning $SYSIDE_REPO at $SYSIDE_TAG ..."
if ! git clone --quiet --depth 1 --branch "$SYSIDE_TAG" "$SYSIDE_REPO" "$work/sysml-2ls" 2>"$work/clone.log"; then
	sed 's/^/  /' "$work/clone.log" >&2
	echo "error: cloning $SYSIDE_REPO at tag $SYSIDE_TAG failed" >&2
	exit 1
fi

version="$(node -p "require('$work/sysml-2ls/package.json').version" 2>/dev/null || true)"
if [[ "$version" != "$SYSIDE_TAG" ]]; then
	echo "error: $SYSIDE_REPO at $SYSIDE_TAG reports version ${version:-<none>}, expected $SYSIDE_TAG" >&2
	echo "       re-pin SYSIDE_TAG deliberately rather than comparing against an unknown release" >&2
	exit 1
fi

stdlib_source="$work/sysml-2ls/packages/syside-base/src/stdlib.ts"
if [[ ! -f "$stdlib_source" ]]; then
	echo "error: $stdlib_source is missing: SysIDE $SYSIDE_TAG does not declare its standard library" >&2
	exit 1
fi
declared_spec="$(sed -n 's/^[[:space:]]*version:[[:space:]]*"\([^"]*\)".*/\1/p' "$stdlib_source" | head -1)"
if [[ "$declared_spec" != "$SYSIDE_SPEC" ]]; then
	echo "error: SysIDE $SYSIDE_TAG targets the $declared_spec standard library, this script pins $SYSIDE_SPEC" >&2
	echo "       re-pin SYSIDE_SPEC, and re-check what the third column may honestly be compared against" >&2
	exit 1
fi
if ! grep -Fq "SysML-v2-Release/${SYSIDE_STDLIB_BRANCH}/sysml.library/" "$stdlib_source"; then
	echo "error: SysIDE $SYSIDE_TAG does not take its standard library from $SYSIDE_STDLIB_BRANCH" >&2
	echo "       re-pin SYSIDE_STDLIB_BRANCH to the branch $stdlib_source names" >&2
	exit 1
fi

echo "Fetching the $SYSIDE_SPEC standard library from $SYSIDE_STDLIB_REPO ..."
if ! git clone --quiet --filter=blob:none --sparse --depth 1 \
	--branch "$SYSIDE_STDLIB_BRANCH" "$SYSIDE_STDLIB_REPO" "$work/stdlib" 2>"$work/stdlib.log"; then
	sed 's/^/  /' "$work/stdlib.log" >&2
	echo "error: cloning $SYSIDE_STDLIB_REPO at $SYSIDE_STDLIB_BRANCH failed" >&2
	exit 1
fi
git -C "$work/stdlib" sparse-checkout set sysml.library
if [[ ! -d "$work/stdlib/sysml.library" ]]; then
	echo "error: sysml.library is missing from $SYSIDE_STDLIB_REPO at $SYSIDE_STDLIB_BRANCH" >&2
	exit 1
fi

# Every library file SysIDE expects must be present, so a silently reduced
# library cannot look like a clean run.
missing="$(node -e '
const fs = require("fs");
const path = require("path");
const source = fs.readFileSync(process.argv[1], "utf8");
const files = source.slice(source.indexOf("files: ["), source.indexOf("],", source.indexOf("files: [")))
	.match(/"([^"]+)"/g) ?? [];
const absent = files
	.map((quoted) => quoted.slice(1, -1))
	.filter((file) => !fs.existsSync(path.join(process.argv[2], file)));
process.stdout.write(absent.slice(0, 5).join(", "));
' "$stdlib_source" "$work/stdlib/sysml.library")"
if [[ -n "$missing" ]]; then
	echo "error: the $SYSIDE_SPEC library at $SYSIDE_STDLIB_BRANCH is missing files SysIDE expects: $missing" >&2
	exit 1
fi

# --ignore-scripts skips the VS Code extension bundle the prepare script builds;
# the language server this harness drives is built explicitly below.
echo "Installing dependencies and building the SysIDE language server ..."
(cd "$work/sysml-2ls" && pnpm install --frozen-lockfile --ignore-scripts >/dev/null)
(cd "$work/sysml-2ls" && pnpm run prebuild >/dev/null && pnpm exec tsc -b tsconfig.build.json)

built="$work/sysml-2ls/packages/syside-languageserver/lib/index.js"
if [[ ! -f "$built" ]]; then
	echo "error: the SysIDE language server did not build: $built is missing" >&2
	exit 1
fi

rm -rf "$target"
mkdir -p "$target"
mv "$work/sysml-2ls" "$checkout"
mv "$work/stdlib/sysml.library" "$library"
cp "$source" "$target/validate-syside.cjs"

# Always rewritten, so a changed pin never leaves a launcher pointing at the old
# checkout.
cat >"$launcher" <<EOF
#!/bin/sh
set -e

SCRIPT_DIR="\$(cd "\$(dirname "\$0")" && pwd)"

if [ "\${1:-}" = "--version" ]; then
	echo "syside $SYSIDE_TAG (SysML v2/KerML $SYSIDE_SPEC standard library)"
	exit 0
fi

SYSIDE_HOME="\$SCRIPT_DIR/sysml-2ls"
export SYSIDE_HOME

has_library=0
for arg in "\$@"; do
	[ "\$arg" = "--library" ] && has_library=1
done
if [ "\$has_library" -eq 0 ]; then
	set -- --library "\$SCRIPT_DIR/sysml.library" "\$@"
fi

# --no-warnings: SysIDE's dependencies emit ExperimentalWarning on stderr, where
# the caller reads diagnostics.
exec node --no-warnings "\$SCRIPT_DIR/validate-syside.cjs" "\$@"
EOF
chmod +x "$launcher"

cat >"$pin" <<EOF
SYSIDE_VERSION=$SYSIDE_TAG
SYSIDE_SPEC=$SYSIDE_SPEC
EOF

echo "Built $launcher (SysIDE $SYSIDE_TAG, $SYSIDE_SPEC standard library)"
echo "The differential harness picks it up automatically:"
echo "  go run -C tools ./cmd/pilot-diff"
