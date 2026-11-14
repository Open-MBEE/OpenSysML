# Derived-property census

**What it is.** A measured gate over the metamodel's served derivation rules: every model file
under the corpus roots is analyzed, every element reachable from a document root through ownership
is read through the `metamodel.Evaluator`, and for every served rule (`internal/semantic/metamodel/rules.go`,
`metamodel.Rules()`) the census records how many carriers — elements whose metaclass carries the
property — answer **a value**, answer **nothing** (null or an empty collection), or are **not
derived** (the evaluator declined, with a reason `Explain` reports). The pinned OMG pilot's export
of the same files is the reference column: the same counts over the elements the pilot wrote. A rule
is **silent** when it answers nothing on every carrier while the pilot answers a value on at least
one; the committed baseline lists the silent rules, and `-check` fails when a rule joins that list.

The evaluator reads the elements through the symbol-backed `metamodel.SymbolStructure`
(`metamodel.New` over `metamodel.NewSymbolStructure`), the Structure `GetElementProperties` is
being moved onto from the API element form (`export.ModelElementForm`, the RDF graph); the report's
`structure` field states it, so a reader of the baseline knows which projection the counts
measured. The graph-versus-symbol differential
(`TestSymbolStructureMatchesGraphStructure`, `internal/translate/export/testdata/structure_differential_expected.txt`)
remains the correctness check between the two projections; the census measures what the served
one answers.

The pinned pilot is `2026-09` (`jupyter-sysml-kernel 0.63.0`), the artifact the reference
validators use (`scripts/pilot-pin.sh`); the exporter is the bridge the
[API-JSON differential](api-json-differential.md) builds.

## Provisioning and running

```sh
./scripts/download-pilot-json-exporter.sh      # builds build/pilot-json-exporter/export-json
./scripts/download-training-examples.sh && ./scripts/download-pilot-corpora.sh
go run -C tools ./cmd/derived-census           # writes build/derived-census/
go run -C tools ./cmd/derived-census -check    # fails on a newly silent rule or a baseline that does not reproduce
go run -C tools ./cmd/derived-census -update   # re-records internal/semantic/metamodel/derived_census_{baseline.json,records.tsv}
```

`-roots examples,testdata` restricts a run to named roots (never with `-update`); `-exporter`,
`-out` and `-timeout` override the launcher, the output directory and the per-root time allowed to
the pilot. A run writes `derived-census.json` (the baseline's shape) and `derived-census.txt` (the
roots, the classes and every silent rule with its counts).

**Without the exporter.** Our side needs no Java, and the pilot's column changes only when the
pin, the bridge or a corpus does. When `build/pilot-json-exporter/export-json` is absent the run
re-measures our side and takes the pilot column from the committed baseline, root by root, saying
so; `-require-exporter` makes the absence a failure instead. A root the baseline does not record
then has an empty pilot column, which no rule can be silent against.

**Re-recording.** The baseline moves whenever a rule, a fixture under a root, the standard library,
the pin or the bridge does. Its `provenance` block records the inputs the pilot's column depends on
by digest — the pin, the bridge, each root, the libraries — and `-check` names the one that moved
before holding the counts to it; this repository's own rules are not digested (a branch would move
the baseline without moving a count), so a rule change is caught where it moves a count or the
`silent` list, and nowhere else. To re-record:

1. provision the exporter and the downloaded roots (above), so the pilot column is re-exported
   rather than carried over;
2. `go run -C tools ./cmd/derived-census -update`;
3. read the movement in `git diff internal/semantic/metamodel/derived_census_baseline.json` —
   every rule whose `ours` counts moved, and the `silent` list — and adjudicate it in the pull
   request: a rule leaving `silent` is the fix the change made, one joining it is a regression the
   gate exists to refuse;
4. `go run -C tools ./cmd/derived-census -check` reproduces the recorded baseline.

`TestDerivedCensusBaselineKeysServedRules` (`internal/semantic/metamodel`) holds the committed
baseline to `rules.go` without corpora or Java: every rule it counts is served and its served total
is the registry's, so a rule added or withdrawn re-records the census.

## Counting

**Roots.** The corpus roots of the [pilot differential](pilot-differential.md) (`tools/referee/diff`)
plus the handbook's worked examples, as the API-JSON differential measures them: the training
examples, the three pilot corpora, the specification examples, `tests/testdata`, the repository's
own `examples/` (its downloaded roots excepted), the differential's probes and `docs/manual/examples`.
A root absent from the checkout is left out of the run and of the provenance; the provenance of a
baseline recorded with every root present therefore does not reproduce on a checkout without them,
and `-check` says which input is missing.

**Our side.** Each root's files are analyzed as one model, so a cross-file import resolves as the
pilot resolves it. From each document root the walk follows `Element::ownedRelationship` and
`Relationship::ownedRelatedElement` as the semantic model states them, which is the same walk the
graph-versus-symbol differential takes. For an element of metaclass *M*, every served rule whose
defining class *M* specializes is read once through `PropertyReadOnlyUncached`: a declined read is
*not derived*, a null or an empty sequence is *empty*, anything else is *a value*. A boolean
`false` is a value.

**The pilot's side.** The bridge writes, per input file, every element the pilot's traversal
reaches with the derived properties the API serializes. An element flagged `isLibraryElement` is
not the file's and is left out, as our walk never reaches a library element. For every other
element the same conformance test applies, from the standard library's metaclass declarations, and
a property **absent** from the payload is *not derived*, one written `null` or `[]` is *empty*,
anything else is *a value*.

**Pilot-nondeterministic cells.** Where the API-JSON referee declares a (metaclass, property) pair
pilot-nondeterministic (`tools/referee/apijson/nondeterminism.go`, the one list both tools read),
the pilot's side counts the carrier as *nondeterministic* instead, whatever the export states. The
declared pair is `FeatureChainExpression::featuringType`: the pilot fills it in transformation
order, so three expressions of the pilot corpora had a value under one JVM and none under another,
and a count of values there recorded the Java that ran the export rather than the model. Scored
apart, the records are the same under either. Our side is counted as everywhere else.

**The declared JVM.** `scripts/pilot-pin.sh` declares the Java feature release the pilot's column
is recorded under (`PILOT_JAVA`, the one the workflows install) and the baseline states it as
`provenance.pilotJava`. `-update` refuses to record under any other, `-check` fails on a baseline
whose `pilotJava` is not the declared one, and a run that only compares under another JVM says so
before it exports, so an undeclared JVM dependence is named where it appears.

**Implied relationships — a deliberate difference, not a failure.** The pilot materializes the
relationships it implies (the implicit specializations, typings and subsettings
`isImpliedIncluded` controls) as elements only when asked to; the bridge runs its transformation
without adding them, so the pilot's column holds no implied relationship (`isImplied` is false on
every element it writes). The semantic model synthesizes the relationships it implies and our walk
visits them like the stated ones, because the rules apply to them too and a reader of
`GetElementProperties` is handed them. Our carriers therefore include implied relationships and
their memberships that the pilot's do not: each root's `implied` count records how many, no detector
compares the two populations, and the gate reads only whether a rule answers anywhere, which a
population difference cannot turn into a failure. The census the gate grew out of excluded implied
relationships from its walk; this one keeps and counts them.

**What the counts are not.** Carriers are not matched element to element across the two sides —
the two populations differ by the implied relationships and by what each side represents — so a
rule's `ours` and `pilot` rows are two scorecards over two populations of the same files, and the
gate compares only *answers somewhere* against *answers nowhere*. Property-by-property agreement is
the [API-JSON differential](api-json-differential.md)'s measurement.

## Results (pilot `2026-09`)

The baseline recorded on the pinned pilot over 935 files, every root present (38 of the
`spec-examples` files are rejected). The figures below are that recording, not the current baseline —
`internal/semantic/metamodel/derived_census_baseline.json` (the provenance and each root's figures)
with `derived_census_records.tsv` beside it (one line per rule and root, so two branches moving
different rules merge without a conflict; the totals are derived on read) is, and
`build/derived-census/derived-census.txt` renders a run.

| Root | Files | Elements (implied) | Pilot elements (copies dropped, files failed) |
|---|---:|---:|---:|
| `training` | 100 | 7926 (4622) | 11906 (5797, 0) |
| `pilot-examples` | 99 | 31138 (18262) | 40642 (10940, 3) |
| `pilot-validation` | 56 | 10692 (6273) | 14222 (3066, 0) |
| `kerml-examples` | 58 | 6906 (4269) | 8762 (1105, 0) |
| `spec-examples` | 511 | 8329 (4729) | 12233 (37, 52) |
| `testdata` | 40 | 1528 (832) | 2284 (52, 4) |
| `examples` | 56 | 31524 (18354) | 50715 (76235, 0) |
| `probes` | 4 | 45 (26) | 53 (0, 0) |
| `manual-examples` | 11 | 3963 (2393) | 9379 (0, 0) |

Of the 297 served rules, **277 answer a value** on at least one element, **10 are empty
everywhere** they apply, **9 are never derived** and 1 applies to no element of any root; the pilot
answers a value for 288. **17 rules are silent** — ours never a value, the pilot a value somewhere:

| Silent rule | Ours: empty / not derived / carriers | Pilot: values / carriers |
|---|---:|---:|
| `Documentation::documentedElement` | 0 / 407 / 407 | 420 / 420 |
| `Flow::flowEnd` | 234 / 0 / 234 | 158 / 236 |
| `Flow::sourceOutputFeature` | 0 / 234 / 234 | 161 / 236 |
| `Flow::targetInputFeature` | 0 / 234 / 234 | 161 / 236 |
| `Import::importedElement` | 0 / 1221 / 1221 | 1193 / 1202 |
| `MultiplicityRange::bound` | 1186 / 0 / 1186 | 1426 / 1426 |
| `MultiplicityRange::lowerBound` | 1186 / 0 / 1186 | 368 / 1426 |
| `MultiplicityRange::upperBound` | 1186 / 0 / 1186 | 1426 / 1426 |
| `Namespace::importedMembership` | 40806 / 637 / 41443 | 634 / 63157 |
| `PortConjugation::originalPortDefinition` | 0 / 134 / 134 | 127 / 127 |
| `RequirementDefinition::text` | 71 / 66 / 137 | 66 / 134 |
| `RequirementUsage::text` | 402 / 104 / 506 | 103 / 492 |
| `TransitionUsage::source` | 0 / 243 / 243 | 343 / 343 |
| `TransitionUsage::succession` | 243 / 0 / 243 | 343 / 343 |
| `TransitionUsage::target` | 0 / 243 / 243 | 343 / 343 |
| `Usage::nestedViewpoint` | 14425 / 0 / 14425 | 1 / 18777 |
| `UseCaseDefinition::includedUseCase` | 0 / 19 / 19 | 3 / 19 |

`ConjugatedPortTyping::conjugatedPortDefinition` left the list when the Structure began deriving it
as the typing's target rather than as the owner a `PortConjugation` has: it answers 61 of its 64
carriers, the three it declines typing a port by the conjugate of a port definition the fragment
never declares (`port p : ~Pd;`), which the pilot still answers.

Three of the seventeen are one derivation stated three ways (`TransitionUsage::source`, `::target`
and `::succession`; `Flow::flowEnd`, `::sourceOutputFeature` and `::targetInputFeature`). The rules
counted *not derived* are declined on every carrier with a reason `Explain` reports —
`Import::importedElement` because the evaluator reads the import's syntax node, which a
symbol-backed element does not carry; the rest answer an empty collection.

The rules the issue's census named, read through the symbol-backed Structure, are not silent but
answer on a fraction of their carriers. `SendActionUsage::payloadArgument` applies to no element on
our side: the symbol-backed structure classifies the `send` actions of these corpora as
`ActionUsage`.

| Rule | Ours: value / empty / not derived / carriers | Pilot: value / empty / not derived / carriers |
|---|---:|---:|
| `Function::result` | 92 / 0 / 331 / 423 | 419 / 0 / 0 / 419 |
| `Expression::result` | 3968 / 0 / 3331 / 7299 | 12942 / 0 / 0 / 12942 |
| `Behavior::step` | 497 / 28 / 124 / 649 | 645 / 0 / 0 / 645 |
| `Import::importedElement` | 0 / 0 / 1105 / 1105 | 1116 / 0 / 1 / 1117 |
| `Connector::association` | 165 / 704 / 17 / 886 | 1579 / 0 / 0 / 1579 |
| `InstantiationExpression::instantiatedType` | 772 / 0 / 1069 / 1841 | 3662 / 0 / 0 / 3662 |
| `Feature::ownedTypeFeaturing` | 72 / 33807 / 0 / 33879 | 72 / 53379 / 0 / 53451 |
| `Usage::isReference` | 13192 / 0 / 15 / 13207 | 17358 / 0 / 0 / 17358 |
| `Step::behavior` | 688 / 8216 / 237 / 9141 | 15115 / 0 / 0 / 15115 |
| `Expression::function` | 578 / 6542 / 179 / 7299 | 12942 / 0 / 0 / 12942 |
| `SendActionUsage::payloadArgument` | 0 / 0 / 0 / 0 | 53 / 0 / 1 / 54 |
| `Type::endFeature` | 143 / 15455 / 21029 / 36627 | 2223 / 53902 / 0 / 56125 |

The pilot's `Feature::ownedTypeFeaturing` is empty on all but 72 of its carriers as well: the
property holds the `TypeFeaturing` relationships a feature *owns*, which only a `featured by`
clause states.
