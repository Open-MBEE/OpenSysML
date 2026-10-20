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
metaclass and the pilot's diagnostics) and `pilot-apijson-diff-examples.json`.

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
- **Provenance.** `TestCommittedBaselineStatesThisRepositorysProvenance` checks, without Java, that
  the baseline's pin, bridge-source digest and every root's file digest are this checkout's, and
  `TestProvenanceGuardFailsOnAMovedPin` that a moved pin is named with the refresh command.
  `TestRecordStatesTheBaselinesFigures` (`make docs-counts`) ties this record's figures to the
  baseline.

The pinned pilot's verdict on the probe (`validate-sysml-batch`): clean.

## Results (pilot `2026-08`, 407 files)

**Scorecard:** 407 files; 67274 elements matched (40208 ours-only, 50337 pilot-only, 0 of them implied); properties: 450166 present-equal, 40094 present-different, 801066 ours-missing, 50995 pilot-missing, 1568678 absent-both, 19050 indeterminate.

The pilot exported every file but 6 (`pilotFailed`: its own export throws on an element without a resource; the files are named in the run's text report), ours refused 0, left the pilot's 6 failed files out and converted 22 documents alone for declaring an element another document of their root declares (`oursColliding`); no root fell back to per-document conversion. 10815 (metaclass, property) rows carry a comparison; 78684 references compared through the unkeyed sentinel, 110175 of our `sysx:` annotations were left out, and 290/185 elements (ours/pilot) were dropped for ambiguous keys, 8129/30333 for having none.

| root | files | matched | ours-only | pilot-only | present-equal | present-different | ours-missing | pilot-missing |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `examples/sysml-v2-training` | 100 | 6097 | 3321 | 4063 | 40825 | 4002 | 72464 | 4393 |
| `examples/pilot-corpora/sysml-examples` | 99 | 19299 | 16517 | 16967 | 131395 | 11216 | 232130 | 14554 |
| `examples/pilot-corpora/sysml-validation` | 56 | 6997 | 5216 | 5590 | 45934 | 5589 | 81691 | 5351 |
| `examples/pilot-corpora/kerml-examples` | 58 | 5399 | 2618 | 2842 | 34810 | 3546 | 55283 | 4352 |
| `tests/testdata` | 30 | 1124 | 132 | 522 | 7242 | 319 | 12288 | 818 |
| `examples` | 52 | 21488 | 11254 | 17900 | 144295 | 12096 | 262173 | 16491 |
| `tools/referee/diff/testdata` | 4 | 36 | 17 | 17 | 224 | 19 | 249 | 32 |
| `docs/manual/examples` | 8 | 6834 | 1133 | 2436 | 45441 | 3307 | 84788 | 5004 |

The scorecard is mostly **ours-missing**: this repository's writer emits a subset of the metamodel's properties, and the referee now measures which, per metaclass. The full ranked scorecard with one concrete example per differing row is `build/pilot-apijson-diff/pilot-apijson-diff.txt` after a run; the baseline keeps every row's five counts.

### Where the two sides state different values

The largest **present-different** rows, with the reading the examples support:

| metaclass | property | present-equal | present-different |
|---|---|---:|---:|
| `FeatureMembership` | `memberElement` | 6542 | 1971 |
| `FeatureMembership` | `ownedMemberElement` | 6542 | 1971 |
| `FeatureMembership` | `ownedMemberFeature` | 6542 | 1971 |
| `FeatureMembership` | `ownedRelatedElement` | 6542 | 1971 |
| `FeatureTyping` | `general` | 2971 | 5 |
| `FeatureTyping` | `relatedElement` | 2971 | 5 |
| `FeatureTyping` | `target` | 2971 | 5 |
| `FeatureTyping` | `type` | 2973 | 5 |
| `AttributeUsage` | `ownedRelationship` | 102 | 1571 |
| `FeatureReferenceExpression` | `ownedMembership` | 29 | 1458 |

### Where the pilot states nothing and we do

| metaclass | property | pilot-missing | clause |
|---|---|---:|---|
| `FeatureMembership` | `owner` | 9270 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `OwningMembership` | `owner` | 7328 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `FeatureValue` | `owner` | 4929 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `FeatureTyping` | `owner` | 4863 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `ReturnParameterMembership` | `owner` | 3469 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `ParameterMembership` | `owner` | 3467 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `Membership` | `owner` | 2304 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `AttributeUsage` | `value` | 1000 | see the `derive*` row for this property in `docs/project/constraint-manifest.json` |

`owner` on every relationship is the pilot leaving `Relationship::owner` null where the metamodel derives `owningRelatedElement`; ours states it. The remaining pilot-missing rows are properties this repository writes outside the metamodel's spelling (`value` on usages) or states for unnamed elements the metamodel leaves null (`qualifiedName`).

## Launch-ready gaps

The (metaclass, property) rows ranked by the corpus elements on which the pilot states a value and we state nothing (**ours-missing**), each with the derivation clause to implement and the in-flight work that already covers it, so that each row can be handed on as it stands. Where a property is an owned attribute with a default rather than a derivation, the row says so: the gap is in the writer, not in a derivation. The missing derivations are not implemented here.

| # | metaclass | property | ours-missing | present-different | in-flight overlap | derivation clause |
|---:|---|---|---:|---:|---|---|
| 1 | `FeatureMembership` | `isLibraryElement` | 9270 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 2 | `FeatureMembership` | `memberElementId` | 9270 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 3 | `FeatureMembership` | `ownedMemberElementId` | 9270 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 4 | `FeatureMembership` | `relatedElement` | 9270 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 5 | `FeatureMembership` | `source` | 9270 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 6 | `FeatureMembership` | `target` | 9270 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 7 | `FeatureMembership` | `visibility` | 9210 | 1 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 8 | `FeatureMembership` | `memberName` | 7660 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberName = memberElement.name` (KerML 8.3.2.3.5, deriveMembershipMemberName) |
| 9 | `FeatureMembership` | `ownedMemberName` | 7660 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberName = ownedMemberElement.name` (KerML 8.3.2.3.6) |
| 10 | `OwningMembership` | `isLibraryElement` | 7328 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 11 | `OwningMembership` | `memberElementId` | 7328 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 12 | `OwningMembership` | `ownedMemberElementId` | 7328 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 13 | `OwningMembership` | `relatedElement` | 7328 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 14 | `OwningMembership` | `source` | 7328 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 15 | `OwningMembership` | `target` | 7328 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 16 | `OwningMembership` | `visibility` | 7309 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 17 | `Feature` | `featureTarget` | 7307 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::featureTarget`: the last `chainingFeature`, else the feature itself (KerML 8.3.3.3, deriveFeatureFeatureTarget; KERML11-1 crossFeature) |
| 18 | `Feature` | `inheritedFeature` | 7307 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::inheritedFeature = inheritedMembership.memberElement->selectByKind(Feature)` (KerML 8.3.3.1, deriveTypeInheritedFeature) |
| 19 | `Feature` | `inheritedMembership` | 7307 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::inheritedMembership = inheritedMemberships(excluded)` over the supertypes (KerML 8.3.3.1, deriveTypeInheritedMembership) |
| 20 | `Feature` | `isConjugated` | 7307 | 0 | none found | `Type::isConjugated = ownedConjugator <> null` (KerML 8.3.3.1 Type; a derived attribute without a named derive constraint); the pilot writes `false` on every type |
| 21 | `Feature` | `isLibraryElement` | 7307 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 22 | `Feature` | `member` | 7307 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Namespace::member = membership.memberElement` (KerML 8.3.2.4, deriveNamespaceMember) |
| 23 | `Feature` | `membership` | 7307 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Namespace::membership = ownedMembership->union(importedMembership)`, and for a Type also `inheritedMembership` (KerML 8.3.2.4, 8.3.3.1) |
| 24 | `Feature` | `feature` | 7302 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::feature = featureMembership.ownedMemberFeature` (KerML 8.3.3.1, deriveTypeFeature) |
| 25 | `Feature` | `featureMembership` | 7302 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::featureMembership = ownedFeatureMembership->union(inheritedMembership->selectByKind(FeatureMembership))` (KerML 8.3.3.1; KERML11-191 restricts the inherited part to the supertypes) |
| 26 | `Feature` | `featuringType` | 7108 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::featuringType = typeFeaturing.featuringType`, plus the owning type of an owned feature (KerML 8.3.3.3, deriveFeatureFeaturingType; KERML11-3/4 multiplicity featuring) |
| 27 | `Feature` | `owningFeatureMembership` | 6962 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningFeatureMembership = owningMembership->selectByKind(FeatureMembership)` (KerML 8.3.3.3) |
| 28 | `Feature` | `owningType` | 6962 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningType = owningFeatureMembership.owningType` (KerML 8.3.3.3, deriveFeatureOwningType) |
| 29 | `Feature` | `type` | 6936 | 7 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::type = typing.type` with the types inherited through chaining and redefinition (KerML 8.3.3.3, deriveFeatureType) |
| 30 | `Feature` | `name` | 6583 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Element::name = effectiveName()`: the declared name, or for a feature the name of the feature it redefines (KerML 8.3.2.1, deriveElementName) |
| 31 | `Feature` | `owningNamespace` | 6577 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Element::owningNamespace = owningMembership.membershipOwningNamespace` (KerML 8.3.2.1, deriveElementOwningNamespace) |
| 32 | `FeatureValue` | `isLibraryElement` | 4929 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 33 | `FeatureValue` | `memberElementId` | 4929 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 34 | `FeatureValue` | `ownedMemberElementId` | 4929 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 35 | `FeatureValue` | `relatedElement` | 4929 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 36 | `FeatureValue` | `source` | 4929 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 37 | `FeatureValue` | `target` | 4929 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 38 | `FeatureValue` | `visibility` | 4929 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 39 | `FeatureTyping` | `isLibraryElement` | 4865 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 40 | `FeatureTyping` | `owningType` | 4863 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningType = owningFeatureMembership.owningType` (KerML 8.3.3.3, deriveFeatureOwningType) |

Rows an evaluator rule in PR #1232 (`internal/semantic/metamodel/rules.go`) already names are marked; that PR adds a lazy metamodel evaluator and layers its derived values into the API-JSON writer, so its merge is expected to move those rows from ours-missing to present-equal or present-different, and this referee is how that movement is measured.

