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

## Results (pilot `2026-08`, 930 files)

**Scorecard:** 930 files; 91817 elements matched (18396 ours-only, 37359 pilot-only, 0 of them implied); properties: 1113691 present-equal, 27125 present-different, 1066771 ours-missing, 67750 pilot-missing, 1959166 absent-both, 26051 indeterminate.

The pilot's export failed on 56 files (`pilotFailed`: its own export throws on an element without a resource, 52 of them specification examples and 4 test fixtures; the files are named in the run's text report), ours refused 36 (all of them specification examples the parser could not read whole), left the pilot's 56 failed files out and converted 26 documents alone for declaring an element another document of their root declares (`oursColliding`); no root fell back to per-document conversion. 11730 (metaclass, property) rows carry a comparison; 111008 references compared through the unkeyed sentinel, 158907 of our `sysx:` annotations were left out, and 285/347 elements (ours/pilot) were dropped for ambiguous keys, 16850/31316 for having none.

| root | files | matched | ours-only | pilot-only | present-equal | present-different | ours-missing | pilot-missing |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `examples/sysml-v2-training` | 100 | 7391 | 1150 | 2769 | 53381 | 1811 | 88043 | 5126 |
| `examples/pilot-corpora/sysml-examples` | 99 | 26739 | 6157 | 9527 | 193152 | 7517 | 318686 | 19458 |
| `examples/pilot-corpora/sysml-validation` | 56 | 9377 | 1882 | 3210 | 67082 | 3118 | 110547 | 6811 |
| `examples/pilot-corpora/kerml-examples` | 58 | 6399 | 1166 | 1842 | 44577 | 2504 | 63934 | 5065 |
| `examples/spec-examples` | 511 | 7074 | 990 | 2634 | 47298 | 1484 | 75012 | 5041 |
| `tests/testdata` | 36 | 1414 | 171 | 495 | 9277 | 401 | 15672 | 997 |
| `examples` | 52 | 26254 | 5956 | 14776 | 186183 | 7989 | 318978 | 19327 |
| `tools/referee/diff/testdata` | 4 | 53 | 0 | 0 | 362 | 1 | 462 | 41 |
| `docs/manual/examples` | 8 | 7020 | 920 | 2082 | 48509 | 1982 | 88453 | 4679 |

The scorecard is mostly **ours-missing**: this repository's writer emits a subset of the metamodel's properties, and the referee now measures which, per metaclass. The full ranked scorecard with one concrete example per differing row is `build/pilot-apijson-diff/pilot-apijson-diff.txt` after a run; the baseline keeps every row's five counts.

### Where the two sides state different values

The largest **present-different** rows, with the reading the examples support:

| metaclass | property | present-equal | present-different |
|---|---|---:|---:|
| `Feature` | `ownedRelationship` | 4159 | 1401 |
| `AttributeUsage` | `ownedRelationship` | 1248 | 1197 |
| `PartUsage` | `ownedRelationship` | 874 | 1004 |
| `PartUsage` | `ownedMember` | 368 | 702 |
| `PartUsage` | `ownedMembership` | 722 | 634 |
| `AttributeUsage` | `ownedMembership` | 1184 | 624 |
| `FeatureMembership` | `memberElement` | 9381 | 570 |
| `FeatureMembership` | `ownedMemberElement` | 9381 | 570 |
| `FeatureMembership` | `ownedRelatedElement` | 9381 | 570 |
| `Feature` | `ownedMembership` | 4271 | 569 |

### Where the pilot states nothing and we do

| metaclass | property | pilot-missing | clause |
|---|---|---:|---|
| `FeatureMembership` | `owner` | 10847 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `OwningMembership` | `owner` | 9596 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `FeatureValue` | `owner` | 7082 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `FeatureTyping` | `owner` | 5978 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `ReturnParameterMembership` | `owner` | 4764 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `ParameterMembership` | `owner` | 4410 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `Membership` | `owner` | 3201 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |
| `Redefinition` | `owner` | 2683 | `Element::owner = owningRelationship.owningRelatedElement`, and `Relationship::owner = owningRelatedElement` (KerML 8.3.2.1, 8.3.2.5.2) |

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
| 1 | `FeatureMembership` | `isLibraryElement` | 10854 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 2 | `FeatureMembership` | `memberElementId` | 10854 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 3 | `FeatureMembership` | `ownedMemberElementId` | 10854 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 4 | `FeatureMembership` | `relatedElement` | 10854 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 5 | `FeatureMembership` | `source` | 10854 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 6 | `FeatureMembership` | `target` | 10854 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 7 | `FeatureMembership` | `visibility` | 10790 | 1 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 8 | `Feature` | `featureTarget` | 10148 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::featureTarget`: the last `chainingFeature`, else the feature itself (KerML 8.3.3.3, deriveFeatureFeatureTarget; KERML11-1 crossFeature) |
| 9 | `Feature` | `inheritedFeature` | 10148 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::inheritedFeature = inheritedMembership.memberElement->selectByKind(Feature)` (KerML 8.3.3.1, deriveTypeInheritedFeature) |
| 10 | `Feature` | `inheritedMembership` | 10148 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::inheritedMembership = inheritedMemberships(excluded)` over the supertypes (KerML 8.3.3.1, deriveTypeInheritedMembership) |
| 11 | `Feature` | `isConjugated` | 10148 | 0 | none found | `Type::isConjugated = ownedConjugator <> null` (KerML 8.3.3.1 Type; a derived attribute without a named derive constraint); the pilot writes `false` on every type |
| 12 | `Feature` | `isLibraryElement` | 10148 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 13 | `Feature` | `member` | 10148 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Namespace::member = membership.memberElement` (KerML 8.3.2.4, deriveNamespaceMember) |
| 14 | `Feature` | `membership` | 10148 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Namespace::membership = ownedMembership->union(importedMembership)`, and for a Type also `inheritedMembership` (KerML 8.3.2.4, 8.3.3.1) |
| 15 | `Feature` | `feature` | 10142 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::feature = featureMembership.ownedMemberFeature` (KerML 8.3.3.1, deriveTypeFeature) |
| 16 | `Feature` | `featureMembership` | 10142 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Type::featureMembership = ownedFeatureMembership->union(inheritedMembership->selectByKind(FeatureMembership))` (KerML 8.3.3.1; KERML11-191 restricts the inherited part to the supertypes) |
| 17 | `Feature` | `featuringType` | 9846 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::featuringType = typeFeaturing.featuringType`, plus the owning type of an owned feature (KerML 8.3.3.3, deriveFeatureFeaturingType; KERML11-3/4 multiplicity featuring) |
| 18 | `Feature` | `type` | 9615 | 14 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::type = typing.type` with the types inherited through chaining and redefinition (KerML 8.3.3.3, deriveFeatureType) |
| 19 | `OwningMembership` | `isLibraryElement` | 9625 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 20 | `OwningMembership` | `relatedElement` | 9625 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 21 | `OwningMembership` | `source` | 9625 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 22 | `OwningMembership` | `memberElementId` | 9622 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 23 | `OwningMembership` | `ownedMemberElementId` | 9622 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 24 | `OwningMembership` | `target` | 9622 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 25 | `OwningMembership` | `visibility` | 9600 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 26 | `Feature` | `name` | 9538 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Element::name = effectiveName()`: the declared name, or for a feature the name of the feature it redefines (KerML 8.3.2.1, deriveElementName) |
| 27 | `Feature` | `owningFeatureMembership` | 9497 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningFeatureMembership = owningMembership->selectByKind(FeatureMembership)` (KerML 8.3.3.3) |
| 28 | `Feature` | `owningType` | 9497 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningType = owningFeatureMembership.owningType` (KerML 8.3.3.3, deriveFeatureOwningType) |
| 29 | `FeatureMembership` | `memberName` | 9305 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberName = memberElement.name` (KerML 8.3.2.3.5, deriveMembershipMemberName) |
| 30 | `FeatureMembership` | `ownedMemberName` | 9305 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberName = ownedMemberElement.name` (KerML 8.3.2.3.6) |
| 31 | `Feature` | `owningNamespace` | 8757 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Element::owningNamespace = owningMembership.membershipOwningNamespace` (KerML 8.3.2.1, deriveElementOwningNamespace) |
| 32 | `FeatureValue` | `isLibraryElement` | 7083 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 33 | `FeatureValue` | `memberElementId` | 7083 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Membership::memberElementId = memberElement.elementId` (KerML 8.3.2.3.5, deriveMembershipMemberElementId) |
| 34 | `FeatureValue` | `ownedMemberElementId` | 7083 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `OwningMembership::ownedMemberElementId = ownedMemberElement.elementId` (KerML 8.3.2.3.6) |
| 35 | `FeatureValue` | `relatedElement` | 7083 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::relatedElement = source->union(target)`, ordered (KerML 8.3.2.5.2, deriveRelationshipRelatedElement) |
| 36 | `FeatureValue` | `source` | 7083 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::source`, which Membership redefines as `[membershipOwningNamespace]` (KerML 8.3.2.3.5) |
| 37 | `FeatureValue` | `target` | 7083 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Relationship::target`, which Membership redefines as `[memberElement]` (KerML 8.3.2.3.5) |
| 38 | `FeatureValue` | `visibility` | 7083 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | owned attribute `Membership::visibility : VisibilityKind`, default `public` (KerML 8.3.2.3.5); not derived |
| 39 | `FeatureTyping` | `isLibraryElement` | 5994 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`; writes `true` on library elements only, never `false`) | `Element::isLibraryElement = libraryNamespace() <> null` (KerML 8.3.2.1, deriveElementIsLibraryElement); the pilot writes `false` on every user element |
| 40 | `FeatureTyping` | `owningType` | 5990 | 0 | PR #1232 (`feature/api-json-derived-properties-layered`) | `Feature::owningType = owningFeatureMembership.owningType` (KerML 8.3.3.3, deriveFeatureOwningType) |
| 41 | `Redefinition` | `redefinedFeature` | 52 | 20 | closed for this repository's libraries (above) | `Redefinition::redefinedFeature : Feature` (KerML 8.3.3.3.8) is an element reference, and every `Element` carries an `elementId` (KerML 8.3.2.1.2) the API's JSON refers to it by; the 330 redefinitions of a feature of this repository's libraries once spelled as a name, `{"@ref": "caption"}`, now link by the element's fixed id — the same movement for `FeatureTyping::type`, `MetadataUsage::type`, `FeatureReferenceExpression::referent` and `ReferenceSubsetting::referencedFeature` into those libraries — and the 58 left are redefinitions of a feature inherited within the model itself that the writer spells as a name (`vehicle_b_1::providePower`) |
| 42 | `AttributeUsage` | `ownedRelationship` | 36 | 1197 | none found | `Element::ownedRelationship : Relationship [0..*] {ordered}` (KerML 8.3.2.1.2), in the order the notation adds them (KerML 8.2.1 Table 3; `FeatureSpecializationPart`, then `ValuePart`, then the body): a head's typings, specializations and multiplicity as spelled, then its `FeatureValue`, then its members. The writer seated the materialized relationships last, in a fixed kind order; it now owns them as spelled, which moved 4861 properties over 101 rows (67 `ownedRelationship`, 22 `ownedSpecialization`, 12 `ownedSubsetting`) from present-different to present-equal, no other bucket touched (this row 2332 → 1197, `PartDefinition` 215 → 20, `PartUsage` 1554 → 1004, `Feature` 1802 → 1401). What remains (1197) is a different shape: on a usage that writes no multiplicity part (`attribute cargoMass : MassValue;`) the pilot owns an `OwningMembership` of a `Multiplicity`, not implied, after the `FeatureTyping`; ours states none |
| 43 | `FeatureReferenceExpression` | `ownedMembership` | 0 | 7 | the writer owns the `Membership` (`export/rdf_expr.go` `featureReferent`); the 7 left are the `FunctionReferenceMember` shape, a `FeatureMembership` owning an `Expression` typed by the named function (KerML 8.2.5.8.3), which ours writes as a `Membership` of the function | `FeatureReferenceExpression::referent` is the `memberElement` of the first `ownedMembership` that is not a `ParameterMembership` (KerML 8.3.4.8.5, deriveFeatureReferenceExpressionReferent); the notation maps a feature reference to that owned `Membership` (8.2.5.8.3, `FeatureReferenceMember`) |

Rows an evaluator rule in PR #1232 (`internal/semantic/metamodel/rules.go`) already names are marked; that PR adds a lazy metamodel evaluator and layers its derived values into the API-JSON writer, so its merge is expected to move those rows from ours-missing to present-equal or present-different, and this referee is how that movement is measured.

