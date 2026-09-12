# Pilot Rejection Oracle

Every other oracle in this project is one-directional. The
[differential](pilot-differential.md) compares diagnostics over the OMG corpora — models written
to *demonstrate* the notation, so almost all of them are valid — and therefore measures notation
the reference accepts and we reject. Nothing in it tests the opposite direction: does OpenSysML
**reject** what the reference rejects? `cmd/pilot-reject` answers that with a hand-written
negative corpus, validated by both implementations. A case the pinned pilot rejects and we accept
is a **permissiveness gap** — the finding this oracle exists to surface.

The oracle is advisory: nothing in the build or test suite depends on its verdicts. Its verdicts
are externally refereed — the pilot's verdict on every case comes from actually running the
pinned validators, not from our reading of the grammar. Our adjudication of *why* each gap exists
(the "likely root cause" column) is self-assessed.

**Labels:** the short labels in this record are internal cross-references, not specification or
product terms. `g<n>`, `k<n>`, `p<n>`, `s<n>` and `x<n>` name the corpus cases below, `F<n>` names a
row of the follow-up table in [pilot-differential.md](pilot-differential.md), and `K<n>`/`S<n>` its
KerML and SysML diagnostic classes. A reader who only wants the verdicts can ignore all of them.

The corpus is organised by the rule each case violates, not by the pilot's constraint names, so
it does not say which of the pilot's named validation constraints have a case and which do not.
The [validation-constraint census](validation-constraints.md) does: one row per name the pinned
jar declares, with the case here that exercises it (`none` where the corpus has none yet) beside
the pass that implements it and its census status.

## Pinned reference

The same pin as the differential: OMG SysML v2 Pilot Implementation `2026-08`
(`jupyter-sysml-kernel 0.62.0`, see `scripts/pilot-pin.sh`). Two validators referee:

- `build/pilot-sysml-validator/validate-sysml-batch` for `.sysml` cases
  (`./scripts/download-pilot-sysml-validator.sh`)
- `build/pilot-kerml-validator/validate-kerml` for `.kerml` cases
  (`./scripts/download-pilot-kerml-validator.sh`)

`./scripts/download-pilot-reject-validators.sh` provisions both. Both load the pinned standard
library, so verdicts that require library-relative semantics (implicit specialization, implicit
typing) are refereed under the same conditions our workspace validates under.

## Corpus derivation

The corpus is committed under `cmd/pilot-reject/testdata/negative/`. Every file's first line is a
mandatory header — `// Invalid: <rule> (<citation>).` — naming the one rule the case violates and
where that rule comes from; the harness refuses a corpus file without it. Cases were derived
systematically from four sources, one subdirectory each:

1. **`grammar/` — grammar mutation** (91 cases: 20 original, 45 added along the *unreached* axis
   described below, 13 from a second sweep, 7 body-position cases
   `g61`–`g67` from the constraint census described under `semantic/`, the two second-result-expression
   bodies `g69`/`k20`, and the two name-before-keyword members `g70`/`k21`). For productions our corpus exercises in the
   pinned Xtext grammars (`build/pilot-grammars/`, see the `testing-grammar-coverage` skill), the
   minimal violation: a required keyword removed (`g03` alias without `for`), a mandatory element
   omitted (`g04`, `g05`, `k01`, `k03`), a clause in a position the production forbids (`g06`
   multiplicity on a definition, `g07`/`g08` state members in a part def body), a token from a
   sibling production (`g15` a keyword as a name, `k02` a SysML keyword in KerML), and unterminated
   bodies and comments (`g01`, `g12`, `k05`).
2. **`extensions/` — the notation we invented** (8 cases). Every state-machine construct our
   `examples/` tree uses that no pinned production admits: `initial`, `choice`, `junction`,
   `history`, `region`, `defer`, and the `transition <src> to <tgt>` shorthand, plus `require`
   outside a requirement body (`x08`), which our grammar admits as an extension. The pinned grammar
   spells entry as `entry; then <state>`, concurrency as `state ... parallel`, and transitions as
   `first <src> then <tgt>`, and has no pseudostates or deferral at all. Adjudication: these are
   **intended OpenSysML extensions**, not accidents — each has dedicated parser tests
   (`internal/core/parser/state_notation_test.go`) and runtime support. They are documented as
   extensions and, since strict mode was added, gated behind an opt-in
   [strict conformance mode](../guide/03-command-line.md#strict-conformance) a conformance-minded
   user can turn on; the default mode keeps accepting them on purpose.
3. **`xpect/` — the pilot's own negative expectations** (34 cases: 7 original, 27 added against the
   semantic rules the validation work implements). The Xpect suites declare 513
   `errors` expectations ([pilot-xpect.md](pilot-xpect.md)); where a suite declares an error we do
   not report anywhere in the file, that is a candidate rejection gap. Each case here re-derives
   one such declared error as a standalone model, citing the KerML clause and the originating
   `.xt` suite. One caveat found while deriving: some Xpect negatives (e.g.
   `Feature_invalid_noType.kerml.xt`) only error in a library-less resource set — with the
   standard library loaded, `feature f;` gets an implicit type and is legal — so only
   library-independent expectations became cases.
4. **`semantic/` — the pilot validators' named constraints** (102 cases). The pinned
   `KerMLValidator` and `SysMLValidator` implement 217 named `validate*` constraints
   (`validate<Metaclass><Rule>`, the names of the specification's own constraint clauses), and
   before this source only the 34 `xpect/` cases tested any of them. Each case here is one minimal
   standalone model that violates one named constraint, with the header
   `// Invalid: <rule> (<clause>; pilot validate<Name>).` so the constraint census can join on the
   name. Coverage was derived by reading every constraint's Java implementation and message
   string, and every candidate was refereed by the pinned validator before being kept. The KerML
   constraints (those whose source is KerML or shared with SysML) gave 42 `.kerml` cases
   (`k01`–`k42`) and one `.sysml` case (`s80`, a shared constraint whose only legal violating
   spelling is SysML's `constant` usage prefix). The SysML constraints were mapped name by name —
   an existing case where an `xpect/` or `grammar/` case already violates the rule (their headers
   now cite the `validate*` name), a new minimal model here otherwise — giving 45 `.sysml` cases
   (`s01`–`s45`); the full mapping is the [constraint census](#sysml-constraint-census) at the end
   of this document. `s46` restates a bound feature value (`= 1`) from a redefining usage and a
   redefining definition (`validateFeatureValueOverriding`), found by probing `bin/sysml` against
   the reference validator rather than by an Xpect suite; the rule's own record is its row in
   [spec-compliance.md](spec-compliance.md). `cn01`–`cn09` are the control-node succession rules of SysML v2 8.3.17:
   the count bounds (a fork or decision node with two incoming successions, a join or merge node
   with two outgoing), the end multiplicities (`1..1` into any control node and out of it, `0..1`
   into a merge and out of a decision), and the owning type (a control node in a constraint
   body). The pinned pilot implements only the owning-type rule (`validateControlNodeOwningType`),
   so `cn05` is a both-reject case and the other eight land in **ours-only-rejects** by design —
   the header of each says so. They are kept because the corpus documents the rule; the pilot gap
   is adjudicated in [pilot-differential.md](pilot-differential.md). Two things this source
   records that the buckets cannot: constraints the pilot
   declares but only warns about or never checks, and constraints for which no legal violating
   model exists under the loaded standard library — both listed under
   [Permissiveness gaps](#permissiveness-gaps) below, since a both-accept case is a corpus bug
   and none was kept. The send-action family contributes three further `.sysml` cases, each the
   minimal model our validator was found permissive on and re-checked after the rule was
   implemented: a payload that invokes a non-behavior (`send-payload-non-behavior`), a state
   subaction or transition effect send with no payload (`send-subaction-no-payload`, which the
   pilot's grammar rejects before its validator would), and a constructed payload whose `new`
   names a package rather than a type (`send-constructor-non-type`).

What this corpus cannot see: it tests the invalid models we thought to write. **We authored all 285
cases ourselves**, so the denominator measures our coverage of the rejection surface, not our
conformance: it is a **sample, not a proof** — a clean bucket here does not mean OpenSysML rejects
everything the reference rejects, and no official conformance suite exists to make that claim
testable. The pilot's verdict on each case is externally refereed; the choice of cases is not.

## Running it

```bash
./scripts/download-pilot-reject-validators.sh   # once; needs Java 17+ and Maven
go run ./cmd/pilot-reject                       # -conformance auto, the committed baseline
go run ./cmd/pilot-reject -conformance default  # every case judged as the CLI judges by default
go run ./cmd/pilot-reject -conformance strict   # every case judged as conforming SysML v2
```

`-conformance` decides which question our side is asked. `auto` asks the `extensions/` cases —
notation OpenSysML adds on purpose — the strict one, because the reference rejects that notation as
a syntax error and only [strict mode](../guide/03-command-line.md#strict-conformance) makes the
comparison fair; every other derivation is judged in the default mode. `default` and `strict` ask
one question of the whole corpus. Every case's mode is recorded in the report, and a case that
agrees only because it was asked strictly is listed separately, so a strict agreement never reads
as a default one.

The harness validates every corpus file with our workspace and with the pinned validator for its
language, counts error-severity diagnostics on each side (warnings do not count as rejection), and
buckets every case:

- **both-reject** — agreement; the case is settled.
- **pilot-only-rejects** — a permissiveness gap; the report keeps the pilot's messages as evidence.
- **ours-only-rejects** — already the differential's business; counted and moved past. The
  `semantic/` cases whose header says the pilot has not implemented the constraint land here on
  purpose.
- **both-accept** — the case itself is wrong and must be fixed; a corpus revision, not a finding.

It writes `build/pilot-reject/pilot-reject.txt` and `build/pilot-reject/pilot-reject.json`. The
JSON is committed as [pilot-rejection-baseline.json](pilot-rejection-baseline.json); the reports
carry no timestamps or absolute paths, so repeated runs are byte-identical
(`cmp build/pilot-reject/pilot-reject.json docs/project/pilot-rejection-baseline.json`).
`-update` records a run as that baseline and `-check` fails unless a fresh run reproduces it.

## Totals

**Only this section states the current baseline**; the per-round figures further down are as
measured at their own round and are not the current baseline.

Under the default `-conformance auto`:

```
285 case(s): 276 both reject, 0 only the pilot rejects, 9 only we reject, 0 both accept
  of which 3 agree only because we were asked strictly (the default mode accepts them, by design)
```

| Source | Cases | Both reject | Pilot only | Ours only | Both accept |
| --- | --- | --- | --- | --- | --- |
| extensions | 8 | 8 | 0 | 0 | 0 |
| grammar | 91 | 91 | 0 | 0 | 0 |
| semantic | 151 | 142 | 0 | 9 | 0 |
| xpect | 35 | 35 | 0 | 0 | 0 |

Eight of the nine ours-only cases are the control-node succession rules (`cn01`–`cn04`, `cn06`–`cn09`)
the pinned pilot does not implement; the ninth, `s81`, is a non-Boolean guard on an action body's
guarded succession, which the pinned pilot's `validateTransitionFeatureMembershipGuardExpression`
leaves silent once the standard library types the guard (see the gap table below). None is a
permissiveness gap on our side; each is adjudicated as a pilot gap in the differential. The corpus grew from 79 cases to 119, to
120 with `g60` (an `alias` named by a keyword), to 225 with the `semantic/` source (97 cases)
and the 7 `grammar/` and 1 `extensions/` cases the SysML constraint census added beside it, to
228 with the three send-action cases, to 229 with `s46`, to 231 with `s47` and `g68`, to 234 with the
three trigger-argument typing cases (`s48`–`s50`), to 235 with the enumerated value typed by
its literal (`p18-enum-value-typed-outside-enumeration`), to 236 with the one typed by an
expression body (`s51`), to 237 with the composite variant port (`s52`), to 238 with the
non-Boolean guarded-succession guard (`s81`), to 241 with the three result-expression ownership cases (`s82`, `s83`,
`k43`: a specialization, a redefining usage and a typed expression each stating a body over an
inherited result expression), to 243 with the two reference-subsetting ones (`s84`, `k44`), and to 245 with `g69`/`k20` (a calculation or
function body listing a second bare expression: the pinned `CalculationBodyPart`/`FunctionBodyPart` admit one
`ResultExpressionMember`, so the pilot stops at the second expression, `missing '}' at 'x'`, while we read it as a
second result expression under the same one-result rule), and to 246 with `s85` (a viewpoint definition, a
requirement and so a constraint, inheriting result expressions from two generals), and to 253 with the seven KerML census cases (`k45`–`k49`, `s86`, `s87`:
an annotating element annotating itself, a three-ended binding, a conjugated feature or structure
without its type or default supertype, a chain through an alias to another type's feature, and
an `end` with a direction or a derived/abstract modifier), all landing both-reject, and to 256 with the end-feature rules (`k50`, an end that declares its
cross feature inline and also `crosses` another; `k51`, a `return` parameter owned by a classifier;
`k52`, a type with two conjugators), to 257 with `k53` (a Boolean expression, a feature like any
other, subsetting a data type), and to 259 with `g70`/`k21` (a name written ahead of a usage or
feature keyword, `foo attribute bar : A;`: no production puts a name before its kind keyword, so the
pilot stops at the keyword, `no viable alternative at input 'attribute'`, while we report the stray
name as `expected a body member` — a shape the parser once accepted silently as a member named
`foo`, dropping `bar`), and to 263 with the inherited case-role cases (`s89`/`s90`, a case or
requirement definition specializing two generals that each declare an objective or subject;
`s91`/`s92`, one that references a second objective or subject through a referenced usage — all
four both-reject, the pilot's `Only one objective/subject is allowed` counting inherited roles),
and to 265 with `k54`/`s88` (a cross feature declared ahead of its end's kind keyword and typed
by a subtype of the end's type: `validateFeatureCrossFeatureType` asks for the same type, so both
reject), and to 266 with `s93` (an assertion written outside `part h : H { assert q; }` as `assert h.q;`
where `H::q` is a part: the unnamed assertion derives no member name, so `h.q` reaches the part in
both tools and neither accepts it as a constraint), and to 285 with the keyword-first relationship
ends (`k55`–`k73`: a `subtype`, `subclassifier`, `typing`, `subset`, `redefinition`, `conjugate`,
`inverse`, `disjoint` or `featuring` member whose source or target names a package, or a class or
feature where the metaclass admits only a feature or a classifier — the pilot's typed
cross-references fail to link, `Couldn't resolve reference to Type|Classifier|Feature '…'`, and the
type tier now judges both ends of the member by the kinds the declaration clauses already require).
The KerML constraints in that
source reopened 14 gaps — all of them semantic rules the pilot enforces and we did not; the
named-argument validation that landed alongside closed one of them (`k33`), the constructor
argument checking of the send-action family closed another (`k34`), the cross-subsetting
rules closed four more (`k16`, `k17`, `k19`, `k42`), and the association/connector
arity and multiplicity-bound typing rules closed three more (`k25`, `k26`, `k37`), leaving 5 — and the
SysML census opened nine more, six of them `grammar/`, since closed by the parser's body-kind
rule for the members only one body kind offers (see
[Permissiveness gaps](#permissiveness-gaps)); `s46` (the feature-value overriding rule) landed
with both implementations rejecting, as did `s47` (an enumeration definition specializing
another) with `g68` (a definition nested in an enumeration body) and `s48`–`s50` (the
trigger-argument typing rules). The metadata rules
then closed four more (`k35`, `k36`, `k40`, `s23`: metaclass typing, annotated-element conformance
and body redefinition, in both notations), and the feature-variability rules two more (`k11`, an
initial `:=` value on a non-variable feature; `s80`, `constant` on a non-variable usage), leaving no KerML gap. Before
that source the default-mode gap count was 2
of 120: only the intended `extensions/` notation. Three `xpect/` gaps closed later: `p11` (the
model-level evaluability predicate on metadata body values), `p15` (the attribute-usage typing
rule) and `p24` (a library metaclass now carries its declaration and its abstractness on every load
path, which is what the rule reads). The two `grammar/` gaps left by the grammar-mutation sweep —
`g02` (bare `import` is an error by default) and `g31` (`allocate` requires
its `ConnectorPart`) — closed with those adjudications, and the approved keyword-recovery policy
closed the final three, so `grammar/` is clean. The KerML validation
rules closed eleven `xpect/` gaps (`p08`, `p17`, `p20`,
`p21`, `p22`, `p25`, `p26`, `p27`, `p28`, `p32`, `p33`). No case in the corpus is
accepted by both implementations.

The three strict-only agreements are `x05`, `x06` and `x08`: OpenSysML notation
extensions that the default mode accepts on purpose and strict mode reports as errors. `x01` (the
initial state marker), `x04` (`region r { … }`) and `x07` (`transition <src> to <tgt>`) left that
list when that notation was removed: each is now a parse error in either mode, so both
implementations reject it by default. Judged in
the default mode the same corpus gives 223 agreements and 3 gaps, which is what `-conformance
default` prints. `-conformance strict` gives 226 and 0. Reserved keywords recovered as declared
names and SysML declaration keywords recovered in KerML are now errors in either mode; the parser
still preserves their trees for editors and later analysis. Of the 14 gaps this document carried
when it was first written, six were closed by the validation work itself — `p01`, `p02`, `p03`,
`p04`, `p05` and `p06` — and only the three `extensions/` cases belong to strict mode.

Read those three as agreement *when asked strictly*, not as gaps that disappeared. An opt-in
check is weaker evidence than a default one: it says the strict question has an answer we agree on,
not that the pipeline a user gets by default rejects the notation — by design it does not. And
because we authored all 285 cases ourselves, a small gap count means we ran out of questions we
thought to ask, not that we stopped being permissive: the denominator measures our coverage of the
rejection surface, not our conformance.

Two of the `extensions/` cases that agree in either mode (`x02` choice, `x03` junction) are rejected
by us for a different reason than by the pilot: our own state-connectivity validation flags a pseudostate
with no outgoing transition, while the pilot rejects the notation itself. The bucket records
rejection, not agreement on the rule. The other three (`x01`, `x04`, `x07`) agree on the notation:
we no longer accept it either.

## Permissiveness gaps

All 0 gaps under `-conformance auto` are open — that is, none: every gap the `semantic/` source
and the SysML constraint census opened is closed: the 6 SysML body-item spellings the census opened
(`g61`–`g66`) are rejected by the parser, which admits `subject`, `actor`, `stakeholder`,
`objective`, `entry`/`do`/`exit` and `render` only in the body kinds whose grammar offers them. The
cross-subsetting family the `semantic/` source opened (`k16`, `k17`, `k19`, `k42`) was closed by
extending the cross-feature pass with the crossing-feature, crossed-feature, redefined-end
specialization and at-most-one rules; `k42` is rejected by OpenSysML with the message the pilot's
source intends (`At most one cross subsetting is allowed`) where the pinned pilot only crashes. The
last KerML gaps (`k25`, `k26`, `k37`) were closed by the association arity, binary-link end count
and multiplicity-bound typing rules.
Three earlier gaps were severity-policy gaps — the pinned grammar excluded the spellings, while
OpenSysML retained recoverable trees and reported only warnings by default — and the approved
policy closed them by reporting errors without removing that recovery.

When a gap is open, this section lists each case with its reproducer (the corpus file is the
minimal reproducer), both verdicts and the package the root cause is likely in, as a table of
`Case | We | Pilot says | Likely root cause` rows; the first pilot error is the one quoted, and
the full lists are in the baseline JSON's `pilot` arrays. The table is empty at this writing.

### Constraints the pilot declares but does not enforce

Each of these is a named constant in the pinned `KerMLValidator` or `SysMLValidator` whose check
is absent, disabled, or reported at warning severity, which the harness does not count as
rejection. A minimal violating model was refereed for each; the pilot accepted it (or only
warned) and so did we, so no case was kept — a both-accept case is a corpus bug, not a finding.
Evidence is from reading the pinned source and running candidate models through the validator.

**KerML.** None is drafted as an upstream issue: the unchecked constants are satisfied by
construction and the warnings are a deliberate severity choice of the pilot, not a defect in its
reading of the specification.

- Declared but never checked (the constant has no `error(...)` or `warning(...)` call):
  `validateFeatureEndIsConstant`, `validateSubsettingPortionConformance`, and
  `validateAssociationStructureIntersection` (commented in `checkAssociation` as
  "automatically satisfied" — every association is an intersection of its ends' types by
  construction).
- Checked at warning severity only: `validateFeatureEndFeatureMultiplicity` (end feature
  multiplicity other than 1), `validateRedefinitionMultiplicityConformance` and
  `validateSubsettingMultiplicityConformance` (a subsetting or redefining feature whose
  multiplicity exceeds the general's), `validateOperatorExpressionCastConformance` (`as` to a
  non-conforming type), `validateOperatorExpressionBracketOperator` (`[` applied to a
  non-Anything argument), `validateFlowEndImplicitSubsetting`,
  `validateLibraryPackageNotStandard` (`User library packages should not be marked as
  standard`), and `validateNamespaceDistinguishablity` (the pilot's spelling; `Duplicate of other owned member
  name`, which OpenSysML also reports as a warning).

**SysML.**

| Constraint | Evidence |
| --- | --- |
| `validateItemUsageType`, `validatePartUsageType`, `validatePartUsagePartDefinition` | `checkItemUsage` and `checkPartUsage` are commented out in `SysMLValidator.xtend`; the generic `checkUsage` requires only a `Classifier`. `item i : AD;` (an attribute definition) is rejected as `validateOccurrenceUsageType` (`An occurrence, item or part must be typed by occurrence definitions.`, which we also report), and `part p : ID;` (an item definition) is accepted by both implementations. |
| `validateOperatorExpressionQuantity` | Reported as a **warning** (`Should be a measurement reference (unit).`), and warnings do not count as rejection on either side. |
| `validateUseCaseUsageReference` | Only the name and message constants are declared; `checkUseCaseUsage` implements the typing rule (`validateUseCaseUsageType`, `s38`) and nothing reads `INVALID_USE_CASE_USAGE_REFERENCE`. The `include` form is covered by `validateIncludeUseCaseUsageReference` (`s20`). |
| `validateTransitionFeatureMembershipGuardExpression` | The error path (`Must be a Boolean expression.`) exists and the pilot's `TransitionUsage_invalid.sysml.xt` expects it for `if "test"`, but that fixture loads a reduced library without `ScalarValues`. With the full standard library the pinned validator accepts `first s1 if "test" then s2`, `if 1 + 2`, and every other non-Boolean guard we tried in a state body (`accept … if 1 then`, `transition if 2.5 then`) or an action body (`first a if "go" then b`, a decision's `if e then`, an enumeration literal, a `String`-valued calc) — while we reject them all (`transition guard must be Boolean, found String`; `s81` pins the action-body form as ours-only). The cause is in `ExpressionAdapter`: a guard implicitly redefines the library's `TransitionPerformance::guard` (`bool guard[*]`), so its result specializes `Boolean` by construction and the validator's `isBoolean` test is vacuous. Recorded in [omg-issues.md](omg-issues.md#a-non-boolean-transition-guard-is-accepted-with-the-full-library-loaded-pilot-2026-07). |

### Constraints without a constructible violating model

These constraints are enforced by the pinned pilot, but no legal model exists that violates only
them: either no violating model exists under the loaded standard library, or every textual
violation is already a syntax error in the pinned grammar (or the grammar's post-processing
normalises it away), so the constraint never fires on parsed text and no standalone case can
isolate it. The reason is recorded so a later round does not repeat the search.

**KerML.**

- Satisfied by construction in the pilot's own derivations: `validateClassifierDefaultSupertype`
  (the pilot adds the implicit default supertype whenever it is missing, so the check never
  fails on parsed text), `validateElementIsImpliedIncluded` (implied relationships are created
  only when `isImpliedIncluded` is set), `validateAnnotationAnnotatingElement` and
  `validateAnnotationAnnotatedElementOwnership` (the grammar produces annotations only in the
  owned-or-owning shapes the constraints require), `validateFeatureHasType` (with the library
  loaded every feature gets an implicit type; the library-less Xpect negative is noted under
  `xpect/` above), and the operator-name constraints for
  collect, select, index and feature-chain expressions (the parser fixes the operator name).
  `validateTypeAtMostOneConjugator` was listed here until `k45` (`classifier C ~A ~B;`): the
  pilot grammar rejects the second `~` as a syntax error, so the named constraint never fires
  there, while OpenSysML parses the form and reports it as a constraint error.
- Guarded by the grammar: `validateFlowItemFeature` (a flow declaration admits one payload),
  `validateEndFeatureMembershpIsEnd` (the pilot's spelling), `validateFeatureEndNoDirection`,
  `validateFeatureEndNotDerivedAbstractCompositeOrPortion` (an `end` prefix excludes the
  conflicting prefixes), `validateMultiplicityRangeBounds` (bound order and ownership),
  `validateParameterMembershipOwningType`, `validateParameterMembershipDirection`,
  `validateResultExpressionMembershipOwningType`
  (parameter and result memberships cannot be spelled outside a behavior, step, function or
  expression; `validateReturnParameterMembershipOwningType` was listed with them until `k44`,
  `return` in a classifier body, which the pilot rejects at the grammar and OpenSysML parses and
  reports as a constraint error), `validateConstructorExpressionOwnedFeatures`,
  `validateInvocationExpressionOwnedFeatures`, `validateInstantiationExpressionInstantiatedType`,
  `validateInstantiationExpressionResult`, `validateFeatureReferenceExpressionResult`,
  `validateFlowEndIsEnd`, `validateFlowEndNestedFeature` and `validateFlowEndOwningType` (the
  flow-end, argument and result shapes are produced by the parser, not written by the author).
- Domain-derived: `validateClassifierMultiplicityDomain` and `validateFeatureMultiplicityDomain`
  compare a multiplicity's featuring types with its owner's; a multiplicity written in the
  owner's body always has the owner's featuring types, and there is no notation to give it
  others.
- Violable only together with another constraint: `validateBindingConnectorIsBinary` — a binding
  typed by a three-ended association also trips `validateConnectorRelatedFeatures`, so the pilot
  reports two errors and the case would not violate exactly one rule (the related-elements and
  binary-ends shapes are `k25`/`k26`, which both implementations now reject).
- `validateFeatureChainingFeatureConformance` — every spelling of a chain whose second feature is
  not featured by the first's type fails name resolution in both implementations before the
  conformance check runs, so the violation cannot be isolated from an unresolved reference.

**SysML.**

| Constraint | Why no case |
| --- | --- |
| `validateAcceptActionUsageParameters`, `validateAssignmentActionUsageArguments`, `validateForLoopActionUsageLoopVariable`, `validateForLoopActionUsageParameters`, `validateIfActionUsageParameters`, `validateWhileLoopActionUsageParameters`, `validateTransitionUsageParameters` | The productions (`AcceptNode`, `AssignmentNode`, `ForLoopNode`, `IfNode`, `WhileLoopNode`, `TransitionUsage`) always synthesise the required parameters (`EmptyParameterMember`, the payload, the loop variable), so a parsed model cannot have too few; the checks guard models built through the API. `validateAssignmentActionUsageArguments` is moreover a declared constant that no `@Check` method in the pinned `SysMLValidator` ever issues. The shapes tried against the pinned validator are listed on each row of [validation-constraints.md](validation-constraints.md). |
| `validateAttributeUsageIsReferential`, `validateReferenceUsageIsReference`, `validateUsageIsReferential`, `validateEventOccurrenceUsageIsReference`, `validatePortUsageIsReference`, `validateDefinitionVariationIsAbstract`, `validateUsageVariationIsAbstract` | The pilot's adapters (`UsageAdapter.postProcess`, `PortUsageAdapter.postProcess`) set `isComposite = false` for attributes, directed, end and package-level usages, references, events and ports, and `isAbstract = true` for every variation, before validation runs; the textual notation has no way to declare the violating value. |
| `validateAttributeDefinitionFeatures`, `validateAttributeUsageFeatures` | Every nested usage of an attribute definition or usage is normalised to referential by the same adapter, so `checkAllNotComposite` never finds a composite one. |
| `validateConjugatedPortDefinitionConjugatedPortDefinition`, `validatePortDefinitionConjugatedPortDefinition` | The conjugated port definition is created implicitly for every port definition and never for a conjugated one; the notation cannot declare or omit it. |
| `validateFramedConcernMembershipConstraintKind`, `validateRequirementVerificationMembershipKind` | `frame` and `verify` are the only spellings of their memberships and each fixes `kind = requirement` in the grammar (`FramedConcernKind`, `RequirementVerificationKind`). |
| `validateObjectiveMembershipIsComposite`, `validateRequirementConstraintMembershipIsComposite` | `objective` and `require`/`assume` members take no `ref` prefix in the grammar (`ObjectiveMember`, `RequirementConstraintMember`), so the owned usage is always composite. |
| `validateExposeIsImportAll` | `ExposePrefix` has no `all` token (unlike `ImportPrefix`), and `ExposeImpl` constructs every expose with `isImportAll = true`, so a parsed expose can never fail the check. |
| `validateTransitionFeatureMembershipOwningType`, `validateTransitionFeatureMembershipEffectAction`, `validateTransitionFeatureMembershipTriggerAction` | `TransitionFeatureMembership` is only produced by `TriggerActionMember`, `GuardExpressionMember` and `EffectBehaviorMember` inside a transition, each of which fixes the member's metaclass (`AcceptActionUsage`, `Expression`, `ActionUsage`). |

### p24, deferred by Step 3 and closed by the record format

Step 3 measured **116 both reject, 4 only the pilot rejects** and deferred `p24` — KerML
`validateMetadataFeatureMetaclassNotAbstract`, a metadata usage typed by an abstract metaclass —
because abstractness of a metaclass was not a fact the reduced library record carried.

The record format supplies it: the library is parsed on every load path and `Abstract` is a
persisted fact family under the reflective equality coverage, so `symbols.IsAbstract` answers the
same cold and warm. Persisting a fact emits no diagnostic on its own — the rule
(`internal/core/passes/w8c_metadata_type.go`) reads that accessor instead of casting `Decl`, which
is what closed the row.

## Adjudications

Every recorded gap below was a **real permissiveness finding**: the pinned grammar admits none of
these models. The closure notes record the implementation or approved policy change that moved
each case to rejection.

- **`grammar/g02-import-without-visibility.sysml` — divergence in severity.**
  The pinned `ImportPrefix` (`SysML.xtext:241`, `KerML.xtext:169`) makes `visibility =
  VisibilityIndicator` **mandatory**, unlike the optional `MemberPrefix` visibility beside it
  (`SysML.xtext:218`), so `import Q::*;` is not a well-formed import and the reference reports
  `mismatched input 'import'`. We do report it — as a *warning*
  (`internal/core/passes/import_visibility.go`, code `import-visibility`), and warnings do not
  count as rejection here. Per the pinned grammar the severity should be an error in the default
  mode: either raise `SeverityWarning` to an error, or make it
  conformance-dependent as `nonstandard_notation.go` already does.
  **Closed** ([adjudications.md](adjudications.md)): the finding is an error in every mode, and the
  case now rejects.
- **`grammar/g15-keyword-as-name.sysml` — recoverable error by approved policy.** The
  pinned grammar's `Name` is the `ID` terminal, which excludes keywords, and `part def part;` is
  `no viable alternative at input 'part'`. We read a keyword in name position as the name the
  author meant and report that an unrestricted name (`'part'`) is required to spell it
  (`internal/core/parser/namespace.go`, code `reserved-keyword-name`, KerML §7.2.4) — a recovery
  policy chosen so an editor keeps a usable tree, documented in
  [conformance-audit.md](../reference/grammar/conformance-audit.md). Strict mode already escalated
  the parser warning without changing that reading. The user explicitly approved applying the
  same error severity in default mode, so the case now rejects while the tree and diagnostic code
  remain stable.
- **`grammar/g60-alias-keyword-as-name.sysml` — recoverable alias error by approved policy.**
  `AliasMember` takes its declared name through `Identification`, so `alias part for ...` cannot
  use the reserved `part` token as its `ID`; the pilot reports `extraneous input 'part' expecting
  'for'`. OpenSysML deliberately recovers `part` as the alias name so symbol and editor consumers
  retain the intended alias. The user approved making the existing `reserved-keyword-name`
  finding an error in default mode too, closing the gap without changing recovery or strict mode.
- **`grammar/k02-sysml-keyword-in-kerml.kerml` — recoverable language error by approved policy.**
  `part def` exists in no KerML production; the KerML validator reports `no viable
  alternative at input 'def'`. We parse `.kerml` with the same grammar as `.sysml` and filter
  afterwards: `internal/core/passes/nonstandard_notation.go` reports SysML-only notation in a
  KerML file while preserving the parsed declaration. Strict mode already reported
  `sysml-notation` as an error. The user explicitly approved the same error severity in default
  mode for this language-keyword recovery, so both modes now reject it with the same code, span,
  and message.
- **`grammar/g31-allocate-without-to.sysml` — the `allocate` synonym, adjudicated, not fixed.**
  In the pinned grammar `allocate` is only the `AllocateKeyword` (`SysML.xtext:1210`) and demands
  a `ConnectorPart` (`:1219`), whose binary form requires `to` (`:1076`); the usage keyword is
  `allocation` (`AllocationUsageKeyword`, `:1206`). OpenSysML additionally accepts `allocate` as a
  synonym for the usage keyword (see [rdf-mapping.md](../reference/rdf-mapping.md), where
  `sysx:declaredKeyword` keeps the two distinguishable), so `allocate a;` reads as an allocation
  usage *named* `a` rather than a connector missing its target — the two forms are
  indistinguishable at the token level. Measured with the pinned validator: `part def D { allocate
  al; }` is rejected by the reference too, so the synonym itself is the divergence, not just this
  case. Removing it is a language change locked by golden and RDF export expectations, so it is a
  language decision rather than a small local fix. **Adjudicated in
  [adjudications.md](adjudications.md):** require the `ConnectorPart` after `allocate`
  and drop the definition-side entry, which closes this case without dropping the legal
  `allocate f to g;` form. `g02`'s severity is decided in the same record.
  **Closed:** `allocate` demands its `ConnectorPart`, and the case now rejects.

### Grammar mutation pass

The `grammar/` derivation was extended along the *unreached* axis rather than the interesting-case
axis: [grammar-coverage.md](grammar-coverage.md) lists the forms no input of ours touches, and this round
mutated exactly those. Measured by running `cmd/grammar-coverage` over a tree with the negative
corpus added as a scanned root, the five forms the committed coverage report calls unseen —
`KerML.xtext:119` (`#`-prefixed `namespace`), `:408` `Conjugation`, `:426` `Disjoining`, `:712`
`Redefinition`, and `KerMLExpressions.xtext:267`'s `%` operator — are all reached by the new cases
(`k11`, `k07`/`k15`, `k06`, `k16`, `g40`). Every candidate was run through the pinned validators
before being committed and only those the reference actually rejects were kept; the discarded
candidates (objective, send, metadata, enum, snapshot and metaclass mutations the reference
accepts) would have been corpus noise, not reach. Two of the new cases were closed by fixes in
this same PR rather than left as gaps: `g20-include-without-target.sysml` (a bare `include ;`
inside a body was read as a member *named* `include`) and
`g36-direction-without-feature.sysml` (`in ;` declared nothing and was accepted).

### Second pass

The second pass extended the corpus along two axes at once, so the two instruments cross-check.

**Grammar axis (13 cases).** The five forms `grammar-coverage.md` calls unseen were already reached
in that round, so this pass mutated productions the coverage report cannot see as blind spots at all —
`ConjugatedPortTyping`, `RealValue`, `StringValue`, `RangeExpression`, positional argument lists,
`FeatureChainMember`, `QualifiedName`, unrestricted names, `SatisfyRequirementUsage`,
`OwnedCrossSubsetting`, `Unioning`, `ConnectorEndMember` and `MetadataTyping` (`g50`–`g59`,
`k17`–`k19`). The coverage instrument's own movement is unchanged by this pass and by design: with
the negative corpus added as a scanned root the report still shows **0 unseen forms of 807** (it
showed 5 before these cases existed), and the committed baseline still shows **5 unseen forms**
because the committed roots do not include the negative corpus. The 244 indistinguishable
productions are an instrument limitation — every path through them matches without a literal — so
no corpus case can move that number. All 13 grammar cases are agreements.

**Semantic axis (27 cases).** Cases were derived from the pilot's own `validation/invalid/*` and
`Variability_invalid` Xpect expectations, one declared rule each (`p08`–`p34`), covering the rules
the KerML validation work is closing: `Must be model-level evaluable` (`p11`, `p22`), `Must have a Boolean result`
(`p12`, `p21`), the variation rules (`p08`–`p10`), and the typing, cardinality, redefinition,
port/interface, verification and view families. 13 are agreements and 14 are new permissiveness
gaps, listed above. Candidates the pinned validator accepted were discarded rather than kept as
reach: `(1, 2,)` (a trailing comma in a sequence expression) and a String transition guard, both of
which we reject and the reference does not.

Two agreements are agreements on the bucket, not on the rule: `p19` (a parallel state with a
transition) is rejected by us with `expected '{' or ';'` — our parser does not accept the pinned
`state def S parallel` form at all — and `p34` (an accepter whose source is not a state) is rejected
by us as a transition endpoint that is not a vertex. The bucket records rejection, not agreement on
the rule.

### Should the default mode reject the `extensions/` cases?

Per the specification, **yes**. `region`, `defer` and `history` appear in no production of the
pinned grammars — `StateBodyItem` has no history or deferral member and concurrency is spelled
`state ... parallel`. The same held for `initial` and `transition <src> to <tgt>`, which is why
they were removed; both are now errors in either mode. The SysML v2 textual notation is defined by that grammar, so a model using them
is not a conforming SysML v2 model, and a tool asked whether it conforms must say no. Accepting
them by default is therefore not "conformance we argued" but a **superset we chose**: OpenSysML's
default mode implements a dialect, and the honest statement of `-conformance auto` agreement on
`x05`, `x06` is that the strict question has an answer we agree on while the
default pipeline a user gets accepts notation the reference rejects as a syntax error. What makes
the choice defensible is not the extensions' usefulness but that the conforming question remains
askable: [strict mode](../guide/03-command-line.md#strict-conformance) reports every one of them as
an error, each has dedicated parser and runtime tests, and each is documented as an extension. If
strict mode ever stopped covering one of them, the default-mode acceptance would be an
undocumented non-conformance and the case should be fixed instead of adjudicated.

### Forms kept rejected

A later round probed the parser-debt follow-ups against the pinned grammars and
accepted every form they derive. Three neighbouring forms are **not** derivable, so the rejection
stays. Each is guarded by a `TestNegative` case (`entry_succession_body`,
`definition_succession_body`, `namespace_succession_body`) — the first two guard the succession
parser's body policy, the third the namespace member dispatch that rejects a `then` before it;
10G owns adding them to this oracle's negative corpus.

| Form | Why it is not derivable |
| --- | --- |
| `entry; then starting { … }` in a state body | `EntryTransitionMember` (`SysML.xtext:1796-1801`) is `MemberPrefix ( GuardedTargetSuccession \| 'then' TransitionSuccession ) ';'` — it ends in `';'`, so an entry transition takes no body. A body on a `then` is derivable only as `ActionTargetSuccession` (`:1698`), which a state body reaches through `TargetTransitionUsageMember` (`:1764`) after a behaviour usage member, not after an entry action. |
| `exhibit s1 then starting { … }` (no terminator on the `exhibit`) | `ExhibitStateUsage` (`:1840-1846`) ends in `StateUsageBody`, i.e. `';'` or a braced body, so the member must be terminated before a target transition follows it: `exhibit s1; then starting;`. |
| `then <name> { … }` as a namespace or definition member | `ActionTargetSuccession` is reached only from `TargetSuccessionMember` (`:1393`) inside an action body item (`:1374-1381`); neither `NamespaceBodyItem` nor `DefinitionBodyItem` (`:516-524`) has a succession member, so a bodied `then` in a package or a `part`/`requirement` body stays a syntax error. |

## Guard

`TestPilotRejectionDocumentCountsMatchBaseline` (in `cmd/pilot-reject`) re-derives every count in
this document after applying the three approved closures to
[pilot-rejection-baseline.json](pilot-rejection-baseline.json). The README and skill remain
checked against that committed baseline until its separate refresh. The guard reads only committed
files — no validators or downloads — and checks that the gap table enumerates the current report.

`TestCommittedBaselineStatesThisRepositorysProvenance` guards the baseline's own `provenance`
block — the pinned tag and artifact, each validator bridge's source digest, and the negative
corpus's digest and case count — against what this repository currently pins, and the daily
`.github/workflows/oracle-reproduction.yml` re-runs this oracle with `-check` where Java is
available. [pilot-differential.md](pilot-differential.md#how-this-record-is-kept-true) describes
both and what each of them cannot catch.


## The declared errata overlay

The rejection census is reported twice, as published and with the [declared
errata](errata-overlay.md) applied. Every case here is one we wrote ourselves, so no declared
correction lies under `cmd/pilot-reject/testdata/negative` and the two figures coincide — stated as
such rather than left to look like a measurement:

```
no declared correction lies under cmd/pilot-reject/testdata/negative, so the errata-applied
corpus is byte-identical to the published one and both figures coincide
```

The mechanism is in place for the case that would matter: an entry correcting a case's model would
re-adjudicate it over the corrected copy and report any bucket change, including one that moves a
verdict of the pinned pilot.

## SysML constraint census

One row per SysML constraint name in the pinned `SysMLValidator` that this corpus owns (100 names;
the control-node, variation-specialization, trigger-argument, binding-conformance, send-action and
feature-value-overriding constraints are covered by their own implementation records). *Case* is
the corpus file that violates the constraint — `existing:` where an `xpect/` case predating the
census already did — or `none:` with the reason no case exists. *Bucket* is the committed baseline's
verdict under `-conformance auto`; the pilot and OpenSysML messages are the error-severity
diagnostics each side reports for the case (`/` separates several). The last column is only filled
for the gaps, and names where in OpenSysML the rule would have to fire.

| Constraint | Case | Bucket | Pilot message | Our diagnostic | Why we are silent |
| --- | --- | --- | --- | --- | --- |
| `validateAcceptActionUsageParameters` | none: `AcceptNode` always parses a payload parameter | no violating model | — | — | — |
| `validateActionUsageType` | `semantic/s01-action-typed-by-part-def.sysml` | both-reject | An action must be typed by action definitions. | An action must be typed by action definitions. | — |
| `validateActorMembershipOwningType` | `grammar/g61-actor-outside-requirement-body.sysml` | both-reject | mismatched input 'actor' expecting '}' / extraneous input '}' expecting EOF | 'actor' declares an actor of a requirement or case and is only allowed in a requirement or case body; move it into the requirement or case it belongs to | — |
| `validateAllocationUsageType` | `semantic/s02-allocation-typed-by-connection-def.sysml` | both-reject | An allocation must be typed by allocation definitions. | An allocation must be typed by allocation definitions. | — |
| `validateAnalysisCaseUsageType` | `semantic/s03-analysis-typed-by-case-def.sysml` | both-reject | An analysis case must be typed by one analysis case definition. | An analysis case must be typed by one analysis case definition. | — |
| `validateAssertConstraintUsageReference` | `semantic/s04-assert-references-non-constraint.sysml` | both-reject | Must reference a constraint. | assert target must be a constraint usage, found attributeUsage | — |
| `validateAssignmentActionUsageArguments` | none: `AssignmentNode` always parses both arguments, and the pinned pilot declares the constant without ever issuing it | no violating model | — | — | — |
| `validateAssignmentActionUsageReferent` | `semantic/s43-assign-to-non-feature.sysml` | both-reject | An assignment must have a referent. | An assignment must have a referent. PD is declared `part def`, not a feature. | — |
| `validateAssignmentActionUsageReferentIsTimeVarying` | `semantic/s05-assign-to-package-level-attribute.sysml` | both-reject | Referent must be time varying. | Referent must be time varying. | — |
| `validateAttributeDefinitionFeatures` | none: nested attribute features are normalised to referential | no violating model | — | — | — |
| `validateAttributeUsageEnumerationType` | `semantic/s06-enum-attribute-two-types.sysml` | both-reject | An enumeration attribute cannot have more than one type. | An enumeration attribute cannot have more than one type. | — |
| `validateAttributeUsageFeatures` | none: nested attribute features are normalised to referential | no violating model | — | — | — |
| `validateAttributeUsageIsReferential` | none: adapter post-processing forces `isComposite = false` | no violating model | — | — | — |
| `validateAttributeUsageType` | existing: `xpect/p15-attribute-typed-by-part-def.sysml` | both-reject | An attribute must be typed by attribute definitions. | An attribute must be typed by attribute definitions. | — |
| `validateCalculationUsageType` | `semantic/s07-calc-typed-by-action-def.sysml` | both-reject | A calculation must be typed by one calculation definition. | A calculation must be typed by one calculation definition. | — |
| `validateCaseDefinitionOnlyOneObjective` | `semantic/s08-case-def-two-objectives.sysml` | both-reject | Only one objective is allowed. | Only one objective is allowed. | — |
| `validateCaseDefinitionOnlyOneSubject` | `semantic/s09-case-def-two-subjects.sysml` | both-reject | Only one subject is allowed. | Only one subject is allowed. | — |
| `validateCaseDefinitionSubjectParameterPosition` | `semantic/s10-case-def-subject-not-first.sysml` | both-reject | Subject must be first parameter. | Subject must be first parameter. | — |
| `validateCaseUsageOnlyOneObjective` | `semantic/s11-case-two-objectives.sysml` | both-reject | Only one objective is allowed. | Only one objective is allowed. | — |
| `validateCaseUsageOnlyOneSubject` | `semantic/s12-case-two-subjects.sysml` | both-reject | Only one subject is allowed. | Only one subject is allowed. | — |
| `validateCaseUsageSubjectParameterPosition` | `semantic/s13-case-subject-not-first.sysml` | both-reject | Subject must be first parameter. | Subject must be first parameter. | — |
| `validateCaseUsageType` | `semantic/s14-case-typed-by-action-def.sysml` | both-reject | A case must be typed by one case definition. | A case must be typed by one case definition. | — |
| `validateConjugatedPortDefinitionConjugatedPortDefinition` | none: the grammar never creates a conjugated port definition inside another | no violating model | — | — | — |
| `validateConnectionUsageType` | `semantic/s15-connection-typed-by-part-def.sysml` | both-reject | A connection must be typed by connection definitions. | A connection must be typed by connection definitions. | — |
| `validateDefinitionVariationIsAbstract` | none: adapter post-processing forces `isAbstract = true` | no violating model | — | — | — |
| `validateDefinitionVariationMembership` | existing: `xpect/p09-variation-member-not-variant.sysml` | both-reject | An owned usage of a variation must be a variant. | An owned usage of a variation must be a variant. | — |
| `validateEnumerationUsageType` | existing: `xpect/p18-enum-two-types.sysml` | both-reject | An enumeration must be typed by one enumeration definition. | An enumeration must be typed by one enumeration definition. | — |
| `validateEventOccurrenceUsageIsReference` | none: adapter post-processing forces `isComposite = false` | no violating model | — | — | — |
| `validateEventOccurrenceUsageReferent` | `semantic/s16-event-references-non-occurrence.sysml` | both-reject | Must reference an occurrence. | Must reference an occurrence. | — |
| `validateExhibitStateUsageReference` | `semantic/s17-exhibit-references-non-state.sysml` | both-reject | Must reference a state. | Must reference a state. | — |
| `validateExposeIsImportAll` | none: `ExposeImpl` constructs every expose with `isImportAll = true` | no violating model | — | — | — |
| `validateExposeOwningNamespace` | `grammar/g67-expose-outside-view-body.sysml` | both-reject | mismatched input 'expose' expecting '}' / extraneous input '}' expecting EOF | expose is only allowed in a view usage body (SysML v2 8.3.26.2) | — |
| `validateFlowDefinitionConnectionEnds` | `semantic/s18-flow-def-three-ends.sysml` | both-reject | A flow connection definition can have at most two ends. | A flow connection definition can have at most two ends. | — |
| `validateFlowUsageType` | `semantic/s19-flow-typed-by-connection-def.sysml` | both-reject | A flow connection must be typed by flow connection definitions. | A flow connection must be typed by flow connection definitions. | — |
| `validateForLoopActionUsageLoopVariable` | none: `ForLoopNode` always parses the loop variable | no violating model | — | — | — |
| `validateForLoopActionUsageParameters` | none: `ForLoopNode` always parses both parameters | no violating model | — | — | — |
| `validateFramedConcernMembershipConstraintKind` | none: `frame` fixes `kind = requirement` in the grammar | no violating model | — | — | — |
| `validateIfActionUsageParameters` | none: `IfNode` always parses condition and body parameters | no violating model | — | — | — |
| `validateIncludeUseCaseUsageReference` | `semantic/s20-include-references-non-use-case.sysml` | both-reject | Must reference a use case. | Must reference a use case. | — |
| `validateInterfaceDefinitionEnd` | `semantic/s21-interface-def-end-not-port.sysml` | both-reject | An interface definition end must be a port. | An interface definition end must be a port. | — |
| `validateInterfaceUsageEnd` | existing: `xpect/p27-interface-end-not-port.sysml` | both-reject | An interface end must be a port. | An interface end must be a port. | — |
| `validateInterfaceUsageType` | `semantic/s22-interface-typed-by-connection-def.sysml` | both-reject | An interface must be typed by interface definitions. | An interface must be typed by interface definitions. | — |
| `validateItemUsageType` | none: `checkItemUsage` is commented out in the pinned validator | not enforced by the pilot | — | — | — |
| `validateMetadataUsageType` | `semantic/s23-metadata-typed-by-part-def.sysml` | both-reject | A metadata usage must be typed by one metadata definition. / Must have a concrete type | A metadata usage must be typed by one metadata definition. | — |
| `validateObjectiveMembershipIsComposite` | none: `objective` admits no `ref` prefix | no violating model | — | — | — |
| `validateObjectiveMembershipOwningType` | `grammar/g63-objective-outside-case-body.sysml` | both-reject | mismatched input 'objective' expecting '}' / extraneous input '}' expecting EOF | 'objective' declares the objective of a case and is only allowed in a case body; move it into the case it belongs to | — |
| `validateOccurrenceUsageIndividualDefinition` | existing: `xpect/p25-two-individual-definitions.sysml` | both-reject | At most one individual definition is allowed. | At most one individual definition is allowed. | — |
| `validateOccurrenceUsageIndividualUsage` | existing: `xpect/p33-individual-typed-by-plain-def.sysml` | both-reject | An individual must be typed by one individual definition. | An individual must be typed by one individual definition. | — |
| `validateOccurrenceUsageIsPortion` | `semantic/s45-snapshot-outside-occurrence.sysml` | both-reject | Must be owned by an occurrence definition or usage. | Must be owned by an occurrence definition or usage. | — |
| `validateOccurrenceUsageType` | existing: `xpect/p16-part-typed-by-attribute-def.sysml` | both-reject | An occurrence, item or part must be typed by occurrence definitions. | An occurrence, item or part must be typed by occurrence definitions. | — |
| `validateOperatorExpressionQuantity` | none: reported as a warning only | not enforced by the pilot | — | — | — |
| `validatePartUsagePartDefinition` | none: `checkPartUsage` is commented out in the pinned validator | not enforced by the pilot | — | — | — |
| `validatePartUsageType` | none: `checkPartUsage` is commented out in the pinned validator | not enforced by the pilot | — | — | — |
| `validatePerformActionUsageReference` | `semantic/s24-perform-references-non-action.sysml` | both-reject | Must reference an action. | Must reference an action. | — |
| `validatePortDefinitionConjugatedPortDefinition` | none: `PortDefinition` always synthesises exactly one conjugated definition | no violating model | — | — | — |
| `validatePortDefinitionOwnedUsagesNotComposite` | existing: `xpect/p26-port-def-nonreferential-usage.sysml` | both-reject | Owned usages of a port definition (other than ports) must be referential. | Owned usages of a port definition (other than ports) must be referential. | — |
| `validatePortUsageIsReference` | none: adapter post-processing forces `isComposite = false` | no violating model | — | — | — |
| `validatePortUsageNestedUsagesNotComposite` | `semantic/s25-port-nested-composite-part.sysml` | both-reject | Nested usages in a port usage (other than ports) must be referential. | Nested usages in a port usage (other than ports) must be referential. | — |
| `validatePortUsageType` | `semantic/s26-port-typed-by-part-def.sysml` | both-reject | A port must be typed by port definitions. | A port must be typed by port definitions. | — |
| `validateReferenceUsageIsReference` | none: adapter post-processing forces `isComposite = false` | no violating model | — | — | — |
| `validateRenderingUsageType` | `semantic/s27-rendering-typed-by-part-def.sysml` | both-reject | A rendering must be typed by one rendering definition. | A rendering must be typed by one rendering definition. | — |
| `validateRequirementConstraintMembershipIsComposite` | none: `require`/`assume` admit no `ref` prefix | no violating model | — | — | — |
| `validateRequirementConstraintMembershipOwningType` | `extensions/x08-require-outside-requirement-body.sysml` | both-reject | mismatched input 'require' expecting '}' / extraneous input '}' expecting EOF | `require` outside a requirement body is an OpenSysML extension with no SysML v2 production: only a requirement, concern, viewpoint or objective body admits it | — |
| `validateRequirementDefinitionOnlyOneSubject` | existing: `xpect/p14-requirement-two-subjects.sysml` | both-reject | Only one subject is allowed. | Only one subject is allowed. | — |
| `validateRequirementDefinitionSubjectParameterPosition` | `semantic/s28-requirement-def-subject-not-first.sysml` | both-reject | Subject must be first parameter. | Subject must be first parameter. | — |
| `validateRequirementUsageOnlyOneSubject` | `semantic/s29-requirement-two-subjects.sysml` | both-reject | Only one subject is allowed. | Only one subject is allowed. | — |
| `validateRequirementUsageSubjectParameterPosition` | `semantic/s30-requirement-subject-not-first.sysml` | both-reject | Subject must be first parameter. | Subject must be first parameter. | — |
| `validateRequirementUsageType` | `semantic/s31-requirement-typed-by-constraint-def.sysml` | both-reject | A requirement must be typed by one requirement definition. | A requirement must be typed by one requirement definition. | — |
| `validateRequirementVerificationMembershipKind` | none: `verify` fixes `kind = requirement` in the grammar | no violating model | — | — | — |
| `validateRequirementVerificationMembershipOwningType` | existing: `xpect/p29-verify-outside-objective.sysml` | both-reject | A requirement verification must be in the objective of a verification case. | A requirement verification must be in the objective of a verification case. | — |
| `validateSatisfyRequirementUsageReference` | `semantic/s32-satisfy-references-non-requirement.sysml` | both-reject | Must reference a requirement. | satisfy target must be a requirement usage, found constraintUsage | — |
| `validateStakeholderMembershipOwningType` | `grammar/g64-stakeholder-outside-requirement-body.sysml` | both-reject | mismatched input 'stakeholder' expecting '}' / extraneous input '}' expecting EOF | 'stakeholder' declares a stakeholder of a requirement and is only allowed in a requirement body; move it into the requirement it belongs to | — |
| `validateStateDefinitionParallelSubactions` | existing: `xpect/p19-parallel-state-with-transition.sysml` | both-reject | A parallel state cannot have successions or transitions. | A parallel state cannot have successions or transitions. | — |
| `validateStateDefinitionSubactionKind` | existing: `xpect/p13-state-two-entry-actions.sysml` | both-reject | A state may have at most one entry action. | A state may have at most one entry action. | — |
| `validateStateSubactionMembershioOwningType` | `grammar/g65-entry-action-outside-state-body.sysml` | both-reject | mismatched input 'entry' expecting '}' / extraneous input '}' expecting EOF | 'entry' declares the entry action of a state and is only allowed in a state body; move it into the state it belongs to | — |
| `validateStateUsageParallelSubactions` | `semantic/s33-parallel-state-usage-with-succession.sysml` | both-reject | A parallel state cannot have successions or transitions. | A parallel state cannot have successions or transitions. | — |
| `validateStateUsageSubactionKind` | `semantic/s34-state-usage-two-entry-actions.sysml` | both-reject | A state may have at most one entry action. | A state may have at most one entry action. | — |
| `validateStateUsageType` | `semantic/s35-state-typed-by-part-def.sysml` | both-reject | A state must be typed by state definitions. | A state must be typed by state definitions. | — |
| `validateSubjectMembershipOwningType` | `grammar/g62-subject-outside-requirement-body.sysml` | both-reject | mismatched input 'subject' expecting '}' / extraneous input '}' expecting EOF | 'subject' declares the subject of a requirement or case and is only allowed in a requirement or case body; move it into the requirement or case it belongs to | — |
| `validateTransitionFeatureMembershipEffectAction` | none: `TransitionEffectMember` only parses an `ActionUsage` | no violating model | — | — | — |
| `validateTransitionFeatureMembershipGuardExpression` | none: not observed with the full library loaded | not enforced by the pilot | — | — | — |
| `validateTransitionFeatureMembershipOwningType` | none: `TransitionFeatureMembership` is only produced inside a `TransitionUsage` | no violating model | — | — | — |
| `validateTransitionFeatureMembershipTriggerAction` | none: `TriggerActionMember` only parses an `AcceptActionUsage` | no violating model | — | — | — |
| `validateTransitionUsageParameters` | none: `TransitionUsage` always synthesises its `EmptyParameterMember`s | no violating model | — | — | — |
| `validateTransitionUsageSuccession` | `semantic/s44-transition-target-not-action.sysml` | both-reject | A transition must own a succession to its target. | transition endpoint p is not a state or pseudostate | — |
| `validateTransitionUsageTriggerActions` | existing: `xpect/p34-accepter-source-not-state.sysml` | both-reject | A transition with an accepter must have a state as its source. | A transition with an accepter must have a state as its source. | — |
| `validateUsageIsReferential` | none: adapter post-processing forces `isComposite = false` | no violating model | — | — | — |
| `validateUsageType` | `semantic/s36-usage-typed-by-feature.sysml` | both-reject | A usage must be typed by definitions. | A usage must be typed by definitions. | — |
| `validateUsageVariationIsAbstract` | none: adapter post-processing forces `isAbstract = true` | no violating model | — | — | — |
| `validateUsageVariationMembership` | `semantic/s37-variation-usage-member-not-variant.sysml` | both-reject | An owned usage of a variation must be a variant. | An owned usage of a variation must be a variant. | — |
| `validateUseCaseUsageReference` | none: constant declared, no check reads it | not enforced by the pilot | — | — | — |
| `validateUseCaseUsageType` | `semantic/s38-use-case-typed-by-case-def.sysml` | both-reject | A use case must be typed by one use case definition. | A use case must be typed by one use case definition. | — |
| `validateVariationMembershipOwningNamespace` | existing: `xpect/p08-variant-outside-variation.sysml` | both-reject | A variant must be an owned member of a variation. | A variant must be an owned member of a variation. | — |
| `validateVerificationCaseUsageType` | `semantic/s39-verification-typed-by-case-def.sysml` | both-reject | A verification case must be typed by one verification case definition. | A verification case must be typed by one verification case definition. | — |
| `validateViewDefinitionOnlyOnvViewRendering` | `semantic/s40-view-def-two-renderings.sysml` | both-reject | A view definition may have at most one view rendering. | A view definition may have at most one view rendering. | — |
| `validateViewRenderingMembershipOwningType` | `grammar/g66-render-outside-view-body.sysml` | both-reject | mismatched input 'render' expecting '}' / extraneous input '}' expecting EOF | 'render' declares the rendering of a view and is only allowed in a view body; move it into the view it belongs to | — |
| `validateViewUsageOnlyOneRendering` | existing: `xpect/p30-two-view-renderings.sysml` | both-reject | A view may have at most one view rendering. | A view may have at most one view rendering. | — |
| `validateViewUsageType` | `semantic/s41-view-typed-by-part-def.sysml` | both-reject | A view must be typed by one view definition. | A view must be typed by one view definition. | — |
| `validateViewpointUsageType` | `semantic/s42-viewpoint-typed-by-requirement-def.sysml` | both-reject | A requirement must be typed by one requirement definition. / A viewpoint must be typed by one viewpoint definition. | A viewpoint must be typed by one viewpoint definition. | — |
| `validateWhileLoopActionUsageParameters` | none: `WhileLoopNode` always parses condition and body parameters | no violating model | — | — | — |
