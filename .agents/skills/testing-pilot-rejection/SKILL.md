---
name: testing-pilot-rejection
description: How to verify the advisory pilot rejection oracle (cmd/pilot-reject + its committed negative corpus) end to end on Linux — provisioning the two pinned reference validators, reproducing the committed baseline, proving determinism, extending the corpus, and inspecting permissiveness gaps.
---

# Testing the pilot rejection oracle (`cmd/pilot-reject`)

Sibling of `testing-pilot-differential` and `testing-pilot-xpect` (same pin
`scripts/pilot-pin.sh`, same committed-baseline shape), but pointed the other way: the
differential measures what the reference accepts and we reject; this oracle measures what the
reference **rejects and we accept** — permissiveness gaps. Its corpus is committed under
`cmd/pilot-reject/testdata/negative/` (301 hand-written invalid models, one violated rule + citation
in each file's mandatory `// Invalid: ...` first line), so no corpus download exists. Method and
findings: `docs/project/pilot-rejection.md`.

## Prerequisites

- `export PATH=/usr/local/go/bin:$PATH`, Java 17+, Maven, network to github.com and Maven Central.
  No secrets of any kind are required.
- `./scripts/download-pilot-reject-validators.sh` — provisions both pinned reference validators
  (`build/pilot-sysml-validator/validate-sysml-batch` and
  `build/pilot-kerml-validator/validate-kerml`) by delegating to their own download scripts.
  First run downloads the pinned pilot jar (~minutes); re-runs are no-ops without `--force`.

## The core check (~10 s per run once provisioned)

```bash
rm -rf build/pilot-reject && go run ./cmd/pilot-reject
cmp build/pilot-reject/pilot-reject.json docs/project/pilot-rejection-baseline.json   # must be silent
```

`docs/project/pilot-rejection-baseline.json` is the only authority for the counts; the numbers
quoted here are as-of values, and `cmd/pilot-reject/doc_counts_test.go` fails if they drift from it.
As of the `semantic/` source (named pilot constraints, KerML and SysML, the control-node
succession rules, the feature-value overriding rule, the enumeration-variation rules, the send-action cases,
the metadata typing, annotated-element and body rules, the trigger-argument typing rules, the owning-body member rules, the cross-subsetting rules, the variant port rule, the association arity, binary-link end
count and multiplicity-bound typing rules, the result-expression ownership rules, and the annotation-ownership, binding-arity, feature-chain conformance,
conjugated typing and end-feature rules, the end-multiplicity, return-owner and single-conjugator rules, the keyword-first relationship end kinds, and the prefix and body-context cases), with a fresh library cache:
`301 case(s): 292 both reject, 0 only the pilot rejects, 9 only we reject, 0 both accept`,
byte-identical to the committed baseline. Any `both accept` case is a bug in the corpus (the case
is not actually invalid under the loaded standard library) — fix the case, never ignore it. A
candidate the pilot accepts because it does not enforce the named constraint is not a case either:
record it under "Constraints the pilot declares but does not enforce" in
`docs/project/pilot-rejection.md` with the evidence from the pinned `KerMLValidator` or
`SysMLValidator` source.

## Conformance policy (`-conformance auto|default|strict`)

The baseline is the default `auto` policy: the `extensions/` cases are judged under strict
conformance (OpenSysML notation the reference rejects as a syntax error), everything else in the
default mode. The report names each case's mode and lists the four strict-only agreements
separately, so a strict agreement never reads as a default one.

```bash
go run ./cmd/pilot-reject -conformance default -out build/pilot-reject-default
# as of the `semantic/` source: 193 agreements, 25 gaps — the numbers strict mode leaves alone
go run ./cmd/pilot-reject -conformance lenient   # must fail: unknown conformance policy
```

The `default` numbers are the ones the other waves' rules produce and must not move: strict mode is
opt in and changes no default verdict.

## Determinism

The reports carry no timestamps and no absolute paths; case order is the sorted corpus paths.
Prove it with two independent runs:

```bash
go run ./cmd/pilot-reject && go run ./cmd/pilot-reject -out build/pilot-reject-2
cmp build/pilot-reject/pilot-reject.json build/pilot-reject-2/pilot-reject.json   # must be silent
rm -rf build/pilot-reject-2
```

## Adversarial behavior worth testing

- **A corpus file without the mandatory header** fails the run with
  `first line must be `// Invalid: <rule> (<citation>).`` — the header is what keeps every case
  non-anecdotal.
- **Missing validators** fail fast with the provisioning command to run, before any validation.
- **Unattributable pilot output** (a diagnostic line for a path outside the corpus) is echoed to
  stderr, never silently dropped — a swallowed error would masquerade as an acceptance.
- **Warnings do not count as rejection** on either side; only error-severity diagnostics do.

## Extending the corpus

Add a file under the subdirectory naming its derivation (`grammar/`, `extensions/`, `xpect/`,
`semantic/`), give it the mandatory first-line header, re-run, and inspect its bucket. A
`semantic/` header cites the pilot's constraint name: `// Invalid: <rule> (<clause>; pilot
validate<Name>).`, so grep the corpus for the name before authoring to avoid a duplicate. Referee
the candidate on both sides before committing it:

```bash
build/pilot-sysml-validator/validate-sysml-batch --root <dir> <dir>/<file>.sysml   # empty output = accepted
build/pilot-kerml-validator/validate-kerml <dir>/<file>.kerml
./bin/sysml -validate <dir>/<file>.sysml
```

Then recommit the baseline
(`go run ./cmd/pilot-reject -update`) and update
the counts and gap table in `docs/project/pilot-rejection.md`, the README's rejection-oracle line,
and the headline above. `TestPilotRejectionDocumentCountsMatchBaseline` (CI-cheap, reads only
committed files) fails on any stale count, and on a gap table that does not enumerate exactly the
baseline's `pilot-only-rejects` cases:

```bash
go test -count=1 ./cmd/pilot-reject
```

## Inspecting a gap

Every `pilot-only-rejects` case keeps the pilot's error messages in the JSON as evidence:

```bash
jq '.cases[] | select(.bucket=="pilot-only-rejects") | {path, rule, pilot}' build/pilot-reject/pilot-reject.json
```

The corpus file itself is the minimal reproducer. `docs/project/pilot-rejection.md` maps each gap
to the package its root cause is likely in. Gaps are findings to report, not to fix from this
harness's PRs.

## Verifying a severity change that closes gaps (notation escalations)

When a PR escalates a notation diagnostic so cases move from `pilot-only-rejects` into
`both reject`, the live harness is expected to *disagree* with the committed baseline until its
separate refresh, so `cmp` against the baseline is the wrong check. Attribute the delta instead:

```bash
git worktree add /home/ubuntu/wt-main origin/main
# run the harness from main, but over the branch checkout's corpus and the provisioned validators
cd /home/ubuntu/wt-main && go run ./cmd/pilot-reject -repo /path/to/branch/checkout -out /tmp/mn
```

A `main` run whose JSON is byte-identical to `docs/project/pilot-rejection-baseline.json` proves the
baseline is not otherwise stale, so any branch/main difference belongs to the PR. Build a
`main`-binary control (`go build -o /tmp/sysml-main ./cmd/sysml`) too — a severity flip is only
demonstrated by showing `warning:` on main and `error:` on the branch at the *same* span and code.

Useful surfaces, cheapest first:

- **CLI:** `sysml -validate -debug <file>` prints `[syntax/<code>]` with `-debug`, so span+code
  stability is checkable in one line.
- **Editor:** drive `bin/sysml-lsp` over stdio and read `diagnostics[].severity` (`1` = error,
  `2` = warning). `initializationOptions.strictConformance = true` selects strict. A default-mode
  `2 → 1` flip is the cheapest proof the change reached the editor path.
- **REPL:** `%strict on` re-runs the buffer, so both modes are observable in one session.

**Error severity does not mean a non-zero exit.** Notation diagnostics set `Notation = true`, so
`Diagnostic.Blocking()` stays false in default mode: the CLI prints `error:` yet still exits **0**
with `no error that stops a check; the notation reported above does not conform`, and
`-instantiate`/`-constraint` still succeed. Only strict mode exits 2. Expect this and assert it
deliberately — treating exit 0 as a failure will send you chasing a non-bug, and asserting only the
exit code would miss the change entirely.

Adversarial checks worth running for such a PR:

- **No span carries both a warning and an error** (the escalated error must replace, not duplicate,
  the parser warning). Keep a pre-escalation build as a *live* vacuity control: it should show one
  span with both severities where the current build shows zero.
- **Strict must never be more permissive than default.** Sweep every construct that can declare a
  reserved keyword as its name in both modes and require the same count in each.
- **Corpus/stdlib regression scan.** Diff severity-normalised diagnostics between the two binaries
  over `examples/pilot-corpora`, `examples/sysml-v2-training` and `internal/core/libs/stdlib`. Note
  this is **structurally vacuous** for `reserved-keyword-name`/`sysml-notation` (0 hits corpus-wide,
  since OMG corpora contain no such names) — report it as a regression control and state the 0-hit
  count, never as coverage proof. Two traps: prefix rows with `awk -v f="$F" '{print f" "$0}'`
  rather than `sed "s|^|$F |"` (corpus paths contain `/` and spaces), and **sort before diffing** if
  you parallelise with `xargs -P`, or output interleaving fakes a huge diff.

## Limitations

The corpus tests the invalid models someone thought to write — a sample of the rejection surface,
not a proof. Some pilot Xpect negatives only error in a library-less resource set (with the
standard library loaded, e.g. `feature f;` gets an implicit type and is legal), so they cannot be
cases here. The pilot's verdicts are externally refereed; the root-cause attributions are
self-assessed.
