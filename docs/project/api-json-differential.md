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

The pinned pilot is `2026-08` (`jupyter-sysml-kernel 0.62.0`), the artifact the reference
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
<files>` (the library defaults to the pinned one). The bridge is the pilot's own export path, the
one the Jupyter kernel's `%export` magic runs (`SysMLInteractive.export`):

- the files of one corpus root are read into one resource set with the standard library
  (`org.omg.sysml.io.SysMLUtil`, the KerML and SysML Xtext setups both registered, so `.kerml` and
  `.sysml` inputs alike resolve), then `transformAll(true)` adds the implied relationships the
  pilot's validation adds, as `%export` runs after validation;
- each input resource's root elements are visited by
  `org.omg.sysml.util.traversal.facade.impl.JsonElementProcessingFacade` with
  `setIsIncludeDerived(true)` through `org.omg.sysml.util.traversal.Traversal`, and the tree is
  `toJsonTree(true)` — the exact calls `SysMLInteractive.export` makes; standard-library elements
  are referenced, never written, as in the pilot;
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

A positional name this repository's writer spells for an unnamed element (`Demo::@1`,
`Legacy::@4::@trigger`) is not a name — the metamodel derives no `qualifiedName` for an unnamed
element — so such an element is keyed structurally and its stated `qualifiedName` is measured
against the pilot's null. Identical copies of one element on a side are one element (the pilot
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
property, so it is kept apart from the four the scorecard ranks). The row is named by the pilot's
metaclass when the two sides disagree on the metaclass (`typeMismatch`).

## Detectors

- **Determinism.** The report is sorted throughout (roots in order, elements by key, properties by
  name, rows by differing count then name); two runs over one checkout write byte-identical reports
  but for the recording date, which is what `-check` verifies in the oracle-reproduction workflow
  and what `TestReportIsDeterministic` pins without Java.
- **Disagreement.** `TestDisagreementDetectorIsLive` compares the committed pilot export of
  `tools/referee/apijson/testdata/probe.sysml` with itself (no row differs), then with a scratch copy
  in which one derived collection of `Derived Probe::Vehicle` is emptied (`ownedUsage` → exactly one
  ours-missing in that row, nowhere else) and one in which one attribute is flipped (`isAbstract` →
  exactly one present-different), each with one example naming the element.
- **Provenance.** The baseline's `provenance` block names both sides of the comparison: the pilot
  pin (`pilotTag`, `pilotCommit`, `pilotArtifact`), the errata-registry digest, one `tools` entry
  per side and every root's file digest. `pilot-json-exporter` digests the bridge source
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

## Results (pilot `2026-08`, 920 files)

**Scorecard:** 920 files; 86974 elements matched (29497 ours-only, 40387 pilot-only, 0 of them implied); properties: 596664 present-equal, 47633 present-different, 1021444 ours-missing, 65920 pilot-missing, 2020649 absent-both, 20867 indeterminate.

The pilot's export failed on 59 files (`pilotFailed`: its own export throws on an element without a resource, 53 of them specification examples; the files are named in the run's text report), ours refused 40 (all of them specification examples the parser could not read whole), left the pilot's 59 failed files out and converted 24 documents alone for declaring an element another document of their root declares (`oursColliding`); no root fell back to per-document conversion. 11311 (metaclass, property) rows carry a comparison; 100645 references compared through the unkeyed sentinel, 140342 of our `sysx:` annotations were left out, and 289/345 elements (ours/pilot) were dropped for ambiguous keys, 8198/32520 for having none.

| root | files | matched | ours-only | pilot-only | present-equal | present-different | ours-missing | pilot-missing |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `examples/sysml-v2-training` | 100 | 7287 | 2131 | 2873 | 50135 | 4171 | 86942 | 5306 |
| `examples/pilot-corpora/sysml-examples` | 99 | 25778 | 10038 | 10488 | 180567 | 11831 | 308696 | 19510 |
| `examples/pilot-corpora/sysml-validation` | 56 | 9246 | 2967 | 3341 | 63000 | 6100 | 108814 | 7092 |
| `examples/pilot-corpora/kerml-examples` | 58 | 6387 | 1637 | 1854 | 43671 | 3292 | 63793 | 5268 |
| `examples/spec-examples` | 511 | 6768 | 1951 | 2809 | 42871 | 5918 | 68945 | 5029 |
| `tests/testdata` | 32 | 1201 | 91 | 487 | 7818 | 393 | 13183 | 879 |
| `examples` | 52 | 23379 | 9363 | 16009 | 162339 | 11623 | 285333 | 17891 |
| `tools/referee/diff/testdata` | 4 | 53 | 0 | 0 | 346 | 17 | 462 | 43 |
| `docs/manual/examples` | 8 | 6663 | 1304 | 2607 | 44506 | 4172 | 82635 | 4749 |

The scorecard is mostly **ours-missing**: this repository's writer emits a subset of the metamodel's properties, and the referee now measures which, per metaclass. The full ranked scorecard with one concrete example per differing row is `build/pilot-apijson-diff/pilot-apijson-diff.txt` after a run; the baseline keeps every row's five counts.

### Where the two sides state different values

The largest **present-different** rows, with the reading the examples support:

| metaclass | property | present-equal | present-different |
|---|---|---:|---:|
| `AttributeUsage` | `ownedRelationship` | 113 | 2047 |
| `FeatureReferenceExpression` | `ownedMembership` | 31 | 2031 |
| `FeatureReferenceExpression` | `ownedRelationship` | 31 | 2031 |
| `Feature` | `ownedRelationship` | 3640 | 1781 |
| `PartUsage` | `ownedRelationship` | 303 | 1514 |
| `FeatureMembership` | `memberElement` | 8714 | 882 |
| `FeatureMembership` | `ownedMemberElement` | 8714 | 882 |
| `FeatureMembership` | `ownedMemberFeature` | 8714 | 882 |
| `FeatureMembership` | `ownedRelatedElement` | 8714 | 882 |
| `ReferenceUsage` | `qualifiedName` | 950 | 829 |

### Where the pilot states nothing and we do

| metaclass | property | pilot-missing | clause |
|---|---|---:|---|
| `FeatureMembership` | `owner` | 10447 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `OwningMembership` | `owner` | 9193 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `FeatureValue` | `owner` | 6646 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `FeatureTyping` | `owner` | 5634 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `ReturnParameterMembership` | `owner` | 4634 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `ParameterMembership` | `owner` | 4347 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `Membership` | `owner` | 3063 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `AttributeUsage` | `value` | 1381 | see the `derive*` row for this property in `docs/project/constraint-manifest.json` |

`owner` on every relationship is the pilot leaving `Relationship::owner` null where the metamodel derives `owningRelatedElement`; ours states it. The remaining pilot-missing rows are properties this repository writes outside the metamodel's spelling (`value` on usages) or states for unnamed elements the metamodel leaves null (`qualifiedName`).

### Where the pilot cannot name a member

A member that takes its name from a naming feature is keyed by that name on our side: KerML
`Feature::effectiveName` is the effective name of `namingFeature()` when nothing is declared
(8.3.3.3.4 Feature, `body: if declaredShortName <> null or declaredName <> null then declaredName
else … namingFeature.effectiveName()`), and the default `namingFeature()` is
`ownedRedefinition->at(1).redefinedFeature`; SysML redefines it for a variant (8.3.6.4 Usage,
`ownedReferenceSubsetting.referencedFeature`), a transition payload (8.3.6.3 ReferenceUsage), a
performed action (8.3.17.14 PerformActionUsage, `performedAction`), a requirement constraint
(8.3.20.4 ConstraintUsage) and a view rendering (8.3.26.6 RenderingUsage). The pilot derives the
same name — when it resolves the naming feature. It cannot resolve a feature of this repository's
own libraries (`internal/workspace/libs/stdlib/OpenSysML Libraries/`, `DocumentQueries` among
them): the bridge loads the pinned `sysml.library` alone, so `attribute redefines title = "…";` in a
`Section` leaves the pilot's `Redefinition` pointing at nothing, its `name` and `qualifiedName`
null, and its key structural (`…::coverage/AttributeUsage[0]`) where ours is `…::coverage::title`.

Naming members by their naming feature moved `docs/manual/examples` from 6834 to 6663 matched
elements while every other root gained: 357 matches lost, 186 gained. The 357 are 87 members —
84 `AttributeUsage` and 3 `ReferenceUsage`, each an `attribute redefines caption|title|text|term|description|kind|direction|style|groupBy = …`
or `ref redefines source|target = …` of a `DocumentQueries` feature — and the 270 elements they own
(their `Redefinition`, `FeatureValue` and literal or `FeatureReferenceExpression` value); every one
is now pilot-only under its structural key, while our named key is ours-only. The 186 gained are
46 `attribute :>> x = …` redefinitions of features the model itself declares, with the 138 elements
they own, plus `render asInterconnectionDiagram;` and its `ReferenceSubsetting`. Both sides state
what the spec derives from what they resolved; the loss measures the pilot's library coverage, not
a derivation, and is a limitation of this referee: a member whose naming feature the pilot does not
resolve cannot match. The same shape is the whole of what the other roots lost, masked there by
their gains: `examples` lost 914 (2805 gained) — 205 such redefinitions of `DocumentQueries` and
`AnalysisRecords` features and two `render` members of `OpenSysMLRenderings` renderings, with
what they own — and `spec-examples` lost 57 (171 gained): 13 members whose naming feature the
pinned excerpt leaves undeclared (`perform VehicleActions::move`, `individual timeslice redefines
preflight`, `render asMyTreeDiagram` under an undefined view definition, …), which we name by the
reference as written and the pilot, resolving nothing, does not name. `training`, `pilot-examples`,
`pilot-validation`, `kerml-examples` and `tests/testdata` lost no match. A run's own list of such
movements comes from `-lost-against`.

## Launch-ready gaps

The (metaclass, property) rows ranked by the corpus elements on which the pilot states a value and we state nothing (**ours-missing**), each with the derivation clause to implement and the in-flight work that already covers it, so that each row can be handed on as it stands. Where a property is an owned attribute with a default rather than a derivation, the row says so: the gap is in the writer, not in a derivation. The missing derivations are not implemented here.

| # | metaclass | property | ours-missing | present-different | in-flight overlap | derivation clause |
|---:|---|---|---:|---:|---|---|
| 1 | `FeatureMembership` | `isLibraryElement` | 10467 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 2 | `FeatureMembership` | `memberElementId` | 10467 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 3 | `FeatureMembership` | `ownedMemberElementId` | 10467 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 4 | `FeatureMembership` | `relatedElement` | 10467 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 5 | `FeatureMembership` | `source` | 10467 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 6 | `FeatureMembership` | `target` | 10467 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 7 | `FeatureMembership` | `visibility` | 10403 | 1 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 8 | `Feature` | `featureTarget` | 9905 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::featureTarget`: the last `chainingFeature`, else the feature itself (KerML 8.3.3.3, deriveFeatureFeatureTarget; KERML11-1 crossFeature) |
| 9 | `Feature` | `inheritedFeature` | 9905 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::inheritedFeature = inheritedMembership.memberElement->selectByKind(Feature)` (KerML 8.3.3.1, deriveTypeInheritedFeature) |
| 10 | `Feature` | `inheritedMembership` | 9905 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::inheritedMembership = inheritedMemberships(excluded)` over the supertypes (KerML 8.3.3.1, deriveTypeInheritedMembership) |
| 11 | `Feature` | `isConjugated` | 9905 | 0 | none found | `Type::isConjugated = ownedConjugator <> null` (KerML 8.3.3.1 Type; a derived attribute without a named derive constraint); the pilot writes `false` on every type |
| 12 | `Feature` | `isLibraryElement` | 9905 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 13 | `Feature` | `member` | 9905 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Namespace::member = membership.memberElement` (KerML 8.3.2.4, deriveNamespaceMember) |
| 14 | `Feature` | `membership` | 9905 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Namespace::membership = ownedMembership->union(importedMembership)`, and for a Type also `inheritedMembership` (KerML 8.3.2.4, 8.3.3.1) |
| 15 | `Feature` | `feature` | 9899 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::feature = featureMembership.ownedMemberFeature` (KerML 8.3.3.1, deriveTypeFeature) |
| 16 | `Feature` | `featureMembership` | 9899 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::featureMembership = ownedFeatureMembership->union(inheritedMembership->selectByKind(FeatureMembership))` (KerML 8.3.3.1; KERML11-191 restricts the inherited part to the supertypes) |
| 17 | `Feature` | `featuringType` | 9626 | 2 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::featuringType = typeFeaturing.featuringType`, plus the owning type of an owned feature (KerML 8.3.3.3, deriveFeatureFeaturingType; KERML11-3/4 multiplicity featuring) |
| 18 | `Feature` | `owningFeatureMembership` | 9345 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningFeatureMembership = owningMembership->selectByKind(FeatureMembership)` (KerML 8.3.3.3) |
| 19 | `Feature` | `owningType` | 9345 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningType = owningFeatureMembership.owningType` (KerML 8.3.3.3, deriveFeatureOwningType) |
| 20 | `Feature` | `type` | 9266 | 120 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::type = typing.type` with the types inherited through chaining and redefinition (KerML 8.3.3.3, deriveFeatureType) |
| 21 | `OwningMembership` | `isLibraryElement` | 9220 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 22 | `OwningMembership` | `relatedElement` | 9220 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 23 | `OwningMembership` | `source` | 9220 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 24 | `OwningMembership` | `memberElementId` | 9217 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 25 | `OwningMembership` | `ownedMemberElementId` | 9217 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 26 | `OwningMembership` | `target` | 9217 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 27 | `OwningMembership` | `visibility` | 9195 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 28 | `Feature` | `name` | 8817 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Element::name = effectiveName()`: the declared name, or for a feature the name of the feature it redefines (KerML 8.3.2.1, deriveElementName) |
| 29 | `FeatureMembership` | `memberName` | 8676 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberName = memberElement.name` (KerML 8.3.2.3.5, deriveMembershipMemberName) |
| 30 | `FeatureMembership` | `ownedMemberName` | 8676 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberName = ownedMemberElement.name` (KerML 8.3.2.3.6) |
| 31 | `Feature` | `owningNamespace` | 8586 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Element::owningNamespace = owningMembership.membershipOwningNamespace` (KerML 8.3.2.1, deriveElementOwningNamespace) |
| 32 | `FeatureValue` | `isLibraryElement` | 6661 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 33 | `FeatureValue` | `memberElementId` | 6661 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 34 | `FeatureValue` | `ownedMemberElementId` | 6661 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 35 | `FeatureValue` | `relatedElement` | 6661 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 36 | `FeatureValue` | `source` | 6661 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 37 | `FeatureValue` | `target` | 6661 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 38 | `FeatureValue` | `visibility` | 6661 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 39 | `FeatureTyping` | `isLibraryElement` | 5646 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 40 | `FeatureTyping` | `owningType` | 5642 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningType = owningFeatureMembership.owningType` (KerML 8.3.3.3, deriveFeatureOwningType) |

Rows an evaluator rule in PR #1232 (`internal/semantic/metamodel/rules.go`) already names are marked; that PR adds a lazy metamodel evaluator and layers its derived values into the API-JSON writer, so its merge is expected to move those rows from ours-missing to present-equal or present-different, and this referee is how that movement is measured.

