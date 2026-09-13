---
name: testing-grammar-coverage
description: How to verify the advisory grammar-production coverage harness (cmd/grammar-coverage + scripts/download-pilot-grammars.sh) end to end on Linux — provisioning the pinned OMG Xtext grammars, reproducing the committed compact baseline, checking the per-file-evidence property, and the adversarial paths worth trying.
---

# Testing the grammar-coverage harness (`cmd/grammar-coverage`)

Sibling of the pilot-differential harness (see `testing-pilot-differential/SKILL.md` — the same
pin, the same "committed artifact, testable by reproduction" shape, the same Konsole recording
setup). This one parses the pinned OMG Xtext grammars and buckets every production as
evidence / no-evidence / indistinguishable from *literal presence in one corpus file*, plus
form-level unseen diagnostics.

## Prerequisites

- Go on PATH (`export PATH=/usr/local/go/bin:$PATH`), `git`, `jq`, network to github.com.
- `./scripts/download-pilot-grammars.sh` — run by the blueprint's `initialize`, and cheap (~2 s
  sparse clone, unlike the multi-minute validator build), so re-provisioning from scratch is free
  and should always be part of the test.
- All eight corpus roots must be present, which takes two more downloads: the OMG training corpus
  (`./scripts/download-training-examples.sh`) and the OMG example/validation corpora
  (`./scripts/download-pilot-corpora.sh`, which writes `examples/pilot-corpora/`, 212 of the files).
  A missing corpus silently lowers the file count and shifts every bucket. Compare the
  `searched N corpus file(s)` line with the count implied by the committed baseline's `roots`
  entries (the sum of their `files` values). A mismatch means either the corpus or the committed
  baseline is stale; do not diagnose it as a missing corpus from the count alone. No test enforces
  reproduction of the compact baseline, so it can go stale silently when fixtures are added.

## The core check

```bash
rm -rf build/pilot-grammars build/grammar-coverage
./scripts/download-pilot-grammars.sh          # 3 .xtext + PILOT_TAG (2026-08) under build/pilot-grammars
go run ./cmd/grammar-coverage -baseline /tmp/nb.json     # ~15 s
cmp /tmp/nb.json docs/project/grammar-coverage-baseline.json    # must be silent
```

Only the **compact** baseline is committed (counts + corpora + the gap rows: no-evidence rows and
rows with an unseen form, ~11 KB). The full per-row JSON, the Markdown tables and the text summary
are build-only under `build/grammar-coverage/{grammar-coverage.json,grammar-coverage-tables.md,grammar-coverage.txt}`,
so **do not expect `-out docs/project` to reproduce the committed tree** — older revisions
committed all three artifacts and used `-out docs/project`; if a doc still says that, it is stale.

Determinism (the point of the committed baseline): run twice with different `-out` and `-baseline`
targets and `diff -r` / `cmp` the pairs. Observed byte-identical at `04afba74`.

Verify idempotence of the provisioning script by **mtime**, not by its message:
`ls --time-style=+%T -l build/pilot-grammars` before and after the second run. Also assert nothing
was vendored: `git status --porcelain | grep -c xtext` must be `0`.

## The property that actually matters: per-file evidence

A row may only be credited when **one** corpus file holds all the literals of a path. Check it
mechanically over the full JSON rather than by eye:

```bash
B=build/grammar-coverage/grammar-coverage.json
jq '[.grammars[].productions[]|select(.bucket=="evidence")|select((.evidence|map(.file)|unique|length)>1)]|length' $B   # 0
jq '[.grammars[].productions[]|select(.bucket=="evidence")|. as $p|select(.evidence|map(.file != $p.file)|any)]|length' $B  # 0
```

Then spot-check honesty: for a few `evidence` rows, `sed -n '<line>p' <cited file>` and confirm the
literal is really there. Expect the *documented* approximations, and do not report them as bugs:
literal position is ignored (a `def` cited from `calc def` credits `PortDefinition`), punctuation is
a substring match, and a path is a lower bound. Unseen forms are the JSON `branches[]` entries that
have **no** `file` key (5 of them at `04afba74`).

## Adversarial paths (expected at `04afba74`)

| Case | Expected |
|---|---|
| grammars absent, plain run | exit 1, `grammars not found at <repo>/build/pilot-grammars: run ./scripts/download-pilot-grammars.sh` |
| `-grammars <empty dir>` | exit 1, `no .xtext grammars in …: run ./scripts/download-pilot-grammars.sh` |
| `-repo /tmp/notrepo` | exit 1, `no .sysml or .kerml files found under /tmp/notrepo: is -repo right?` (fatal, unlike `pilot-diff`, which only warns) |
| `-out /nonexistent-root/x` | exit 1, `mkdir /nonexistent-root: permission denied`, nothing written |
| `-baseline /nonexistent-dir/b.json` / `-baseline /tmp` | exit 1, `open …: no such file or directory` / `open /tmp: is a directory` |
| truncated grammar (`head -c 4000 KerML.xtext`) | exit 1, `KerML.xtext: production NamespaceBodyElement: line 143: expected ";", found ""` — a parse error, never a panic and never still 176 productions |
| `PILOT_TAG=9999-99 ./scripts/download-pilot-grammars.sh` | exit 1, `error: could not clone <repo> at 9999-99, the tag scripts/pilot-pin.sh pins` after git's own `fatal:` lines; nothing is provisioned, which is the property that matters |
| `PILOT_COMMIT=<other sha> ./scripts/download-pilot-grammars.sh` | exit 1, `error: <repo> tag 2026-08 resolves to 692170b7…, scripts/pilot-pin.sh pins <other sha>` — the shared `pilot_clone` refuses a tag that no longer resolves to the pinned commit; nothing is provisioned |
| second run | `Grammars already present at build/pilot-grammars (pin 2026-08 692170b7…)`, exit 0; a directory with a different or missing `.pilot-pin` stamp is re-downloaded whole |

`mv build/pilot-grammars` aside instead of deleting it if you want the original bytes back — and
beware moving it onto an existing backup path, which nests it (`build/pilot-grammars/pilot-grammars`).

## Doc adjudication claims worth re-checking

`docs/project/grammar-coverage.md` asserts parser behavior; verify it, do not fix it:
`go run ./cmd/sysml -e '7 % 3'` → `1`; `#Meta namespace N { part def A; }` analyses clean **only
with a `metadata def Meta;` in scope** (the bare form fails with `unresolved reference: Meta`, which
is not a defect); and `disjoining …`, `conjugation …`, `redefinition …` (plus `specialization …`)
are each rejected with `expected a namespace member`.

**Always redirect stdin** when driving `./cmd/sysml <file>` from a recorded Konsole: after analysing
the file it drops into the REPL and blocks, so the next typed command is swallowed as SysML. Use
`go run ./cmd/sysml file </dev/null`.

## Measuring an extra corpus root

There is no flag for extra scanned roots (`cmd/grammar-coverage` takes only `-repo`, `-grammars`,
`-out`, `-baseline`), so claims of the form "corpus X closes the unseen-form gap" need a temporary
`corpusRoot` appended to `evidenceRoots` in `cmd/grammar-coverage/corpus.go`, then reverted. Example
at `PILOT_TAG=2026-07`: adding `cmd/pilot-reject/testdata/negative` contributes 119 files / 842
lines and takes unseen forms 5 → 0 with `indistinguishable` unchanged at 244. Such a configuration
is not reproduced by CI, so report it as a local measurement, not a baseline movement.

## Recording

CLI work: maximized Konsole on `DISPLAY=:0`, `ctrl+plus` a few times for font size. Put each phase
in a small `/tmp/*.sh` so the video shows a labelled block with a visible pass line
(`cmp … && echo "REPRODUCED BYTE-FOR-BYTE"`) instead of raw output — a bare run prints only four
progress lines and nothing checkable. Heredocs and nested `jq` quoting typed into Konsole break
easily; scripts avoid that entirely.

## Devin Secrets Needed

None. Network access to github.com is required only to provision the grammars.
