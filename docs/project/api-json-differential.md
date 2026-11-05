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

**Scorecard:** 935 files; 99783 elements matched (18471 ours-only, 31542 pilot-only, 0 of them implied); properties: 1874277 present-equal, 42899 present-different, 434358 ours-missing, 76533 pilot-missing, 2124817 absent-both, 53621 indeterminate, 516 pilot-nondeterministic.

The pilot's export failed on 59 files (`pilotFailed`: its own export throws on an element without a resource, 52 of them specification examples, 4 test fixtures and, since pilot `2026-09`, the three `Metadata Examples/{Requirement,Risk,Verification}MetadataExample.sysml` whose enumeration-valued metadata bindings the pilot now evaluates into elements with no resource; the files are named in the run's text report), ours refused 36 (all of them specification examples the parser could not read whole), left the pilot's 59 failed files out and converted 30 documents alone for declaring an element another document of their root declares (`oursColliding`); no root fell back to per-document conversion. 11735 (metaclass, property) rows carry a comparison; 112895 references compared through the unkeyed sentinel, 162741 of our `sysx:` annotations were left out, and 303/368 elements (ours/pilot) were dropped for ambiguous keys, 17040/32319 for having none.

| root | files | matched | ours-only | pilot-only | present-equal | present-different | ours-missing | pilot-missing |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `examples/sysml-v2-training` | 100 | 8026 | 1130 | 2140 | 152959 | 2822 | 33902 | 5616 |
| `examples/pilot-corpora/sysml-examples` | 99 | 28252 | 6149 | 7864 | 539991 | 12093 | 123551 | 20736 |
| `examples/pilot-corpora/sysml-validation` | 56 | 10015 | 1881 | 2584 | 189531 | 5209 | 41866 | 7416 |
| `examples/pilot-corpora/kerml-examples` | 58 | 6405 | 1153 | 1836 | 111542 | 4118 | 24856 | 7399 |
| `examples/spec-examples` | 511 | 7371 | 989 | 2359 | 124903 | 2260 | 29866 | 6097 |
| `tests/testdata` | 40 | 1514 | 175 | 457 | 26663 | 706 | 6274 | 1143 |
| `examples` | 56 | 30660 | 6072 | 12480 | 583778 | 12955 | 136428 | 23022 |
| `tools/referee/diff/testdata` | 4 | 53 | 0 | 0 | 839 | 3 | 162 | 41 |
| `docs/manual/examples` | 11 | 7487 | 922 | 1822 | 144071 | 2733 | 37453 | 5063 |

**An `individual` definition's and a named connector end's `[1..1]`** (re-recorded when the writer
began stating the empty `Multiplicity` an `individual` definition's prefix owns — first among its
relationships, SysML.xtext `OccurrenceDefinitionPrefix`, `EmptyMultiplicityMember` — and the `[1..1]`
of a connector usage's named end, `connect e1 ::> x to e2 ::> y`, last; a positional end `connect x to y`
states none, which is what the pilot does: its parser post-processes a declared usage, not the
`ConnectorEnd` production; measured on the record below, which types the individuals): 122 rows moved.
80 more Multiplicity elements match (every `Multiplicity` row 3047 → 3127; `owningMembership` 3113
equal, the same 14 different), their memberships with them (`OwningMembership.isImplied` 12744 → 12824
equal; matched elements 99623 → 99783, pilot-only 31702 → 31542, ours-only unchanged — every default
written has a pilot element). The definitions' `multiplicity` rows move from ours-missing to
present-equal — `PartDefinition` 0 → 14, `ItemDefinition` 0 → 6, `ActionDefinition` 0 → 3,
`AnalysisCaseDefinition` 0 → 1, `OccurrenceDefinition` 0 → 36 — and their `ownedMember`,
`ownedMembership`, `ownedRelationship` and `ownedElement` rows with them; the named ends read on
`ReferenceUsage.multiplicity` 210 → 216 equal (7 → 1 ours-missing) and `PortUsage.multiplicity`
395 → 409 (19 → 5), and `Documentation.owningMembership`/`owningRelationship` 366 → 370 equal (a
definition's comment is numbered after the prefix's membership, as the pilot numbers it). Of the 176
pilot-only Multiplicity elements under a matched owner the earlier record names, the 107 the checker
withholds on a subsetting of a type-owned feature remain.

**An `individual` is an occurrence** (re-recorded when the writer began typing an `individual def` as
the `OccurrenceDefinition` and an `individual` usage as the `OccurrenceUsage` the pilot's grammar
reduces them to — `IndividualDefinition returns SysML::OccurrenceDefinition`, `IndividualUsage returns
SysML::OccurrenceUsage` — with `isIndividual`, in place of two names the metamodel does not declare):
558 rows moved from one cause. 47 elements whose `@type` disagreed match (type-mismatched 1729 → 1682),
and with them the elements they own that the referee had set aside as unmatched on both sides (matched
99535 → 99623, ours-only 18530 → 18471, pilot-only 31790 → 31702; `FeatureMembership.isImplied` 10944 →
10984 equal, `OccurrenceUsage.ownedRelationship` 114 → 134 equal, 17 → 2 different, `.definition` 52 → 59,
`.isIndividual` 144 → 149, and the `ItemUsage`, `ReferenceUsage`, `LiteralRational`, `FeatureValue`,
`Redefinition`, `FeatureTyping` and `Subclassification` rows of what those usages own). The API-JSON
writer reads a property's cardinality from the metaclass, so these elements' collections are now lists
(`ownedRelationship`, `ownedMember`, …) where a single value was written as an object before. Nothing
else moved.

**A usage states its implicit [1..1] by an owned Multiplicity** (re-recorded when the semantic model
began deriving, for an attribute, item, part, view, rendering or port usage that a type owns through a
FeatureMembership and that declares no multiplicity, the nameless `Multiplicity` the pilot adds
(`UsageParserPostProcessor.isAddDefaultMultiplicity`, SysML v2 §7.6.3) — the member of a non-implied
`OwningMembership` the usage owns last, and its `Type::multiplicity`; none for a declared range, a KerML
feature, an expression body's parameters, an action, a subject or objective, a variant or enumerated
value, a redefinition whatever it redefines, or a usage whose subsetting, reference or cross-subsetting
targets a type-owned feature): 134 rows moved. 3047 Multiplicity elements match where 2 did, so every
`Multiplicity` row reads 3047 (`owner`, `owningNamespace`, `isAbstract` … present-equal; `ownedMember`,
`ownedSubsetting` … absent-both; `type` ours-missing, as `MultiplicityRange.type` already is: the pilot
types every multiplicity `ScalarValues::Natural`) but `owningMembership`/`owningRelationship` 3033 equal,
14 different — the pilot numbers the membership after a plain `/* … */` comment's `OwningMembership`,
which the writer does not export (nor the `Comment`), the same 14 behind `OwningMembership.memberElement`
431 → 444 different. `OwningMembership` rows gain 3045 present-equal each (9706 → 12751). The usages'
`multiplicity` rows move from ours-missing to present-equal — `AttributeUsage` 94 → 1359 (10 ours-missing
left), `PartUsage` 272 → 1296 (64), `PortUsage` 24 → 395 (19), `ItemUsage` 49 → 301 (7), `ReferenceUsage`
137 → 210 (7), `ViewUsage` 3 → 10 (1), `RenderingUsage` 2 → 35, `Usage` 0 → 9 — and their `ownedMember`,
`ownedMembership`, `ownedRelationship` and `ownedElement` rows with them (`PartUsage.ownedMember` 564 →
1594 equal, 754 → 136 different; `AttributeUsage.ownedRelationship` 1257 → 2522 equal, 1245 → 15
different); no `.multiplicity` row is pilot-missing. What the pilot still states and this writer does
not, 176 Multiplicity elements under a matched owner: 60 under an `individual` definition (the pilot
gives one `[1..1]` too), 9 under a connector usage's `::>` ends (its implied binding ends `thisThing`/
`sameThing` likewise, under owners not matched), and 107 under usages whose subsetting or reference
targets a type-owned feature (`verdict1 :> verdicts`, `photo : Picture :> photos`), where the checker
withholds the default (`ImplicitMultiplicityApplies`) and the pilot withholds only on redefinition
(its `TODO: SYSML21-185`) — a maintainer's call, left as it stands. The `EnumerationUsage` rows do not
move: an enumerated value is a variant, and takes none.

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
`Membership::source`, `target`, `relatedElement`, `memberName` and `memberShortName` as KerML 8.3.2.4.3 and
8.3.2.1.3 derive them — the owning namespace, the member, both in that order, the member's effective
names — on every membership shape it mints, and the reflective read began deriving `memberName` as the
member's *effective* name, KerML 8.3.3.3.4, so an unnamed parameter is named by the one it redefines):
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
(`subj`) — and `Import::source` and `target`, KerML 8.3.2.4.2, as the import's owning namespace and the
namespace or membership it imports; and when the reflective read stopped naming the plain Membership
an expression owns to its referent, which only an alias specifies, KerML 8.3.2.4.3 — the pilot leaves
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

Not completed, and why: `Type::feature`, `featureMembership`, `inheritedMembership`, `inheritedFeature`,
`directedFeature`, `input`, `output`, `endFeature`, a type's `member`/`membership`, `Usage::usage`,
`Definition::usage` and `directedUsage` (about 215000 rows) need the inherited members of every general
type; the structure-mode evaluator sees neither the implied library bases the graph does not state nor
the members of a general type outside the graph, and its own-members-only answer would be wrong under
§8.3.3.1.10, so the rows stay ours-missing until the exporter is given the semantic inheritance table
(`internal/semantic/semantics` `Model.MembersOf`). `Behavior::step` (0 equal / 257 different),
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
| `Feature` | `ownedRelationship` | 4206 | 1532 |
| `AttributeUsage` | `ownedRelationship` | 2522 | 15 |
| `PartUsage` | `ownedRelationship` | 1840 | 109 |
| `PartUsage` | `ownedMember` | 1594 | 136 |
| `AttributeUsage` | `ownedMembership` | 2459 | 11 |
| `PartUsage` | `ownedMembership` | 1693 | 74 |
| `Feature` | `ownedMembership` | 4447 | 571 |
| `FeatureMembership` | `memberElement` | 9510 | 573 |
| `FeatureMembership` | `ownedMemberElement` | 9510 | 573 |
| `FeatureMembership` | `ownedRelatedElement` | 9510 | 573 |

### Where the pilot states nothing and we do

| metaclass | property | pilot-missing | clause |
|---|---|---:|---|
| `FeatureMembership` | `owner` | 10944 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.1.3) |
| `OwningMembership` | `owner` | 9702 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.1.3) |
| `FeatureValue` | `owner` | 7319 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.1.3) |
| `FeatureTyping` | `owner` | 6069 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.1.3) |
| `ReturnParameterMembership` | `owner` | 4853 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.1.3) |
| `ParameterMembership` | `owner` | 4585 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.1.3) |
| `Membership` | `owner` | 3269 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.1.3) |
| `Redefinition` | `owner` | 2693 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.1.3) |

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

The (metaclass, property) rows ranked by the corpus elements on which the pilot states a value and we state nothing (**ours-missing**), each with the derivation clause and the state of its implementation, so that each row can be handed on as it stands. Where a property is an owned attribute with a default rather than a derivation, the row says so: the gap is in the writer, not in a derivation. The rows the writer's completion closed (`isLibraryElement`, `memberElementId`, `visibility`, `owningType`, `isConjugated`, … — § Results) are no longer listed; the rows left are the inherited-member family and the implied names.

| # | metaclass | property | ours-missing | present-different | status | derivation clause |
|---:|---|---|---:|---:|---|---|
| 1 | `Feature` | `inheritedFeature` | 10418 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::inheritedFeature = inheritedMembership.memberElement->selectByKind(Feature)` (KerML 8.3.3.1, deriveTypeInheritedFeature) |
| 2 | `Feature` | `inheritedMembership` | 10418 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::inheritedMembership = inheritedMemberships(excluded)` over the supertypes (KerML 8.3.3.1, deriveTypeInheritedMembership) |
| 3 | `Feature` | `member` | 10418 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Namespace::member = membership.memberElement` (KerML 8.3.2.4, deriveNamespaceMember) |
| 4 | `Feature` | `membership` | 10418 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Namespace::membership = ownedMembership->union(importedMembership)`, and for a Type also `inheritedMembership` (KerML 8.3.2.4, 8.3.3.1) |
| 5 | `Feature` | `feature` | 10412 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::feature = featureMembership.ownedMemberFeature` (KerML 8.3.3.1, deriveTypeFeature) |
| 6 | `Feature` | `featureMembership` | 10412 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::featureMembership = ownedFeatureMembership->union(inheritedMembership->selectByKind(FeatureMembership))` (KerML 8.3.3.1; KERML11-191 restricts the inherited part to the supertypes) |
| 7 | `Feature` | `type` | 9663 | 37 | completed where the graph states a typing (§ Results); the rows left are types inherited from a library feature or through an implied subsetting the graph does not state — the same inheritance table | `Feature::type = typing.type` with the types inherited through chaining and redefinition (KerML 8.3.3.3, deriveFeatureType) |
| 8 | `Feature` | `name` | 8883 | 0 | completed from the declared name and a declared redefinition (§ Results); the rows left are implied parameters and ends the pilot names by the library feature they implicitly redefine (`result`, `source`, `logical`), which needs the implied-redefinition table | `Element::name = effectiveName()`: the declared name, or for a feature the name of the feature it redefines (KerML 8.3.2.1, deriveElementName) |
| 9 | `ReturnParameterMembership` | `memberName` | 4724 | 0 | open: the pilot names an implied parameter by the library feature it implicitly redefines (`result`, `source`, `logical`); our effective name follows a declared redefinition only (§ Results, step 2) — needs the implied-redefinition table | `Membership::memberName = memberElement.name` (KerML 8.3.2.4.3, deriveMembershipMemberName) |
| 10 | `ReturnParameterMembership` | `ownedMemberName` | 4724 | 0 | open: the pilot names an implied parameter by the library feature it implicitly redefines (`result`, `source`, `logical`); our effective name follows a declared redefinition only (§ Results, step 2) — needs the implied-redefinition table | `OwningMembership::ownedMemberName = ownedMemberElement.name` (KerML 8.3.2.3.6) |
| 11 | `ParameterMembership` | `memberName` | 4545 | 0 | open: the pilot names an implied parameter by the library feature it implicitly redefines (`result`, `source`, `logical`); our effective name follows a declared redefinition only (§ Results, step 2) — needs the implied-redefinition table | `Membership::memberName = memberElement.name` (KerML 8.3.2.4.3, deriveMembershipMemberName) |
| 12 | `ParameterMembership` | `ownedMemberName` | 4545 | 0 | open: the pilot names an implied parameter by the library feature it implicitly redefines (`result`, `source`, `logical`); our effective name follows a declared redefinition only (§ Results, step 2) — needs the implied-redefinition table | `OwningMembership::ownedMemberName = ownedMemberElement.name` (KerML 8.3.2.3.6) |
| 13 | `Membership` | `membershipOwningNamespace` | 3131 | 0 | open: neither stated by the writer nor completed by the evaluator (§ Results, *Not completed*); `owningRelatedElement` is a trivial completion left for the next step | `Membership::membershipOwningNamespace : Namespace` redefines `owningRelatedElement` (KerML 8.3.2.4.3) |
| 14 | `Multiplicity` | `feature` | 3127 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::feature = featureMembership.ownedMemberFeature` (KerML 8.3.3.1, deriveTypeFeature) |
| 15 | `Multiplicity` | `featureMembership` | 3127 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::featureMembership = ownedFeatureMembership->union(inheritedMembership->selectByKind(FeatureMembership))` (KerML 8.3.3.1; KERML11-191 restricts the inherited part to the supertypes) |
| 16 | `Multiplicity` | `inheritedFeature` | 3127 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::inheritedFeature = inheritedMembership.memberElement->selectByKind(Feature)` (KerML 8.3.3.1, deriveTypeInheritedFeature) |
| 17 | `Multiplicity` | `inheritedMembership` | 3127 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::inheritedMembership = inheritedMemberships(excluded)` over the supertypes (KerML 8.3.3.1, deriveTypeInheritedMembership) |
| 18 | `Multiplicity` | `member` | 3127 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Namespace::member = membership.memberElement` (KerML 8.3.2.4, deriveNamespaceMember) |
| 19 | `Multiplicity` | `membership` | 3127 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Namespace::membership = ownedMembership->union(importedMembership)`, and for a Type also `inheritedMembership` (KerML 8.3.2.4, 8.3.3.1) |
| 20 | `Multiplicity` | `type` | 3127 | 0 | open: the pilot types every multiplicity `ScalarValues::Natural`, an implied typing the writer states for neither a default `Multiplicity` nor a `MultiplicityRange` (`MultiplicityRange.type` is ours-missing alike) | `Feature::type = typing.type` with the types inherited through chaining and redefinition (KerML 8.3.3.3, deriveFeatureType) |
| 21 | `AttributeUsage` | `feature` | 2557 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::feature = featureMembership.ownedMemberFeature` (KerML 8.3.3.1, deriveTypeFeature) |
| 22 | `AttributeUsage` | `featureMembership` | 2557 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::featureMembership = ownedFeatureMembership->union(inheritedMembership->selectByKind(FeatureMembership))` (KerML 8.3.3.1; KERML11-191 restricts the inherited part to the supertypes) |
| 23 | `AttributeUsage` | `inheritedFeature` | 2557 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::inheritedFeature = inheritedMembership.memberElement->selectByKind(Feature)` (KerML 8.3.3.1, deriveTypeInheritedFeature) |
| 24 | `AttributeUsage` | `inheritedMembership` | 2557 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::inheritedMembership = inheritedMemberships(excluded)` over the supertypes (KerML 8.3.3.1, deriveTypeInheritedMembership) |
| 25 | `AttributeUsage` | `member` | 2557 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Namespace::member = membership.memberElement` (KerML 8.3.2.4, deriveNamespaceMember) |
| 26 | `AttributeUsage` | `membership` | 2557 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Namespace::membership = ownedMembership->union(importedMembership)`, and for a Type also `inheritedMembership` (KerML 8.3.2.4, 8.3.3.1) |
| 27 | `AttributeUsage` | `isReference` | 2553 | 0 | open: not completed (§ Results, *Not completed*); it follows `isComposite`, which the graph states only where the notation spells `ref` | `Usage::isReference = not isComposite` (SysML 8.4.4.1, deriveUsageIsReference) |
| 28 | `ReferenceUsage` | `feature` | 2329 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::feature = featureMembership.ownedMemberFeature` (KerML 8.3.3.1, deriveTypeFeature) |
| 29 | `ReferenceUsage` | `featureMembership` | 2329 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::featureMembership = ownedFeatureMembership->union(inheritedMembership->selectByKind(FeatureMembership))` (KerML 8.3.3.1; KERML11-191 restricts the inherited part to the supertypes) |
| 30 | `ReferenceUsage` | `inheritedFeature` | 2329 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::inheritedFeature = inheritedMembership.memberElement->selectByKind(Feature)` (KerML 8.3.3.1, deriveTypeInheritedFeature) |
| 31 | `ReferenceUsage` | `inheritedMembership` | 2329 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::inheritedMembership = inheritedMemberships(excluded)` over the supertypes (KerML 8.3.3.1, deriveTypeInheritedMembership) |
| 32 | `ReferenceUsage` | `member` | 2329 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Namespace::member = membership.memberElement` (KerML 8.3.2.4, deriveNamespaceMember) |
| 33 | `ReferenceUsage` | `membership` | 2329 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Namespace::membership = ownedMembership->union(importedMembership)`, and for a Type also `inheritedMembership` (KerML 8.3.2.4, 8.3.3.1) |
| 34 | `FeatureReferenceExpression` | `behavior` | 2162 | 0 | open: the derivation reaches the implied library base (`Performances::evaluations`, `Base::things`) the graph does not state (§ Results, *Not completed*); the same inheritance table supplies it | `Step::behavior = type->selectByKind(Behavior)` (KerML 8.3.4.6.3, deriveStepBehavior) |
| 35 | `FeatureReferenceExpression` | `directedFeature` | 2162 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::directedFeature`, the `feature`s whose `FeatureMembership` has a direction (KerML 8.3.3.1, deriveTypeDirectedFeature) |
| 36 | `FeatureReferenceExpression` | `feature` | 2162 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::feature = featureMembership.ownedMemberFeature` (KerML 8.3.3.1, deriveTypeFeature) |
| 37 | `FeatureReferenceExpression` | `featureMembership` | 2162 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::featureMembership = ownedFeatureMembership->union(inheritedMembership->selectByKind(FeatureMembership))` (KerML 8.3.3.1; KERML11-191 restricts the inherited part to the supertypes) |
| 38 | `FeatureReferenceExpression` | `inheritedFeature` | 2162 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::inheritedFeature = inheritedMembership.memberElement->selectByKind(Feature)` (KerML 8.3.3.1, deriveTypeInheritedFeature) |
| 39 | `FeatureReferenceExpression` | `inheritedMembership` | 2162 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Type::inheritedMembership = inheritedMemberships(excluded)` over the supertypes (KerML 8.3.3.1, deriveTypeInheritedMembership) |
| 40 | `FeatureReferenceExpression` | `isModelLevelEvaluable` | 2162 | 0 | deferred: `modelLevelEvaluable()` needs the expression's function and argument classification (§ Results, *Not completed*) | `Expression::isModelLevelEvaluable = modelLevelEvaluable(Set{})` (KerML 8.3.4.7.3, deriveExpressionIsModelLevelEvaluable) |
| 41 | `FeatureReferenceExpression` | `member` | 2162 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Namespace::member = membership.memberElement` (KerML 8.3.2.4, deriveNamespaceMember) |
| 42 | `FeatureReferenceExpression` | `membership` | 2162 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Namespace::membership = ownedMembership->union(importedMembership)`, and for a Type also `inheritedMembership` (KerML 8.3.2.4, 8.3.3.1) |
| 43 | `FeatureReferenceExpression` | `output` | 2162 | 0 | deferred here (§ Results, *Not completed*): needs the inherited members of every general type, which the structure-mode evaluator cannot see — the semantic inheritance table (`internal/semantic/semantics` `Model.MembersOf`) has to be given to the exporter | `Step::output`, the `directedFeature`s whose direction is `out` or `inout` (KerML 8.3.4.6.3), over the inherited features |
| 44 | `FeatureReferenceExpression` | `type` | 2162 | 0 | open: the derivation reaches the implied library base (`Performances::evaluations`, `Base::things`) the graph does not state (§ Results, *Not completed*); the same inheritance table supplies it | `Feature::type = typing.type` with the types inherited through chaining and redefinition (KerML 8.3.3.3, deriveFeatureType) |
| 45 | `Feature` | `featureTarget` | 21 | 131 | completed here (§ Results); the rows left are the positional or implied memberships that record adjudicates | `Feature::featureTarget`: the last `chainingFeature`, else the feature itself (KerML 8.3.3.3, deriveFeatureFeatureTarget; KERML11-1 crossFeature) |
| 46 | `Feature` | `featuringType` | 101 | 20 | completed here (§ Results); the rows left are the positional or implied memberships that record adjudicates | `Feature::featuringType = typeFeaturing.featuringType`, plus the owning type of an owned feature (KerML 8.3.3.3, deriveFeatureFeaturingType; KERML11-3/4 multiplicity featuring) |
| 47 | `Feature` | `owningFeatureMembership` | 3 | 221 | completed here (§ Results); the rows left are the positional or implied memberships that record adjudicates | `Feature::owningFeatureMembership = owningMembership->selectByKind(FeatureMembership)` (KerML 8.3.3.3) |
| 48 | `Feature` | `owningType` | 3 | 0 | completed here (§ Results); the rows left are the positional or implied memberships that record adjudicates | `Feature::owningType = owningFeatureMembership.owningType` (KerML 8.3.3.3, deriveFeatureOwningType) |
| 49 | `FeatureMembership` | `memberName` | 365 | 4 | closed for a named member — the conversion states `memberName` as the member's effective name; the 362 left are implied parameters the pilot names by the library feature they implicitly redefine | `Membership::memberName = memberElement.name` (KerML 8.3.2.4.3, deriveMembershipMemberName) |
| 50 | `FeatureMembership` | `ownedMemberName` | 182 | 4 | open: the pilot names an implied parameter by the library feature it implicitly redefines (`result`, `source`, `logical`); our effective name follows a declared redefinition only (§ Results, step 2) — needs the implied-redefinition table | `OwningMembership::ownedMemberName = ownedMemberElement.name` (KerML 8.3.2.3.6) |
| 51 | `Feature` | `owningNamespace` | 6 | 0 | completed here (§ Results); the rows left are the positional or implied memberships that record adjudicates | `Element::owningNamespace = owningMembership.membershipOwningNamespace` (KerML 8.3.2.1, deriveElementOwningNamespace) |
| 52 | `Redefinition` | `redefinedFeature` | 52 | 20 | closed for this repository's libraries (above) | `Redefinition::redefinedFeature : Feature` (KerML 8.3.3.3.8) is an element reference, and every `Element` carries an `elementId` (KerML 8.3.2.1.2) the API's JSON refers to it by; the 330 redefinitions of a feature of this repository's libraries once spelled as a name, `{"@ref": "caption"}`, now link by the element's fixed id — the same movement for `FeatureTyping::type`, `MetadataUsage::type`, `FeatureReferenceExpression::referent` and `ReferenceSubsetting::referencedFeature` into those libraries — and the 58 left are redefinitions of a feature inherited within the model itself that the writer spells as a name (`vehicle_b_1::providePower`) |
| 53 | `AttributeUsage` | `ownedRelationship` | 1 | 15 | none found | `Element::ownedRelationship : Relationship [0..*] {ordered}` (KerML 8.3.2.1.2), in the order the notation adds them (KerML 8.2.1 Table 3; `FeatureSpecializationPart`, then `ValuePart`, then the body): a head's typings, specializations and multiplicity as spelled, then its `FeatureValue`, then its members. The writer seated the materialized relationships last, in a fixed kind order; it now owns them as spelled, which moved 4861 properties over 101 rows (67 `ownedRelationship`, 22 `ownedSpecialization`, 12 `ownedSubsetting`) from present-different to present-equal, no other bucket touched (at that re-record this row 2332 → 1197, `PartDefinition` 215 → 20, `PartUsage` 1554 → 1004, `Feature` 1802 → 1401; the current baseline, with the re-pinned libraries and the examples that say `tolerate`, reads 1245, 20, 1054 and 1535). What remains (1245 on this row) is a different shape: on a usage that writes no multiplicity part (`attribute cargoMass : MassValue;`) the pilot owns an `OwningMembership` of a `Multiplicity`, not implied, after the `FeatureTyping`; ours states none |

Rows an evaluator rule in PR #1232 (`internal/semantic/metamodel/rules.go`) already names are marked; that PR adds a lazy metamodel evaluator and layers its derived values into the API-JSON writer, so its merge is expected to move those rows from ours-missing to present-equal or present-different, and this referee is how that movement is measured.
