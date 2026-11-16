#!/usr/bin/env bash

# Moves one count of each count baseline and requires the family's check to fail, restoring the file. Usage:
# scripts/baseline-injection-test.sh [family...], with the corpora and pilot tools provisioned (AGENTS.md §6.2).
set -uo pipefail

census="go run -C tools ./cmd/derived-census -check -recorded-pilot"
apijson="go run -C tools ./cmd/pilot-apijson-diff -check -pilot-record build/pilot-apijson-record"
structure="go test -count=1 ./internal/translate/export -run TestSymbolStructureMatchesGraphStructure"

# family | file | line to move (a regular expression; its last standing count moves) | check
families=(
	"census-records-ours|internal/semantic/metamodel/derived_census_records.tsv|^Feature::type	training	|$census"
	"census-records-pilot|internal/semantic/metamodel/derived_census_records.tsv|^Feature::type	training	|$census"
	"census-baseline|internal/semantic/metamodel/derived_census_baseline.json|^ *\"elements\": [0-9]+,\$|$census"
	"structure-differential|internal/translate/export/testdata/structure_differential_expected.txt|^  [A-Za-z]+::[A-Za-z]+: equal [0-9]+,|$structure"
	"apijson-index|docs/project/api-json-differential/index.json|^ *\"presentEqual\": [0-9]+,\$|$apijson"
	"apijson-fragment|$(git ls-files 'docs/project/api-json-differential/training/*.json' | head -1)|^ *\"presentEqual\": [0-9]+,\$|$apijson"
	"pilot-differential|docs/project/pilot-differential-baseline.json|^ *\"agreement\": [0-9]+,\$|go run -C tools ./cmd/pilot-diff -check"
)

# move FILE PATTERN FIELD adds one to a standing count of the first matching line: the last count,
# or the FIELD-th tab-separated one.
move() {
	python3 - "$1" "$2" "${3:-}" <<'PY'
import re, sys
path, pattern, field = sys.argv[1], re.compile(sys.argv[2]), sys.argv[3]
lines = open(path, encoding="utf-8").read().split("\n")
for i, line in enumerate(lines):
    if not pattern.search(line):
        continue
    if field:
        cells = line.split("\t")
        cells[int(field)] = str(int(cells[int(field)]) + 1)
        lines[i] = "\t".join(cells)
    else:
        counts = list(re.finditer(r"(?:(?<=[ \t])|^)[0-9]+(?=[ \t,]|$)", line))
        if not counts:
            continue
        last = counts[-1]
        lines[i] = line[: last.start()] + str(int(last.group()) + 1) + line[last.end() :]
    open(path, "w", encoding="utf-8").write("\n".join(lines))
    print(f"  moved {path}:{i + 1}: {line.strip()!r} -> {lines[i].strip()!r}")
    sys.exit(0)
sys.exit(f"no line of {path} matches {sys.argv[2]}")
PY
}

failed=0
for family in "${families[@]}"; do
	IFS='|' read -r name file pattern check <<<"$family"
	if [ "$#" -gt 0 ] && ! printf '%s\n' "$@" | grep -qx "$name"; then
		continue
	fi
	echo "== $name: $check"
	if ! $check >/dev/null 2>"build/injection-$name.log"; then
		echo "  FAIL: the check does not pass on the unmoved baseline (build/injection-$name.log)"
		failed=1
		continue
	fi
	saved=$(mktemp)
	cp "$file" "$saved"
	field=""
	case "$name" in
	census-records-ours) field=3 ;;
	census-records-pilot) field=7 ;;
	esac
	if ! move "$file" "$pattern" "$field"; then
		failed=1
	elif $check >/dev/null 2>"build/injection-$name.log"; then
		echo "  FAIL: the check passes over the moved baseline"
		failed=1
	else
		echo "  ok: the check fails: $(grep -v '^\.*$' "build/injection-$name.log" | grep -iv '^exit status' | tail -1 | cut -c1-200)"
	fi
	cp "$saved" "$file"
	rm -f "$saved"
done
exit "$failed"
