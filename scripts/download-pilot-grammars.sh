#!/usr/bin/env bash
# Download the OMG Xtext grammars the grammar-coverage measurement reads, into
# build/pilot-grammars/.
#
# Like every other corpus we compare against, the grammars are not vendored:
# they belong to the OMG pilot implementation and are licensed there. The
# measurement is advisory and nothing in the build or the test suite reads the
# download, so this script is optional for building and testing.
#
# The release is pinned in scripts/pilot-pin.sh, the same pin the training
# corpus, the additional OMG corpora and the reference validator use, so the
# grammars and the models are always from one release. The directory records
# that pin in a .pilot-pin stamp and is re-downloaded when the stamp does not
# match.
set -euo pipefail

# shellcheck source=scripts/pilot-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

# Each entry is "<path in the pilot repository>:<file name under build/pilot-grammars>".
GRAMMARS=(
	"org.omg.kerml.expressions.xtext/src/org/omg/kerml/expressions/xtext/KerMLExpressions.xtext:KerMLExpressions.xtext"
	"org.omg.kerml.xtext/src/org/omg/kerml/xtext/KerML.xtext:KerML.xtext"
	"org.omg.sysml.xtext/src/org/omg/sysml/xtext/SysML.xtext:SysML.xtext"
)

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="$repo_root/build/pilot-grammars"
stamp="$target/.pilot-pin"
pin="$(pilot_pin)"

if [[ -f "$stamp" ]] && [[ "$(cat "$stamp")" == "$pin" ]]; then
	echo "Grammars already present at $target (pin $PILOT_TAG $PILOT_COMMIT)"
	echo "Remove that directory to re-download."
	exit 0
fi
if [[ -f "$stamp" ]]; then
	echo "Stale pin at $target: fetched from $(cat "$stamp"), pin is now $pin; re-downloading."
elif [[ -d "$target" ]]; then
	echo "No pin recorded at $target: it predates the stamp or was fetched by hand; re-downloading at $PILOT_TAG."
fi

declare -a wanted_paths=()
declare -a wanted_names=()
for grammar in "${GRAMMARS[@]}"; do
	wanted_paths+=("${grammar%%:*}")
	wanted_names+=("${grammar#*:}")
done

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

pilot_clone "$work/pilot" "${wanted_paths[@]}"

# Assembled aside and installed whole, so a refresh cannot leave a grammar
# from the previous pin beside the new ones.
staged="$work/grammars"
mkdir -p "$staged"
for index in "${!wanted_paths[@]}"; do
	source_path="${wanted_paths[$index]}"
	name="${wanted_names[$index]}"
	if [[ ! -f "$work/pilot/$source_path" ]]; then
		echo "error: $source_path is missing from $PILOT_REPO at $PILOT_TAG" >&2
		exit 1
	fi
	mv "$work/pilot/$source_path" "$staged/$name"
	lines="$(wc -l <"$staged/$name" | tr -d ' ')"
	echo "Downloaded $name ($lines lines) from $source_path"
done

# Recorded so the report can name the release it measured.
printf '%s\n' "$PILOT_TAG" >"$staged/PILOT_TAG"
printf '%s\n' "$pin" >"$staged/.pilot-pin"

pilot_install_dir "$staged" "$target"

echo "Measure our production coverage against them with:"
echo "  go run -C tools ./cmd/grammar-coverage"
