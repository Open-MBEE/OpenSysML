# MOSA library — design

Status: **implemented.** `internal/core/libs/stdlib/OpenSysML Libraries/MOSA.sysml` is bundled
with the other OpenSysML extensions, enters the same conformance, strict-notation and snapshot
gates, and is exercised by `examples/mosa-demo/mosa-demo.sysml` (`TestExamplesAnalyseCleanly`).
`passes.MOSAPass` audits models written against it.

## The problem

The Modular Open Systems Approach (MOSA) is the "integrated business and technical strategy"
that 10 U.S.C. § 4401 requires of a United States Department of Defense major defense
acquisition program: a modular design, with the interfaces between modules defined by widely
supported, consensus-based standards or documented in machine-readable form, and the rights in
technical data that let a module be competed, upgraded or replaced over the life cycle. The
statute (§ 4401(b)) also fixes the vocabulary — *major system platform*, *major system
component*, *modular system*, *modular system interface* — and asks each program to identify its
modular system interfaces, the standards each is verified against, and its proprietary
elements. The OUSD(R&E) *MOSA Info Sheet* (March 2025) and the *MOSA Implementation Guidebook*
add the practitioner's terms: five *pillars* (modular design, key interfaces, open standards,
conformance, enabling environment), *key interfaces*, and a set of *conformance assessment*
criteria.

In SysML v1 a program carried this with a profile of stereotypes and a document set kept
alongside the model. SysML v2 has no profiles; the vocabulary is a *library*, and the documents
are views and document queries over the same model, so the interface control document a program
delivers is generated from the model it is describing.

## Design

**Statutory names first.** Every concept keeps the statute's name; the guidebook's terms are
layered on top. `ModularSystemInterface` is the primary concept; `#keyInterface` is a keyword
that *specialises* `#modularSystemInterface`:

```sysml
metadata def <modularSystemInterface> ModularSystemInterfaceMetadata :> SemanticMetadata {
    :> annotatedElement : SysML::InterfaceDefinition;
    :> annotatedElement : SysML::InterfaceUsage;
    :>> baseType = binaryModularSystemInterfaces meta SysML::Usage;
}
metadata def <keyInterface> KeyInterfaceMetadata :> ModularSystemInterfaceMetadata;
```

so a `#keyInterface` is a modular system interface to every view, query and check, and a model
that does not use the guidebook's word never sees it. A semantic metadata definition that binds no
`baseType` of its own inherits the binding of the definition it specialises
(`semantics.Model.baseTypeOf`); this is what makes the alias one declaration rather than a copy.

**One shape for every artefact**, as in the [OOSEM library](oosem-library.md): a definition
specialising the native SysML type, a base usage subsetting the native base usage, and a semantic
metadata keyword applying both at once. `#majorSystemComponent part radar : Radar;` types the
usage by `Radar` and adds it to `majorSystemComponents :> parts`, so `%search`, the document
queries and the RDF export reach MOSA artefacts by role with no MOSA-specific tooling.

**Openness as metadata over the standard model.** Standards conformance, data rights, interface
control and conformance assessment are facts *about* ordinary elements, so they are metadata and
connections, not new element kinds:

| Fact | Written as |
| --- | --- |
| an interface is verified against a standard | `#conformance connection { end #conformant ::> link; end #conformsTo ::> std; }` (`StandardConformance`) |
| the rights held in a component's technical data | `@DataRights { kind = DataRightsKind::limited; asserter = …; basis = …; }` |
| an element stays proprietary, and why | `@Proprietary { owner = …; rationale = …; }` |
| who controls an interface, and in which document | `@InterfaceControl { authority = …; document = …; }` |
| an assessment against a conformance criterion | `@MOSAConformance { criterion = MOSAConformanceCriterion::modularity; status = StatusKind::done; evidence = …; }` |
| the pillar a package serves | `@MOSAPackage { pillar = MOSAPillar::keyInterfaces; }` |

**Reuse before invention.** Three domain libraries are re-exported so that `import MOSA::*`
brings their keywords along with the approach's own:

| MOSA concept | Comes from |
| --- | --- |
| measures of effectiveness / performance on MOSA objectives (`#moe`, `#mop`) | `ParametersOfInterestMetadata` |
| requirement derivation (`#derivation`, `#original`, `#derive`) | `RequirementDerivation` |
| status, rationale, issue and owner (`StatusKind`, `@Rationale`, `@Issue`) | `ModelingMetadata` |

`DocumentQueries` supplies the document, section, table and query machinery the interface control
document is built on, and `StandardViewDefinitions`/`Views` the view definitions and renderings
the MOSA views specialise.

**What the library declares.**

| Area | Definitions / keywords |
| --- | --- |
| Modular design (§ 4401(b)) | `MajorSystemPlatform` `#majorSystemPlatform`, `MajorSystemComponent` `#majorSystemComponent`, `ModularSystem` `#modularSystem` |
| Modular system interfaces | `ModularSystemInterface` `#modularSystemInterface` (with `characteristics : InterfaceCharacteristicKind[0..*]` — electrical, mechanical, fluidic, optical, radio-frequency, data, networking, software), `BinaryModularSystemInterface`, alias `#keyInterface` |
| Open standards | `Standard` `#technicalStandard` (`organization`, `identifier`, `version`, `registry`), `ConsensusStandard` `#consensusStandard`, `StandardConformance` `#conformance` with ends `#conformant` / `#conformsTo` |
| Technical data | `DataRights` (`DataRightsKind`: unlimited, government purpose, limited, restricted, SBIR, specially negotiated), `Proprietary`, `InterfaceControl` |
| Requirements | `MOSAObjective` `#mosaObjective`, `ModularityRequirement` `#modularityRequirement`, `InterfaceRequirement` `#interfaceRequirement` |
| Conformance | `MOSAConformance` over `MOSAConformanceCriterion` (objectives, key interfaces, standards compliance, modules, modularity, openness, life-cycle characteristics, conformance) |
| Pillars | `@MOSAPackage { pillar = MOSAPillar::…; }` |
| Viewpoints & views | `ModularDesignViewpoint`, `ModularInterfaceViewpoint`, `TechnicalDataViewpoint`, `ConformanceViewpoint`; view definitions `ModularDecompositionView`, `ModularInterfaceView`, `InterfaceRegisterView`, `StandardsView`, `ProprietaryElementsView`, `DataRightsView`, `MOSARequirementsView`, `ConformanceView` |
| Documents | queries `ModularSystemInterfaces`, `InterfaceRegister`, `Standards`, `StandardsRegister`, `ProprietaryElements`, `ProprietaryRegister`, `DataRightsRegister`; `InterfaceControlDocument :> Document` |

**Views.** Each MOSA view definition specialises a `StandardViewDefinitions` view
(`BrowserView`, `InterconnectionView`, `GridView`), carries the `filter` that admits the
approach's artefacts by their metadata, and states its `render`. A model writes only the usage
and what it exposes — `view interfaces : ModularInterfaceView { expose 'Modular Design'::vehicle; }`
— and `sysml -render-all` draws the components and the modular system interfaces between them.

**Interface control document.** `InterfaceControlDocument` is a `DocumentQueries::Document`
whose sections tabulate the registers: a model specialises it and binds each register's `root`
to the usage that owns the elements (`calc rows : InterfaceRegister { in :>> root = vehicle; }`).
The registers walk the descendants of `root`, select by conformance to the library's definitions
(`WhereType`) or by annotation (`WhereMetadata`), and project the columns; `sysml -render-document`
writes the document as Markdown, HTML or PDF. Each register also takes a `depth` — the number of
ownership levels walked below `root`, sixteen by default — and lists nothing that nests deeper, so
a document whose elements do binds it too (`in :>> depth = 24;`); the query executor's visit budget
still caps the work however deep the walk.

**Checks.** `passes.MOSAPass` (constraint tier, source `mosa`) audits a model against the
openness the approach asks of it. Every finding is a **warning** — an incomplete model is a normal
state of the work — and every rule waits until the model states the kind of fact it looks for:

| Code | Rule | Applies once the model has |
| --- | --- | --- |
| `mosa-interface-no-standard` | a modular system interface usage is the `#conformant` end of a `#conformance` connection (itself, its definition, or a usage it subsets) whose `#conformsTo` end names a standard, or is marked `@Proprietary` | any `#technicalStandard` |
| `mosa-interface-no-control` | a modular system interface usage carries an `@InterfaceControl` naming a non-empty `authority`, itself or through what it specialises | any `@InterfaceControl` naming an `authority` |
| `mosa-interface-not-traced` | a modular system interface usage is the `by` of some `satisfy` (itself or what it specialises), or declares a `satisfy` in its body; a negated satisfy does not count | any `#interfaceRequirement` |
| `mosa-component-no-data-rights` | a major system component or modular system usage carries `@DataRights`, itself or through what it specialises | any `@DataRights` |
| `mosa-proprietary-no-rationale` | a `@Proprietary` annotation states a non-empty `rationale` | always |
| `mosa-boundary-not-designated` | a `connect`, `connection` or `interface` whose ends attach to two distinct major system components or modular systems is a modular system interface; connectors within one component, to the platform or to an undesignated part are not judged | any modular system interface |

Classification follows the metadata *or* the typing, so `interface l : DataLink` where
`DataLink` is a `#modularSystemInterface` definition, `#modularSystemInterface interface l`, and
`#keyInterface interface l` are all modular system interfaces. Library documents are not audited,
and a model that never touches `MOSA` gets no finding; a workspace package that merely reuses the
name is not the vocabulary.

## How OpenSysML serves the approach

What § 4401(b)(1)(B) asks of the program's digital engineering environment — that modular
system interfaces be *documented in machine-readable form*, that the relationship of each to
its standard be recorded, and that the documentation be deliverable — is met with the tools
already in place: the interface definitions, ports and connectors are the SysML v2 model; the
standard each conforms to is a connection the RDF export writes as triples
(`sysml -convert ttl`); the interface control document is generated from the model; and every
element is reachable through the model API, the REPL and the language server.

## Alternatives considered

- **Guidebook names as the primary vocabulary (`KeyInterface`).** Rejected: the guidebook itself
  notes "key interface" is not a statutory term, and a program is assessed against the statute.
  The alias keeps the practitioner's word without making it the concept.
- **A dedicated `Standard` element kind with its own metaclass.** Rejected: a standard is an item
  the model refers to; `Standard :> Item` lets it be owned, imported, queried and exported like
  any other, and a standards registry is a package of them.
- **Data rights as an attribute of the component definitions.** Rejected: the rights held are a
  contractual fact about the technical data, differ per usage of the same definition (the same
  radio bought under two contracts), and belong to the contracting view rather than the
  structural one; metadata attaches to the usage or the definition as the fact requires.
- **Checks as errors, or unconditional.** Rejected, as for OOSEM: MOSA is worked across a program
  and a model with interfaces but no standards registry yet is not wrong.

## Known limitations

- The approach's *interface repository* — a queryable, version-controlled catalogue of a
  program's modular system interfaces shared across programs — is served here by the model, the
  registers and the RDF export; OpenSysML does not itself host such a repository or expose it
  over OSLC.
- `ModularInterfaceView` renders the interconnection diagram the `Views` renderer draws; a
  reviewer-grade internal block diagram with laid-out ports is not offered because the geometry
  renderer does not draw yet.
- The checks cover the facts the statute names. Not checked: that a standard is in fact
  consensus-based or its registry entry current, that a `@DataRights` matches the contract, or
  that severability claimed in a `#modularityRequirement` holds — those are assessments, and the
  library records them as `@MOSAConformance` rather than judging them.
- The MOSA Implementation Guidebook is written in SysML v1 and commercial-tool terms; this
  library is its SysML v2 reading, not a transcription.
