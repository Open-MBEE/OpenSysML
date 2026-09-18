# OOSEM library — design

Status: **implemented.** `internal/core/libs/stdlib/OpenSysML Libraries/OOSEM.sysml` is bundled
with the other OpenSysML extensions, enters the same conformance, strict-notation and snapshot
gates, and is exercised by `examples/oosem-demo/oosem-demo.sysml`
(`TestExamplesAnalyseCleanly`). Both the library and the example also validate cleanly under the
pinned OMG pilot implementation, so the notation they use is standard SysML v2.

## The problem

The Object-Oriented Systems Engineering Method (OOSEM, INCOSE OOSEM Working Group; Friedenthal,
Moore & Steiner, *A Practical Guide to SysML*, ch. 17) is a top-down, scenario-driven method:
analyse stakeholder needs, analyse system requirements, define the logical architecture,
synthesise candidate physical architectures, optimise and evaluate alternatives, validate and
verify. In SysML v1 it was carried by a UML profile of stereotypes (`«system»`, `«logical»`,
`«node»`, `«moe»`, …) and a package template. SysML v2 has no profiles; a method vocabulary is
a *library*: definitions, base usages and semantic metadata.

Within Open-MBEE the only prior SysML v2 material is the stub `OOSEM.sysml` in
[DesertKite.sysml](https://github.com/Open-MBEE/DesertKite.sysml) — `MOE`/`MoP` abstract
attributes and three empty view placeholders. The closest worked library-development pattern is
[structured-use-cases](https://github.com/Open-MBEE/structured-use-cases), which extends native
`UseCase`/`Action`/`ConstraintCheck` and attaches a `SemanticMetadata` keyword to each
extension. This library follows that pattern and keeps the stub's names reachable.

## Design

**One shape for every artefact.** Each OOSEM concept is three declarations:

```sysml
part def SystemOfInterest :> Part { doc /* ... */ }
abstract part systemsOfInterest : SystemOfInterest[0..*] nonunique :> parts;
metadata def <system> SystemOfInterestMetadata :> SemanticMetadata {
    :> annotatedElement : SysML::PartDefinition;
    :> annotatedElement : SysML::PartUsage;
    :>> baseType = systemsOfInterest meta SysML::Usage;
}
```

A modeller may write `part sat : SystemOfInterest;` and get the type alone, or
`#system part sat : Satellite;` and get the type *and* membership in `systemsOfInterest`
(SysML v2 §7.27.4 semantic metadata: the annotated usage implicitly subsets `baseType`, a
definition implicitly specialises its type). Every base usage is a subset of the native
`parts`/`items`/`actions`/`requirementChecks`/`useCases`/`analysisCases`, so `%search` and the
document queries reach OOSEM artefacts by method role without any OOSEM-specific tooling.

**Reuse before invention.** Nothing is redeclared that the OMG domain libraries already hold;
the four domain libraries below are re-exported so that `import OOSEM::*` brings their keywords
along with the method's own:

| OOSEM concept | Comes from |
| --- | --- |
| measures of effectiveness / performance (`#moe`, `#mop`) | `ParametersOfInterestMetadata` |
| trade study, objective, evaluation function | `TradeStudies` |
| requirement derivation (`#derivation`, `#original`, `#derive`) | `RequirementDerivation` |
| causal analysis chains (`#cause`, `#effect`) | `CauseAndEffect` |

The systems-library vocabulary the method also leans on — requirement text, subjects, actors and
stakeholders (`Requirements`), included use cases (`UseCases`), verification and analysis cases
(`VerificationCases`, `AnalysisCases`) — is not re-exported: the `requirement`, `use case`,
`verification` and `analysis` keywords reach it implicitly, and a model that names one of those
libraries' members directly imports it as it would without OOSEM (the example does exactly this
for `ScalarValues`, `ISQ` and `SI`).

The DesertKite stub's `'OOSEM Measures'::MOE` and `MoP` survive as aliases of the standard base
usages, so a model written against the stub resolves the same features.

**What the library declares.**

| Activity | Definitions / keywords |
| --- | --- |
| Enterprise & stakeholder needs | `Enterprise` `#enterprise`, `Stakeholder` `#stakeholderRole`, `CausalAnalysis` `#causalAnalysis`, `EnterpriseUseCase` `#enterpriseUseCase`, `StakeholderNeed` `#stakeholderNeed`, `MissionRequirement` `#missionRequirement` |
| System requirements | `SystemOfInterest` `#system`, `ExternalSystem` `#externalSystem`, `User` `#user`, `Environment` `#environment`, `SystemContext` `#systemContext` (with `systemOfInterest`, `externalSystems`, `users`, `environment` parts), `SystemUseCase` `#systemUseCase`, `IOEntity` `#ioEntity`, `Store` `#store`, `SystemRequirement` `#systemRequirement` |
| Logical architecture | `LogicalComponent` `#logical`, `LogicalScenario` `#logicalScenario`, `Node` `#node`, `ComponentRequirement` `#componentRequirement` |
| Physical architecture | `PhysicalComponent` `#physical`, `HardwareComponent` `#hardware`, `SoftwareComponent` `#software`, `OperationalProcedure` `#operationalProcedure`, `DataComponent` `#data` |
| Lifecycle | `#asIs` / `#toBe` marking (`EnterpriseStateKind`) |
| Package template | `@OOSEMPackage { kind = OOSEMPackageKind::…; }` — the activity a package holds, for navigating a model by method rather than by package name |
| Viewpoints & views | `EnterpriseViewpoint`, `SystemContextViewpoint`, `RequirementsViewpoint`, `LogicalArchitectureViewpoint`, `PhysicalArchitectureViewpoint` framing one concern each; view definitions `EnterpriseModelView`, `SystemContextView`, `SystemUseCaseView`, `RequirementsView`, `MeasuresView`, `LogicalArchitectureView`, `LogicalScenarioView`, `PhysicalArchitectureView` |

**Views.** Each OOSEM view definition specialises a `StandardViewDefinitions` view (`BrowserView`,
`InterconnectionView`, `GridView`, `ActionFlowView`), carries the `filter` that admits the
activity's artefacts by their metadata, and states its `render` (`asTreeDiagram`,
`asInterconnectionDiagram`, `asElementTable`). A model then writes only the usage and what it
exposes — `view requirements : RequirementsView { expose 'System Requirements'::*; }` — and
`sysml -render-all` produces the artefact. This relies on a view usage honouring the filters of
its definition and that definition's supertypes; `resolve.Resolver.importAdmits` applies those
inherited conditions to `expose` imports (before, only the usage's own `filter` counted).

**Method checks.** `passes.OOSEMMethodPass` (constraint tier, source `oosem`) audits a model
against the traces the method expects. Every finding is a **warning**, since an incomplete
model is a normal state of the work, and every rule waits until the model has reached the level
it checks against:

| Code | Rule | Applies once the model has |
| --- | --- | --- |
| `oosem-requirement-not-derived` | a mission / system / component requirement is the `#derive` end of a `#derivation` connection whose `#original` end is a requirement of the level above (stakeholder need / mission / system requirement); a derivation from any other level does not count | any requirement of the level above |
| `oosem-requirement-not-satisfied` | a system or component requirement is the requirement of some `satisfy`; a negated satisfy or a view satisfying a viewpoint does not count | any requirement `satisfy` |
| `oosem-logical-component-not-allocated` | a `#logical` component usage is the source of an `allocate` (itself, an enclosing usage, or its type) whose destination is a `#node` or physical component; an allocation to anything else does not count | any `#node` or physical component |
| `oosem-use-case-subject` | the subject of a `#systemUseCase` is a `#systemContext`, of an `#enterpriseUseCase` an `#enterprise`; the subject's effective types count, whether declared, inherited, redefined or subsetted | always, for subjects typed by more than what every system context or enterprise is anyway (`Anything`, `Part`) |

Classification follows the metadata *or* the typing, so `requirement r : SystemRequirement` and
`#systemRequirement requirement r` are both system requirements. Library documents are not
audited, and a model that never touches `OOSEM` gets no finding.

Naming follows the SysML v1 OOSEM profile where the word is not a SysML v2 reserved keyword.
`stakeholder`, `analysis` and `verification` are keywords, hence `#stakeholderRole`,
`OOSEMPackageKind::analysisModel` and `::verificationModel`; the pilot's grammar rejects them as
names where OpenSysML's would not, and the library is written so both accept it.

## Alternatives considered

- **An OOSEM rendering engine.** Rejected: OOSEM's diagrams are the SysML diagrams applied to
  the artefacts above, so the view definitions specialise `StandardViewDefinitions` and reuse the
  `Views` renderings; the only OOSEM-specific part is the filter.
- **Method checks as errors, or unconditional.** Rejected: OOSEM is worked top-down over weeks,
  and a model with system requirements and no mission requirements yet is not wrong. The rules
  gate on the presence of the level they trace to and stay at warning severity.
- **Requirement categories as `RequirementCheck` subtypes vs. metadata only.** Subtypes were
  chosen so a requirement can be typed by its level with no keyword, and so the level is an
  ordinary specialisation the checker and RDF export already understand.
- **Redeclaring MOE/MOP.** Rejected; `ParametersOfInterestMetadata` is normative and its
  `#moe`/`#mop` are what other libraries and tools recognise.

## Known limitations

- The method checks cover traceability (derivation, satisfaction, allocation) and use-case
  subjects. Not checked: that every MOE has a trade-study objective, that every system
  requirement is verified, or that a `#logicalScenario` decomposes a use case.
- `LogicalScenarioView` renders only what `ActionFlowView` renders; a `GeometryView`-based
  physical layout is not offered because the geometry renderer does not draw yet.
- No dedicated vocabulary for risk, design margins or verification-method assignment beyond
  native `VerificationCases`.

## Parser fix made along the way

Prefix metadata before the two-word kind `use case` (`#systemUseCase use case def …`) was
rejected as `expected a namespace member`, because the prefix lookahead in
`parser.leadingPrefixIsDefUsage` only recognised single-token kind keywords. Fixed, with the
golden fixture `tests/parser/testdata/parse/prefix_metadata_use_case.sysml`.
