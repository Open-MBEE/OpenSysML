---
name: testing-errata-overlay
description: How to end-to-end test the declared-errata overlay (tools/oracle/errata + the errata blocks in tools/referee/diff, tools/referee/xpect, tools/referee/reject and the doccounts errata sentence) on Linux — proving the published corpora are never edited, that the pilot really re-runs over the corrected copy, and that registry provenance checks are load-bearing.
---

# Testing the declared-errata overlay

`tools/oracle/errata` declares defects in OMG-published reference material (file, line,
as-published bytes, corrected bytes, spec citation, derivation); the entry type, the overlay
applied on read and the bundled library's entries are `internal/workspace/libs/errata`, which the
registry builds on. Each oracle driver reports its
census twice: as published (the conformance statement) and over a **materialised corrected
copy**.

## Setup

Standard blueprint provisioning is enough (`scripts/download-pilot-corpora.sh`,
`download-pilot-sysml-validator.sh`, `download-pilot-kerml-validator.sh`,
`download-pilot-xpect.sh`; java 21 + go). `build/syside` is optional — without it the
differential is two-way, which is what the committed baselines record.

**Always use a fresh library cache per oracle run**: `XDG_CACHE_HOME=$(mktemp -d) go run -C tools ./cmd/pilot-diff`.
Stale on-disk index records have produced wrong oracle numbers before.

## Where the corrected copy actually lives

`Materialize` writes to `<out>/errata-corpora/<root>` (pilot-diff, pilot-xpect) and
`<out>/errata-corpus` (pilot-reject) — i.e. **under the oracle's `-out` directory**, not a
top-level `build/errata-corpora/`. Each copy and its parent are removed on the way out, so a
clean run leaves nothing: verify with
`find <out> -name 'errata-corpora' -o -name 'errata-corpus'` returning nothing.

## Immutability check that actually proves something

```bash
hash_tree(){ find "$1" -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum; }
hash_tree examples/pilot-corpora; hash_tree build/pilot-xpect-corpus   # before and after
git status --porcelain examples/
grep -rn '(22/2\*25.4 + 110)' examples/ build/pilot-xpect-corpus       # corrected bytes must not appear
```
Every write path in `tools/oracle/errata/errata.go` must be rooted at `dst`; `repo/dir` is only ever
read (`os.CopyFS(dst, os.DirFS(repo/dir))`).

## Proving the *pilot* re-runs over the corrected text (the important one)

A silently-zeroed pilot column (missing jar) would fake agreement. Wrap the validator:

```bash
cat > /tmp/wrap/validate-sysml-batch <<'EOF'
#!/bin/bash
echo "ARGV: $*" >> /tmp/pilot-wrap.log
prev=""; root=""
for a in "$@"; do [ "$prev" = "--root" ] && root="$a"; prev="$a"; done
f="$root/Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml"
[ -f "$f" ] && echo "  ROOT=$root line38=$(sed -n '38p' "$f")" >> /tmp/pilot-wrap.log
exec /path/to/build/pilot-sysml-validator/validate-sysml-batch "$@"
EOF
chmod +x /tmp/wrap/validate-sysml-batch
XDG_CACHE_HOME=$(mktemp -d) go run -C tools ./cmd/pilot-diff -validator /tmp/wrap/validate-sysml-batch -out /tmp/pd-wrap
```
Expect two logged invocations: one with `--root …/examples/pilot-corpora/sysml-examples` showing
the published line and one with `--root <out>/errata-corpora/pilot-examples` showing the
corrected line. Also assert `jq '.errata.totals.pilotDiagnostics'` > 0 and `.errata.totals.agreement` > 0.
The wrapped run's JSON differs from the committed baseline only in the `validator` field, so
compare with `diff <(jq -S 'del(.validator)' …) <(jq -S 'del(.validator)' docs/project/pilot-differential-baseline.json)`.

## Fast mini-repo trick for adversarial registry runs

A full `pilot-diff` run takes minutes. To exercise the oracle's rot guard cheaply, build a repo
containing only the erratum's file and point the oracle at it:

```bash
mkdir -p /tmp/mini/"examples/pilot-corpora/sysml-examples/Geometry Examples"
cp "examples/pilot-corpora/sysml-examples/Geometry Examples/VehicleGeometryAndCoordinateFrames.sysml" /tmp/mini/…/
XDG_CACHE_HOME=$(mktemp -d) go run -C tools ./cmd/pilot-diff -repo /tmp/mini \
  -validator $PWD/build/pilot-sysml-validator/validate-sysml-batch \
  -kerml-validator $PWD/build/pilot-kerml-validator/validate-kerml -out /tmp/pd-mini
```
Runs in seconds and still exercises materialisation, the pilot re-run and the finding. With a
mutated `AsPublished` it must exit 1 with `F82: …:38 reads "…", the entry records "…"`.

## Registry mutation matrix

`cp tools/oracle/errata/errata.go /tmp/bak` first (library entries: `internal/workspace/libs/errata/errata.go`),
apply one mutation at a time with a Python string replace (watch the trailing comma — a dropped
`,` yields a *build* failure, which is not evidence the check works), run
`go test -count=1 ./internal/workspace/libs/errata` and `go test -C tools -count=1 ./oracle/errata`, then restore and confirm
`git status --porcelain tools/oracle/errata internal/workspace/libs/errata` is empty. Mutations that must
fail: AsPublished byte, empty Citation, empty Derivation, wrong Line, no-op Corrected, second
entry for the same file, path outside `errata.Roots`.

## Doc gate

`make docs-counts` → `doc-counts: already current`; `go run -C tools ./cmd/doc-counts -check` → exit 0.
Mutating the generated "Declared errata:" sentence in `README.md` (e.g. `325 of 353` → `326 of 353`)
must make `-check` exit 1 with `README.md is stale` and a line diff; restore with `git checkout README.md`.

## Devin Secrets Needed

None.
