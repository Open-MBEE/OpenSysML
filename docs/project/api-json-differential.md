# API-JSON derived-property differential

**Labels.** This record cross-references the other conformance records under `docs/project/` by
their private labels — `derive*` names the metamodel's derivation constraints as
`docs/project/constraint-manifest.json` lists them (`deriveElementQualifiedName`, …), and OMG issue
keys (`KERML11-191`, `SYSML21-419`) name the Beta 2 corrections the referee can adjudicate. A
reader outside the records needs none of them.

**What it is.** An advisory referee, in the shape of the [pilot differential](pilot-differential.md)
and the [execution referee](pilot-execution-referee.md): every model file under the corpus roots is
exported as the SysML v2 API's element JSON twice — by this repository's writer
(`sysml -convert api-json`, `internal/translate/export`) and by the pinned OMG pilot
implementation — the elements are matched by a stable key, and every property of every matched
element is bucketed. Bucketed per (metaclass, property), the counts read as a scorecard of the
metamodel's derived properties: SysML v2's abstract syntax is mostly derivation (235 `derive*`
constraints in the manifest, nearly all of status *unknown* before this referee), and the pilot's
export is the only reference for what each derived property should hold. The referee never
decides a pass; it measures.

The pinned pilot is `2026-09` (`jupyter-sysml-kernel 0.63.0`), the artifact the reference
validators use (`scripts/pilot-pin.sh`).

## Provisioning and running

```sh
./scripts/download-pilot-json-exporter.sh      # builds build/pilot-json-exporter/export-json
./scripts/download-training-examples.sh && ./scripts/download-pilot-corpora.sh
go run -C tools ./cmd/pilot-apijson-diff          # writes build/pilot-apijson-diff/
go run -C tools ./cmd/pilot-apijson-diff -check   # fails unless the committed baseline reproduces
go run -C tools ./cmd/pilot-apijson-diff -update  # re-records docs/project/api-json-differential-baseline.json
```

`-roots training,probes` restricts a run to named roots (never with `-update`); `-exporter`, `-out`
and `-timeout` override the launcher, the output directory and the per-root time allowed to the
pilot. A run writes `pilot-apijson-diff.json` (the baseline's shape), `pilot-apijson-diff.txt`
(the ranked scorecard with one concrete example per differing row, the unmatched elements by
metaclass and the pilot's diagnostics), `pilot-apijson-diff-examples.json` and
`pilot-apijson-diff-matches.json`, the run's inventory of every key per root under the fate the
matching gave it (matched, ours-only, pilot-only). `-lost-against <earlier run>/pilot-apijson-diff-matches.json`
then names, in `pilot-apijson-diff-movements.txt` and `.json`, each element the earlier run matched
and this one does not (*lost*) and the reverse (*gained*), grouped by root and metaclass, each with
the key's fate in the run that does not match it; a change to either side's naming is read from
that list rather than from a moved total.

### The pilot's exporter

`scripts/download-pilot-json-exporter.sh` shares `scripts/pilot-pin.sh` with the validators,
compiles `scripts/pilot-json-exporter/ExportJSON.java` against the pinned shaded jar and writes the
launcher `build/pilot-json-exporter/export-json --library <sysml.library> --root <dir> --out <dir>
[--extension-library <dir>]... <files>` (the library defaults to the pinned one). The bridge is the
pilot's own export path, the one the Jupyter kernel's `%export` magic runs
(`SysMLInteractive.export`):

- the standard library is read first, as `SysMLInteractive.loadLibrary` reads it (`Kernel
  Libraries` as KerML, then `Systems Library` and `Domain Libraries` as SysML, all as library
  resources); then each `--extension-library` directory is read the same way — its `.kerml` files, then
  its `.sysml` files, as library resources — which is how the pilot's command-line utilities read
  the library paths that follow their input. The referee passes this repository's own libraries,
  `internal/workspace/libs/stdlib/OpenSysML Libraries/`, the directory the [pilot
  differential](pilot-differential.md) hands its validators: the one inside the standard-library
  root OpenSysML itself loads (the bundled tree unless `OPENSYSML_LIBRARY_PATH` names another),
  never a free command-line choice, so both sides resolve the same library text; the baseline
  records it as the `opensysml-libraries` input. A member whose naming feature, type or redefined
  feature lives in `DocumentQueries`, `AnalysisRecords` or `OpenSysMLRenderings` thus resolves on
  the pilot's side as it does on ours;
- the files of one corpus root are read into one resource set with those libraries
  (`org.omg.sysml.io.SysMLUtil`, the KerML and SysML Xtext setups both registered, so `.kerml` and
  `.sysml` inputs alike resolve), then `transformAll(false)` runs the pilot's model transformation
  as its validation does before an export, without adding the implied relationships as elements
  (the traversal otherwise adds them while it iterates);
- each input resource's root elements are visited by
  `org.omg.sysml.util.traversal.facade.impl.JsonElementProcessingFacade` with
  `setIsIncludeDerived(true)` through `org.omg.sysml.util.traversal.Traversal`, and the tree is
  `toJsonTree(true)` — the exact calls `SysMLInteractive.export` makes; standard-library elements
  are referenced, never written, as in the pilot, and the elements of this repository's libraries
  are written as library elements (`isLibraryElement` true, the `library package` keyword deciding
  it) and left out of the comparison like the standard library's;
- one document per input file is written under `--out`, mirroring the file's path:
  `{"file", "elements": [{"payload": {…}, "identity": {"@id"}}], "keys": {"<elementId>": "<key>"}}`,
  `keys` resolving every id the elements mention, library elements included, to the stable key
  described below; nulls are serialized as the facade emits them (`Gson.serializeNulls`).

The pilot mints a fresh random UUID for every user-model element on each run (library elements
keep their name-based ones), so the export is never byte-identical twice; the keys are what make
it comparable.

### Our side

Each root's files are parsed and converted together (`export.ModelToRDFWith` with qualified-name
ids, then `export.WriteAPIJSON`), so cross-file imports resolve as they do for the pilot. A file the
parser cannot read whole is refused and counted (`oursRefused`), as `sysml -convert` refuses it; a
file the pilot failed on is left out (`pilotFailed`), so that its elements do not count as
ours-only against an export that never existed. Two documents that declare the same element
(`tests/testdata` holds two versions of one library package; the interchange fixtures redeclare
their packages) cannot share one graph, since the qualified-name ids collide: each document is
first converted alone, the documents whose ids collide stay converted alone (`oursColliding`),
and the rest are converted together. Only if the rest still do not convert as one model is every
document kept alone (`oursSeparate`); a document the writer refuses alone is counted under
`oursUnconvertible`. Cross-file references out of a document converted alone stay unresolved and
are measured as such.

## Matching and normalization

Elements are matched by a **stable key**, computed identically on both sides:

- a named element by its `qualifiedName`, spelled with every segment unquoted
  (`'Derived Probe'::Vehicle` and `Derived Probe::Vehicle` are one key; the `qualifiedName`
  *property* is still compared as written, so the quoting difference is measured);
- an unnamed element by its owner: `<ownerKey>/<metaclass>[i]`, `i` counting the owner's preceding
  owned relationships (for a relationship — the owner is its `owningRelatedElement`) or owned
  elements (otherwise) of the same metaclass; the owned elements are the owned relationships'
  owned related elements, in order, which is how the metamodel derives `ownedElement`;
- a document's root elements by the document: `/<file>/<metaclass>#i`.

A positional address this repository's writer spells for an unnamed element (`Demo::@1`,
`Legacy::@4::@trigger`) is not a name — the metamodel derives no `qualifiedName` for an unnamed
element, and the writer states the address as `sysx:positionalName`, not as `qualifiedName` — so
such an element is keyed structurally, and its `qualifiedName` is measured as the pilot's null. Identical copies of one element on a side are one element (the pilot
writes an element in every file whose traversal reaches it; `duplicateElements`); elements two of
which compute one key but differ are dropped from matching (`ambiguousKeys`); elements no key
addresses (`unkeyableElements`: on the pilot's side the ownerless connectors it synthesizes and
what they own) are counted and left out.

Every normalization equates representations of one fact, never two facts; the list is written into
every report (`normalizations`):

1. the pilot's `{payload, identity}` envelope is unwrapped;
2. every `{"@id": x}` reference compares as the key of `x`, the pilot's key table covering library
   elements; a reference no key addresses is counted (`unkeyedReferences`) and makes the
   comparison *indeterminate* (two unknown targets are not known equal, nor different), a bucket
   kept apart from the four the scorecard ranks;
3. `elementId`, `memberElementId` and `ownedMemberElementId` hold ids as strings and are mapped
   through the same keys; `elementId` is identity, not derivation, and is not bucketed;
4. JSON null, a missing property and an empty collection all state nothing — *absent*;
5. a Boolean attribute written at its metamodel default (`model/kerml.ecore` and `model/SysML.ecore`
   in the pinned jar: `isUnique` defaults to `true`, every other defaulted Boolean to `false`)
   against a side that leaves it out is absent on both sides, since leaving it out states the
   default; a value the other side lacks is missing; the derived Booleans (`isConjugated`,
   `isLibraryElement`, `isModelLevelEvaluable`, `isReference`, `mayTimeVary`), `isVariation` and a
   `LiteralBoolean`'s `value` have no default, so leaving one out states nothing;
6. property order and object-key order are ignored; the order of collections is compared;
7. properties outside the metamodel (this repository's `sysx:` annotations) are left out and
   counted (`extensionProperties`);
8. the pilot's implied relationships (`isImplied`) have no counterpart in the notation; those the
   writer does not emit are counted under `pilotOnlyImplied`;
9. standard-library elements the pilot writes (`isLibraryElement`, one copy per file that refers
   to them) are left out and counted (`pilotLibraryElements`); this repository's writer refers to
   library elements and never writes them.

Each property of each matched pair lands in one bucket: **present-equal**, **present-different**,
**ours-missing** (the pilot states a value, we state nothing), **pilot-missing** (the reverse), or
*absent-both* (neither states anything — agreement, but not evidence that either side derives the
property, so it is kept apart from the four the scorecard ranks). A property declared
*pilot-nondeterministic* (below) lands in a seventh column whatever the two sides state. The row is
named by the pilot's metaclass when the two sides disagree on the metaclass (`typeMismatch`).

## Detectors

- **Determinism.** The report is sorted throughout (roots in order, elements by key, properties by
  name, rows by differing count then name); two runs over one checkout write byte-identical reports
  but for the recording date, which is what `-check` verifies in the oracle-reproduction workflow
  and what `TestReportIsDeterministic` pins without Java. The pilot's own output is not
  deterministic across JVMs for every property; a property shown to move is declared
  pilot-nondeterministic (below) rather than re-recorded, so `-check` holds on any JVM.
- **Disagreement.** `TestDisagreementDetectorIsLive` compares the committed pilot export of
  `tools/referee/apijson/testdata/probe.sysml` with itself (no row differs), then with a scratch copy
  in which one derived collection of `Derived Probe::Vehicle` is emptied (`ownedUsage` → exactly one
  ours-missing in that row, nowhere else) and one in which one attribute is flipped (`isAbstract` →
  exactly one present-different), each with one example naming the element.
- **Provenance.** The baseline's `provenance` block names both sides of the comparison: the pilot
  pin (`pilotTag`, `pilotCommit`, `pilotArtifact`), the errata-registry digest, one `tools` entry
  per side, every root's file digest and, as the `opensysml-libraries` input, the digest of the
  library directory the bridge reads beside the standard library. `pilot-json-exporter` digests the bridge source
  `scripts/pilot-json-exporter/ExportJSON.java` against the pinned release; `api-json-writer`
  digests this repository's writer, `internal/translate/export`: its non-test `.go` files, sorted by
  file name, each hashed by name and content, so the digest is the same on any machine and in any
  checkout path. It moves on any edit to a production source of the package (a renamed member, a
  dropped property, a reordered emission) and stays put on edits to its tests, fixtures or prose. It
  does not cover what the writer consumes — the parser, name resolution, the metamodel or the library
  — so a semantic change upstream still surfaces as count movement with no named cause; it names
  writer changes only. `TestCommittedBaselineStatesThisRepositorysProvenance` checks, without Java,
  that every one of those fields is this checkout's; `-check` holds the same block to the baseline
  before the counts, so a writer edit stops it at `provenance.tools[api-json-writer].sourceDigest:
  baseline records …, repository now has …` rather than at bare count drift (the fresh report it
  wrote still holds the moved buckets). The writer digested is the one linked into the referee —
  the checkout it was built from — which `-repo` pointing elsewhere does not change; a `-trimpath`
  build keeps no record of that checkout and is refused rather than guessed at. `TestProvenanceGuardFailsOnAMovedPin`
  and `TestProvenanceGuardFailsOnAnEditedWriter` pin that a moved pin and an edited writer are each
  named with the refresh command. `TestRecordStatesTheBaselinesFigures` (`make docs-counts`) ties
  this record's figures to the baseline.

The pinned pilot's verdict on the probe (`validate-sysml-batch`): clean.

## Declared pilot nondeterminism

The oracle is one process per root over sorted files, and two runs of it on one machine write the
same elements and values. Across JVMs they do not, for one property: `FeatureChainExpression.featuringType`.
At pin `2026-08` (`0.62.0`) the export under OpenJDK 21 (the workflow's `setup-java`) and under
OpenJDK 25 of the same checkout differ on exactly nine values, the three `FeatureChainExpression`s
valuing the event occurrences of `ServerSequenceModelOutside::PubSubSequenceOutside` in
`examples/pilot-corpora/sysml-examples/Interaction Sequencing Examples/ServerSequenceModelOutside.sysml`
(`then event occurrence :>> publish_target_event = publish_message.done;` and its two siblings), each
written again by `ServerSequenceOutsideRealization-2.sysml` and `-3.sysml`, which import it; the
copies collapse to three matched elements. Under one JVM each has no `featuringType`; under the other
each has one reference, to the enclosing `SuccessionAsUsage`'s `defaultFeaturingType`, an element the
pilot neither exports nor keys (the one `unkeyed` reference in the row). Nothing else moves: no other
element, property or key differs between the two exports, and a run from a differently named checkout
matches the first. The pilot derives an expression's `featuringType` from its adapter's implicit
featuring types, which are filled while the owner is transformed (`FeatureAdapter.addImplicitFeaturingTypesIfNecessary`,
`FeatureUtil.insertImplicitTypeFeaturings` in `org.omg.sysml.logic`), so the value depends on
transformation order, not on the model. At pin `2026-09` (`0.63.0`) the disagreement reproduces: two exports of the
same corpus moved three comparisons of this row between ours-missing and absent-both (284/230 against
287/227, three unkeyed references) and nothing else. The metamodel (KerML 1.1 Beta 2 8.3.3.3.4, `featuringType :
Type [0..*] {ordered}`, `deriveFeatureFeaturingType`) derives it from the `typeFeaturing`s and the
first chaining feature, which this expression has neither of, so the stable answer — none — is the one
this repository's writer states; the other is not a different fact but a different run.

A single comparison cannot tell the two answers apart (each is a legitimate shape), so the referee
carries a **declared list** (`pilotNondeterminism` in `tools/referee/apijson/nondeterminism.go`, written
into the report and the baseline as `pilotNondeterministic` with the evidence). Every comparison of a
declared `(metaclass, property)` row is scored **pilot-nondeterministic**, the seventh column, and in
no other bucket: the row is reproducible on every JVM and the instability stays visible in the
scorecard and the headline rather than being dropped or tolerated. An undeclared property is scored
exactly as before, the same metaclass's other properties included. The declaration is kept honest by
`TestDeclaredNondeterminismScoresTheWholeRow` (the column takes every comparison of the declared row,
nothing else, and never an example) and `TestDeclaredNondeterminismIsEvidencedAndRecorded` (the
baseline's list is the comparison's, every declared row has evidence and sits wholly in the seventh
column, and no undeclared row is in it). An entry is removed when a pin stops reproducing the
disagreement, which `-check` then shows as that row moving out of the column.

## Results (pilot `2026-09`, 935 files)

**Scorecard:** 935 files; 102568 elements matched (16299 ours-only, 28746 pilot-only, 0 of them implied); properties: 1982570 present-equal, 149977 present-different, 303717 ours-missing, 79915 pilot-missing, 2187951 absent-both, 33556 indeterminate, 523 pilot-nondeterministic.

Re-recorded on the effective-name change (a feature its clause leaves nameless is named, on every reader, by the feature it implicitly redefines, as KerML 1.1 §8.3.3.3 derives `Feature::effectiveName`: a positional parameter by its behavior's — `in item;` under `action shoot : Shoot` is `shoot::image` — a `return` parameter by `result`, a nameless objective by `obj`, a connector or allocation end by the end it occupies, `source`/`target`, `logical`/`physical`; `declaredName` stays the written one): 2734 more elements match (99834 → 102568; ours-only 19033 → 16299, pilot-only 31480 → 28746, type-mismatched 1682 → 1821, the newly matched ends and objectives this repository types otherwise), present-equal 1926398 → 1982570 and present-different 148945 → 149977 — the newly matched elements bring their subtrees under comparison, differences included. The seven `EndFeatureMembership` member rows go from 129 equal / 395 present-different to 524 / 0, `FeatureMembership.memberElement` 9537 / 573 → 9552 / 560, `Feature.name` 1115 → 1254 equal and `ReferenceUsage.name` 2203 → 2430, and the `FeatureChaining` rows 627 → 1130 equal, the chains under the newly named ends and parameters. Of the 1917 rows that moved, no row has fewer equal; the 151 that gained present-different rows are the subtrees the newly matched ends and objectives bring under comparison for causes the tables already state — an end's `definition`/`type`/`multiplicity` and its `ReferenceSubsetting`'s ends (`ReferenceUsage.definition` 4 → 39, `ReferenceSubsetting.referencedFeature` 15 → 35), an interface end's `PortUsage` rows, and an objective's `verify` requirements (`RequirementUsage.isComposite` 27 → 86, `RequirementVerificationMembership.referencedConstraint` 2 → 43). An end states its derived name alone and no `qualifiedName`: it is positional in this mapping, its name the end it redefines rather than an address of its own, so `Feature.qualifiedName` moves only by the parameters it names (1114 → 1253 equal). A membership names its member by the model's effective name whatever the member's identity, as before — `require q.k;` is `k` to its `RequirementConstraintMembership` and positional in its id.

Re-recorded on develop's individual-definition change (#1943: an `individual def`/`individual usage` is written as `OccurrenceDefinition`/`OccurrenceUsage` with `isIndividual`): 88 more elements match (99534 → 99622), and the 32 `OccurrenceDefinition` rows of the inherited family move from ours-missing to present-different for the causes the paragraph below already lists; no row grew.

Re-recorded again on develop's further defaults (#1942: an `individual def` owns its empty `Multiplicity` first and a named connector end its `[1..1]` last): 160 more elements match (99622 → 99782, the new default `Multiplicity` elements and their memberships), the seven `Multiplicity` rows of the gap table grow 3047 → 3127 for the cause they already state (no multiplicity implied base in the side table), and the `OwningMembership` pilot-missing counts fall by four; no new row.

Re-recorded on the operand visibility change (an operator's operand, a `->` form's receiver and its body are owned by private `ParameterMembership`s, an argument list's arguments and a trigger's `after`/`at`/`when` argument by public ones — the pilot's `OperandEList` against KerMLExpressions.xtext `ArgumentMember`): `ParameterMembership.visibility` moves from 1296 equal / 3289 present-different to 4557 equal / 28 present-different, present-different properties 128353 → 125092; no other row moved and no element's match changed. The 28 left are the operand-less forms `(as T)`, `@T` and `istype T` of a filter: the pilot gives them an implicit self operand (`operand += SelfReferenceExpression`), owned first by a private `ParameterMembership`, and writes a cast's type as a `ReturnParameterMembership` (`TypeResultMember`); this repository writes the type argument alone, as a public `ParameterMembership` — a writer follow-up.

Re-recorded when the self-model's build pipeline moved the library snapshot to build time (`pipeline.sysml` loses the `checkTarget` attribute, `quality.sysml`'s invariant gains a `not`): 99784 → 99783 elements matched. The removed attribute, its string literal and its implied multiplicity take one row off every `AttributeUsage`, `Multiplicity`, `LiteralString` and `OwningMembership` family; the `not` adds one `OperatorExpression` with its `ParameterMembership` and `ReturnParameterMembership`, so those families, the `FeatureMembership` and `FeatureTyping` rows of the new parameters and the `Feature` rows that count them gain one or two. No row appears or disappears and no verdict changes.

The pilot's export failed on 59 files (`pilotFailed`: its own export throws on an element without a resource, 52 of them specification examples, 4 test fixtures and, since pilot `2026-09`, the three `Metadata Examples/{Requirement,Risk,Verification}MetadataExample.sysml` whose enumeration-valued metadata bindings the pilot now evaluates into elements with no resource; the files are named in the run's text report), ours refused 36 (all of them specification examples the parser could not read whole), left the pilot's 59 failed files out and converted 30 documents alone for declaring an element another document of their root declares (`oursColliding`); no root fell back to per-document conversion. 11735 (metaclass, property) rows carry a comparison; 112895 references compared through the unkeyed sentinel, 162741 of our `sysx:` annotations were left out, and 303/368 elements (ours/pilot) were dropped for ambiguous keys, 17040/32319 for having none.

| root | files | matched | ours-only | pilot-only | present-equal | present-different | ours-missing | pilot-missing |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `examples/sysml-v2-training` | 100 | 8082 | 1082 | 2084 | 158767 | 13491 | 20022 | 5692 |
| `examples/pilot-corpora/sysml-examples` | 99 | 28788 | 5717 | 7328 | 567000 | 45280 | 82656 | 21437 |
| `examples/pilot-corpora/sysml-validation` | 56 | 10084 | 1824 | 2515 | 196743 | 17282 | 26096 | 7565 |
| `examples/pilot-corpora/kerml-examples` | 58 | 6731 | 903 | 1510 | 120684 | 9506 | 18839 | 8265 |
| `examples/spec-examples` | 511 | 7531 | 872 | 2199 | 131961 | 7571 | 22094 | 6367 |
| `tests/testdata` | 40 | 1517 | 192 | 454 | 27659 | 2128 | 4163 | 1190 |
| `examples` | 56 | 31863 | 5131 | 11266 | 623639 | 45176 | 99362 | 23991 |
| `tools/referee/diff/testdata` | 4 | 53 | 0 | 0 | 887 | 31 | 90 | 41 |
| `docs/manual/examples` | 11 | 7919 | 578 | 1390 | 155230 | 9512 | 30395 | 5367 |

**Every connector end's `[1..1]`** (re-recorded when the writer began stating the implicit `[1..1]`
of a positional end — `connect a to b`, `bind a = b`, `first s1 then s2`, `allocate a to b` — as it
already did a named end's, SysML v2 §7.13.2 "If a multiplicity is not explicitly declared for an end
feature, then a default of 1..1 is implicitly declared for it"; the pinned pilot states one on every
such end (`source`/`target`, `thisThing`/`sameThing`, `earlierOccurrence`/`laterOccurrence`, an
interface definition's ends) and none on a flow usage's `FlowEnd`s, which the writer leaves alone):
90 rows moved. 490 Multiplicity elements are ours-only, not matched: the positional end owning each
is an end the referee cannot pair (the pilot names it `source`/`target`, ours is positional —
`ConnectionUsage.ownedEndFeature` 8 equal, 81 different, unchanged), so its Multiplicity has no pilot
counterpart to pair with either. The one positional end it does pair gains its Multiplicity:
`ReferenceUsage.multiplicity` ours-missing 1 → 0, every `Multiplicity` row 3127 → 3128 and
`OwningMembership` row 12824 → 12825 (the same columns as before), `ReferenceUsage.ownedElement`,
`ownedMember` and `ownedMembership` ours-missing 10/9/9 → 9/8/8; matched elements 99782 → 99784,
ours-only 18470 → 18960, pilot-only 31543 → 31541; nothing else moved (the same 90 rows against develop's
#1915 writer as against the one before it; the gap table's seven `Multiplicity` rows grow 3127 → 3128 for the
cause they already state).

**Every multiplicity specializes `Base::naturals`, an `individual` definition's its own `Base::zeroOrOne`**
(re-recorded with the implied specializations of KerML 8.3.3.1.9 `checkMultiplicitySpecialization` and
SysML 8.3.9.3 `checkOccurrenceDefinitionMultiplicitySpecialization`, on the writer of #1950, #1990, #1970 and #1932 — develop's
#1915 write-time completion with every connector end's `[1..1]`, keyed by its id, the nested usages'
subsettings and the disjoint port types): 127 rows moved, matched elements 99773 → 99834,
present-equal +1286, present-different +23211, ours-missing −23603, pilot-missing +61, ours-only +73,
pilot-only −61. (1) With a base to read, the completed lists now reach every `Multiplicity`: its `feature`,
`featureMembership`, `inheritedFeature`, `inheritedMembership`, `member` and `membership` rows (61–67 of
§ Launch-ready gaps) move 3126 ours-missing → 3125 present-different (1 stays ours-missing), and `MultiplicityRange`'s four 1215 → 1215
present-different — different, not equal, for the reasons the inherited family already carries (causes 1–2
above: the pilot lists `Base::naturals`' members by ids the key does not reach and orders implied bases first);
`Multiplicity.type` (3128) and `MultiplicityRange.type` (1215) stay ours-missing as `PartUsage.type` (580) does:
the writer states `type` for a written typing only, not from an implied base. (2) The Turtle and API-JSON
stubs now name `Base::naturals`, its range and memberships, which the completed lists reference: the library
owners' `ownedFeature`/`ownedFeatureMembership`/`ownedMember`/`ownedMembership`/`ownedRelationship` rows move
present-different → present-equal (`Class` 152, `Feature` 90, `Type` 70, `Structure` 67, `Behavior` 33,
`Association`/`Step` 4, `Predicate` 1), and the stubs' `FeatureMembership`, `OwningMembership` and
`Specialization` properties leave absent-both for present-equal (540, 99 and 217) with the pilot missing a few
(30, 9, 22 — members the pilot's stubs do not state; `OwningMembership.relatedElement`/`target`/
`memberElementId` 432 → 437 pilot-missing, `Feature.owningFeatureMembership` 221 → 191); `Specialization.
general` gains 3 ours-missing and `BooleanExpression.type` 1 (library targets the key does not reach). An
earlier record of this branch read a KerML `multiplicity m subsets n;` member's `subsets` as its declared
general (22 more `Specialization` and 30 more `FeatureMembership` elements matched, all in `kerml-examples`,
`spec-examples` and `tests/testdata`); that movement is part of the figures above. The empty `Multiplicity`
an `individual` definition is parsed with is the one #1942's entry already matches (its `multiplicity` rows
stay present-equal). Not copied from the pilot, the clauses in hand: its `Base::exactlyOne` for a
feature-owned and `Base::zeroOrOne` for every classifier-owned `Multiplicity` (`ImplicitGeneralizationMap`),
which no clause requires — only an `individual` definition's takes `zeroOrOne`.

**Defaulted parameters of an invocation** (re-recorded with the extension libraries at
`DocumentQueries::WhereType(…, exact = false)` and `Differ`): the pilot's `InvocationExpression.parameter`
and `.input` list every parameter of the called calculation, a defaulted one the call does not bind
included, while this repository's writer lists the bound ones — so the 34 `WhereType` calls of
`examples` and `docs/manual/examples` moved from present-equal to present-different on both
properties (`InvocationExpression.input` 368 → 359 equal, `.parameter` 382 → 374) when the library
gained the parameter. The further pilot-library elements (198 for `exact`, 810 for `Differ`) are the
new declarations themselves. No other row moved. Listing the defaulted parameters is a writer follow-up.

**A body's result membership is a FeatureMembership** (re-recorded when the writer began stating a
`ResultExpressionMembership` as the `FeatureMembership` subclass it is, KerML 8.3.4.1): the body
Expression of `{ in x; … }` now lists it under `ownedFeatureMembership` and its result under
`ownedFeature`, and the membership states `ownedMemberFeature`, as the pilot does — so
`Expression.ownedFeatureMembership` moved 11 → 39 present-equal (26 present-different, 1 ours-missing
gone), `Expression.ownedFeature` 6 → 25 equal (30 → 13 different) and
`ResultExpressionMembership.ownedMemberFeature` 382 → 428 equal (29 → 2 ours-missing). The
`Expression.ownedFeature` rows still different are bodies whose `in` parameter the pilot types
`Feature` where ours writes `ReferenceUsage` (`Simple Tests/Expansion.kerml`), so the two
parameters key apart. No other row moved.

**A receiver-reached call instantiates a chain Feature** (re-recorded when the writer began spelling
`q.p.f(1)` as KerML 8.2.5.8.3 does — an `OwningMembership` owning a chain `Feature` with one
`FeatureChaining` per link, `sysml:function` the Function typing the final link, the arguments alone as
parameters — where it had written the receiver `q.p` as an unowned `operand`): the receiver-reached
calls of the pilot and specification corpora now key their chain Features against the pilot's (`Feature`
+6 matched, `FeatureChaining` +8, `OwningMembership` +4; 93417 → 93445 matched overall, 1156659 → 1157055
present-equal, 28156 → 28075 present-different), so `InvocationExpression.ownedFeature`/`.result` moved
533 → 537 equal (4 → 0 different), `.ownedMembership`/`.ownedRelationship` 508 → 512, `.parameter`
374 → 378, `.input` 359 → 362, `.argument` 495 → 498, `.operand` lost its 4 pilot-missing rows,
`Feature.chainingFeature` 195 → 199 equal and `Feature.ownedFeatureChaining` 271 → 275 (4 ours-missing
gone each), with the chains' `ReturnParameterMembership`/`ParameterMembership`/`LiteralInteger` rows
following. `InvocationExpression.function` moved 117 → 119 equal, 17 → 13 ours-missing, 397 → 399 indeterminate:
the writer now states the `Function` typing the chain's final link (`Expression::function`, KerML
8.3.4.7.3) — the link's effective type set as the reflective read derives it: declared, inherited
through a subsetting or redefinition, or the library Function its kind implies — so every
receiver-reached call states one; two agree with the pilot, the other two state the Function the
final link inherits or its kind implies, a library element the comparison cannot key (indeterminate).
The chains' `FeatureReferenceExpression.function` and `LiteralInteger.function` rows gain an
ours-missing each from the newly matched expressions, as the rest of that family does. No other row moved.

The writer now states `isConstant = true` for an end that varies in time — a declared `end` usage of a
connection, flow, interface or allocation definition, and the implied end usage of a connector inside an
Occurrence — as SysML v2.1 §8.2.2.6.1 derives it (`isEnd` ∧ `mayTimeVary` ⇒ `isConstant`, satisfying
KerML `checkFeatureEndIsConstant`) and as the reflective read already did, where the export stated only
the `constant` keyword: every `isConstant` row that was different is now equal — `ReferenceUsage` 2236 → 2327
(91 → 0), `PortUsage` 397 → 493 (96 → 0), `PartUsage` 1968 → 1989 (21 → 0), `ItemUsage` 345 → 368 (21 → 0),
`Usage` 21 → 30 (9 → 0) — 1157055 → 1157295 present-equal, 28075 → 27835 present-different overall. No other
row moved.

The scorecard is mostly **ours-missing**: this repository's writer emits a subset of the metamodel's properties, and the referee now measures which, per metaclass. The full ranked scorecard with one concrete example per differing row is `build/pilot-apijson-diff/pilot-apijson-diff.txt` after a run; the baseline keeps every row's seven counts.

**A membership states its ends and its member's names** (re-recorded when the writer began stating
`Membership::source`, `target`, `relatedElement`, `memberName` and `memberShortName` as KerML 8.3.2.3.5 and
8.3.2.5.2 derive them — the owning namespace, the member, both in that order, the member's effective
names — on every membership shape it mints, and the reflective read began deriving `memberName` as the
member's *effective* name, KerML 8.2.4, so an unnamed parameter is named by the one it redefines):
75 rows moved, all of them `source`, `target`, `relatedElement`, `memberName` or `memberShortName` of a
membership metaclass; 1157295 → 1291382 present-equal, 1067050 → 923659 ours-missing, 27835 → 32953
present-different, 26342 → 30528 indeterminate, 69936 → 70249 pilot-missing. `source` is now equal
wherever the pilot states it (42604 rows, 0 different). `target` and `relatedElement` inherit each
metaclass's `memberElement` row verbatim — `FeatureMembership` 9476 equal / 570 different / 898
indeterminate, `OwningMembership` 9046 / 431 / 226, `ReturnParameterMembership` 4512 / 343, … — because the
end *is* the member, so where the member keys apart (an unnamed member we key positionally, a
library element the comparison cannot key) the end does too; no new disagreement is introduced.
`memberName` moved 15290 rows to equal; the 158 different are memberships keyed by position whose
n-th member differs between the two sides — the rows `memberElement` already counts different, e.g.
`Comment Example/OwningMembership[0]` names `Comment1` on our side and the pilot's `Comment[0]` — not two
names for one member: re-exporting the training, pilot and specification roots with the pilot and
joining the memberships by their member's qualified name finds no membership named differently. The
309 pilot-missing are the pilot's: it leaves `memberName` null on the `OwningMembership` of a
`Comment` (`Comment Example::Comment1`), on a `VariantMembership` (`Vehicle::engine` → `smallEngine`),
a `FramedConcernMembership` and a `ViewRenderingMembership`, where ours states the member's name as the
clause derives it. The 136 `Membership.source` / 137 `.target` still
ours-missing are the Memberships an expression owns to a name it could not resolve to an element
(`sysml:memberElement "StraightLineDynamicsEquations"`, a literal): with no element to be the end,
none is stated. The `memberName` rows still ours-missing — `ParameterMembership` 4545,
`ReturnParameterMembership` 4724, `EndFeatureMembership` 523, `TransitionFeatureMembership`,
`StateSubactionMembership`, `SubjectMembership` 67 — are the pilot naming an implied parameter or end by the
library feature it implicitly redefines (`result`, `source`, `logical`, `accepter`, `vehicle`): our
effective name follows a declared redefinition or reference, not an implied one, on both ledgers; a
separate family. No other row moved.

**An implied membership states its member's name and an import its ends** (re-recorded when the
writer began stating `memberName` on the memberships it mints without a written member — a connector
end's `EndFeatureMembership`, named `source`/`target` for a binary connector or by the end it redefines,
a conjugated port definition's `OwningMembership` (`~P`), a satisfy subject's `SubjectMembership`
(`subj`) — and `Import::source` and `target`, KerML 8.3.2.3.3, as the import's owning namespace and the
namespace or membership it imports; and when the reflective read stopped naming the plain Membership
an expression owns to its referent, which only an alias specifies, KerML 8.3.2.3.5 — the pilot leaves
it null on all 3089 such rows): 11 scorecard rows moved; 1291410 → 1293814 present-equal, 923659 →
920639 ours-missing, 32925 → 32931 present-different, 70249 → 70251 pilot-missing, 30528 → 31138
indeterminate. `source` is equal on every import (670 `NamespaceImport`, 389 `MembershipImport`, 29 +
75 exposes); `NamespaceImport.target` is equal on 318, indeterminate on 344 — a library namespace the
comparison cannot key — and ours-missing on 8 whose name resolves to nothing; `MembershipImport.target`
110 equal / 3 different / 10 ours-missing / 266 indeterminate and `MembershipExpose.target` 71 / 3 / 1
inherit the imported `OwningMembership[n]`'s positional key, as a membership's `target` inherits its
member's. `EndFeatureMembership.memberName` moved all 523 rows to equal — a binding's ends read `thisThing`/`sameThing`, a
succession's `earlierOccurrence`/`laterOccurrence` on both sides, the ends of `Links::selfLinks` and
`Occurrences::happensBeforeLinks` they redefine, and a declared connector's ends the end of its type they
redefine, under its effective name (below). `SubjectMembership.memberName` moved 65 rows to equal; the
3 different predate this change. `OwningMembership.memberName` moved 125 rows to equal (the conjugated
definitions); the 2 new pilot-missing are conjugated definitions of an unnamed port definition, where
the pilot states no name either way.

**A binding and a succession subset their KerML bases** (re-recorded when the model began giving a
SysML binding connector `Links::selfLinks` and a succession `Occurrences::happensBeforeLinks` as their
implied base, KerML 1.1 §8.3.4.5.2 and §8.4.4.6.2 for a binding, §8.3.4.5.4 and §8.4.4.6.3 for a succession, where both had fallen back to `Base::things`): 4
scorecard rows moved, all to equal; 1291382 → 1291410 present-equal, 32953 → 32925 present-different.
`BindingConnectorAsUsage.isVariable`/`mayTimeVary` (3 rows each) and `SuccessionAsUsage.isVariable`/
`mayTimeVary` (12 each) now read `false` on both sides: SysML v2 §8.3.6.4 derives `mayTimeVary` false
for a usage conforming to `Links::SelfLink` or `Occurrences::HappensLink`, which the model could not
see while the base was missing. The 18 `EndFeatureMembership.memberName` rows those usages' implied
ends still differ on (`source`/`target` here, `thisThing`/`sameThing` and `earlierOccurrence`/
`laterOccurrence` in the pilot) are the end names the ends now redefine; they moved when the writer
began stating a minted end's `memberName` (the implied membership names, above).

**A succession's ends are named** (re-recorded when the writer began stating `memberName` on the ends of a
succession the notation mints — `first`, `then` and transition successions — as `earlierOccurrence` and
`laterOccurrence`, the ends of `Occurrences::happensBeforeLinks` they redefine, and on the ends of a
declared succession by the end of its declared type they redefine, under that end's effective name —
`succession s1 : AB first a then b` with `connection def AB { end a; end b; }` names them `a`/`b`
(`ConnectionTest::P::s1`), `succession redefines d_before_s : MyDry_Before_Ship_Link` whose association
declares `end feature redefines earlierOccurrence` names them `earlierOccurrence`/`laterOccurrence`
(`ChangingFeatureValuesExecution::MyManufacture::d_before_s`); and when a general that declares end
features — `Links::selfLinks`, `Occurrences::happensBeforeLinks`, library features rather than connectors —
began naming the ends that redefine them, so a KerML `binding`'s ends are `thisThing`/`sameThing`
(`Connectors::A::ab`, where they were `source`/`target`); the decoder reads a named end as the
positional end it is, not as an alias or an addressable element): `EndFeatureMembership.memberName`
alone moved, to 523 equal, no row different or ours-missing; it is counted with the implied membership
names above.

**The OpenSysML Libraries pin moves to `Descendants(relationships = true)`** (re-recorded when the vendored
libraries took OpenSysML-Extensions-Library's merge of its #19, which adds the optional `relationships`
parameter to `DocumentQueries::Descendants`; the baselines record the libraries' digest): the pin moves 25
`InvocationExpression.input` and `InvocationExpression.parameter` rows from equal to different
(1293814 → 1293764 present-equal), every one in the `examples` and `manual-examples` roots — the invocations of
`DocumentQueries::Descendants` that pass no `relationships` argument; the training, KerML-example and
specification-example roots read as before. The pilot lists among an invocation's `input` and `parameter` the
parameters of the invoked function that no argument binds (`DocumentQueries::Descendants::relationships` here;
`Time::DurationOf::clock` in the training corpus's `Time Constraints::healthStates`, different before and after
this record), where this repository lists only the argument features the invocation owns. A fix of its own
follows; until it lands, a library parameter gaining a default moves the rows of every caller that omits it.

**An invocation lists the parameters no argument binds** (re-recorded when the writer began listing
under `InvocationExpression.parameter` the invoked behavior's parameters that no argument binds and no
owned feature redefines — `Time::DurationOf::clock` for `DurationOf(o)`, a defaulted parameter the call
leaves to its default, a behavior's `out` for `A().y` — by the parameter's own element, after the owned
arguments and the result, the `in` ones under `.input` too, as KerML 8.3.4.8.8 derives an invocation's
parameters from its function's and the pilot lists and orders them; this closes the writer follow-up the
`WhereType(…, exact = false)` record above left open): 2 scorecard rows moved, present-different →
indeterminate — a library parameter has no key on either side, the bucket every reference to a
standard-library element already lands in
([below](#references-into-this-repositorys-libraries-link-by-id)): `InvocationExpression.input`
[337 equal, 183 different, 1 ours-missing, 16 absent-both] → [338, 0, 0, 16, 183 indeterminate] and `.parameter` [353, 184] → [354, 0, 183 indeterminate]. Totals: 1293764 → 1293766 present-equal,
32981 → 32614 present-different, 920639 → 920638 ours-missing, 31138 → 31504 indeterminate; no element count
moved. The row to equal on each is `A().y` (`Simple Tests/Behaviors.kerml`), whose unbound `A::x` and `A::y` are features of the file and so keyed; the 183 are calls of a library function — the 25 `Descendants(…)` calls of `examples` and `docs/manual/examples` the library pin above moved, the `WhereType(…, exact = false)` calls, every `TimeOf`/`DurationOf`. No other row moved.

**The self-model drops its `port-type-mismatch` pass** (re-recorded when `examples/self-model/pipeline.sysml`
lost the `part portTypeMismatch : ConstraintCheck { attribute :>> goType = "PortTypeMismatchPass"; }`
declaration together with the lint it described): the `examples` root lost that declaration's 10 matched
elements — the `PartUsage` with its `FeatureTyping`, implied `Multiplicity` and the `OwningMembership` of that
multiplicity, the `AttributeUsage` with its `Redefinition`, `FeatureValue` and `LiteralString`, and the two
`FeatureMembership`s; 99783 → 99773 matched, 1925327 → 1925112 present-equal, 125740 → 125734
present-different, 321317 → 321288 ours-missing, 77522 → 77514 pilot-missing, 32774 → 32771 indeterminate;
the ours-only and pilot-only counts are unchanged. Each scorecard row of those nine kinds fell by one per
element in the column it already occupied; no row changed column and no other root moved.

### The element form completes the derived properties

The writer now completes, from the metamodel evaluator in its graph-structure mode over the exported
graph and the library, the derived properties the graph does not state (`docs/reference/rdf-mapping.md`
§ Derived properties; the table is `apiJSONCompletedProperties` in `export/api_json_derived.go`), each as
its `derive*` constraint defines it, and names them per element in `sysx:completedProperties`. Measured in
four steps against the baseline above it (1293766 present-equal, 32614 present-different, 920638
ours-missing):

1. `Element::isLibraryElement` (KerML 1.1 Beta 2 §8.3.2.1.2 `deriveElementIsLibraryElement`): 93013 rows
   ours-missing → present-equal, none to present-different; 1386779 / 32614 / 827625. The 432 still
   missing sit on elements whose metaclass the two sides state differently.
2. `Membership::memberElementId` (§8.3.2.4.3 `deriveMembershipMemberElementId`),
   `OwningMembership::ownedMemberElementId`, `ownedMemberName`, `ownedMemberShortName` (§8.3.2.4.8; the
   ids and names redefine `memberElementId`, `memberName`, `memberShortName`, so a stated member name is
   the owned member name): 1476607 / 37735 / 729459. Each id row inherits its metaclass's `memberElement`
   row verbatim — the 5121 new present-different are the positionally keyed members the `memberElement`
   row already counts different (the `target`/`relatedElement` adjudication above); the 88
   `Membership.memberElementId` still missing name a member that resolves to no element.
3. A `Membership`'s `visibility` the graph leaves at its default is completed as `public` (§8.3.2.4.3), an
   `Import`'s as `private` (§8.3.2.4.2): 39407 rows present-equal, 3331 present-different. 3289 of the
   different are `ParameterMembership`s of expression arguments, which the pilot states `private`: the
   notation's `ArgumentMember` (§8.2.5.8.4) carries no visibility indicator and the metamodel's default is
   `public`, so the spec's value stands. 24 `MembershipImport`s and 7 `NamespaceImport`s without an indicator
   the pilot states `public`; the metamodel's `Import::visibility` default is `private`, which stands.
   11 `FeatureMembership`s of a `for` loop's variable the pilot states `protected` (its grammar's own
   choice; the metamodel default stands).
4. The ownership, naming, typing and owned-collection properties (§8.3.2.1.2 `name`, `shortName`,
   `owningMembership`, `owningNamespace`, `ownedElement`, `documentation`, `textualRepresentation`;
   §8.3.2.4.5 `ownedMembership`, `ownedMember`, `ownedImport`, `importedMembership`; §8.3.2.3.2/§8.3.2.3.5
   the annotation ends; §8.3.3.1.10 `isConjugated`, `multiplicity`, the owned specializations, feature
   memberships, features, end features and conjugator; §8.3.3.2.2 `ownedSubclassification`; §8.3.3.3.4
   `owningType`, `owningFeatureMembership`, `endOwningType`, `featureTarget`, `featuringType`, `type` and
   the owned typings, subsettings, redefinitions, type featurings, chainings; the `owningType` of a
   FeatureMembership, Specialization, Conjugation, Disjoining, Differencing, Intersecting and Unioning;
   §8.3.4.6.3 `Step::behavior`; §8.3.4.7.3 `Expression::function`, `result`; §8.3.4.9.2 the flow ends,
   payload feature and type; §8.3.4.11.2 `MultiplicityRange::bound`; SysML 2.1 Beta 2 §8.4 a usage's
   `owningDefinition`, `owningUsage`, `definition`, nested usages and variants, a definition's owned usages
   and variants, each usage kind's `*Definition`, the requirement, case, state, accept/send properties):
   1757290 present-equal, 49344 present-different, 421304 ours-missing, 73331 pilot-missing, 50584
   indeterminate. `owningType` 40891, `owningNamespace` 19350, `name` 13378, `owningUsage` 5089,
   `owningDefinition` 4324, `bound` 1215, `endOwningType` 482 and the `*Definition` properties moved to
   present-equal without a present-different row; `isConjugated` 34849 / 7, `featuringType` 12641 / 20
   (8652 indeterminate: a value naming a library type the pilot keys differently), `featureTarget`
   32111 / 131 (the 131 are a `FeatureChainExpression`'s chain, where the pilot targets the referenced
   usage and we the chain's last link), `owningFeatureMembership` 19617 / 441 (ours `ParameterMembership`
   where the pilot's is a `ReturnParameterMembership` — the owning membership's own metaclass row). The
   collection rows inherit the `ownedRelationship`/`ownedMembership` differences recorded above —
   `ownedElement` 17874 / 3665, `ownedMember` 12438 / 1951, `nestedUsage` 2113 / 519, `ownedUsage`
   1122 / 235 and the narrower nested/owned kinds in proportion: the pilot lists an implied member
   (a `Multiplicity`, a result parameter) the graph does not state. `definition` and the typed
   `*Definition` rows are indeterminate where the type is a library element (2640 / 1584 / 447 / 411).

5. The inherited-member family — `Type::inheritedMembership`, `feature`, `featureMembership`, `inheritedFeature`,
   `directedFeature`, `input`, `output` (§8.3.3.1.10), a type's `Namespace::membership`/`member` (§8.3.2.4.5),
   `Usage::usage`/`directedUsage` and `Definition::usage`/`directedUsage` (SysML 2.1 Beta 2 §8.3.6.4/§8.3.6.2) —
   is completed from the semantic model's inheritance table rather than from the graph alone: a general type the
   graph does not describe (an explicit or implied library base, `Parts::Part` for a part definition,
   `Parts::parts` for a part usage) contributes its `nonPrivateMemberships` (public, then protected, then its own
   inherited ones; a private membership is not inherited, §8.3.3.1.10 `inheritableMemberships`), anonymous
   members (documentation, invariants, unnamed features) included, with `removeRedefinedFeatures` applied
   across graph and library features by one key. Measured against step 4: 1806828 present-equal (+49538),
   130800 present-different (+81456), 309622 ours-missing (−111682), 74321 pilot-missing (+990), 31238
   indeterminate (−19346). Per property (present-equal / present-different / ours-missing, summed over
   metaclasses): `member` 1602 / 19721 / 15964, `membership` 1650 / 19719 / 15964, `feature` 561 / 10916 /
   21328, `featureMembership` 2978 / 10837 / 21328, `inheritedMembership` 155 / 8793 / 26337,
   `inheritedFeature` 560 / 5870 / 26338, `directedFeature` 5157 / 1521 / 5683, `input` 3007 / 629 / 1256,
   `output` 5486 / 785 / 5306, `usage` 4988 / 2019 / 3384, `directedUsage` 1001 / 705 / 580; no row outside
   the family moved by more than 44 (`function`), and none grew in ours-missing. An unnamed library member has
   no normative id, so the writer mints one (`identity.AnonymousMemberID`: uuid5 under the owner's id of the
   member's metaclass and ordinal among the owner's anonymous members of that metaclass); the referee joins it
   to the bridge's positional key through the standard library's declaration order (`anonymousLibraryKeys`),
   which is what turned 19346 indeterminate rows into equal ones. The Turtle form changes additively for
   this: so that every id the element form leaves unheld is one the graph names, the encoder states the
   library members a type inherits — named ones by metaclass, id and qualified name under their owning
   membership, unnamed ones by `sysx:libraryPath`, the bridge's positional path `Owner/Metaclass[i]`
   (`identity.AnonymousMemberOf`, one derivation for the writer, the reader, the decoder and the referee)
   — and the compact form's `libraryReferences` table carries that path as `libraryPath`; no existing
   statement changes. The reader of the element form restates the stubs from the ids the completed lists
   reference (and, in the compact form, from every `libraryReferences` entry, typed as the entry states).
   Fallout: an inherited member's stub states the metaclass the semantic model classifies it by (a KerML
   `feature` as `Feature`), while the pre-existing stub of a member the model text references directly
   keeps its declaration's (`AttributeUsage` for the same feature); unifying them changes existing Turtle
   statements and is left to a follow-up. Re-recorded once more on this writer (the `ReferenceUsage`
   correction: a reference usage has no kind base beyond `Base::things`, so its lists no longer carry
   `Base::DataValue`'s members): present-equal +775, present-different −1642, indeterminate +867,
   ours-missing unchanged — `ReferenceUsage.featureMembership` 507 rows different → equal;
   `ReferenceUsage.feature`/`inheritedFeature` 506 rows different → 83 equal and 424 indeterminate, lists that
   now agree at every keyed position and reference a library member neither key table addresses; `Feature`
   27 rows different → equal in each of the three; `Usage`, `OccurrenceUsage` and `PartUsage` by one to
   fourteen rows. The present-different rows were classified
   on the training root by the exact difference of the two lists, and fall into four causes:
   - Elements the structure-mode evaluator gives no implied base: expressions, their anonymous result
     features (the `Feature` rows of the gap table below) and multiplicity ranges. The semantic model
     derives their bases (`Performances::evaluations`, `Performances::Evaluation::result`, `Base::naturals`,
     KerML §8.3.4.7, §8.3.4.11) from the AST node, which a graph element does not have, and
     `ImpliedBaseFQNsForMetaclass` answers usage and definition metaclasses only. These rows are also the
     bulk of what remains ours-missing.
   - Implied relationships the graph does not state and the metaclass alone does not determine: a nested
     usage's implied subsettings (SysML §8.3.6.4 — a part usage owned by a part definition subsets
     `Items::Item::subparts`, `subitems` and `Occurrences::Occurrence::suboccurrences`, whose members the
     pilot inherits and we do not), and the positional redefinition of an unnamed parameter by its
     definition's parameter (SysML §8.3.6.4 `Usage` parameters; the pilot's `in item;` of `action shoot :
     Shoot` redefines `Shoot::image` and takes its name, so `Shoot::image` leaves `usage`, `input` and
     `feature`; ours keeps it). The semantic model computes both for AST nodes (`ImplicitSubsettings`,
     `ImpliedBaseFQNs`); structure mode would need rules keyed on metaclass, owner and position.
   - Two side-table gaps, recorded rather than approximated: a library feature's owned `MultiplicityRange` was
     not a member symbol of the feature (`Parts::parts/MultiplicityRange[0]` and its `OwningMembership` were
     absent from every usage's `member`/`membership`/`inheritedMembership`, one per implied usage base) — closed
     since the semantic model states a usage's written range as an owned `MultiplicityRange` member (see the
     re-record below) — and a plain `comment` in a library type's body is not a scope member
     (`Occurrences::Occurrence/Comment[0]` is absent from every occurrence-derived type's lists). On a part
     usage these two accounted for the whole difference (99 of the pilot's 105 inherited memberships).
   - Order: 14 `featureMembership` lists hold the same memberships in a different order. §8.3.3.1.10
     `supertypes()` orders `ownedSpecialization.general` as declared and gives an implied Specialization no
     place among the declared ones; the pilot lists the implied bases' memberships first, we list the
     declared generals' first. Both are conformant; ours stands. The bridge cannot key a library type's
     anonymous connectors (`?` on the pilot's side), which leaves those items indeterminate, not different;
     and one inherited membership (`Occurrences::Occurrence::suboccurrences::localClock`, a redefinition
     nested in a library feature's body) resolves to the redefined feature's id and so appears twice.

   Re-recorded on the writer merged with `develop`, where the semantic model now states a usage's multiplicity
   as an owned member: its written range as an unnamed `MultiplicityRange` and its implicit `[1..1]` as an
   unnamed `Multiplicity` (SysML §7.6.3), both implied symbols with no declaration of their own. Two
   movements, adjudicated separately. (1) The implied-default `Multiplicity` the writer now emits per model
   usage (3047 matched elements) gets from the pilot the inherited family and `type` from `Base::naturals`
   (KerML §8.3.3.3.4), while the semantic model computes no implied base for a multiplicity — the seven
   `Multiplicity` rows of the gap table, recorded rather than approximated. (2) A library usage's range is now a
   member of the usage, so `Parts::parts/MultiplicityRange[0]` enters the inherited lists of every part usage,
   closing the first side-table gap above; but an implied member is not among the scope's declared anonymous
   members, which is where the identity derivation looked, so its id could not be minted and the writer withheld
   the whole family from every usage with an implied usage base — a regression the first merged record carried
   as 39768 ours-missing (`PartUsage` 1689 per property, `ReferenceUsage` 1018, `PortUsage` 473,
   `ActionUsage` 471, …; `internal/translate/export` and the service's `convert_library_references` case
   fail on it). `identity.AnonymousMemberOf` and the shared index now derive every unnamed member — declared
   or implied — from the owner's `ownedMember` order (ordinal and positional path alike; declared members keep
   their ids), which restores the lists: 128450 present-different (+39330), 322654 ours-missing (−39768),
   32682 indeterminate (+438), 12 `Feature.feature`/`featureMembership` rows absent-both → pilot-missing;
   present-equal unchanged, no row grew in ours-missing, element counts unchanged. The lists that return are
   different, not equal, for the reasons already itemised (causes 1–2 and the `comment` gap); the
   `MultiplicityRange` stub and its `OwningMembership` join the Turtle stubs and the `libraryReferences`
   table (`Parts::parts/MultiplicityRange[0]`), additively.

   The present-different movement against `develop`'s record (+85166, every row in the family) by
   property, with the verdict of the training-root classification above: `member` +19584 and
   `membership` +19581 — pilot-right where the pilot's list carries a nested usage's implied-subsetting
   members, a positional parameter redefinition or a library `comment` member (causes 2 and 3, recorded
   gaps), ours-right where only the order differs (implied bases first on the pilot's side, declared generals
   first on ours; §8.3.3.1.10 fixes neither); `feature` +12278, `featureMembership` +12221,
   `inheritedMembership` +8655 and `inheritedFeature` +7247 — the same two verdicts plus the pilot-right
   expression, result-feature and multiplicity rows whose implied bases structure mode does not reach (cause
   1, a side-table gap); `usage` +2019, `directedUsage` +705, `directedFeature` +1521, `input` +448 and
   `output` +785 — projections of the same lists, so they inherit the verdict of their `feature` row;
   identity noise is confined to the one `localClock` duplicate. Nothing in the movement is a list ours
   states and the spec forbids; each pilot-right row is an itemised gap, not an approximation.

   Re-recorded with a nested usage's implied subsetting derived in structure mode (SysML 2.1 Beta 2 §8.3.6.4,
   the `check*Specialization` constraints: `checkPartUsageSubpartSpecialization` — "A composite PartUsage
   whose owningType is a ItemDefinition or ItemUsage must directly or indirectly specialize the PartUsage
   Items::Item::subparts" — and its siblings for items, occurrences, actions, states, constraints,
   requirements, cases, ports and views). The semantic model's declaration-keyed rule chains (`nested.go`)
   are now also answered for a graph element from what the graph states — its metaclass, its owning type's
   metaclass, `isComposite` and `portionKind` (`ImpliedSubsettingFQNForMetaclass`; the perform, exhibit and
   include forms are metaclasses of their own there, and the owning membership's metaclass and `kind` tell a
   requirement's `assume`/`require` constraint, a state's `entry`/`exit` action and a variant apart), with the KerML step fallback consulted when the owning
   type is a library symbol — and the feature named joins the element's general types, so its
   `nonPrivateMemberships` are inherited like an implied kind base's. Movement against the record above:
   present-equal +353, present-different +643, ours-missing −1001, indeterminate +5, elements unchanged, no
   row grew in ours-missing. By metaclass: `PerformActionUsage` `feature`/`featureMembership` 86,
   `inheritedMembership` 110, `inheritedFeature` 112, `member`/`membership` 83 rows ours-missing →
   present-different and `usage` 39 → equal, 44 → different; `AssertConstraintUsage` 52 inherited rows and 39
   `directedFeature`/`output` rows → different, `usage` 39 → equal; `ExhibitStateUsage` 23 inherited and 5
   `member`/`feature` rows → different, `usage` 14 → equal; `SatisfyRequirementUsage` 16 inherited and 16
   `output` rows → different; `IncludeUseCaseUsage` one row per property → different; `StateUsage.usage` 216,
   `ActionUsage.usage` 35, `CalculationUsage.usage` 6 and `UseCaseUsage.usage` 2 different → equal;
   `FlowUsage.usage` 2 different → indeterminate. Verdicts: the ours-missing → present rows are the nested
   forms whose metaclass has no kind base in `ImpliedBaseFQNsForMetaclass` (a `perform action` is an
   `ActionUsage` by `Actions::actions`, SysML §8.3.16.3), so until now nothing was inherited and the family
   withheld; the nesting feature (`Parts::Part::performedActions`, `exhibitedStates`,
   `Requirements::RequirementCheck::assumptions`/`constraints`, …) now carries most of the pilot's list, and
   the rows are different where the pilot's also carries the kind base's own members — the kind base of
   these metaclasses is the recorded gap (cause 1's metaclass-keyed bases), not an approximation. The `usage`
   rows → equal are lists the nesting feature completes exactly. The `PartUsage`, `ItemUsage` and
   `OccurrenceUsage` rows stay present-different with a smaller difference: a part usage's lists now carry
   `Items::Item::subparts/MultiplicityRange[0]`, its documentation and `suboccurrences`' members (the
   pilot's `eng` of the training `Part Definition Example` lists 113 inherited memberships, ours 105, from
   100), and the pilot's also carry the `MultiplicityRange` of each intermediate feature of the subsetting
   chain (`Items::Item::subitems/OwningMembership[1]`, `Objects::Object::subobjects/OwningMembership[1]`),
   which the semantic model inherits through a subsetting only for the directly subsetted feature — a
   side-table gap recorded here. The Turtle and compact forms change additively (the new stubs and
   `libraryReferences` rows; two conversion goldens cross 200 KiB into digest form); the unkeyed
   references rise by 774, the pilot keying a nesting feature's `OwningMembership`s positionally while the
   compact form spells a membership stub without a `libraryPath`.


Not completed, and why: `Type::endFeature` (not taken up in this step; it awaits a measurement of its own) and the
causes itemised under step 5. `Behavior::step` (0 equal / 257 different),
`Step::parameter` and `Behavior::parameter` (21 / 530), `InstantiationExpression::instantiatedType`
(143 / 464), `Usage::nestedTransition`, `Definition::ownedTransition`, `objectiveRequirement`,
`Definition::ownedRequirement` were measured and withdrawn: the pilot's lists differ from the evaluator's in
content or order and the difference is not yet adjudicated. `Relationship::relatedElement`, `source` and
`target` of a non-membership relationship were withdrawn because an import's library-membership end is
derivable only on the first write, not after a read. `Expression::isModelLevelEvaluable`,
`Import::importedElement`, `MetadataFeature::metaclass`, `EventOccurrenceUsage::eventOccurrence`,
`AssertConstraintUsage::assertedConstraint`, `Association::relatedType`/`sourceType`/`targetType`,
`ActionDefinition::action`, `AssignmentActionUsage::referent`/`valueExpression`, `ViewUsage::exposedElement`,
`SatisfyRequirementUsage::satisfyingFeature`, `VerificationCaseUsage::verifiedRequirement`,
`Package::filterCondition`, the loop, `if` and analysis-result properties have no evaluator rule yet
(`docs/project/constraint-manifest.json` lists them as not implemented).

### Where the two sides state different values

The largest **present-different** rows, with the reading the examples support:

| metaclass | property | present-equal | present-different |
|---|---|---:|---:|
| `Feature` | `ownedRelationship` | 4532 | 1624 |
| `AttributeUsage` | `ownedRelationship` | 2520 | 15 |
| `PartUsage` | `ownedRelationship` | 1841 | 109 |
| `PartUsage` | `ownedMember` | 1595 | 136 |
| `AttributeUsage` | `ownedMembership` | 2457 | 11 |
| `PartUsage` | `ownedMembership` | 1694 | 74 |
| `Feature` | `ownedMembership` | 4537 | 578 |
| `FeatureMembership` | `memberElement` | 9552 | 560 |
| `FeatureMembership` | `ownedMemberElement` | 9552 | 560 |
| `FeatureMembership` | `ownedRelatedElement` | 9552 | 560 |

### Where the pilot states nothing and we do

| metaclass | property | pilot-missing | clause |
|---|---|---:|---|
| `FeatureMembership` | `owner` | 10944 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `OwningMembership` | `owner` | 9702 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `FeatureValue` | `owner` | 7319 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `FeatureTyping` | `owner` | 6069 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `ReturnParameterMembership` | `owner` | 4853 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `ParameterMembership` | `owner` | 4585 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `Membership` | `owner` | 3269 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `Redefinition` | `owner` | 2693 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |

`owner` on every relationship is the pilot leaving `Relationship::owner` null where the metamodel derives `owningRelatedElement`; ours states it. The remaining pilot-missing rows are properties this repository writes outside the metamodel's spelling (`value` on usages) or states for unnamed elements the metamodel leaves null (`qualifiedName`).

### Where a member is named by a library feature

A member that takes its name from a naming feature is keyed by that name on both sides: KerML
`Feature::effectiveName` is the effective name of `namingFeature()` when nothing is declared
(8.3.3.3.4 Feature, `body: if declaredShortName <> null or declaredName <> null then declaredName
else … namingFeature.effectiveName()`), and the default `namingFeature()` is
`ownedRedefinition->at(1).redefinedFeature`; SysML redefines it for a variant (8.3.6.4 Usage,
`ownedReferenceSubsetting.referencedFeature`), a transition payload (8.3.6.3 ReferenceUsage), a
performed action (8.3.17.14 PerformActionUsage, `performedAction`), a requirement constraint
(8.3.20.4 ConstraintUsage) and a view rendering (8.3.26.6 RenderingUsage). The pilot derives the
same name when it resolves the naming feature, which is why the bridge reads this repository's
own libraries beside the standard one: `attribute redefines title = "…";` in a `Section` names
`DocumentQueries::Section::title` on both sides and is keyed `…::coverage::title` on both.

Reading those libraries moved 2983 elements from unmatched to matched and lost none:
`docs/manual/examples` from 6663 to 7020 (87 members — 84 `AttributeUsage` and 3 `ReferenceUsage`,
each an `attribute redefines caption|title|text|term|description|kind|direction|style|groupBy = …`
or `ref redefines source|target = …` of a `DocumentQueries` feature — with the 270 elements they
own) and `examples` from 23379 to 26005: 914 such members and owned elements of `DocumentQueries`
and `AnalysisRecords` features, with the two `render` members of `OpenSysMLRenderings` renderings,
in six files the pilot already exported, and the 1712 elements of `mosa-demo/mosa-demo.sysml` and
`oosem-demo/oosem-demo.sysml`, whose export the pilot had abandoned (`pilotFailed`, an element
without a resource) while `MOSA` and `OOSEM` were unloaded. `tests/testdata` matched the same 1201
elements with two more properties present-equal; no other root moved, and the pilot reported no
diagnostic on any library file (its batch validator reads the sixteen as one resource set without
an error). The members that gained a match disagreed with the pilot in three shapes when the libraries were
first loaded, none of them a derivation this repository gets wrong and none new — each a shape the
scorecard already carried for the standard library or for members the model declares itself, now
measured on library-backed members too. The first and the third are closed (the first in the next section, the third by the writer owning the referent `Membership`, below); the second remains:

- a reference to an element of this repository's libraries was spelled as a name, `{"@ref":
  "caption"}`, where the pilot writes the element: `Redefinition::redefinedFeature` (KerML
  8.3.3.3.8) on all 327 of those redefinitions, `FeatureTyping::type` (8.3.3.3.7) on 157, the `type`
  of 123 `MetadataUsage` and 16 `ViewUsage` typed by `MOSA` and `OOSEM` metadata and view
  definitions, `FeatureReferenceExpression::referent` (8.3.4.8.5) and the `memberElement` of its
  `Membership` on 28 enumeration literals, and `ReferenceSubsetting::referencedFeature` on the two
  `render` members. The writer gave a library element an id only where the pilot's XMI of the
  standard library fixes one; this repository's libraries have no such norm, so a reference to one
  of their elements fell back to the name as written, and the library element itself was not
  stated. Every `Element` has an `elementId` (KerML 8.3.2.1.2) and the API's JSON refers to an
  element by it, so the pilot's reading is the metamodel's — the writer now fixes an id for these
  elements too;
- the order of `ownedRelationship` on a usage that both redefines and has a value: the pilot lists
  the `Redefinition` before the `FeatureValue`, in the order the notation declares them, where ours
  listed the value first (275 `AttributeUsage`, 97 `ReferenceUsage`, 2 `RenderingUsage`).
  `ownedRelationship` is ordered (KerML 8.3.2.1.2 Element, `ownedRelationship : Relationship [0..*]
  {ordered}`), and the textual notation adds an element's owned relationships as it parses each
  clause (KerML 8.2.1 Table 3, `ownedRelationship += …`), so the pilot's order is the notation's. The
  writer now owns a head's relationships in that order (the row below records the re-measurement),
  and these sites agree;
- a `FeatureReferenceExpression` owns a `Membership` of its referent, which both sides now write:
  `referent` is derived as the `memberElement` of the first `ownedMembership` that is not a
  `ParameterMembership` (KerML 8.3.4.8.5, deriveFeatureReferenceExpressionReferent), and the
  notation maps the reference to that owned `Membership` (8.2.5.8.3, `FeatureReferenceMember`), so
  the `Membership` is part of the abstract syntax; the writer owns it before the result parameter
  and lists it under `ownedMembership`, as it does the target-feature `Membership` of a
  `FeatureChainExpression` (8.3.4.8.4) and the callee `Membership` of an invocation. The
  `FeatureReferenceExpression` `ownedMembership` row keeps 7 differing elements, all of them a
  function named as an argument (`->reduce '+'`), which the pilot owns as a `FeatureMembership` of
  an `Expression` typed by the function (8.2.5.8.3, `FunctionReferenceMember`) where ours writes a
  `Membership` of the function itself.

The two files the pilot now exports add, besides those three shapes, every shape the scorecard
already carries for user models (the pilot quoting a `qualifiedName` segment that holds a space,
`'Modular Ground Vehicle'::vehicle`, where ours does not; the structural ordinals of unnamed
members; the naming of connector ends). The other roots' movements under earlier changes to the
keys were of the first kind too: `spec-examples` names 13 members by a naming feature the pinned
excerpt leaves undeclared (`perform VehicleActions::move`, `individual timeslice redefines
preflight`, `render asMyTreeDiagram` under an undefined view definition, …), which the pilot,
resolving nothing, does not name. A run's own list of such movements comes from `-lost-against`.

### References into this repository's libraries link by id

Every named element of this repository's own libraries (`DocumentQueries`, `MOSA`, `OOSEM`, …)
now carries a fixed id, derived as a standard library element's normative id is — a library
package's id is `uuid5(URL name space, "https://github.com/Open-MBEE/OpenSysML-Extensions-Library/" +
package)`, each named element under it `uuid5(package id, escaped qualified name)` — so a reference
to one is written by id, `{"@id": …}`, the element itself named by a library stub in the graph, and
never as `{"@ref": "name"}` ([the RDF mapping](../reference/rdf-mapping.md#normative-library-identity)).
Those ids are deterministic and the same on every run, but they are **not the pilot's**: no
specification fixes ids for a non-normative library, and the pilot, reading the same files as a
library directory, hashes every library package under `https://www.omg.org/spec/SysML/`, so the two
sides' ids for `MOSA::ConsensusStandard` differ by that prefix alone. The referee cannot therefore
call such a reference equal: our side writes no element for a library target (the element form
carries the model's own elements only, [above](#our-side)), so the id has no key and the comparison
is **indeterminate** — the bucket that already holds references to standard library elements —
rather than equal or different. What this measures is presence and shape: the property is stated,
as an element reference, where it was stated as a name.

Movement on the same 920 files, pilot and keys unchanged (the exact movement a `-check` after this
change reports): present-different 50027 → 45688 (−4339), indeterminate 21034 → 25703 (+4669),
present-equal 618674 → 619268 (+594), absent-both 2102500 → 2103094 (+594), elements matched 89957
→ 90023 (+66, pilot-only 38863 → 38797), ours-missing 1058325 → 1058259 (−66), pilot-missing
68494 → 68560 (+66); no row's present-equal count fell, and no root but `examples/spec-examples`
(−5 present-different), `tests/testdata` (−10), `examples` (−2875) and `docs/manual/examples` (−1449)
moved. By row, present-different → indeterminate, one movement per site, the end properties of a
relationship (`target`, `relatedElement`, `general`, `subsettedFeature`) moving with the end they
restate:

| row | present-different | indeterminate | what moved |
|---|---:|---:|---|
| `Redefinition.redefinedFeature` (and `target`, `relatedElement`, `general`, `subsettedFeature`) | 388 → 58 | 542 → 938 | 330 redefinitions of a library feature (`attribute redefines caption = …`, `:>> organization = …`), plus 66 redefinitions newly matched (below); the 58 left are redefinitions the writer cannot link inside the model itself (`vehicle_b_1::providePower` redefining the feature inherited through `vehicle_b`) |
| `FeatureTyping.type` (and `target`, `relatedElement`, `general`) | 670 → 338 | 1904 → 2236 | 332 features typed by a library definition (`item ethernet : ConsensusStandard`, metadata typed by `MOSA` and `OOSEM` definitions) |
| `MetadataUsage.type` | 147 → 13 | 82 → 216 | 134 metadata usages typed by a library metadata definition (`#majorSystemComponent`, `@MOSAPackage`) |
| `ViewUsage.type` | 19 → 3 | 29 → 45 | 16 views typed by a `MOSA` or `OOSEM` view definition |
| `FeatureReferenceExpression.referent` | 99 → 71 | 436 → 464 | 28 references to a library enumeration literal (`MOSAPillar::openStandards`) |
| `Membership.memberElement` | 353 → 99 | 646 → 900 | 254 memberships of a referenced library element, the `referent` memberships among them |
| `InvocationExpression.function` | 245 → 19 | 107 → 333 | 226 calls of a calculation or function of these libraries |
| `Subclassification.superclassifier` (and `target`, `relatedElement`, `general`) | 126 → 13 | 136 → 249 | 113 definitions specializing a library definition (`:> MOSA::Standard`) |
| `PartUsage.type` | 235 → 63 | 0 → 172 | 172 parts typed by a definition of these libraries |
| `Subsetting.subsettedFeature` (and `target`, `relatedElement`, `general`) | 44 → 34 | 425 → 435 | 10 features subsetting a library feature |
| `NamespaceImport.importedNamespace`, `MembershipImport.importedMembership` | 25 → 8, 15 → 13 | 331 → 348, 265 → 267 | 17 imports of a library package, 2 of a library member |
| `CalculationUsage.type`, `ItemUsage.type`, `ReferenceUsage.type`, `ReferenceSubsetting.referencedFeature` (and its ends) | 7 → 3, 20 → 16, 48 → 46, 40 → 38 | 0 → 4, 16 → 20, 501 → 503, 24 → 26 | 4, 4, 2 and 2 references to a library element |

The 66 newly matched elements are `Redefinition`s: a feature in a metadata usage's body that
binds a feature of the metadata definition by name (`@MOSAPackage { pillar = MOSAPillar::openStandards; }`,
which redefines `MOSAPackage::pillar`) was written as a
`ReferenceUsage` with the declared name `pillar` and a value, because its redefinition reached a
library feature the writer could not link; it is now written as the pilot writes it — unnamed, with
an owned `Redefinition` of `MOSAPackage::pillar` — so `ReferenceUsage.declaredName` falls from 70
to 4 pilot-missing, `ReferenceUsage.ownedRedefinition`/`ownedSpecialization`/`ownedSubsetting`
gain 66 present-equal each, and the 66 `Redefinition`s match the pilot's, carrying every shape the
`Redefinition` rows already carry (`isLibraryElement`, `owningType` ours-missing; `owner`
pilot-missing; `redefinedFeature` indeterminate).

## Launch-ready gaps

The (metaclass, property) rows ranked by the corpus elements on which the pilot states a value and we state nothing (**ours-missing**), each with the derivation clause to implement and the in-flight work that already covers it, so that each row can be handed on as it stands. Where a property is an owned attribute with a default rather than a derivation, the row says so: the gap is in the writer, not in a derivation. The missing derivations are not implemented here.

| # | metaclass | property | ours-missing | present-different | in-flight overlap | derivation clause |
|---:|---|---|---:|---:|---|---|
| 1 | `Feature` | `inheritedMembership` | 10167 | 673 | this record's step 5 (completed from the semantic inheritance table; the rows still ours-missing are the expression, result-feature and multiplicity elements whose implied bases structure mode does not reach, and the lists still different are itemised there) | `Type::inheritedMembership = inheritedMemberships(excluded)` over the supertypes (KerML 8.3.3.1, deriveTypeInheritedMembership) |
| 2 | `Feature` | `inheritedFeature` | 10167 | 707 | this record's step 5 (completed from the semantic inheritance table; the rows still ours-missing are the expression, result-feature and multiplicity elements whose implied bases structure mode does not reach, and the lists still different are itemised there) | `Type::inheritedFeature = inheritedMembership.memberElement->selectByKind(Feature)` (KerML 8.3.3.1, deriveTypeInheritedFeature) |
| 3 | `Feature` | `featureMembership` | 10167 | 701 | this record's step 5 (completed from the semantic inheritance table; the rows still ours-missing are the expression, result-feature and multiplicity elements whose implied bases structure mode does not reach, and the lists still different are itemised there) | `Type::featureMembership = ownedFeatureMembership->union(inheritedMembership->selectByKind(FeatureMembership))` (KerML 8.3.3.1; KERML11-191 restricts the inherited part to the supertypes) |
| 4 | `Feature` | `feature` | 10167 | 701 | this record's step 5 (completed from the semantic inheritance table; the rows still ours-missing are the expression, result-feature and multiplicity elements whose implied bases structure mode does not reach, and the lists still different are itemised there) | `Type::feature = featureMembership.ownedMemberFeature` (KerML 8.3.3.1, deriveTypeFeature) |
| 5 | `Feature` | `type` | 9841 | 41 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::type = typing.type` with the types inherited through chaining and redefinition (KerML 8.3.3.3, deriveFeatureType) |
| 6 | `Feature` | `name` | 8948 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Element::name = effectiveName()`: the declared name, or for a feature the name of the feature it redefines (KerML 8.3.2.1, deriveElementName) |
| 7 | `Feature` | `membership` | 5667 | 5173 | this record's step 5 (completed from the semantic inheritance table; the rows still ours-missing are the expression, result-feature and multiplicity elements whose implied bases structure mode does not reach, and the lists still different are itemised there) | `Namespace::membership = ownedMembership->union(importedMembership)`, and for a Type also `inheritedMembership` (KerML 8.3.2.4, 8.3.3.1) |
| 8 | `Feature` | `member` | 5667 | 5174 | this record's step 5 (completed from the semantic inheritance table; the rows still ours-missing are the expression, result-feature and multiplicity elements whose implied bases structure mode does not reach, and the lists still different are itemised there) | `Namespace::member = membership.memberElement` (KerML 8.3.2.4, deriveNamespaceMember) |
| 9 | `ReturnParameterMembership` | `ownedMemberName` | 4764 | 0 | not completed — the owned member's name is stated on the member, the pilot restates it on the membership | `OwningMembership::ownedMemberName = ownedMemberElement.name`, redefining `memberName` (KerML 8.3.2.4.8); the pilot states it on anonymous result parameters whose name is their effective name |
| 10 | `ReturnParameterMembership` | `memberName` | 4764 | 0 | not completed (as `ownedMemberName`) | `Membership::memberName`, the owned member's name (KerML 8.3.2.4.3) |
| 11 | `ParameterMembership` | `ownedMemberName` | 4572 | 0 | not completed (as `ReturnParameterMembership::ownedMemberName`) | `OwningMembership::ownedMemberName = ownedMemberElement.name` (KerML 8.3.2.4.8); the pilot states the effective name of an unnamed parameter that redefines a named one by position |
| 12 | `ParameterMembership` | `memberName` | 4572 | 0 | not completed (as `ownedMemberName`) | `Membership::memberName`, the owned member's name (KerML 8.3.2.4.3) |
| 13 | `Membership` | `membershipOwningNamespace` | 3160 | 0 | not completed | `Membership::membershipOwningNamespace`, the owning Namespace of the Membership, which redefines `owningRelatedElement` (KerML 8.3.2.4.3) |
| 14 | `AttributeUsage` | `isReference` | 2551 | 0 | not completed | `Usage::isReference = not isComposite` (SysML 8.3.6.4, deriveUsageIsReference); the pilot states it on every attribute usage |
| 15 | `ReferenceUsage` | `isReference` | 2324 | 0 | not completed | `Usage::isReference = not isComposite` (SysML 8.3.6.4, deriveUsageIsReference); the pilot states it on every reference usage, the connector ends and positional parameters the effective-name change now matches among them |
| 16 | `AttributeUsage` | `membership` | 2294 | 261 | this record's step 5 (completed from the semantic inheritance table; the rows still ours-missing are the expression, result-feature and multiplicity elements whose implied bases structure mode does not reach, and the lists still different are itemised there) | `Namespace::membership = ownedMembership->union(importedMembership)`, for a Type with the inherited memberships (KerML 8.3.2.4.5, deriveNamespaceMembership; 8.3.3.1.10) |
| 17 | `AttributeUsage` | `member` | 2294 | 261 | this record's step 5 (completed from the semantic inheritance table; the rows still ours-missing are the expression, result-feature and multiplicity elements whose implied bases structure mode does not reach, and the lists still different are itemised there) | `Namespace::member = membership.memberElement` (KerML 8.3.2.4.5, deriveNamespaceMember) |
| 18 | `AttributeUsage` | `inheritedMembership` | 2294 | 261 | this record's step 5 (completed from the semantic inheritance table; the rows still ours-missing are the expression, result-feature and multiplicity elements whose implied bases structure mode does not reach, and the lists still different are itemised there) | `Type::inheritedMembership = inheritedMemberships(Set{}, Set{}, false)` (KerML 8.3.3.1.10, deriveTypeInheritedMembership) |
| 19 | `AttributeUsage` | `inheritedFeature` | 2294 | 261 | this record's step 5 (completed from the semantic inheritance table; the rows still ours-missing are the expression, result-feature and multiplicity elements whose implied bases structure mode does not reach, and the lists still different are itemised there) | `Type::inheritedFeature = inheritedMembership.memberElement->selectByKind(Feature)` (KerML 8.3.3.1.10, deriveTypeInheritedFeature) |
| 20 | `AttributeUsage` | `featureMembership` | 2294 | 261 | this record's step 5 (completed from the semantic inheritance table; the rows still ours-missing are the expression, result-feature and multiplicity elements whose implied bases structure mode does not reach, and the lists still different are itemised there) | `Type::featureMembership = ownedFeatureMembership->union(inheritedMembership->selectByKind(FeatureMembership))` (KerML 8.3.3.1.10, deriveTypeFeatureMembership) |
| 21 | `AttributeUsage` | `feature` | 2294 | 261 | this record's step 5 (completed from the semantic inheritance table; the rows still ours-missing are the expression, result-feature and multiplicity elements whose implied bases structure mode does not reach, and the lists still different are itemised there) | `Type::feature = featureMembership.ownedMemberFeature` (KerML 8.3.3.1.10, deriveTypeFeature) |
| 22 | `FeatureReferenceExpression` | `type` | 2181 | 0 | not completed — the result-feature family of this record's step 5 | `Feature::type`: the general of every FeatureTyping of the expression, including the implied typing by `Performances::Evaluation` (KerML 8.3.4.8.4; the implied base is `Performances::evaluations`) |
| 23 | `FeatureReferenceExpression` | `isModelLevelEvaluable` | 2181 | 0 | not completed — no evaluator rule (`docs/project/constraint-manifest.json`) | `Expression::isModelLevelEvaluable = modelLevelEvaluable(Set{})` (KerML 8.3.4.7.3, deriveExpressionIsModelLevelEvaluable) |
| 24 | `FeatureReferenceExpression` | `inheritedMembership` | 2181 | 0 | not completed — the result-feature family of this record's step 5 | `Type::inheritedMembership = inheritedMemberships(Set{}, Set{}, false)` (KerML 8.3.3.1.10, deriveTypeInheritedMembership) |
| 25 | `FeatureReferenceExpression` | `inheritedFeature` | 2181 | 0 | not completed — the result-feature family of this record's step 5 | `Type::inheritedFeature = inheritedMembership.memberElement->selectByKind(Feature)` (KerML 8.3.3.1.10, deriveTypeInheritedFeature) |
| 26 | `FeatureReferenceExpression` | `behavior` | 2181 | 0 | not completed — the result-feature family of this record's step 5 | `Step::behavior = type->selectByKind(Behavior)` (KerML 8.3.4.6.3, deriveStepBehavior); for an expression the implied `Performances::Evaluation` typing |
| 27 | `FeatureMembership` | `memberName` | 365 | 4 | closed for a named member — the conversion states `memberName` as the member's effective name; the 362 left are implied parameters the pilot names by the library feature they implicitly redefine | `Membership::memberName = memberElement.name` (KerML 8.3.2.3.5, deriveMembershipMemberName) |
| 28 | `FeatureMembership` | `ownedMemberName` | 182 | 4 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberName = ownedMemberElement.name` (KerML 8.3.2.3.6) |
| 29 | `Feature` | `featuringType` | 101 | 39 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::featuringType = typeFeaturing.featuringType`, plus the owning type of an owned feature (KerML 8.3.3.3, deriveFeatureFeaturingType; KERML11-3/4 multiplicity featuring) |
| 30 | `Redefinition` | `redefinedFeature` | 52 | 22 | closed for this repository's libraries (above) | `Redefinition::redefinedFeature : Feature` (KerML 8.3.3.3.8) is an element reference, and every `Element` carries an `elementId` (KerML 8.3.2.1.2) the API's JSON refers to it by; the 330 redefinitions of a feature of this repository's libraries once spelled as a name, `{"@ref": "caption"}`, now link by the element's fixed id — the same movement for `FeatureTyping::type`, `MetadataUsage::type`, `FeatureReferenceExpression::referent` and `ReferenceSubsetting::referencedFeature` into those libraries — and the 58 left are redefinitions of a feature inherited within the model itself that the writer spells as a name (`vehicle_b_1::providePower`) |
| 31 | `AttributeUsage` | `ownedRelationship` | 1 | 15 | none found | `Element::ownedRelationship : Relationship [0..*] {ordered}` (KerML 8.3.2.1.2), in the order the notation adds them (KerML 8.2.1 Table 3; `FeatureSpecializationPart`, then `ValuePart`, then the body): a head's typings, specializations and multiplicity as spelled, then its `FeatureValue`, then its members. The writer seated the materialized relationships last, in a fixed kind order; it now owns them as spelled, which moved 4861 properties over 101 rows (67 `ownedRelationship`, 22 `ownedSpecialization`, 12 `ownedSubsetting`) from present-different to present-equal, no other bucket touched (at that re-record this row 2332 → 1197, `PartDefinition` 215 → 20, `PartUsage` 1554 → 1004, `Feature` 1802 → 1401; the current baseline, with the re-pinned libraries and the examples that say `tolerate`, reads 1245, 20, 1054 and 1535). What remains (1245 on this row) is a different shape: on a usage that writes no multiplicity part (`attribute cargoMass : MassValue;`) the pilot owns an `OwningMembership` of a `Multiplicity`, not implied, after the `FeatureTyping`; ours states none |
| 32 | `Feature` | `featureTarget` | 23 | 153 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::featureTarget`: the last `chainingFeature`, else the feature itself (KerML 8.3.3.3, deriveFeatureFeatureTarget; KERML11-1 crossFeature) |
| 33 | `Feature` | `owningNamespace` | 26 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Element::owningNamespace = owningMembership.membershipOwningNamespace` (KerML 8.3.2.1, deriveElementOwningNamespace) |
| 34 | `Feature` | `owningFeatureMembership` | 3 | 194 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningFeatureMembership = owningMembership->selectByKind(FeatureMembership)` (KerML 8.3.3.3) |
| 35 | `Feature` | `owningType` | 3 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningType = owningFeatureMembership.owningType` (KerML 8.3.3.3, deriveFeatureOwningType) |
| 36 | `FeatureMembership` | `target` | 0 | 560 | closed — the RDF conversion states a membership's ends (this record's membership paragraph); the present-different are the member's own keying, as `memberElement` | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 37 | `FeatureMembership` | `relatedElement` | 0 | 560 | closed — the RDF conversion states a membership's ends (this record's membership paragraph); the present-different are the member's own keying, as `memberElement` | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 38 | `FeatureMembership` | `ownedMemberElementId` | 0 | 560 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 39 | `FeatureMembership` | `memberElementId` | 0 | 560 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 40 | `OwningMembership` | `relatedElement` | 0 | 525 | closed — the RDF conversion states a membership's ends (this record's membership paragraph); the present-different are the member's own keying, as `memberElement` | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 41 | `OwningMembership` | `target` | 0 | 522 | closed — the RDF conversion states a membership's ends (this record's membership paragraph); the present-different are the member's own keying, as `memberElement` | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 42 | `OwningMembership` | `ownedMemberElementId` | 0 | 522 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 43 | `OwningMembership` | `memberElementId` | 0 | 522 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 44 | `FeatureValue` | `target` | 0 | 310 | closed — the RDF conversion states a membership's ends (this record's membership paragraph); the present-different are the member's own keying, as `memberElement` | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 45 | `FeatureValue` | `relatedElement` | 0 | 310 | closed — the RDF conversion states a membership's ends (this record's membership paragraph); the present-different are the member's own keying, as `memberElement` | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 46 | `FeatureValue` | `ownedMemberElementId` | 0 | 310 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 47 | `FeatureValue` | `memberElementId` | 0 | 310 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 48 | `FeatureMembership` | `visibility` | 0 | 11 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 49 | `FeatureReferenceExpression` | `ownedMembership` | 0 | 7 | the writer owns the `Membership` (`export/rdf_expr.go` `featureReferent`); the 7 left are the `FunctionReferenceMember` shape, a `FeatureMembership` owning an `Expression` typed by the named function (KerML 8.2.5.8.3), which ours writes as a `Membership` of the function | `FeatureReferenceExpression::referent` is the `memberElement` of the first `ownedMembership` that is not a `ParameterMembership` (KerML 8.3.4.8.5, deriveFeatureReferenceExpressionReferent); the notation maps a feature reference to that owned `Membership` (8.2.5.8.3, `FeatureReferenceMember`) |
| 50 | `Feature` | `isConjugated` | 0 | 6 | none found | `Type::isConjugated = ownedConjugator <> null` (KerML 8.3.3.1 Type; a derived attribute without a named derive constraint); the pilot writes `false` on every type |
| 51 | `OwningMembership` | `visibility` | 0 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 52 | `OwningMembership` | `source` | 0 | 0 | closed — the RDF conversion states a membership's ends (this record's membership paragraph); the present-different are the member's own keying, as `memberElement` | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 53 | `OwningMembership` | `isLibraryElement` | 0 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 54 | `FeatureValue` | `visibility` | 0 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 55 | `FeatureValue` | `source` | 0 | 0 | closed — the RDF conversion states a membership's ends (this record's membership paragraph); the present-different are the member's own keying, as `memberElement` | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 56 | `FeatureValue` | `isLibraryElement` | 0 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 57 | `FeatureTyping` | `owningType` | 0 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningType = owningFeatureMembership.owningType` (KerML 8.3.3.3, deriveFeatureOwningType) |
| 58 | `FeatureTyping` | `isLibraryElement` | 0 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 59 | `FeatureMembership` | `source` | 0 | 0 | closed — the RDF conversion states a membership's ends (this record's membership paragraph); the present-different are the member's own keying, as `memberElement` | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 60 | `FeatureMembership` | `isLibraryElement` | 0 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 61 | `Feature` | `isLibraryElement` | 0 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 62 | `Multiplicity` | `feature` | 1 | 3376 | present-different as the rest of the inherited family (the base `Base::naturals` is stated; causes 1–2 of the inherited-member entry: library member ids the key does not reach, implied bases ordered first) | `Multiplicity`/`MultiplicityRange` must specialize `Base::naturals` (KerML 8.3.3.3.4 checkMultiplicitySpecialization, 8.3.3.3.5); the semantic side tables compute no implied base for a multiplicity (`ImpliedBaseFQNs` has no multiplicity case, and `Base::naturals` is named nowhere in `internal/semantic/semantics`), so structure mode inherits nothing for the implied-default `Multiplicity` elements develop writes since #1930 — a side-table gap, recorded rather than approximated |
| 63 | `Multiplicity` | `featureMembership` | 1 | 3376 | present-different as the rest of the inherited family (the base `Base::naturals` is stated; causes 1–2 of the inherited-member entry: library member ids the key does not reach, implied bases ordered first) | `Multiplicity`/`MultiplicityRange` must specialize `Base::naturals` (KerML 8.3.3.3.4 checkMultiplicitySpecialization, 8.3.3.3.5); the semantic side tables compute no implied base for a multiplicity (`ImpliedBaseFQNs` has no multiplicity case, and `Base::naturals` is named nowhere in `internal/semantic/semantics`), so structure mode inherits nothing for the implied-default `Multiplicity` elements develop writes since #1930 — a side-table gap, recorded rather than approximated |
| 64 | `Multiplicity` | `inheritedFeature` | 1 | 3376 | present-different as the rest of the inherited family (the base `Base::naturals` is stated; causes 1–2 of the inherited-member entry: library member ids the key does not reach, implied bases ordered first) | `Multiplicity`/`MultiplicityRange` must specialize `Base::naturals` (KerML 8.3.3.3.4 checkMultiplicitySpecialization, 8.3.3.3.5); the semantic side tables compute no implied base for a multiplicity (`ImpliedBaseFQNs` has no multiplicity case, and `Base::naturals` is named nowhere in `internal/semantic/semantics`), so structure mode inherits nothing for the implied-default `Multiplicity` elements develop writes since #1930 — a side-table gap, recorded rather than approximated |
| 65 | `Multiplicity` | `type` | 3377 | 0 | open: the base is stated (`Base::naturals`, KerML 8.3.3.1.9) but the writer states `type` for a written typing only, as for `PartUsage.type` (580 ours-missing) | `Multiplicity`/`MultiplicityRange` must specialize `Base::naturals` (KerML 8.3.3.3.4 checkMultiplicitySpecialization, 8.3.3.3.5); the semantic side tables compute no implied base for a multiplicity (`ImpliedBaseFQNs` has no multiplicity case, and `Base::naturals` is named nowhere in `internal/semantic/semantics`), so structure mode inherits nothing for the implied-default `Multiplicity` elements develop writes since #1930 — a side-table gap, recorded rather than approximated |
| 66 | `Multiplicity` | `inheritedMembership` | 1 | 3376 | present-different as the rest of the inherited family (the base `Base::naturals` is stated; causes 1–2 of the inherited-member entry: library member ids the key does not reach, implied bases ordered first) | `Multiplicity`/`MultiplicityRange` must specialize `Base::naturals` (KerML 8.3.3.3.4 checkMultiplicitySpecialization, 8.3.3.3.5); the semantic side tables compute no implied base for a multiplicity (`ImpliedBaseFQNs` has no multiplicity case, and `Base::naturals` is named nowhere in `internal/semantic/semantics`), so structure mode inherits nothing for the implied-default `Multiplicity` elements develop writes since #1930 — a side-table gap, recorded rather than approximated |
| 67 | `Multiplicity` | `member` | 1 | 3376 | present-different as the rest of the inherited family (the base `Base::naturals` is stated; causes 1–2 of the inherited-member entry: library member ids the key does not reach, implied bases ordered first) | `Multiplicity`/`MultiplicityRange` must specialize `Base::naturals` (KerML 8.3.3.3.4 checkMultiplicitySpecialization, 8.3.3.3.5); the semantic side tables compute no implied base for a multiplicity (`ImpliedBaseFQNs` has no multiplicity case, and `Base::naturals` is named nowhere in `internal/semantic/semantics`), so structure mode inherits nothing for the implied-default `Multiplicity` elements develop writes since #1930 — a side-table gap, recorded rather than approximated |
| 68 | `Multiplicity` | `membership` | 1 | 3376 | present-different as the rest of the inherited family (the base `Base::naturals` is stated; causes 1–2 of the inherited-member entry: library member ids the key does not reach, implied bases ordered first) | `Multiplicity`/`MultiplicityRange` must specialize `Base::naturals` (KerML 8.3.3.3.4 checkMultiplicitySpecialization, 8.3.3.3.5); the semantic side tables compute no implied base for a multiplicity (`ImpliedBaseFQNs` has no multiplicity case, and `Base::naturals` is named nowhere in `internal/semantic/semantics`), so structure mode inherits nothing for the implied-default `Multiplicity` elements develop writes since #1930 — a side-table gap, recorded rather than approximated |

Rows an evaluator rule in PR #1232 (`internal/semantic/metamodel/rules.go`) already names are marked; that PR adds a lazy metamodel evaluator and layers its derived values into the API-JSON writer, so its merge is expected to move those rows from ours-missing to present-equal or present-different, and this referee is how that movement is measured.
