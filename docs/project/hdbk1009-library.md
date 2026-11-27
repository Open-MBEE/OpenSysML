# HDBK1009 library — design

Status: **implemented.** `internal/workspace/libs/stdlib/OpenSysML Libraries/HDBK1009.sysml` —
maintained upstream at Open-MBEE/OpenSysML-Extensions-Library and vendored here pinned by
`scripts/extension-libraries-pin.sh` — is bundled with the other OpenSysML extensions, enters the
same conformance, strict-notation and snapshot gates, and is exercised by
`examples/hdbk1009-demo/hdbk1009-demo.sysml` (`TestExamplesAnalyseCleanly`). `passes.HDBK1009Pass`
audits models written against it.

## The problem

NASA-HDBK-1009A, *NASA Systems Modeling Handbook for Systems Engineering* (Office of the Chief
Engineer, approved 2025-03-12,
[NASA Technical Standards](https://standards.nasa.gov/standard/NASA/NASA-HDBK-1009)), is NASA's
guidance for carrying out four of the NPR 7123.1 system design and product realization processes
— Stakeholder Expectations Definition, Technical Requirements Definition, Product Verification
and Product Validation — in a system model. Section 7 gives a metamodel: the model elements of
the requirements, behavior, structure and parametric pillars and the relationships among them
(Figure 7-1), and the elements of verification and validation (Figure 7-2). Sections 8 and 9
then show, work product by work product, which diagram or table presents each.

The Handbook is written for SysML v1.7: its figures name stereotypes and v1 relationships
(`«deriveReqt»`, `«refine»`, `«trace»`, `«satisfy»`, `«verify»`, `«allocate»`), and it states that
another language may stand in for SysML. A project working in SysML v2 needs the same metamodel
in v2 terms — which native relationship carries each arrow of Figure 7-1 — and the Handbook's
tables generated from the model. The [`NPR7123`](npr7123-library.md) library carries the NPR's
processes, reviews and compliance matrix; this library carries the modeling vocabulary the
Handbook lays over them, and the two are used together.

## Design

**The Handbook's names first.** Every concept keeps the Handbook's name and cites the section or
figure it comes from. The library is non-normative, as the Handbook is guidance: nothing in it is
stricter than the Handbook's text. Following `MOSA`, `OOSEM` and `NPR7123`, each model element is
a definition specialising the native SysML type, an abstract base usage subsetting the native base
usage, and a `SemanticMetadata` keyword applying both at once:

```sysml
#need requirement <'n-1'> characterizeDust;
#missionObjective requirement <'o-1'> profileToOneKilometre;
#systemRequirement requirement <'sys-1'> rangeRequirement { :>> verifyMethod = VerificationMethodKind::test; }
#verificationRequirement requirement <'ver-1'> rangeTest { :>> verifyMethod = VerificationMethodKind::test; }
```

| NASA-HDBK-1009A | Library element |
| --- | --- |
| Figure 7-1, requirements pillar | `RequirementsPillarElement :> RequirementCheck` (`id`) and its kinds `StakeholderExpectationStatement`, `Need`, `Goal`, `Objective`, `TechnicalRequirement` (`SystemRequirement`, `SubsystemRequirement`, `ComponentRequirement`), `MeasureOfEffectiveness`, `MeasureOfPerformance`; `Stakeholder :> Part` |
| Figure 7-1, behavior pillar | `SystemUseCase :> UseCase`, `MissionActivity`, `SystemFunction` and `ComponentFunction :> Action`, `ComponentStateMachine :> StateAction` |
| Figure 7-1, structure pillar | `SystemContext`, `ExternalElement`, `SystemUser`, `MissionElement`, `SystemElement`, `ComponentElement`, each `:> Part` |
| Figure 7-1, parametric pillar | `technicalPerformanceMeasures` (keyword `#tpm`, an attribute usage) and `ParametricAnalysis :> AnalysisCase` |
| Figure 7-2, verification and validation | `VVRequirement` (`verifyMethod`, `successCriteria`, `closureStatus`, `closureDate`, `closureNotes`) with `VerificationRequirement` and `ValidationRequirement`; `VVCase :> VerificationCase`; `VVEvent`, `VVCaseActivity` and `VVStep :> Action`; `VVConfiguration` with its articles under test, `VVPersonnel`, `VVEquipment` and `VVFacility`; `VVArtifact :> Item` |
| Sections 8.1–8.22, the work products | `ProductKind`, one literal per product; the queries and documents below |

Two keywords differ from the Handbook's names because `stakeholder` and `objective` are SysML v2
keywords: `#stakeholderActor` and `#missionObjective`.

**Relationships are the native ones.** The library adds no relationship types. Each arrow of the
two figures is the SysML v2 relationship that carries the SysML v1 one the Handbook names, so the
semantic model, the queries and every other tool read them without knowing the library:

| Handbook (SysML v1.7) | SysML v2 |
| --- | --- |
| Derives From, Decompose `«deriveReqt»` | `#derivation connection { end #original ::> upper; end #derive ::> lower; }` |
| Refines `«refine»` | `#refinement dependency from refining to refined;` |
| Traced To, Provided By `«trace»` | `dependency from element to source;` |
| Satisfies `«satisfy»` | `satisfy requirement by element;` |
| Verifies `«verify»` | `verify requirement;` in the objective of a verification case |
| Allocated To `«allocate»` | `allocate element to target;` |
| Has, Involved with | ownership and feature typing |

The one place the mapping is not one-to-one is verification. In Figure 7-2 a verification
*requirement* verifies a requirement, and a V&V case refines the verification requirement. SysML
v2 states `verify` only in the objective of a verification case, so the V&V case carries both:
its objective verifies the technical requirements, and a refinement ties it to the verification
requirement it carries out.

**Work products.** Each table the Handbook shows is a `DocumentQueries` query over the model, and
a `ProductDocument :> Document` per product arranges them; a model specialises the document and
binds `root` to the element holding its content.

| Query | Handbook | Columns from |
| --- | --- | --- |
| `StakeholderRegister` | 8.1 | the stakeholders |
| `ExpectationRegister` | 8.1, 8.2 | expectation statements, needs, goals and objectives; traces and derivations |
| `RequirementsRegister` | 8.11 | technical requirements; derivation, refinement, satisfaction, verify method |
| `MeasureRegister` | 8.12, 8.13 | MOEs and MOPs; what each refines, derives from and is satisfied by |
| `RequirementsVerificationRegister` | 8.16, Figure 8.16-1 | technical requirements and the V&V cases verifying them |
| `VerificationRequirementsRegister` | 8.17, Figure 8.17-1 | verification requirements: method, success criteria, refining case, artifact, event, closure |
| `ValidationRequirementsRegister` | 8.22, Figure 8.22-1 | validation requirements: objective, method, artifact, event, closure |

The relationship columns are `RelatedColumn`s at depth 1, so a table shows the direct relationship
and not its transitive closure. `RelatedElements` gained the relationship kind `"dependency"` for
the trace columns: a plain dependency from client to supplier, which a refinement also is.

**Checks.** `passes.HDBK1009Pass` (constraint tier, source `hdbk1009`) audits a model against the
relationships the two figures draw. Every finding is a **warning**, and every rule waits until
the model declares the elements the relationship would reach: a model with needs and no goals yet
is early, not wrong.

| Code | Rule | NASA-HDBK-1009A |
| --- | --- | --- |
| `hdbk1009-requirement-not-derived` | a goal derives from a need, an objective from a goal, a system requirement from an objective, a subsystem or component requirement from a system or subsystem requirement, a MOP from a MOE; with no system requirement declared, a subsystem requirement among several that neither derives from another nor is another's origin | Figure 7-1 |
| `hdbk1009-expectation-not-traced` | an expectation statement is traced to a stakeholder; a need, goal or objective to an expectation statement | Figure 7-1 |
| `hdbk1009-measure-not-refining` | a MOE refines an objective; a MOP refines a technical requirement | Figure 7-1 |
| `hdbk1009-requirement-not-verified` | a technical requirement is verified by a verification case, once the model declares verification requirements | Figure 7-2, §8.16 |
| `hdbk1009-vv-requirement-not-refined` | a verification or validation requirement is refined by a V&V case, once the model declares V&V cases | Figure 7-2 |
| `hdbk1009-vv-requirement-missing-method` | a verification or validation requirement states a `verifyMethod` | §8.17, §8.22 |
| `hdbk1009-objective-not-validated` | a validation requirement satisfies an objective, and each objective is satisfied by one once the model declares validation requirements | Figure 7-2, §8.22 |

Classification follows the typing or the keyword, as for `NPR7123`. The relationships are read
across the workspace: a derivation or a V&V case kept in another document answers the rule. To
keep that true when the other document is held as an interface record, the record now carries the
dependencies, refinements and verifications a body states, and every derivation the model reads —
a connection typed by `Derivation` as well as an annotated one — beside the derivations,
allocations and satisfactions it already carried. A `not satisfy` validates nothing, and a
`satisfy requirement r : T` declares a requirement the rules judge like any other. Library documents are not audited, a model that never touches
`HDBK1009` gets no finding, and a workspace package that merely reuses the name is not the
vocabulary.

## Alternatives considered

- **Link features on the definitions** (`derivedFrom`, `tracedTo`, `verifiedBy` as reference
  features). Rejected: SysML v2 has each of these relationships natively, the semantic model
  already exposes them as edges, and a second way to state them would let the two disagree.
- **`verify` on the verification requirement**, as Figure 7-2 draws it. Not expressible: SysML v2
  admits a requirement verification only in the objective of a verification case.
- **One document per table.** Rejected: the Handbook groups its tables by work product, and a
  product document with one section per table follows it.
- **Checks as errors, or unconditional.** Rejected, as for MOSA, OOSEM and NPR7123: the Handbook is
  guidance, and a model part-way down the metamodel is a normal state of the work.

## Known limitations

- The Handbook's diagrams (context, use case, activity, state machine, parametric) are presented
  by the general view machinery; the library adds grid views for the tabular products only.
- The system context, use cases and activities (8.3–8.9) and V&V planning (8.18–8.20) have
  vocabulary — use cases, mission activities, V&V events, steps, configurations and artifacts —
  and no check: the Handbook states no model-checkable fact about them beyond the relationships
  above.
- Allocation of V&V steps to personnel, equipment and facilities is vocabulary only.
- The Handbook says it will be updated for changes such as SysML version 2.0 (section 7); the
  mapping here is this project's reading of the v1.7 text until that update is published.
