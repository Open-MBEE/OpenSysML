# Pilot Xpect Expectations

## Overview

**Reference:** the OMG pilot implementation's own Xpect test suites, [`org.omg.kerml.xpect.tests`](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/2026-07/org.omg.kerml.xpect.tests) and [`org.omg.sysml.xpect.tests`](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/2026-07/org.omg.sysml.xpect.tests), at release `2026-07`, commit `c7fc737d56da9e2d78f9d7df6d38efbec2e7e965` — the same pin as the corpora and the reference validators (`scripts/pilot-pin.sh`)
**Provision:** `./scripts/download-pilot-xpect.sh` (the shared downloader of `scripts/pilot-pin.sh`, restricted to `*.xt`: the clone is refused unless the tag resolves to the pinned commit, each suite is stamped with the pin it was fetched at, and a suite stamped otherwise or not at all is re-fetched; writes `build/pilot-xpect-corpus/{kerml,sysml}`, gitignored, not vendored — under `build/` rather than `examples/` because the `.kerml`/`.sysml` models the suites ship are inputs to this harness, and everything that walks `examples/` would otherwise adopt them)
**Run:** `go run ./cmd/pilot-xpect` (writes `build/pilot-xpect/pilot-xpect.txt` and `build/pilot-xpect/pilot-xpect.json`)
**Baseline:** the last committed run is [pilot-xpect-baseline.json](pilot-xpect-baseline.json), which carries every non-agreeing row, so a later run can be diffed against it; `-update` re-records it and `-check` fails unless a fresh run reproduces it
**Status:** advisory only — nothing here gates CI, for the same reason [pilot-differential.md](pilot-differential.md) does not: the corpus is an unvendored network fetch at the pinned tag, and this is a report, not a ratchet

**Labels:** the short labels in this record are internal cross-references, not specification or
product terms. `F<n>` names a row of the
follow-up table in [pilot-differential.md](pilot-differential.md), and `K<n>`/`S<n>` its KerML and
SysML diagnostic classes. A reader who only wants the verdicts can ignore all of them.

**Differential figures quoted in this record are as measured at the round they document, not the
current baseline** — the current one is the generated block in [README](../../README.md) and
[architecture](../internals/architecture.md), regenerated and gated by `make docs-counts`.

[pilot-differential.md](pilot-differential.md) compares us against *observed* pilot behaviour: it runs
the pinned validator and records what it says. That answers "do we agree with the reference?" but not
"was the reference's behaviour intended?" — the question that has kept several rows there advisory.

These `.xt` files are the other kind of evidence. Every assertion in them is a **declared**
expectation, written inline by the pilot's implementers next to the model it constrains. We read them
as data — no Java, no Xtext, no Eclipse — and adjudicate each one against our own front end.

The `linkedName` assertions are the point of the exercise: they are the first external oracle we have
for **name resolution**, the largest block of self-assessed rows in
[spec-compliance.md](spec-compliance.md). They are broken out in their own section below. The first
run of this harness put them at 151 of 194; after the alias-identity work they are **194 of
194**, and that section now records what the oracle found and what closed it.

---

## What this oracle can and cannot adjudicate

Can:

- **Name resolution.** `linkedName` declares the qualified name a given reference must resolve to.
  This is a direct, per-reference verdict on our scoping, import, inheritance and alias handling.
- **Name *visibility*.** `scope` declares the complete set of names visible at a point, so it catches
  both halves of a scoping defect — a name that should be visible and is not, and a name that is
  visible and should not be. `linkedName` can only see the first half. See
  [scope](#scope--230-of-230-agree-exactly).
- **Diagnostic presence and placement.** `errors`/`warnings` declare a severity and a message at a
  source location; `noErrors` declares silence over a whole resource set.
- **The pilot's *intent*.** When we disagree with a declared expectation, the pilot's behaviour on
  that construct is not incidental — its implementers wrote the expectation down.

Cannot:

- **Execution semantics.** Nothing here exercises action or state execution. The suites contain no
  execution assertions, and the pinned pilot has no headless action/state execution surface at all
  (see [pilot-execution-referee.md](pilot-execution-referee.md), and issue #386 for the referee's
  scope). Nothing in this report bears on a behaviour row.
- **Diagnostic wording.** Our messages are our own; a declared message and ours can describe the same
  defect in different words, and the specification does not settle whose words are right. The harness
  therefore admits a `wording-only` row into agreement only when it can verify that both messages
  state the same rule about the same element. See
  [How agreement is decided](#how-agreement-is-decided).
- **What a declared library file leaves unresolved.** A fixture's resource set is often a subset of
  the library its own files reference — `Objects.kerml` without `Occurrences.kerml`, say. Those
  unresolved references arrive against the file under test, because name resolution resolves the
  whole index rather than one document, and the harness drops them by their location: it counts them
  as *foreign* and adjudicates only diagnostics inside the fixture's own model text.
- **Anything the suites do not cover.** These are the pilot's *tests*, not the specification. A
  construct with no assertion is not endorsed by their absence.

The comparison loads what each fixture declares: the `.xt` body itself, the `/src/` files its
`XPECT_SETUP` `ResourceSet` names, and exactly the `/library*` copies it names — never our embedded
standard library in their place. A declared resource absent from the download is reported as such,
never silently treated as loaded.

---

## How agreement is decided

Agreement is **strict** on substance. A row agrees when it agrees word-for-word, or when it is
**wording-only**: the same rule about the same element, at the same severity and the same offset, in
our words rather than the pilot's. Wording-only rows are counted inside agreement with their own
sub-count, because the wording is not something the specification settles and nothing new is detected
when one is admitted:

| Kind | Agrees when |
|---|---|
| `errors`, `warnings` | we report a diagnostic of the declared severity **at the declared offset**, whose message either matches the declared one (whitespace and a trailing period aside) or states the same rule about the same element in our own words |
| `noErrors` | we report **no error** anywhere in the declared resource set |
| `linkedName` | the reference at the declared text resolves, and the resolved element's qualified name **equals** the declared one |
| `scope` | the set of names we enumerate at the anchor **equals** the declared set, name for name, after filtering by the metatype the anchor's cross-reference admits |

Wording-only is not a tolerance and is not granted on span and severity alone: the harness requires
the declared and our message to state the same rule about the same element
(`cmd/pilot-xpect/wording.go`). A different rule landing on the same token stays a disagreement, and
those rows are what the `same-location` tolerance now holds.

No tolerance ever turns a disagreement into an agreement. Weaker rules are recorded beside each
disagreement, as evidence about *how far off* we are, and are summarized per kind in the report:

| Tolerance | Meaning |
|---|---|
| `same-location` | right severity at the declared offset, but a *different rule* — wording alone would be agreement |
| `same-line` | right severity on the declared line, at a different offset |
| `severity-differs` | a diagnostic of ours is there, of the *other* severity |
| `elsewhere-in-file` | right severity, but nowhere near the declaration |
| *(none)* | nothing of ours is there at all |

The `scope` kind has its own tolerance classes, on the same rule — none of them turns a disagreement
into an agreement:

| Tolerance | Meaning |
|---|---|
| `other-paths` | every declared name we do not offer names an element we *do* offer under a different path — a spelling difference, not a visibility one |
| `extra-names` | we offer names the declaration does not, and miss none |
| `missing-names` | we miss declared names and offer no extra ones |
| `missing-and-extra` | both |
| `library-names` | every difference is a path tail through the implicit members `Base` contributes (`self`, `that`) — see the scope section |

Two further honest limits of the mapping:

- `noErrors` is adjudicated as file-wide silence. Where a declared `noErrors` names a target, we still
  require silence everywhere, which is the stricter reading — so a `noErrors` disagreement always
  means we report *an* error, not necessarily one at the named target.
- A declared location is matched by finding the declared text after the assertion, ignoring
  whitespace. Text inside XPECT notes is masked out first so an assertion cannot match itself.

---

## Corpus and census reconciliation

`./scripts/download-pilot-xpect.sh` fetches **303 KerML + 126 SysML = 429** `.xt` files, exactly the
expected count. **0 files are unparsed** and **0 declared resources are missing**, so no assertion is
silently dropped.

The reader recovers **1263 assertions**, declaring **1325 individual expectations** (a single
`errors`/`warnings` note may list several diagnostics; each is one adjudicated row). Reconciled
against the published per-kind census:

| Kind | Published | `//`-form notes | `//*` block notes | Reader total |
|---|---:|---:|---:|---:|
| `errors` | 459 | 459 | 27 | 486 |
| `noErrors` | 275 | 275 | 0 | 275 |
| `linkedName` | 192 | 194 | 0 | 194 |
| `warnings` | 58 | 58 | 17 | 75 |
| `scope` | 1 | 1 | 229 | 230 |
| `exportedObjects` | — | 0 | 1 | 1 |
| **Total** | **985** | **987** | **274** | **1261** |

Every difference from the published numbers is accounted for, and none of it is the reader guessing:

- **Block notes (274).** `//* XPECT errors --- ... --- */` is the multi-line form of the same
  assertion, and the suites use it heavily — for all but one `scope` assertion, for 27 `errors` and
  17 `warnings`. A line-oriented census counts only the single-line form. These are real assertions
  and are adjudicated exactly like their single-line equivalents.
- **`linkedName` 192 → 194.** Two notes are written with a tab after the slashes
  (`//\t\tXPECT linkedName at aa --> test.A.a`) rather than `// ` or `//`, so a grep for the two
  common spellings misses them: `MemberNameTests_NamedMemberFromInheritance.kerml.xt`:41 and
  `MemberNameTests_NamedMemberFromInheritance_Rdef.kerml.xt`:50. Both are ordinary executable notes;
  both were disagreements on the first run, so dropping them would have flattered us by two rows.
- **`exportedObjects` (1).** One note in `indexing/NameEscape.kerml.xt` opens with a bare `//*` and
  puts `XPECT exportedObjects` on the following line. It is adjudicated against our index, and
  it agrees, so no kind is reported as not adjudicated any more.
- **The `at` clause and unescaped quotes (−3 expectations).** The reader was corrected: an
  `at "…"` clause runs to the last quote on its line, because Xpect does not escape the quotes
  inside it (`at "filter new A(null, 1, "", false);"`). Four assertions were previously read as two
  expectations each, a truncated one and a junk one, so the declared population is **1323**, which is
  what the totals below are measured over. See
  [adjudications.md](adjudications.md).
- **XPECT-shaped text that is not a note (10).** Eight `XPECT scope`/`XPECT errors` fragments sit
  inside `/* ... */` comments and two are disabled by their authors as `// (TBD) XPECT noErrors`.
  These open no `//` or `//*` note, so the harness does not run them; all ten are listed by file and
  line in the report's *XPECT-shaped text outside a note* section rather than being dropped.

---

## Totals

```
429 .xt file(s), 0 unparsed, 0 missing declared resource(s)
1263 assertion(s) declaring 1325 expectation(s)
agree 1297 (of which wording-only 248) | disagree 28 | unlocated 0 | not adjudicated 0
```

| Kind | Expectations | Agree | of which wording-only | Disagree | Not adjudicated | `same-location` | `same-line` | `severity-differs` | `elsewhere` | nothing |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `errors` | 511 | 492 | 248 | 19 | 0 | 10 | 7 | 0 | 2 | 0 |
| `noErrors` | 276 | 268 | — | 8 | 0 | — | — | — | — | — |
| `linkedName` | 194 | 194 | — | 0 | 0 | — | — | — | — | — |
| `warnings` | 113 | 112 | — | 1 | 0 | 0 | 0 | 0 | 0 | 1 |
| `scope` | 230 | 230 | — | 0 | 0 | — | — | — | — | — |
| `exportedObjects` | 1 | 1 | — | 0 | 0 | — | — | — | — | — |

Per suite:

| Suite | Files | Expectations | Agree | Disagree | Not adjudicated |
|---|---:|---:|---:|---:|---:|
| `kerml` | 303 | 967 | 949 | 18 | 0 |
| `sysml` | 126 | 358 | 348 | 10 | 0 |

**Read the `errors` row carefully: 248 of its 492 agreements are wording-only, so more than half of
that column is us stating the pilot's rule in our own words rather than a rule written against its
text.** The 244 word-for-word rows are the ones where a rule was implemented against the declared
message. `warnings` shows the same effect from the other side: its 112 agreements are the
duplicate-member-name, visibility and library rules written against the pilot's declared
wording, so they match by construction rather than by luck. `noErrors` and `linkedName` are
wording-independent, and they are where this oracle adjudicates most directly.

Every figure above is checked against the committed baseline by
`TestPilotXpectDocumentCountsMatchBaseline`, and the baseline's own provenance — the pin (tag and
commit), the suite digests and the declared errata — by
`TestCommittedBaselineStatesThisRepositorysProvenance`; both read only committed files. A daily
Java-backed run re-checks the measurement itself, as described in
[pilot-differential.md](pilot-differential.md#how-this-record-is-kept-true).

Movement since the first run of this harness (the harness itself is unchanged; every difference is a
change in our behaviour). The `Now` column is checked against the committed baseline; the `First
run` column is that run's own measurement:

| Kind | First run | Now | What moved |
|---|---|---|---|
| `linkedName` | 151 / 194 | **194 / 194** | alias-introduced names resolve to the aliased element, and the `~ B::f` conjugation form parses |
| `noErrors` | 231 / 275 | **268 / 276** | 6 `ParsingTests_*` files, 4 inherited-name-conflict files and 2 others no longer draw an error, the visibility reconciliation's protected/shadowed path reconciliation cleared 11 more, the anchor-and-residue work closed the 6 protected-import rows the specialization-visibility restoration had made unsatisfiable by modelling `noErrors` as Xpect's residue rather than as file-wide silence, the parser productions cleared `ParsingTests_Indexing` and `SemanticMetadata_valid`, and the connector-end and import work closed two more rows |
| `warnings` | 0 / 113 | **112 / 113** | the duplicate-member-name warnings, the earlier rules written against the declared wording, the library rules, the warnings residue, the usage-typing rules, and Step 3's binary-interface end typing |
| `errors` | 0 / 510 | **492 / 511** | 244 rows are ours word-for-word; the other 248 are wording-only, admitted centrally after the rule and element were checked, not by adopting the pilot's phrasing |
| `scope` | 73 / 230 | **230 / 230** | the library-member resolver work resolves implicit and inherited members through the library (`library-names` 125 → 27), the visibility reconciliation reconciles the protected and shadowed paths, the re-entry bound bounds re-entry to one per name, the anchor-and-residue work fixes the quoted anchor and stops a recursive import's descent carrying implicit generals, and the scope-traversal work bounds derived `self`/`that` paths and anchors a scope assertion on the reference its text names |

**These tables and the baseline are a fresh-cache run after the transition-guard and KerML
subsetting-metaclass rules became element-scoped.** Agreement is unchanged, while four rows now
carry their own diagnostics: `same-location` 7 → **10**, `same-line` 6 → **7**, and
`elsewhere-in-file` 6 → **2**. The differential census was byte-identical across that round,
including the 84 only-ours diagnostics it measured then; the current figure is in
[pilot-differential.md](pilot-differential.md)'s Results table. The largest movement was detection, not classification: the 11
`severity-differs` `errors` rows are **0**, because the usage-typing and specialization rules the
pilot declares there were implemented instead of a severity being adjusted, and the resolver work canonicalized
the resolver's inherited-name warning that had been standing in their place. `errors` silence falls
8 → **0**, `elsewhere-in-file` 42 → **7**, `same-line` 25 → **6**, and `noErrors` rises 248 → **267**.

### Step 3 obligation round

Independent fresh-cache runs of exact base `4b9baf2d` and this tree move Xpect from
**1293 agree / 248 wording-only / 30 disagree** to
**1295 / 248 / 28**. Two rows recover and none regress:

| Row | Base | Step 3 | Movement |
|---|---|---|---|
| `AssignmentActionUsage_invalid.sysml.xt:44` | declared `Referent must be time varying.`; ours elsewhere in the file | word-for-word agreement | closed by SysML v2 §8.3.17.5 assignment-referent validation |
| `InterfaceUsage_Invalid.sysml.xt:78` | declared `Duplicate of inherited member name 'self' from Part, Port`; ours had an interface-end error | word-for-word agreement | exactly two-ended binary typing exposes the inherited port end and its diamond |

No lower-tier fix unmasks a new Xpect diagnostic. `InterfaceUsage_Invalid.sysml.xt:49` stays a
same-location disagreement and is a **pilot limitation**: SysML v2 §7.14.1 permits three or more
interface ends, while §7.14.2 and §8.3.14.2 constrain the binary subtype only.
`BindingConnector_Invalid2.sysml.xt:42` stays the sole warnings gap: KerML's binding rule constrains
the related features of a binding connector, but no numbered normative constraint was found for the
pilot's argument-level warning on the operator expression `rearWheel+1`; the pilot validator source
marks that conformance check TODO. It is therefore recorded as a **pilot limitation**, not
implemented by special-casing the expression.

**The `nothing` column is empty for the first time**: the added parser productions let
`Type_Multiplicity_invalid` reach its validation rule and gave `ScopeWithFourDotAndDot` a real
diagnostic, so no file whose implementers declared an error is now accepted in silence.

**What the wording-only class did and did not buy.** It moved 248 rows from `same-location` into
agreement without changing what we detect — the same severity, the same offset, the same rule, the
same element, our phrasing (`unresolved reference: A::a1 — did you mean …` against `Couldn't resolve
reference to Classifier 'A::a1'.`). **Nothing was newly detected by it, and the sub-count exists so
that a future reader cannot book the jump as detection.** The 10 rows left in `same-location` are the
interesting residue: there we flag the declared offset for a *different* reason than the pilot does,
such as `ParsingTests_BadScopeWithOnlyTwoDot.kerml.xt`:26, where it cannot resolve `test` and we say
the reference resolves to a package where a type is required.

**The `errors` regression is closed by the specialization-visibility restoration, and the rule it restores is the one the visibility reconciliation
collapsed.** KerML resolves a qualified name by resolving its qualification and then asking that
namespace for the last segment through *visible* resolution (8.2.3.5.3, 8.2.3.5.4): only public owned
memberships and public imports are read, and what the *referring* namespace specializes is not
consulted. Only the first segment uses *local* resolution, which does include the protected members a
specialization inherits. The visibility reconciliation applied the local rule to every segment, so a protected member became
nameable through any namespace path a specialization could write. The restoration takes the specialization
relaxation out of the qualified tail (`namedThroughNamespace` in `resolve/visibility.go`, applied by
`walkQualifiedTail`) and leaves local and inherited resolution, `import all` and scope enumeration
untouched — no divergence from the specification had to be recorded for it.

All 12 rows we had gone silent on are back at the pilot's own location, and the 2
`VisibilityTests_Protected_0.kerml.xt` rows that landed elsewhere in the file now land on the declared
references; all 14 are admitted as wording-only. **The trade the specialization-visibility restoration appeared to force is gone, and
it was the harness's reading rather than a contradiction in the corpus:** it cost `noErrors`
six rows because those fixtures declare file-wide silence *and* the protected-import errors it
restores, and the anchor-and-residue work showed that Xpect scores a `noErrors` note against what its sibling
expectations leave over, not against total silence (`consumedLine`, `cmd/pilot-xpect/compare.go`).
With that modelled, the six rows close without exempting a fixture or widening the wording-only class,
and **no expectation in this suite is now recorded as unsatisfiable.** The 42 private/protected rejections of the earlier import round are unchanged.

The `nothing` column counts what neither agreement nor any tolerance accounts for, so it subtracts
the agreements — including the wording-only ones, which are no longer counted under `same-location`.

The **complete** per-row evidence — every disagreement and every wording-only row with its file,
line, declared expectation and our actual behaviour — is in
[pilot-xpect-baseline.json](pilot-xpect-baseline.json). The sections below group the disagreements by
cause and name the files; they do not repeat 300 rows.

---

## linkedName — 194 of 194 agree

**This oracle is now clean, and it was not clean when it was built: the first run agreed on 151 of
194.** The section is kept because what it found is the reason the harness exists, and because a
future regression here is the single most informative failure this report can produce.

### What it found — an alias resolved to itself instead of to its target (41 of the 43)

The pilot's `linkedName` reports the qualified name of the **element** a reference reaches. Where the
name reached it through an `alias`, that is the aliased element's own qualified name; we reported the
qualified name of the **alias**:

```kerml
package test {
    classifier A;
    alias A_alias for A;
    //XPECT linkedName at A_alias --> test.A
    classifier B specializes A_alias;
}
```

`testsuite/MemberNameTests_LocalNamedMember.kerml.xt`:37 declared `test.A` and we answered
`test.A_alias`. The reference *resolved* — nothing downstream broke — so what differed was the
identity of the thing it resolved to: our symbol index made an alias a symbol in its own right rather
than a second name for an existing element. The pilot was right. A KerML `alias` declares a
`Membership` with a name whose `memberElement` is the existing element: it introduces a name, not an
element, so the resolved element's qualified name cannot contain the alias.

All 41 were the single-declaration form `alias X for <target>;`, spread over targets in the same or an
imported package (28), inherited members (9) and private members (4) — one defect in
`internal/core/symbols`/`internal/core/resolve` plus an inheritance hop, not four.

### What it found — a member reached through a conjugated type was not a reference at all (2 of the 43)

```kerml
classifier B conjugates A;
//XPECT linkedName at B::f --> test.A.f
feature g ~ B::f;
```

`testsuite/MemberNameTests_NamedMemberFromConjugation.kerml.xt`:41 and :43 declared `test.A.f`; we
indexed no name reference at that offset at all, because we did not parse the `feature g ~ B::f;`
conjugation form. The root cause was in the parser, not in resolution, and the same file's `noErrors`
assertion failed with it.

### What closed them

Both causes closed with the alias-identity work (alias-introduced names resolving to the aliased element, and
the conjugation form parsing), and the same file's `noErrors` row closed with the second. The split
of the 43 between the individual PRs in that round is **not** separately measured here: this report
compares two revisions of the tree, not one PR at a time.

### What the agreements are worth

The 194 are not trivial passes: they cover qualified names through public and private imports,
`import ::*` recursion, short names and `<id>` forms, shadowing between an import and an inner
classifier, redefinition scoping, inherited-member lookup, and every alias shape above. This is the
only external, per-reference verdict on our name resolution that exists at the pinned tag, and it is
also the narrowest: it says which element a written reference reaches, never which names *were*
visible. That second question is the 230 `scope` assertions below.

## noErrors — 268 of 276 agree

8 disagreements: we report an error where the pilot's implementers declared the file clean. Grouped
by our first diagnostic:

| Cause | Rows | Read |
|---|---:|---|
| **Parse recovery** — `expected a namespace member` | 3 | **Pilot limitation**, adjudicated in [adjudications.md](adjudications.md): the three `QPE-*` query-path-expression files live under the pilot's `failing/` tree and the pinned validator rejects them too, so the declared silence is not spec-derivable. `SemanticMetadata_valid.sysml.xt` left this family in the parser-production work, which was a real false positive on a valid file. |
| **Specialization cycle** — `x participates in a specialization cycle` | 3 | **Adjudicated divergence, not a defect of ours.** All three fixtures declare a real cycle: `part p1 :> p2; part p2 :> p3; part p3 :> p1;` and `part p4 :> p4;` (`simpletests/PartTest.sysml.xt`:67-71), `part def A :> C` with `part def C :> A, B` (`Redefinition_OwningType_Cyclic_Gen.sysml.xt`:28-34), and `classifier a specializes b` / `classifier b specializes a` (`SimpleImportTests_CircleInheritanceInCircleImport.kerml.xt`:29,37). The pilot has no such check at all — the finding F4/K5 settled in [pilot-differential.md](pilot-differential.md#specialization-cycles-f4) — so closing them would mean deleting a correct rule. |
| **Conformance** — `try (typed by a1) redefines b (typed by A): types do not conform` | 2 | **Adjudicated divergence**, decided in [adjudications.md](adjudications.md) (E4): a redefinition is a subsetting (KerML 7.4.9, 8.3.4.2), so a non-conforming type describes an unsatisfiable model; the pilot validates subsetting conformance nowhere, so its silence records an absent check. Both rows are `SimpleImportTestsFromOtherFile_Import3{,_FT}`. |

Two more **specialization cycle** rows joined this family when `DirectSupertypes` stopped memoizing
an answer computed while the resolver's cycle guard had cut a lookup short (the trigger-argument
typing change). Both fixtures declare a real cycle through a nested member:
`ShadowingTests_CircleInheritance.kerml.xt`:17 has `classifier A specializes A::B` with
`classifier B specializes A, Base::Anything` nested in `A`, and `ShadowingTests_CircleProblem5.kerml.xt`:29
closes `A → D → A::B → C → A`. We were silent before only because resolving `B`'s general re-entered
`A`'s resolution, the guard answered "nothing", and that answer was memoized — `B` lost `A` as a
supertype for good. The pinned validator has no cycle check, as above. Locked by
`TestDirectSupertypesKeptAcrossOwnerCycleGuard` and `TestConstraintNestedMemberSpecializationCycle`;
the committed baseline is not re-recorded here, as a separate `scope` movement is still being adjudicated.

One **arithmetic operand** row joined when an untyped collection-body parameter began taking the
element type of the collection its body is applied to: `ParsingTests_Expressions.kerml.xt`:40
declares the file clean, and its `c = x->collect {in xx; xx + 1};` and `c1 = x.{in xx; xx + 1};`
(lines 55–56) now draw `operator '+' is not defined for String and Natural`, because `x` is the
`String` that `ToString` returns. The same body written `in xx : String` drew that error already;
the adjudication is in
[pilot-differential.md](pilot-differential.md#collection-body-element-typing-round). The
committed baseline is likewise not re-recorded for it. The same row's `ours` column later grew by
one error when a bare feature reference began taking the effective type of the feature it names:
line 52, `grp = -x + x * y * y + a ** 3 ^ 4;`, draws `operator '-' requires a numeric operand,
found String` for the same `x`; the three `x == <n>` comparisons on lines 81–83 draw warnings,
which `noErrors` does not count. No row moves — the file was already disagreeing — and the
adjudication is in
[pilot-differential.md](pilot-differential.md#bare-feature-reference-typing-round).

The two former unresolved-reference rows were checked independently before Step 2 and were already
closed at its merge base. `AllocationTest.sysml.xt:31` was the n-ary connector-end parser defect;
`KernelLibraryTest.sysml.xt:72` was recursive import re-export traversal. Neither depends on prefix
annotation lookup, and the fresh-cache Step 2 Xpect control and head remain identical.

The **inherited-name-conflict** family that cost 4 rows on the first run is gone: those files are the
same defect as the `warnings` severity finding below, and making a duplicate inherited name a warning
rather than an error made all four files clean. The two state/transition rows
(`simpletests/StateTest.sysml.xt`:73, `DecisionTest.sysml.xt`:69) closed with the
transition-endpoint reading, not by relaxing the rule.

By suite: 3 KerML, 7 SysML. In every one of the 10 the declared expectation is *silence*, so every
one is a place where we reject something the reference accepts — the same class of finding as the
"only ours" column in [pilot-differential.md](pilot-differential.md), but here backed by a declared
intent rather than an observed verdict. **2 of the 10 are ours; 5 are adjudicated divergences where
the pilot has no check, and 3 are the `QPE-*` pilot limitation.** The kind's history is worth keeping in view, because it
moved in both directions: 244 → 243 (four parse rows closed, six visibility rows
opened), then 243 → 254, then 254 → 248 when the specialization-visibility restoration restored the protected-import rejections, and
248 → 263 once the anchor-and-residue work modelled `noErrors` as Xpect's residue and closed those six
without giving the rejections back, and 263 → **265** with the parser-production work on the indexing and
semantic-metadata parse rows, then 265 → **267** with the connector-end and import work on the allocation and recursive-import
rows. No row here is unsatisfiable any more.

---

## warnings — 112 of 113, and the severity finding is closed

112 rows agree, all of them word-for-word: no `warnings` row is wording-only. The first 11 were duplicate-member-name warnings implemented with the alias-identity work
round from the pilot's declared text — 6 in `MembershipTests_Distinguishability.kerml.xt`, and 5
across the `Redefinition_Diamond*_invalid` / `RedefinitionDiamond*_invalid` pairs; a later round added 12
more, the multiplicity-upper-bound rule among them; **the library-rule work added 66**, the library inherited-name
diamond chief among them, and later rounds closed 10 and then 12 more — the nested
`perform b.a;` / `exhibit s.sa;` offsets and four of the five rows that sat behind another rule's
error. All of them match by construction rather than by luck, because each was written against the
declared text.

The remaining row is `BindingConnector_Invalid2.sysml.xt:42`: the pilot declares `Bound features
should have conforming types` on `rearWheel+1`, while OpenSysML has no feature endpoint to compare
at that expression. Step 3 classifies it as a **pilot limitation**: KerML constrains the related
features of a binding connector, but no numbered normative constraint was found for this
argument-level operator-expression warning, and the pilot validator source marks the check TODO.

**The severity defect the first run found is closed: it was 60 rows before the library-rule work and is 0 now.**
The pilot declares:

```
//* XPECT warnings ---
"Duplicate of inherited member name 'p' from A2" at "feature redefines"
--- */
```

and on 60 rows we produced an error at that line until the library-rule work made the rule a warning over library
bases. Step 3 closed the remaining interface-end row by deriving binary interface typing for exactly
two-ended interfaces; the inherited end at that position supplies the port type and exposes the
`Part, Port` diamond. The `Subsetting/redefining feature should not have
larger multiplicity upper bound` rule, 8 rows of nothing on the first run, and
`User library packages should not be marked as standard`, 1 row, are both implemented and agreeing.

One reading trap in the per-kind table above: the `nothing` column subtracts the agreements as well
as the tolerances, so it reads **1** — the row where nothing of ours is there at all.

### The library diamond, as a warning

The severity finding above is what the library-rule work addressed, and the four rules it adds live in
`internal/core/passes` (`w9c_inherited_name_conflict.go`, `w9c_owned_name_and_library.go`,
`w9c_bound_feature_types.go`), registered in `analyze.go` and level-scoped: the distinguishability
and library-package rules at `LevelNameResolution`, the diamond and bound-feature rules at
`LevelType` (`w9c_rules_test.go:TestW9CPassesAreRegistered`).

The diamond rule is the one that moved the family. The pilot keeps an ill-typed declaration and
unions the implicit base of the declaration's kind with the base its *declared* type reaches, then
names, per conflicting member, the types that declare it — which is why one declaration draws
`'self' from AnalysisCase, Part` beside `'start' from Action, Part`
(`CaseUsage_Invalid.sysml.xt:75-77`). The rule reproduces that union over library bases only, at
warning severity, and stays silent where the diamond is not real: a variant references a member of
its variation rather than specializing it, a redefinition of an untyped library feature replaces its
implicit value typing, and a member another candidate redefines is not inherited twice. It reads
library members through `symbols.Index.LookupDirectChildren`, so it behaves identically whether the
standard library was parsed or restored from cache (each focused test runs both states).

#### The diamond beside a failed typing is not a consequence to suppress

A validation assessment proposed suppressing this warning on an element whose usage typing had
already been reported — `timeslice t : A;` with `A` an attribute def draws
`An occurrence, item or part must be typed by occurrence definitions.` and then
`Duplicate of inherited member name 'self' from DataValue, Occurrence`, and the second was read as
noise that only exists because of the first. Refereed against the pinned pilot (`2026-07`,
`0.61.0`), that reading does not hold: the pilot reports **both** diagnostics on that probe, at the
same line and severity as ours, and does the same for every other kind mismatch tried (`part`,
`item`, `action`, `port` and `snapshot` typed by an attribute def; `port` and `action` typed by a
part def; a KerML `feature` typed by a datatype and a class at once). The assessment's "two errors"
was one error and this warning.

The pilot's own suite declares the pair deliberately: at this pin, 76 `Duplicate of inherited
member name` `warnings` rows in 12 files (`ActionUsage_invalid`, `OccurrenceUsage_invalid`,
`PartUsage_invalid`, `ItemUsage_invalid`, `PortUsage_Invalid`, `StateUsage_invalid`,
`CaseUsage_Invalid`, `CalculationUsage_Invalid1`, `ConstraintUsage_Invalid`,
`RequirementUsage_Invalid`, `FlowConnectionUsage_Invalid`, `AttributeUsage_invalid`) are anchored
at, or inside, the anchor of a declared `… must be typed by …` error in the same file. Suppressing
the warning per element would turn those rows silent. The rule is therefore kept as it is, and
`TestW9CActionPartDiamondWarns`, `TestW11ASpecializationCycleKeepsImplicitBase` and
`TestW10BReferenceSubsettingContributesABase` continue to assert the warning next to the error.
What the probe did surface is a placement and wording divergence of ours on `timeslice`/`snapshot`
alone: we report `timeslice usage cannot be typed by attributeDef (…)` at the type reference where
the pilot reports the occurrence-typing rule at the usage, and `part`/`item` already match.

What is still open in the family, by reproducer:

| Rows | Reproducer | Why it still disagrees |
|---:|---|---|
| 0 | `InterfaceUsage_Invalid.sysml.xt:78` | Closed in Step 3: exactly two-ended interfaces implicitly specialize `Interfaces::BinaryInterface`, and positional end redefinition supplies the inherited port-typed end. |
| 1 | `BindingConnector_Invalid2.sysml.xt:42` | Pilot limitation: no numbered normative constraint was found for argument-level conformance on the operator expression `rearWheel+1`; the pilot validator marks that check TODO. |
| 0 | `ActionUsage_invalid.sysml.xt:61`, `StateUsage_invalid.sysml.xt:87`, `OccurrenceUsage_invalid.sysml.xt:59` | Closed: the warning now lands on the nested `perform b.a;` / `exhibit s.sa;` reference usage and the `b.a` expression inside it, where the pilot reports it, rather than on the referenced declaration. |
| 0 | `Specialization_invalid.kerml.xt:56,60` | Closed in the KerML structural-residue work, which runs `validateSpecializationSpecificNotConjugated` at the type tier so a metaclass error in the same file no longer hides it. |
| 0 | `AttributeUsage_invalid.sysml.xt:47,52` | Closed with the declared-type reading, without reintroducing the `'self' from DataValue, …` false positives across the pilot-corpora roots. |
| 0 | `ShadowingTests_ImportAndInnerClassesNamesAreTheSameBadCase3_Rdef.kerml.xt:28` | Closed in the anchor-and-residue work — see below. Not a library diamond, so the resolver's own rule owns it. |

### A supertype's imports are memberships it has

`ShadowingTests_ImportAndInnerClassesNamesAreTheSameBadCase3_Rdef.kerml.xt:28` declares
`Duplicate of inherited member name 'B' from OuterPackage` where `inner1 subsets inner` and `inner`
publicly imports `OuterPackage::*`. We were silent because `Resolver.inheritableMembers` collected
only a supertype's *owned* members. KerML 8.4.3.2 derives inherited memberships from the
supertypes' non-private memberships, and 8.3.3.1 makes a namespace's memberships its owned ones
*plus* its imported ones, so a name a supertype imported is inherited exactly as an owned one is —
and redeclaring it in the subtype is the distinguishability violation the fixture declares.

`Resolver.importedMembers` now contributes those, subject to the same two limits the owned side
already has: a private import contributes nothing, and library elements are excluded because library
supertypes are not walked (that family is `passes/w9c_inherited_name_conflict.go`, deliberately left
as the only producer over library bases). The redefinition in the fixture was a red herring: the
`hasUnresolvedRedefinition` suppression was not what swallowed the warning — the same file with the
redefinition removed was silent too.

---

## errors — 492 of 511, of which 248 wording-only

Agreement here is 243 rows word-for-word plus 248 wording-only: the same rule about the same element
at the same offset and severity, in our phrasing. Almost all of the wording-only rows are one family,
`Couldn't resolve reference to <kind> 'X'.` against `unresolved reference: X — did you mean …?`, and
the harness admits them only after matching the rule and the element named, never on span and
severity alone. What is left:

| Tolerance | Rows | Meaning |
|---|---:|---|
| `same-location` | 10 | we flag the exact declared offset for a **different rule** |
| `same-line` | 7 | we flag the declared line at a different offset — almost certainly the same defect |
| `severity-differs` | 0 | **empty:** the usage-typing rule work implemented the declared rules instead |
| `elsewhere-in-file` | 2 | we report errors, but not where the declaration points |
| nothing | 0 | **empty since the parser-production work:** no declared-error file is accepted in silence |

The disagreements split 15 KerML / 4 SysML across this kind. The SysML suite's assertions anchor at a whole
declaration (`at "part def P { ... }"`) while ours land on the offending token inside it, so
`same-line` there often means what `same-location` means in KerML. Together, **508 of 510 declared
errors are ours at the declared location or line.**

**The 10 remaining `same-location` rows are the ones the wording-only class deliberately refuses.**
They sit at the declared offset with the declared severity and state a *different rule*, so admitting
them would hide the following divergences:

- **6 parse-shape rows** (`ParsingTests_BadScopeWithOnlyTwoDot.kerml.xt`:21 and :26,
  `ParsingTests_BadScopeWithOnlyTwoDotAtTheEnd.kerml.xt`:21,
  `ParsingTests_BadScopeWithOnlyTwoSingleDot.kerml.xt`:21, and
  `ParsingTests_BadScopeWithOnlyTwoSingleDotAtTheEnd.kerml.xt`:21 and :26) where the pilot cannot
  resolve the reference at all and we resolve it and reject the *kind*
  (`type must be a type, found package`, `subsets target must be a feature, found kermlType`) — the
  rows here where our answer is arguably the more precise one. Element-scoped subsetting validation
  brought the three `non` rows into this class out of `elsewhere-in-file`.
- **`ParsingTests_ScopeWithFourDotAndDot.kerml.xt`:22**, where the parser-production work replaced a false negative with
  `feature chain segment must be a feature, found kermlType` on the same declared reference —
  a KerML 8.3.4.7 chain rule rather than the pilot's unresolved-reference verdict.
- **2 bare-import rows** (`ParsingTests_Import_Visibility.kerml.xt`:23,
  `Import_Visibility_Invalid.sysml.xt`:23), which the conformance-mode work moved *into* this class: our
  `import without a visibility indicator: S` is now an error by default rather than a warning, so
  these left `severity-differs`. The pilot rejects the same line as a syntax error instead — an
  **adjudicated divergence** since the scope-traversal work, whose reading is in
  [adjudications.md](adjudications.md).
- **`InterfaceUsage_Invalid.sysml.xt`:49**, a **pilot limitation**: SysML v2 §7.14.1 permits three
  or more interface ends, while §7.14.2 and §8.3.14.2 constrain only `BinaryInterface`; our
  independent port-kind error is at :53.
The two `Specialization_invalid.kerml.xt` specialization rows and
`CaseSubjectObjective_Invalid.sysml.xt`:80 left this class once the usage-typing rule work implemented the
specialization metaclass rules and the structural-residue work re-attached the conjugation rule at the type tier, and the
objective count is now ours.

**The `nothing` column is now empty: 49 → 8, then 8 → 4, then 4 → 3 with the scope-traversal work** — which
closed E3 by resolving a connector end's participant where the connector is featured — **and 3 → 0 in
the parser-production work**, which closed the last three:

- **`ParsingTests_ScopeWithFourDotAndDot.kerml.xt`:22 (two rows)** — the false negative is gone; we now
  report a feature-chain kind error at the declared reference, so both rows moved into
  `same-location`/`same-line` rather than staying silent.
- **`Type_Multiplicity_invalid.kerml.xt`:20** — E1 **closed in the parser-production work**: the parser now accepts the
  surplus `multiplicity` member (`Type::multiplicity` is single-valued, KerML 8.3.3.1.1) and the rule
  reports `Only one multiplicity is allowed` where the pilot does.

The four `Feature_invalid_noType` rows — `Features must have at least one type` and its implicit-base
half — closed in the KerML structural-residue work, which implemented both halves in both suites.

**`severity-differs` is empty, and that is the clearest result here.** Every one of its 11 rows
declared a typing or specialization rule (`An action must be typed by action definitions.`,
`Cannot specialize class or association`) while what we put on the line was the inherited-name
warning. They were missing detection wearing a cosmetic label: the usage-typing rule work implemented the six rules,
the resolver work added the use-case analogues and canonicalized the resolver's inherited-name warning so it stops
standing in for them, and the column closed by implementation rather than by relabelling.

Every row that is not word-for-word — all 248 wording-only and all 28 disagreements — is recorded
individually with the declared message and ours in
[pilot-xpect-baseline.json](pilot-xpect-baseline.json).

### Attribution of the `same-line` and `elsewhere-in-file` rows

The 9 rows in these two tolerance classes are attributed below. The Xpect row is the line containing
the assertion; its declared diagnostic generally anchors in the following model line. The citations
are to the published KerML 1.0 and SysML v2.0 specifications that govern the pinned 2026-07
implementation. There is no published KerML 1.1 specification to cite; where parser behavior matters,
the pinned `2026-07` grammar was checked as well. No category below is inferred from diagnostic
wording alone.

| Xpect row | Declared | OpenSysML | Specification reading | Category | Owner |
|---|---|---|---|---|---|
| `ParsingTests_BadScopeWithOnlyTwoSingleDot.kerml.xt`:26 (`..`) | `no viable alternative at input '..'` | One `expected a name after '.'` at model line 31. | A qualified name uses `::` (KerML 8.2.3.4.1), while a feature chain is qualified names separated by one or more single `.` tokens (KerML 7.3.4.6 and 8.2.4.3.5). `test..A::a` is malformed; the specification does not prescribe ANTLR alternatives, recovery offsets, diagnostic wording, or one diagnostic per recovered token. | **adjudicated divergence** | parser recovery (the parser-production work) |
| `ParsingTests_BadScopeWithOnlyTwoSingleDot.kerml.xt`:26 (`::`) | `no viable alternative at input '::'` | The same single `expected a name after '.'` at model line 31. | Same malformed `test..A::a` and the same KerML 8.2.3.4.1/8.2.4.3.5 reading; this is a second pilot recovery trace for one malformed chain, not a second semantic obligation. | **adjudicated divergence** | parser recovery (the parser-production work) |
| `ParsingTests_BadScopeWithOnlyTwoSingleDot.kerml.xt`:26 (`A`) | `no viable alternative at input 'A'` | The same single `expected a name after '.'` at model line 31. | Same malformed `test..A::a` and parser-recovery reading. | **adjudicated divergence** | parser recovery (the parser-production work) |
| `ParsingTests_BadScopeWithOnlyTwoSingleDotAtTheEnd.kerml.xt`:26 (`..`) | `no viable alternative at input '..'` | One `expected a name after '.'` at model line 32. | `test::A..a` violates the same qualified-name and feature-chain productions (KerML 8.2.3.4.1, 7.3.4.6 and 8.2.4.3.5). Exact recovery traces are not specified. | **adjudicated divergence** | parser recovery (the parser-production work) |
| `ParsingTests_BadScopeWithOnlyTwoSingleDotAtTheEnd.kerml.xt`:26 (`A`) | `no viable alternative at input 'A'` | The same single `expected a name after '.'` at model line 32. | Same malformed `test::A..a` and parser-recovery reading. | **adjudicated divergence** | parser recovery (the parser-production work) |
| `ParsingTests_Import_Visibility.kerml.xt`:25 | `extraneous input '}' expecting EOF` | No diagnostic at the brace; the nearest is the direct bare-import error at model line 24. | Import visibility is mandatory (KerML 7.2.5.4 and 8.2.3.4.2). OpenSysML reports that violated rule once; the brace message is the pilot parser's cascade, and the specification does not require it. | **adjudicated divergence** | parser recovery (the scope-traversal work) |
| `ParsingTests_ScopeWithFourDotAndDot.kerml.xt`:22 | `Couldn't resolve reference to Feature 'b'.` | `feature chain segment must be a feature, found kermlType` on model line 27. | Every chaining element is a Feature (KerML 7.3.4.6, 8.2.4.3.5 and 8.3.3.3.5). OpenSysML resolves `OuterPackage::B` and reports its wrong metaclass; the pilot filters/fails the lookup. Both reject the same chain, and KerML does not require failed resolution rather than a direct metaclass diagnostic. | **adjudicated divergence** | feature-chain type validation (the parser-production work) |
| `Import_Visibility_Invalid.sysml.xt`:25 | `extraneous input '}' expecting EOF` | No diagnostic at the brace; the nearest is the direct bare-import error at model line 24. | Import visibility is mandatory in the SysML package-import production (SysML v2 7.5.3 and 8.2.2.5.1). The pilot's brace error is an unspecified recovery cascade after the same bare import OpenSysML rejects directly. | **adjudicated divergence** | parser recovery (the scope-traversal work) |
| `TransitionUsage_invalid.sysml.xt`:60 | `Must be a Boolean expression.` | `transition guard must be Boolean, found String` on the declared model line. | A transition guard must be a Boolean-valued expression of multiplicity 1 (SysML v2 7.18.1, 7.18.3 and 8.3.18.8 `validateTransitionFeatureMembershipGuardExpression`). OpenSysML now evaluates the guard independently of the unrelated earlier name-resolution error; the remaining difference is diagnostic wording and the expression-only span. | **adjudicated divergence** | transition-guard validation |

Step 3 closes the former `AssignmentActionUsage_invalid.sysml.xt:44` row with an element-scoped
constraint pass implementing SysML v2 §8.3.17.5
`referent.featureTarget.mayTimeVary`; it moves from `elsewhere-in-file` to word-for-word agreement.
The same element-scoped pattern now lets the three previously gated `non` rows report the existing
subsetting-metaclass rule at their declared offsets, and lets the transition guard report on its
declared line. The five malformed-chain rows remain parser-recovery divergences.

---

## scope — 230 of 230 agree exactly

`scope` is a different oracle from everything else here. `linkedName` asks what one reference
resolves to; a `scope` assertion declares **the complete set of names visible at a point**, so it
sees the half of a scoping defect nothing else we run can see — a name that is visible and should not
be. All 230 live in the KerML suite; 229 of them use the `//*` block form.

### What the assertion means

The pilot's Xpect method takes the offset's `EObject` **and its cross-reference**, asks the scope
provider for the `IScope` that reference sees, and compares the declared list against
`scope.getAllElements()`:

```java
IScope scope = scopeProvider.getScope(arg1.getEObject(), arg1.getCrossEReference());
expectation.assertEquals(new ScopeAllElements(scope), new IsInScope(converter, scope));
```

Read literally, that fixes six things, and each one had to be reproduced to adjudicate a row:

- **The anchor is a reference, not a position.** `XPECT scope at P1::A` anchors at the cross-reference
  written after the note; the metatype of that reference filters the result, so a specialization
  reference sees types and a feature-typing reference sees features. Where a `scope` note anchors at a
  declaration rather than a reference, the harness adjudicates the unfiltered scope at that offset.
- **Every spelling is a separate entry.** The declared lists are qualified-name *paths*, not elements:
  `A`, `P1.A`, `Import_Circular.P1.A` are three declared names for one element, and a path continues
  through members — `Test1.x.that.self` is declared as well as `Test1.x`.
- **Inherited, imported and recursively imported members are in**, under every path that reaches them.
- **Implicit library members are in** when the fixture's resource set loads the library file that
  declares them: `Base.kerml` gives a `classifier` `self` (hence `A.self`, `A.self.that`) and a
  `feature` both `self` and `that`.
- **A circular containment truncates.** `classifier A { import B::*; classifier B specializes A }`
  declares `A.B.B` and stops there.
- **Order is not meaningful** — the comparison is set-based. Our own output is sorted by name so two
  runs are byte-identical.

### The surface

`Workspace.VisibleNames` / `VisibleNamesAt` (`internal/core/model/scope_names.go`) is a read-only
enumeration over the same resolver the LSP and `resolve/unqualified.go` use — `Resolver.ResolveName`,
`visibility.go`'s import and specialization rules, `semantics.MembersOf` — with no parallel scope walk
in the harness, which is the only way the answer can be evidence about *our* behaviour rather than
about the harness. It walks the anchor scope and its ancestors, their inherited and imported members,
and the non-library index roots, emitting each path once, and returns `{Name, FQN, Kind, Depth}` so a
caller can filter by metatype above it. `internal/core/model/scope_names_test.go` locks locals,
private and protected members, imports, alias binding names, short-name membership imports,
inheritance through typing, library gating, circular imports, redefinition anchors and determinism.

### The result, by class

| Class | Rows | Reading |
|---|---:|---|
| agree (exact) | 230 | the declared set, name for name |
| `library-names` | 0 | — |
| `extra-names` | 0 | — |
| `missing-and-extra` | 0 | — |
| `missing-names` | 0 | — |
| `other-paths` | 0 | — |

**Exact agreement on every row is a result about one rule, not about 230 independent ones.** The class
that dominated this kind early on — implicit members, 96 rows then 125 — was two separate
defects, both fixed in the library-member resolver work. The visibility reconciliation emptied
`other-paths` and took agreement to 183; the re-entry bound's
one-re-entry bound and per-anchor accounting took it to 212; the anchor-and-residue work took it to 221 by fixing the
quoted `scope` anchor and stopping a recursive import's descent from carrying implicit generals. The
nine rows left after that were **one** enumeration rule and **one** harness rule, both closed in the scope-traversal work and both derived in [adjudications.md](adjudications.md):

1. **How far a derived path may be re-derived (`library-names`, `extra-names`, `missing-and-extra`, 8
   rows).** A path may continue through a member source it has not already traversed, and a
   feature's declared type is entered before its implicit base, so `self` and `that` — which
   `Base::Anything` declares as `subsets things chains things.that` — extend a path exactly as far as
   the declaration they were derived from allows, and a circular containment truncates where the
   pilot truncates. That single rule closed the `CircleProblem4` family, the two
   `ImportPackageAndInheritanceFromContainer` variants, `Import_Recursive3`, and the two
   `CircleProblem3` anchors that were 394 and 328 names short. Locked by
   `TestVisibleNamesInheritedFeatureEndsThePath` and
   `TestVisibleNamesMutualImportBoundsPathsNotImplicitMembers`.
2. **Which occurrence of the declared text a `scope` note anchors at (`missing-and-extra`, 1 row).**
   The `at` text names the *reference* the question is about, so an occurrence that starts a longer
   identifier — `c_Public` in `specializes c_Public_Id` — is the anchor when it carries one, and
   otherwise the first whole identifier is (`scopeAnchor`, `cmd/pilot-xpect/scope.go`). That is a
   harness rule, not a rule about our behaviour, and it closes
   `imports/recursive/ShortName_Import_Valid1.kerml.xt`:25 — previously classified a pilot limitation
   — without loosening diagnostic matching, which still requires a whole identifier.

**A closed class is not a conformance claim about names the corpus never asks about.** These 230
anchors are the pilot's own tests; a construct with no `scope` note is not endorsed by its absence.

Short-name membership imports were the one defect inside this record's ownership and are fixed in the
surface: `public import VP::VP2::A_Id` where the element is declared `classifier <'A_Id'> B` now
surfaces both of the element's names, as `imports/ShortName_Import_Valid4.kerml.xt` declares, while an
alias membership import still surfaces the alias name only.

The per-row evidence — declared count, our count, and the first missing/extra names for each
disagreement — is in [pilot-xpect-baseline.json](pilot-xpect-baseline.json).

---

## The global namespace, and what the self/that residue really is

Four adjudications and one open question came out of the 34 rows the anchor-and-residue work owned (all 18 `scope` rows
and the 16 `noErrors` rows). They are recorded here because each one decides what an answer *means*,
not merely how it is computed.

**1. Two root namespaces of the same name are not an ambiguity.**
`imports/global/DependencySamePackageName.kerml.xt` loads two files that each declare
`package SamePackage`, declares the file clean, and declares both subtrees visible under one name
(`SamePackage.container.A` *and* `SamePackage.container.B`, with `SamePackage` itself listed twice).
Its own authors left the comment `//What global scope should be?????`. Resolution in the global
namespace is single-valued in KerML 8.2.3.5 — a qualified name resolves to one membership or to
none — and distinguishable naming constrains a *Namespace's* own members, which the global namespace
is not. So the repeat is not ill-formed and the first root is the answer:
`Resolver.lookupGlobalTop` returns `syms[0]` instead of reporting `ambiguous`, and a qualified tail
walked from one namespace no longer picks up the same-named other one's members
(`notConflatedWith`, `internal/core/resolve/qualified.go`). That closes the four
`ambiguous reference: SamePackage::container (2 candidates)` `noErrors` rows and the
`DependencySamePackageName` `scope` row, which was missing exactly the second root's paths.
`TestNameResolutionPassResolvesARepeatedTopLevelNameToTheFirst` replaces the ambiguity test that
asserted the opposite; a `$::`-rooted name is exempt from the filter, since it names a path in the
global namespace where every root's members are reachable.

**2. `noErrors` is Xpect's residue, not "no error anywhere".** Xpect matches each issue against the
expectations' regions and fails a file on what is left over, so an error a sibling `errors`
expectation declares is not the file's residue. The harness now models that
(`consumedLine`, `cmd/pilot-xpect/compare.go`) — which is what the six protected-import
contradictions above were really about, and it closes them without exempting a fixture or extending
the wording-only class.

**3. A `scope` anchor may be spelled with quotes.** `scoping/ShortName_Scoping_Valid1.kerml.xt`
anchors `at 1` on the reference `specializes '1'`; the declared text omits the quotes our span
carries, so the harness indexed no reference there and adjudicated the *unfiltered* scope. Accepting
a segment that starts one byte before the located text fixes the anchor, not the enumeration.

**4. A recursive import's descent carries no implicit generals.** A membership named directly by an
import keeps its implicit members; a name the recursive descent *finds* is entered through the
import, and the path does not then traverse the general types the element has by its kind rather
than by declaration (`Model.ImplicitGenerals`, `internal/core/semantics/implicit.go`, applied by
`declaredSources` in `scope_names.go`). That is what closes `Import_Recursive1`, `_4` and `_5`.

**The open question — the pilot's expansion bound is not the one recorded earlier.** *(Settled in the scope-traversal work: the bound is neither a name count nor a step budget but a per-path traversal rule — see
[adjudications.md](adjudications.md). The paragraph below is kept as the anchor-and-residue work recorded it.)* Six rows remain
whose extras or omissions are all `self`/`that` tails, and the pair
`ShadowingTests_CircleProblem4.kerml.xt` / `_FT` shows the bound is not a name-occurrence count:
the two fixtures differ only in `classifier A specializes A::B` versus `feature A : A::B`, and the
plain form declares `b.B.b.self` while the `_FT` form declares `b.B.b` and stops. Every `_FT` path
that stops one step early ends where a *feature* is re-entered, and the plain form's do not, so the
bound looks like a budget on the derivation steps a path takes through a type rather than on repeated
names. We have no specification statement that fixes either bound, and adopting the pilot's stopping
points without one would be fitting output. **This is an adjudication question, not a defect**, and
with it stand the two `ShadowingTests_CircleProblem3.kerml.xt` rows: 456 names at `A` against 829
declared and 382 at `B` against 696, where per-anchor filtering between two mutually
wildcard-importing scopes and the `A.B.B` re-entry both hang off the same bound.
`imports/recursive/ShortName_Import_Valid1.kerml.xt` is a seventh, and a harness question rather
than ours: its `at c_Public` matches inside `c_Public_Id` in the pilot, which anchors it on a
classifier reference in `Test1`, while our identifier-boundary rule walks past it to a declaration in
`VP::VP1`. Relaxing the rule globally costs ten other rows, so the anchor needs Xpect's own matching,
not a looser one.

### The rows that stay open, each labelled

An open row with no label reads as a defect, so every one of the twelve is classified here. The
categories are: **our defect** (the specification says one thing and we do another), **spec-derived
obligation not implemented** (we agree what is required and have not built it), **pilot limitation**
(the reference is the one departing from the specification, or the harness cannot express the
question), and **adjudicated divergence** (a difference we keep deliberately, with the reasoning
recorded).

| Row | Shape | Category |
|---|---|---|
| `ShadowingTests_CircleProblem4.kerml.xt`:32 | 30 missing / 1 extra, all `self`/`that` tails | **Open adjudication — not yet classifiable.** The expansion bound is underdetermined by the specification (the question above); the mismatch is real but no reading makes it our defect or the pilot's until the bound is settled. |
| `ShadowingTests_CircleProblem4_FT.kerml.xt`:43 | 3 extra: `b.B.b.self`, `.that`, `.that.self` | same |
| `ShadowingTests_CircleProblem4_Rdef.kerml.xt`:43 | 3 extra, identically | same |
| `SimpleImportTests_ImportPackageAndInheritanceFromContainer_FT.kerml.xt`:23 | 3 extra: `a.A.a.self`, `.that`, `.that.self` | same |
| `SimpleImportTests_ImportPackageAndInheritanceFromContainer_Rdef.kerml.xt`:23 | 3 extra, identically | same |
| `Import_Recursive3.kerml.xt`:55 | 2 missing (`s.self.that`), both reachable by another path | same |
| `ShadowingTests_CircleProblem3.kerml.xt`:23 | 456 of 829, 21 extra | same, plus the per-anchor filtering carry-over from the re-entry bound, which hangs off the same bound |
| `ShadowingTests_CircleProblem3.kerml.xt`:183 | 382 of 696, 14 extra | same |
| `ShortName_Import_Valid1.kerml.xt`:25 | declared 75, ours 170 at a different anchor | **Pilot limitation (harness).** The pilot's `at c_Public` matches inside `c_Public_Id`; reproducing that needs Xpect's own substring matching, and loosening our identifier-boundary rule costs ten other rows. |
| `SimpleImportTests_CircleInheritanceInCircleImport.kerml.xt`:18 | `a participates in a specialization cycle` | **Adjudicated divergence.** The fixture declares `classifier a specializes b` / `b specializes a`; the pilot has no cycle check (finding F4/K5 in [pilot-differential.md](pilot-differential.md#specialization-cycles-f4)). Closing the row would mean deleting a correct rule. |
| `simpletests/PartTest.sysml.xt`:29 | `p1 participates in a specialization cycle` | same — `p1 :> p2 :> p3 :> p1` and `p4 :> p4` |
| `Redefinition_OwningType_Cyclic_Gen.sysml.xt`:25 | `A participates in a specialization cycle` | same — `A :> C` with `C :> A, B` |

None of the twelve is a *spec-derived obligation we have not implemented*: the eight scope rows are
enumeration-bound questions, the ninth is a harness one, and the three `noErrors` rows are a rule we
do implement and the reference does not.

**The scope-traversal work closed all nine `scope` rows in this table**, the eight by the traversal rule and
`ShortName_Import_Valid1` by anchoring a `scope` note on the reference its `at` text names; both
derivations, with the before/after missing and extra multisets, are in
[adjudications.md](adjudications.md). The three specialization-cycle `noErrors` rows stand as
adjudicated divergences.

### Oracle movement, measured on both trees

Control `main` at `b86aeb18` against `main` at `ae4fdf9e` (both anchor-and-residue PRs merged, and the commits between them):

| Oracle | `b86aeb18` | `ae4fdf9e` |
|---|---|---|
| xpect | 1197 agree (239 wording-only) / 129 disagree | 1221 agree (241 wording-only) / 105 disagree |
| differential | 353 files, **312** fully agreeing; 25 agreed, **139** only ours, 73 only the pilot's | identical |
| rejection | 120 cases: 115 both reject, 5 only the pilot, 0 only us | identical |

**Both columns are now known to be understated, and the reconciliation the note below asked for is
done: the discrepancy was a stale on-disk library index cache, not a measurement dispute.** Records
in `~/.cache/sysml-ls/libs` were keyed on content, library-set digest and record format but not on
the binary, so records written by an earlier build stayed "valid" across every merge and changed
what later builds saw. Re-running the same commits with a fresh `XDG_CACHE_HOME` gives higher
agreement in every oracle, identically cold and warm. The cache key now includes a build identity and
a library is index-only on both load paths, so a hit and a miss expose the same state
(`internal/core/libs`). Every figure in this document is a fresh-cache run on `main` with that work merged; the two columns above are kept as recorded history rather than corrected in place.

The handoff quoted the differential at `b86aeb18` as `311` fully agreeing with `142` only
ours, against the `312`/`139` above: the same cache effect, and the difference is not averaged
anywhere.

---

## Not adjudicated

One assertion is read, counted and reported, and deliberately not scored:

| Kind | Assertions | Why |
|---|---:|---|
| `exportedObjects` | 1 | `indexing/NameEscape.kerml.xt` — declares the index contents for escaped names. One assertion, no general mechanism worth building. |

They are `not-adjudicated` in the report and the baseline, never counted as agreements.

---

## What this report does not fix

**No disagreement recorded here is fixed by this document**; it records what the oracle says, and the
fixes are scoped from it in later rounds. The first two items of the original list are now closed
(alias identity, and the duplicate-member-name warnings existing at all); what is open, in the order
this report reads it:

1. **`Duplicate of inherited member name` location and coverage** — the severity half is closed by
   the library-rule work (60 rows ours as an error → 0) and a later round closed the offsets; 1 `warnings` row sits behind
   another rule's error and 1 draws nothing.
2. **No declared error goes unreported** — down from 195 on the first run, 49 after the visibility reconciliation, 20
   before the specialization-visibility restoration restored the protected-import rejections, 8 after it, 4 later, 3 after the scope-traversal work, and **0** after the parser-production work closed E1 with a parser production and gave
   `ScopeWithFourDotAndDot` a feature-chain diagnostic in place of a false negative.
3. **The 3 remaining parse-recovery `noErrors` rows** — the `QPE-*` query-path-expression fixtures,
   adjudicated as a pilot limitation in [adjudications.md](adjudications.md) because the pinned
   validator rejects them too.
4. **The 2 unresolved-reference `noErrors` rows** — the import family, which `linkedName`'s
   194 agreements do not reach. The protected-import rows are **not** on this list and are no longer a
   contradiction either: the anchor-and-residue work closed them by scoring a `noErrors` note against Xpect's residue
   rather than against file-wide silence.
5. **No `scope` disagreement is left**: the 9 that stood after the anchor-and-residue work were one enumeration rule and
   one harness rule, both closed in the scope-traversal work — see [scope](#scope--230-of-230-agree-exactly) and
   [adjudications.md](adjudications.md).
6. **The 5 rows we keep deliberately** — 3 specialization-cycle `noErrors` rows, where the reference
   has no cycle check at all, and the 2 `SimpleImportTestsFromOtherFile_Import3{,_FT}` rows, where it
   validates subsetting type conformance nowhere (E4). Both are adjudicated divergences, not backlog.

The KerML validation and visibility residue the KerML structural-residue work left open is enumerated with a category and an
owner per row in [adjudications.md](adjudications.md): **E1** `Type_Multiplicity_invalid` and
**E2** `AssociationTest_CrossFeatures_invalid` were unimplemented obligations owned by the parser and
closed in the parser-production work ([adjudications.md](adjudications.md)), **E3** `ConnectorTest_ConnectorEndSubsettingBadCase` and **E5**
`VisibilityTests_Protected_FeatureChaining` were our defects owned by the resolver and closed in the scope-traversal work ([adjudications.md](adjudications.md)), and **E4** is the adjudicated divergence above. Read that page beside this one: an open row with no category reads as a
defect, and two of these six are not.


## The declared errata overlay

This oracle reports its census twice, as published and with the [declared errata](errata-overlay.md)
applied. No declared correction lies under `build/pilot-xpect-corpus`, so the two figures coincide
and the report says why in those words rather than printing an unexplained equality:

```
no declared correction lies under build/pilot-xpect-corpus, so the errata-applied corpus is
byte-identical to the published one and both figures coincide
```

The registry is still restated here with each entry's citation, and a correction landing in a `.xt`
suite later would materialise a corrected copy of it and re-adjudicate every assertion against that
copy, leaving the published corpus untouched.
