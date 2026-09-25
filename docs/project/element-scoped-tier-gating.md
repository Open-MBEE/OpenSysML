# Tier gating per element (roadmap L2)

> This is an engineering record. **Oracle figures in it are as measured when the mechanism landed;
> they are not the current baseline.** The current baseline is the generated block in
> [README](../../README.md) and [architecture](../internals/architecture.md), regenerated and gated
> by `make docs-counts`.

Roadmap item **L2** recorded that `Registry.Run` skipped *every* pass at a strictly higher level
once any pass emitted a blocking error, for the whole document, while the reference (EMF/Xtext)
keeps validating the elements a linking failure did not touch. This page records what was
implemented, what it measures, and — the larger part of the result — which rows the coarse gate was
**not** hiding.

## What was implemented

The gate is now two gates, and a pass says in code which one applies to it.

- **Document-scoped (unchanged default).** A pass with no marker is still skipped once a strictly
  lower level reported a blocking diagnostic. Passes whose subject is the file — imports, notation,
  syntax — legitimately stay here: there is no element to ask about.
- **Element-scoped (opt-in).** `passes.ElementScoped` generalizes an earlier marker that exempted
  exactly one pass, the metadata-metaclass rule. An `ElementScoped` pass runs
  even when a lower tier failed, and must gate itself per subject by asking
  `Context.DownstreamOfFailure(ref)` about the reference its subject's meaning rests on.

`Registry.Run` collects the blocking spans of each level as it goes (seeded with the parse
diagnostics for `LevelSyntax`) and installs the spans of every level *below* the pass about to run
into the `Context`. `Diagnostic.Blocking()` remains the only definition of what gates: a `Notation`
error concerns the writing rather than the meaning and still gates nothing.

`DownstreamOfFailure` answers by span containment — a blocking diagnostic reported inside the
reference it is asked about. That is the narrow reading on purpose: it says "this reference did not
resolve", not "something in this element's neighbourhood failed", so it cannot silence a rule about
a subject that resolved.

Two passes opt in, and each names its subject:

| Pass | Subject | Reference it rests on |
|---|---|---|
| `MetadataTypePass` (`w8c_metadata_type.go`) | one annotation | the metaclass the annotation names |
| `ElementFilterPass` (`filter.go`) | one filter condition | the type named by a classification operator (`@`, `@@`, `istype`, `hastype`) **and** every operand of any operator whose result is Boolean regardless — the classification set, `not`/`and`/`or`/`xor`/`implies`, and the comparisons `==`, `!=`, `===`, `!==`, `<`, `<=`, `>`, `>=` |

`ElementFilterPass` gates only the checks that need something unresolved to have resolved. An
unresolved *name* standing alone as the condition is still reported — there the unresolved reference
is the verdict, and the pilot reports its own rule at the same place, so
`TestAnUnresolvedNameInAnImportFilterIsReported` keeps that honest.

An operator whose result is Boolean *whatever its operands are* is the one place where an unresolved
*ordinary* name gates too. `A and B` and `A == B` yield a truth value however `A` and `B` turn out,
so a fault they draw can only come from an operand — and an operand that did not resolve already has
its verdict. Adversarial fixtures found this the hard way, twice: gating classification types alone
made `filter @Safe and Undefined;` and `filter Undefined1 and Undefined2;` draw a `Must be
model-level evaluable` of ours where the pilot reports nothing, and the same held for
`filter Undefined == 1;` and for the *tested operand* of `filter Undefined istype Safe;` (pilot in
both: only `Couldn't resolve reference to Element 'Undefined'`), which review caught in two further
rounds. These are false positives on shapes that appear in no corpus, so no oracle count would have
caught them; the gate therefore checks the type reference **and** the operands of every such
operator.
`TestAnUnresolvedOperandOfABooleanFilterYieldsOnlyTheUnresolvedReference` locks all of them, and the
three oracles are byte-identical with and without the extra gating.

A *bare* unresolved condition (`filter Undefined;`) still reports, because the pilot reports its own
rule at that line too.

## Measurements (fresh cache, pinned pilot `2026-05` / `0.60.1`)

Re-measured after merging `main` at `b21a30eb` (the annotation-body LSP work, PR #499): every
figure below is from a fresh-cache run of that merged tree, with the base column re-run on
`b21a30eb` itself. #499 moves none of the three oracles, so the controls are the same as those first
measured on `0d4eb14f`.

| Oracle | Base `b21a30eb` | With per-element gating |
|---|---|---|
| Xpect | 428 files, 0 unparsed; 1269 agree (246 wording-only) / 54 disagree | **identical** |
| Differential | 353 files, 317 fully agreeing; 25 agreed, 119 only ours, 73 only the pilot's | 353 files, 317 fully agreeing; **32 agreed**, **119 only ours**, **66 only the pilot's** |
| Rejection | 120 cases: 116 both reject, 4 pilot-only, 0 ours-only | **identical** (one case's own error count 1 → 2) |

**Only-ours does not move.** All seven newly agreed rows are diagnostics of ours that the pilot
also reports, at the same line and severity, on a subject the document-wide gate used to silence
because of an unrelated name-resolution error in the same file:

| File | Line | Ours | The pilot's |
|---|---|---|---|
| `testdata/parse/expressions.sysml` | 2 | `Must have a Boolean result` | `Must have a Boolean result` |
| `testdata/parse/expressions.sysml` | 3, 5, 6 | `Must be model-level evaluable` | `Must have a Boolean result` |
| `testdata/parse/expressions.sysml` | 4 | `Must be model-level evaluable` | `Must invoke a behavior or a behavioral feature` |
| `testdata/passes/errors.sysml` | 3 | `Must be model-level evaluable` | `Must have a Boolean result` |
| `testdata/resolve/errors.sysml` | 3 | `Must be model-level evaluable` | `Must have a Boolean result` |

Read those honestly: only line 2 is word-for-word agreement. The other six agree by **category**
(`kind-mismatch`, same line, same severity) while stating a different rule — both implementations
complain about the same filter or expression, ours about model-level evaluability and the pilot's
about the result type. They are counted as agreement by the harness's category matching, and they
are *not* wording-only agreement in the strict sense (same rule, same element, same offset), so
they remain listed as an open wording divergence rather than a closed row.

One category mapping moved with this: `tools/referee/diff/category.go` now maps `must have` to
`kind-mismatch` on our side, as it already did on the pilot's. Before the change our own
`Must have a Boolean result` was `unmapped` while the pilot's identical string was
`kind-mismatch`, so the two could never agree. On the base tree the change is inert — no diagnostic
of ours in the corpus carried that wording.

### The control that decides the design

The gate was removed entirely as a measurement (not shipped): only-ours **119 → 166**, agreement 25
→ 31, pilot-only 73 → 67 — and the **Xpect oracle does not move at all**. So an uncontained
narrowing buys six rows and costs 47 false positives on the reference's own corpora, which is why
the gate stays and passes opt in one at a time with a stated subject.

## Rows the gate was expected to close, and did not

The Xpect run being byte-identical with the gate fully removed is the proof: none of these four was
behind the gate. The table records their current disposition after Step 3.

| Row | Declared | Ours | Category | Owner |
|---|---|---|---|---|
| `AssignmentActionUsage_invalid.sysml.xt:44` | `Referent must be time varying.` | word-for-word agreement | closed in Step 3 | element-scoped constraint validation derives the referent's `mayTimeVary` property |
| `AssociationTest_CrossFeatures_invalid.kerml.xt:52` | `Must be the cross feature` | nothing at the line; our cross-feature rules fire at line 42 in the same file | unimplemented obligation (since closed) | the parser — the nested `end x1 [0..1] feature x : C1 crosses y.b` form. The file's own constraint-tier diagnostics are reported, so nothing in it was ever gated |
| `InterfaceUsage_Invalid.sysml.xt:49` | `Cannot have more than two ends` (declared *beside* `An interface definition end must be a port.`, which we do report) | only the port rule | pilot limitation | the normative library types `Interfaces::Interface`'s participants `[2..*]`; two ends is a property of `BinaryInterface`. SysML v2 §7.14.1 permits a general interface to have three or more ends |
| `InterfaceUsage_Invalid.sysml.xt:78` | `Duplicate of inherited member name 'self' from Part, Port` (warning) | word-for-word agreement | closed in Step 3 | exactly two-ended interfaces receive `Interfaces::BinaryInterface`; positional end redefinition then supplies the inherited port end |
| `Must have a concrete type` family | — | — | not present | the string appears in neither oracle report at this pin, so there is no row to close |

The gate is not on any row's path, measured rather than argued. Later closure or adjudication does
not change that finding.

One pre-existing defect surfaced while checking the concrete-type family and is **not** caused by
this change — it reproduces identically on the base commit. Our `Must have a concrete type` is
reported at `1:1`, the start of the file, where the pilot reports it at the annotation's own line;
toggling the annotation confirms the annotation is what causes it. Category: **our defect**, owner
`w8c_metadata_type.go` (the diagnostic's span). While the span is wrong the row can never register
as agreement in the differential no matter how the gate is scoped, so ungating it here would not
have helped.

## Verification

`go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...`, `make docs-counts` and
`python3 scripts/check-doc-links.py` are clean. All three committed baselines are byte-identical to
a final fresh-cache run of the shipped tree — confirmed again after the boolean-composition gating
was added, so that fix moves no oracle count. The ratchets pass with
`OPENSYSML_REQUIRE_PILOT_CORPORA=1 OPENSYSML_REQUIRE_TRAINING_CORPUS=1` (no skips), the differential
is byte-identical across two runs under different fresh caches, and the only-ours multiset was
compared against the base commit row by row (keyed by root, file, line, severity and category with
multiplicity): **no row present on this branch is absent on base**. Repeated against `b21a30eb`
after the merge, the row-level diff is exactly the seven rows above moving from pilot-only to
agreement — no only-ours row added, and no row that agreed on base disagrees on the branch.

A malformed classification test (`filter x istype ;`, `filter @;`) parses to an operator with no type
reference, and handing that typed nil to `DownstreamOfFailure` panicked. Merely skipping the nil
traded the panic for a false positive — a `Must be model-level evaluable` of ours where the pinned
pilot reports only the syntax error — so a type the parser never produced now counts as its own
resolution failure and gates the condition, the same reading the mechanism takes elsewhere.
`TestAClassificationFilterMissingItsTypeIsDiagnosedNotFatal` locks all four shapes to reporting the
syntax fault alone. No corpus contains one, so no oracle count moves: all three oracles re-measured
identically after the fix (1269/54 Xpect, 32/119/66 differential, 116/4/0 rejection).

Other malformed conditions whose operand the parser recovered as an error node (`filter x == ;`,
`filter and x;`) can still draw an evaluability diagnostic of ours in a document that already has a
parse error, which the pilot does not report. This is confined to already-invalid documents, appears
in no corpus, and is the residual of the wider question of whether an unparseable subtree should be
judged at all — left as an open item rather than widened here.
