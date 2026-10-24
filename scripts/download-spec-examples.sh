#!/usr/bin/env bash
# Download the SysML v2 and KerML specification PDFs at the pinned release and
# extract their textual-notation examples into examples/spec-examples/, for the
# spec-examples gate in tests/corpus (TestSpecExamples) and the pilot
# differential (tools/referee/diff).
#
# The PDFs are published in the pilot's release repository under the same tag as
# the pilot itself, so the release is pinned in scripts/pilot-pin.sh next to the
# pilot's. They are rendered with `pdftotext -layout` into build/spec-docs/ and
# tools/cmd/spec-examples reads that text alone; its extraction rules and the
# per-file completion are documented in docs/project/spec-examples.md. The
# extracted files are not vendored: they are the OMG's text, licensed there.
# The gate skips while the directory is absent, so this script is optional for
# building and testing; CI sets OPENSYSML_REQUIRE_SPEC_EXAMPLES=1 so an absent
# copy fails there. The download records the pin in a .pilot-pin stamp and is
# re-fetched when the stamp does not match.
set -euo pipefail

# shellcheck source=scripts/pilot-pin.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/pilot-pin.sh"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
docs="$repo_root/build/spec-docs"
target="$repo_root/examples/spec-examples"

if ! command -v pdftotext >/dev/null 2>&1; then
	echo "error: pdftotext (poppler-utils) is required to render the specification PDFs" >&2
	exit 1
fi

pilot_from_release
PILOT_FETCH_GLOBS=('*.pdf')
pilot_fetch_subtrees "doc:$docs/pdf"

sysml_pdf="$docs/pdf/2a-OMG_Systems_Modeling_Language.pdf"
kerml_pdf="$docs/pdf/1-Kernel_Modeling_Language.pdf"
for pdf in "$sysml_pdf" "$kerml_pdf"; do
	if [[ ! -f "$pdf" ]]; then
		echo "error: $pdf is missing from the release documents" >&2
		exit 1
	fi
done

# The PDFs are cached by pin above; the extraction always reruns, since the
# fixtures depend on the extractor and the library index as much as on the text.
pin="$(cat "$docs/pdf/.pilot-pin")"
pdftotext -layout "$sysml_pdf" "$docs/sysml.txt"
pdftotext -layout "$kerml_pdf" "$docs/kerml.txt"
printf '%s\n' "$pin" >"$docs/.pilot-pin"
(cd "$repo_root/tools" && go run ./cmd/spec-examples -repo "$repo_root" -text "$docs" -out "$target")

count="$(find "$target" -name '*.sysml' -o -name '*.kerml' | wc -l)"
if [[ "$count" -eq 0 ]]; then
	echo "error: no example was extracted into $target" >&2
	exit 1
fi
echo "Total $count specification example(s)."
echo "Gate them with:"
echo "  go test -count=1 ./tests/corpus -run TestSpecExamples"
echo "Compare them against the pilot implementation with:"
echo "  go run -C tools ./cmd/pilot-diff"
