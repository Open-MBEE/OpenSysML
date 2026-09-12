---
name: testing-pilot-execution-referee
description: How to verify the pilot execution referee (cmd/pilot-exec-diff + scripts/download-pilot-evaluator.sh) end to end on Linux — provisioning the headless pilot expression evaluator, reproducing the committed bucket counts, proving the determinism and disagree detectors are live, and the adversarial paths that actually distinguish working from broken.
---

# Testing the pilot execution referee

The execution referee compares model-level expression evaluation between the
pinned OMG SysML v2 pilot and OpenSysML. It does not compare actions, state
machines, token flow, traces, or exhibit/perform execution: those surfaces are
out of reach of the pinned pilot artifact.

## Provision

The pinned shaded jar and standard library must already be available from the
pilot validator setup (`build/pilot-validator/target/sysml-download/sysml/`,
provisioned by `./scripts/download-pilot-validator.sh` — slow, **check before
rebuilding**). Then:

```sh
./scripts/download-pilot-evaluator.sh
```

The script checks the pinned artifact, Java 21+, the standard library, and both
expression-evaluation classes before writing `build/pilot-evaluator`.

Provisioning notes worth knowing before you test it:

- There is **no already-built early exit**: every run recompiles and rewrites
  the launcher. Observed at `f3af23a2`: a second run reprints
  `Compiling ...` / `Built ... (pilot 2026-07, 0.61.0)`, exit 0, and both
  `classes/EvalSysML.class` and `eval-sysml` are **byte-identical**
  (same sha256) with only the mtime advancing. So assert idempotency by
  `sha256sum`, not by mtime or by an "already built" message.
- All fail-loud checks (`pilot_jar`, `library`, `source`, java/javac/jar, java
  version, the two required classes) run **before** `mkdir -p "$classes"`, so a
  failure leaves `build/pilot-evaluator` completely absent. Verify that by
  `mv build/pilot-evaluator /tmp/...` first, then `ls` after — a stale directory
  from a previous run makes the "wrote nothing" assertion vacuous.
- `PILOT_ARTIFACT_VERSION=wrong-version` → exit 1,
  `error: pilot shaded jar not found at .../jupyter-sysml-kernel-wrong-version-all.jar`.
  Moving the pinned jar aside gives the same error with the real version;
  moving `sysml.library` aside gives
  `error: pilot standard library not found at .../sysml.library`.
- **`PILOT_TAG` is not a real pin for this script.** `PILOT_TAG=bogus
  ./scripts/download-pilot-evaluator.sh` exits **0** and builds normally,
  printing `Built ... (pilot artifact 0.61.0)`. The artifact is located purely
  by `PILOT_ARTIFACT_VERSION`; `PILOT_TAG` is provisioned and checked by
  `download-pilot-validator.sh`, not used to locate it here.
- `grep -c jupyter-sysml-kernel-<version>-all.jar build/pilot-evaluator/eval-sysml`
  must be 1 and `__PILOT_ARTIFACT_VERSION__` must not survive in the launcher.
- `build/pilot-evaluator/eval-sysml --help` → exit 0 and
  `usage: eval-sysml --library DIR --cases FILE [--model FILE]...`.

## Run

```sh
go run ./cmd/pilot-exec-diff          # ~13 s wall with the default fixtures
```

Use `-cases DIR` for another directory of `.cases` files, `-out DIR`,
`-launcher PATH`, `-repo DIR`. Each `.cases` file lists `model: <repo-relative-path>`
lines followed by `id :: target :: expression` lines. Reports go to
`build/pilot-exec-diff/pilot-exec-diff.{txt,json}`.

Reference values at the current implementation (434 cases, all eighteen default
fixtures):
`agree 207 · kind-only 1 · order-only 0 · disagree 26 · pilot-unevaluated 124 ·
pilot-silent 21 · pilot-error 9 · ours-error 6 · ours-undetermined 28 · both-error 12 ·
nondeterministic 0`.
Four of the nine `pilot-error` are the whole of `unknown_bounds.cases`: the pilot rejects a
model whose multiplicity bound names a valueless feature (`a : Real[n]`, `Must have a Natural
value`) and resolves nothing in it afterwards, which is why those cases have a model of their
own; two more are the whole of `vast_bounds.cases`, whose bound `[0..9223372036854775807]` the
pilot rejects the same way. Another is `subsequence-unbound-valid`, where the pilot indexes into the one unevaluated
usage element (`IndexOutOfBoundsException: toIndex = 2`) and we answer `<undetermined>`.
Five of the twenty-two `disagree` are in `undetermined_operands`: `size-slots`, where
`size(rack.slots)` for a `part slots[3]` is `3` here, since `[3]` fixes the count, and `1`
from the pilot, which counts the one unevaluated feature-reference operand;
`includes-subsetter`, where `includes(rack.gear, rack.fixed)` for a `part fixed :> gear` is
`true` here and `false` from the pilot, which compares the two unevaluated usage elements;
`size-vacant` and `isempty-vacant`, where a `part vacant[0]` is empty here (`0`, `true`) and
the pilot again counts the unevaluated usage (`1`, `false`); and `size-if-pairs`, where
`size(if u > 0 ? (1, 2) else (3, 4))` is `2` here, both branches fixing it, and `1` from the
pilot, counting the unevaluated `if`. No such pilot answer is a semantic one. The twenty
`ours-undetermined` are the model-level reads of an unbound `attribute u;` and of usages
whose multiplicity leaves the count open (`gear[1..*]`, `loose[0..2]`, `many[10001..*]`,
`xs : Real[2..4]`), where we answer `<undetermined>` and the pilot leaves the expression
unevaluated.
Eight of the other seventeen `disagree` are the `enumeration_classification.cases`
adjudicated ours: the pilot never consults an enumeration's enumerated values
(`3 istype Level` false) and folds a scalar-valued literal to its Integer
(`Level::high istype Level` false). Seven more are unrefereeable rather than verdicts against us:
`extent-variation-count` and `extent-uninstantiated-count`, where the pilot does
not evaluate `all T` (KerML 1.0 §8.2.5.8.1 Table 5 marks it not model-level
evaluable) and `size(all T)` counts the one unevaluated node as `1` whether `T`
has two variants or no instance;
`w6d:complex-is-zero-qualified`, where the pilot answers `false` for
`isZero(rect(0.0, 0.0))` *and* for `isZero(rect(3.0, 4.0))`, because its
`re`/`im` have no evaluable body; and the three `w6d:subsetting-*-count`
cases, where the pilot counts a `[*]` collection as `1` whether two parts
subset it or none does (and folds a `default null` one to `0`); and
`meta-doc-body`, where the pilot prints the comment body `/* Turns. */` with its
trailing blank (`Turns. `) and we print `Turns.`. The other two are adjudicated
ours: `natural-feature-istype-natural`, since a feature's values are instances of
all its types (KerML 1.0 §8.3.3.3.4), so `nat : Natural = 7` answers
`nat istype Natural` `true` where the pilot reads the literal's type alone; and
`meta-rep-represented-name`, since `TextualRepresentation::representedElement`
`subsets owner` in `KerML.kerml`, so we answer the owning part's name where the
pilot answers the representation's own. See
[pilot-execution-referee.md](../../../docs/project/pilot-execution-referee.md).

## Checks that actually distinguish working from broken

- **Determinism.** Run twice into different `-out` dirs and diff the JSONs after
  stripping pilot UUIDs
  (`sed -E 's/\([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\)//g' | jq -S .`).
  The stripped diff must be empty; the **unstripped** diff must be non-empty and
  contain only `rawPilot` UUID lines (observed 100 diff lines) — that is what
  proves the stripping is doing real work rather than the comparison passing
  trivially.
- **`nondeterministic` is not vacuously 0.** Point `-launcher` at a throwaway
  `/tmp/flaky/eval-sysml` shell wrapper that calls the real launcher and, on its
  **second** invocation only (a counter file), `sed`s one result line to a
  different literal. The harness invokes the launcher twice per `.cases` file,
  so invocation 2 mutates the first file's second run. Observed: `dot-perform-out`
  moves to `nondeterministic`, `ours-error` drops 3 → 2, `nondeterministic: 1`.
- **`disagree` is reachable.** A *stable* wrapper (mutates every invocation,
  e.g. `LiteralInteger 7 ` → `LiteralInteger 8 `) puts case `int` in `disagree`
  with `pilot={"kind":"int","value":"8"} ours={"kind":"int","value":"7"}` and
  keeps `nondeterministic: 0`.
- **Artifact-absent path.** `-launcher /nonexistent/eval-sysml` exits **0**,
  prints `pilot execution artifact is absent at <path>; run
  ./scripts/download-pilot-evaluator.sh to provision it`, and creates **no**
  output directory (the check precedes any `MkdirAll`). Same with the real
  launcher moved aside. Delete/`-out` to a fresh path so an existing
  `build/pilot-exec-diff` cannot fake the assertion.
- **Malformed cases.** A case line with the wrong number of ` :: ` fields →
  exit 1, `pilot-exec-diff: <file>:<line>: expected id :: target :: expression`,
  no output dir. An unknown `model:` path → exit 1 and no output dir, with the
  source line included:
  `pilot-exec-diff: <file>:<line>: model no/such/model.sysml: stat <abs>: no
  such file or directory`.
- **Additivity.** `go run ./cmd/pilot-diff` must still print the headline the
  committed baseline holds (`371 file(s), 337 fully agreeing; 34 agreed
  diagnostic(s), 37 only ours, 1111 only the pilot's` after the MOSA example joined `examples/` and the bare feature-reference typing round — read it from the baseline JSON, not from this line, since each
  fix round moves it) and `jq -S` diff clean against
  `docs/project/pilot-differential-baseline.json`; `git status --porcelain`
  empty at the end.

## The doc-counts gate is the usual casualty of a compliance-row change

Any wave that flips a status flag in `docs/project/spec-compliance.md` (for
example a row moving ⚠️ → ✅ because the pilot can now referee it) must also
update **two aggregate count lines**, or
`cmd/pilot-diff/doc_counts_test.go:TestPilotDifferentialDocumentCountsMatchBaseline`
fails with `coverage ✅ faithful: want N (spec-compliance.md ✅ rows), got M`:

- `README.md` — the `**Measured status:**` line
- `docs/internals/architecture.md` — the `**Current coverage:**` line

This is invisible to `go run ./cmd/pilot-exec-diff` and to the `jq -S`
baseline comparison, and it is **not** in the blueprint's abbreviated
`-run 'TestTrainingExamples|TestPilotCorpora|TestCorpusGates'` selector, so
always run the unfiltered `go test ./...` too. Confirm any failure is
attributable to the branch by building a `git worktree` at `HEAD~1` and running
that one test there; count the flags per row with a script that reads the
*last* table cell rather than `grep -c "✅"`, since the prose in other cells
contains the same glyphs.

## Interpret — and the empty-output trap

`agree` means the normalized results match. `disagree` means they do not.
`kind-only` records an exact numeric match whose kinds differ, while
`order-only` records a sequence with the same multiset in another order.
`pilot-unevaluated` means the pilot returned a non-Literal node,
`pilot-silent` means it emitted no output, and `pilot-error` means it emitted
`ERROR:` or `EXCEPTION:`. Together these are the four pilot-side states:
value, error, unevaluated, and silence. `ours-error` and `both-error` record
failures from either side. `ours-undetermined` records our `<undetermined>` —
the model-level value of an expression over a feature the model leaves open —
whatever the pilot said; the pilot has no such answer, so these are adjudicated in
the referee page rather than counted as agreement or disagreement.
`nondeterministic` takes precedence whenever either side differs from itself
between the two runs.

Both runtimes render values as sequences; single-element sequences are unwrapped
on both sides, so a scalar-versus-singleton distinction is unobservable here.
Reals are compared after rounding both sides to two decimal places, matching
OpenSysML's display precision. Pilot exponent-form rationals are parsed exactly
with `math/big` before integer-vs-real comparison.

**Known limitation to re-check on any change to `normalize.go`:** the pilot
emits *literally nothing* between its `== case`/`== end` markers both when an
expression legitimately evaluates to an empty sequence (`Probe::Empty()`) and
when it silently gives up (`1 / 0`). The harness cannot distinguish those
pilot behaviors, so both are reported as `pilot-silent` rather than as a
successful empty value. The report says explicitly that the pilot cannot
distinguish "no value" from "declined to evaluate" when it emits no output.
Reproduce by hand with
`build/pilot-evaluator/eval-sysml --cases <tsv> --model <model>`; the pilot
does report real failures as `ERROR:` or `EXCEPTION:` lines (for example,
`Probe::Nope` uses `ERROR:`).

## Adjudicating a `disagree` before believing it

A pilot `LiteralBoolean` is **not** evidence the pilot evaluated the operands.
The pinned artifact folds a comparison against an *unevaluated* operand to
`false`, so a predicate over a library function it cannot fold answers `false`
for every input. Prove this rather than assuming it, by asking the evaluator for
the operands and for the predicate on an input whose answer must differ:

```sh
printf 'a\t\tComplexFunctions::re(ComplexFunctions::rect(0.0,0.0)) == 0.0\n' > /tmp/c.tsv
printf 'b\t\tComplexFunctions::isZero(ComplexFunctions::rect(3.0,4.0))\n' >> /tmp/c.tsv
build/pilot-evaluator/eval-sysml --cases /tmp/c.tsv --model <model>
```

Observed at `40fd4184`: `re`, `im`, `abs`, `isZero` and even `rect` all come
back as `InvocationExpression <name>` (unevaluated), `re(rect(0,0)) == 0.0` is
`LiteralBoolean false`, and `isZero(rect(3,4))` is **also** `false`. A predicate
that answers `false` for both a zero and a non-zero argument is an artifact, not
a semantic verdict — adjudicate it as unrefereeable. Note the qualified/
unqualified split is itself a pilot resolution artifact: `ComplexFunctions::isZero(...)`
folds to `false` while bare `isZero(...)` stays `InvocationExpression isZero`.

## Where the value-conformance checks stop (expect under-reports, not false positives)

`passes/typecheck_value.go:checkValueConformance` returns early when the
*declaring feature's* type is scalar (`ec.model.PrimTypeOf(want) != PrimUnknown`),
so every branch after it — including `invocationResultTypeSymbol` — is
unreachable for a scalar-typed target. Practical consequence when probing these
checks against `build/pilot-validator/validate-sysml`: a mismatch reports only
when the target is a non-scalar (`part w : Wheel = GetReal()` errors) and stays
silent when it is scalar (`attribute s : String = GetReal()` is silent while the
pilot warns `Bound features should have conforming types`). Likewise a behavior
with **no** result parameter (`part w : Wheel = Build()` for `action def Build`)
is deliberately left unjudged. Classify these as conservative gaps; only an
error we raise where the pilot is clean is a real false positive.

## Reproducing a reported case by hand

The report's `rawPilot`/`rawOurs` are byte-reproducible (modulo pilot UUIDs):

```sh
printf 'big\t\tProbe::Big()\n' > /tmp/c.tsv          # id <TAB> target <TAB> expression
build/pilot-evaluator/eval-sysml --cases /tmp/c.tsv \
  --model cmd/pilot-exec-diff/testdata/models/expr_values.sysml
# → LiteralRational 1.099511627776E12 (<uuid>)   [~150 library "Reading ..." lines first]
make build-sysml
./bin/sysml -e 'Probe::Big()' cmd/pilot-exec-diff/testdata/models/expr_values.sysml
# → ✓ package Probe / ✓ Probe::Big() /   = 1099511627776
```

`bin/sysml` prints an extra `✓ package Probe` load line that the harness's
`repl.Session` path does not; ignore it when byte-comparing against `rawOurs`.
For a case with a non-empty target, our side goes through
`%eval in <target> : <expr>`, not `-e`.

## Devin Secrets Needed

None. Network access is required only for re-provisioning the pilot validator.

## Recording

CLI-only work; per the testing-mode guidance this needs no screen recording —
collect verbatim stdout/stderr and exit codes (captured without a pipe) instead.
