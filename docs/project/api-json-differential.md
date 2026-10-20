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
parser cannot read whole is refused and counted (`oursRefused`), as `sysml -convert` refuses it. A
root that does not convert as one model — `tests/testdata` holds two versions of one library
package — is converted document by document instead (`oursSeparate`, with any document the writer
then refuses under `oursUnconvertible`); its cross-file references stay unresolved and are measured
as such.

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
   elements; a reference no key addresses compares as the sentinel `?` and is counted
   (`unkeyedReferences`);
3. `elementId`, `memberElementId` and `ownedMemberElementId` hold ids as strings and are mapped
   through the same keys; `elementId` is identity, not derivation, and is not bucketed;
4. JSON null, a missing property and an empty collection all state nothing — *absent*;
5. a Boolean `false` against a property the other side leaves out is absent on both sides: false
   is every Boolean attribute's default, so neither side states anything; a `true` the other side
   lacks is missing;
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

**Scorecard:** 407 files; 66910 elements matched (42562 ours-only, 50701 pilot-only, 0 of them implied); properties: 445989 present-equal, 61801 present-different, 709745 ours-missing, 50782 pilot-missing, 1654480 absent-both.

The pilot exported every file but 6 (`pilotFailed`: its own export throws on an element without a resource; the files are named in the run's text report), ours refused 0 and converted 4 as separate documents. 10815 (metaclass, property) rows carry a comparison; 78684 references compared through the unkeyed sentinel, 110175 of our `sysx:` annotations were left out, and 290/185 elements (ours/pilot) were dropped for ambiguous keys, 8129/30333 for having none.

| root | files | matched | ours-only | pilot-only | present-equal | present-different | ours-missing | pilot-missing |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `examples/sysml-v2-training` | 100 | 6093 | 3319 | 4067 | 40172 | 6194 | 63871 | 4387 |
| `examples/pilot-corpora/sysml-examples` | 99 | 19097 | 16719 | 17169 | 130205 | 16657 | 204646 | 14447 |
| `examples/pilot-corpora/sysml-validation` | 56 | 6884 | 5329 | 5703 | 45269 | 7591 | 71342 | 5290 |
| `examples/pilot-corpora/kerml-examples` | 58 | 5399 | 2618 | 2842 | 34536 | 4701 | 49436 | 4352 |
| `tests/testdata` | 30 | 1122 | 154 | 524 | 7224 | 647 | 10805 | 817 |
| `examples` | 52 | 21474 | 13244 | 17914 | 143107 | 20753 | 233576 | 16470 |
| `tools/referee/diff/testdata` | 4 | 28 | 25 | 25 | 176 | 41 | 155 | 28 |
| `docs/manual/examples` | 8 | 6813 | 1154 | 2457 | 45300 | 5217 | 75914 | 4991 |

The scorecard is mostly **ours-missing**: this repository's writer emits a subset of the metamodel's properties, and the referee now measures which, per metaclass. The full ranked scorecard with one concrete example per differing row is `build/pilot-apijson-diff/pilot-apijson-diff.txt` after a run; the baseline keeps every row's five counts.

### Where the two sides state different values

The largest **present-different** rows, with the reading the examples support:

| metaclass | property | present-equal | present-different |
|---|---|---:|---:|
| `FeatureMembership` | `memberElement` | 6542 | 2728 |
| `FeatureMembership` | `ownedMemberElement` | 6542 | 2728 |
| `FeatureMembership` | `ownedMemberFeature` | 6542 | 2728 |
| `FeatureMembership` | `ownedRelatedElement` | 6542 | 2728 |
| `FeatureTyping` | `general` | 2686 | 2177 |
| `FeatureTyping` | `relatedElement` | 2686 | 2177 |
| `FeatureTyping` | `target` | 2686 | 2177 |
| `FeatureTyping` | `type` | 2688 | 2177 |
| `AttributeUsage` | `ownedRelationship` | 102 | 1571 |
| `FeatureReferenceExpression` | `ownedMembership` | 29 | 1458 |

### Where the pilot states nothing and we do

| metaclass | property | pilot-missing | clause |
|---|---|---:|---|
| `FeatureMembership` | `owner` | 9270 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `OwningMembership` | `owner` | 7151 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `FeatureValue` | `owner` | 4929 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `FeatureTyping` | `owner` | 4863 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `ReturnParameterMembership` | `owner` | 3469 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `ParameterMembership` | `owner` | 3458 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `Membership` | `owner` | 2304 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `AttributeUsage` | `value` | 1000 | see the `derive*` row for this property in `docs/project/constraint-manifest.json` |

`owner` on every relationship is the pilot leaving `Relationship::owner` null where the metamodel derives `owningRelatedElement`; ours states it. The remaining pilot-missing rows are properties this repository writes outside the metamodel's spelling (`value` on usages) or states for unnamed elements the metamodel leaves null (`qualifiedName`).

## Launch-ready gaps

The (metaclass, property) rows ranked by the corpus elements on which the pilot states a value and we state nothing (**ours-missing**), each with the derivation clause to implement and the in-flight work that already covers it, so that each row can be handed on as it stands. Where a property is an owned attribute with a default rather than a derivation, the row says so: the gap is in the writer, not in a derivation. The missing derivations are not implemented here.

| # | metaclass | property | ours-missing | present-different | in-flight overlap | derivation clause |
|---:|---|---|---:|---:|---|---|
| 1 | `FeatureMembership` | `memberElementId` | 9270 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 2 | `FeatureMembership` | `ownedMemberElementId` | 9270 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 3 | `FeatureMembership` | `relatedElement` | 9270 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 4 | `FeatureMembership` | `source` | 9270 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 5 | `FeatureMembership` | `target` | 9270 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 6 | `FeatureMembership` | `visibility` | 9210 | 1 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 7 | `FeatureMembership` | `memberName` | 7660 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberName = memberElement.name` (KerML 8.3.2.3.5, deriveMembershipMemberName) |
| 8 | `FeatureMembership` | `ownedMemberName` | 7660 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberName = ownedMemberElement.name` (KerML 8.3.2.3.6) |
| 9 | `Feature` | `featureTarget` | 7298 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::featureTarget`: the last `chainingFeature`, else the feature itself (KerML 8.3.3.3, deriveFeatureFeatureTarget; KERML11-1 crossFeature) |
| 10 | `Feature` | `inheritedFeature` | 7298 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::inheritedFeature = inheritedMembership.memberElement->selectByKind(Feature)` (KerML 8.3.3.1, deriveTypeInheritedFeature) |
| 11 | `Feature` | `inheritedMembership` | 7298 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::inheritedMembership = inheritedMemberships(excluded)` over the supertypes (KerML 8.3.3.1, deriveTypeInheritedMembership) |
| 12 | `Feature` | `member` | 7298 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Namespace::member = membership.memberElement` (KerML 8.3.2.4, deriveNamespaceMember) |
| 13 | `Feature` | `membership` | 7298 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Namespace::membership = ownedMembership->union(importedMembership)`, and for a Type also `inheritedMembership` (KerML 8.3.2.4, 8.3.3.1) |
| 14 | `Feature` | `isUnique` | 7294 | 0 | none found | owned attribute `Feature::isUnique : Boolean`, default `true` (KerML 8.3.3.3); not derived |
| 15 | `Feature` | `feature` | 7293 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::feature = featureMembership.ownedMemberFeature` (KerML 8.3.3.1, deriveTypeFeature) |
| 16 | `Feature` | `featureMembership` | 7293 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::featureMembership = ownedFeatureMembership->union(inheritedMembership->selectByKind(FeatureMembership))` (KerML 8.3.3.1; KERML11-191 restricts the inherited part to the supertypes) |
| 17 | `OwningMembership` | `memberElementId` | 7151 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 18 | `OwningMembership` | `ownedMemberElementId` | 7151 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 19 | `OwningMembership` | `relatedElement` | 7151 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 20 | `OwningMembership` | `source` | 7151 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 21 | `OwningMembership` | `target` | 7151 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 22 | `OwningMembership` | `visibility` | 7132 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 23 | `Feature` | `featuringType` | 7099 | 1 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::featuringType = typeFeaturing.featuringType`, plus the owning type of an owned feature (KerML 8.3.3.3, deriveFeatureFeaturingType; KERML11-3/4 multiplicity featuring) |
| 24 | `Feature` | `owningFeatureMembership` | 6953 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningFeatureMembership = owningMembership->selectByKind(FeatureMembership)` (KerML 8.3.3.3) |
| 25 | `Feature` | `owningType` | 6953 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningType = owningFeatureMembership.owningType` (KerML 8.3.3.3, deriveFeatureOwningType) |
| 26 | `Feature` | `type` | 6927 | 172 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::type = typing.type` with the types inherited through chaining and redefinition (KerML 8.3.3.3, deriveFeatureType) |
| 27 | `Feature` | `name` | 6574 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Element::name = effectiveName()`: the declared name, or for a feature the name of the feature it redefines (KerML 8.3.2.1, deriveElementName) |
| 28 | `Feature` | `owningNamespace` | 6568 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Element::owningNamespace = owningMembership.membershipOwningNamespace` (KerML 8.3.2.1, deriveElementOwningNamespace) |
| 29 | `FeatureValue` | `memberElementId` | 4929 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 30 | `FeatureValue` | `ownedMemberElementId` | 4929 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 31 | `FeatureValue` | `relatedElement` | 4929 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 32 | `FeatureValue` | `source` | 4929 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 33 | `FeatureValue` | `target` | 4929 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 34 | `FeatureValue` | `visibility` | 4929 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 35 | `FeatureTyping` | `owningType` | 4863 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningType = owningFeatureMembership.owningType` (KerML 8.3.3.3, deriveFeatureOwningType) |
| 36 | `OwningMembership` | `memberName` | 4292 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberName = memberElement.name` (KerML 8.3.2.3.5, deriveMembershipMemberName) |
| 37 | `OwningMembership` | `ownedMemberName` | 4292 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberName = ownedMemberElement.name` (KerML 8.3.2.3.6) |
| 38 | `Feature` | `ownedElement` | 3702 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Element::ownedElement = ownedRelationship.ownedRelatedElement` (KerML 8.3.2.1, deriveElementOwnedElement) |
| 39 | `Feature` | `ownedMember` | 3527 | 102 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Namespace::ownedMember = ownedMembership.ownedMemberElement` (KerML 8.3.2.4) |
| 40 | `ReturnParameterMembership` | `memberElementId` | 3469 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |

Rows an evaluator rule in PR #1232 (`internal/semantic/metamodel/rules.go`) already names are marked; that PR adds a lazy metamodel evaluator and layers its derived values into the API-JSON writer, so its merge is expected to move those rows from ours-missing to present-equal or present-different, and this referee is how that movement is measured.

