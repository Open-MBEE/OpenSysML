## What We Don't (Yet) Support

### Major Features Not Implemented (UML-referenced; no SysML v2 notation or KerML performance)

The remaining entries below are genuine implementation work or deliberate language/design
boundaries; the landed Track E behavior is recorded in the execution rows and the roadmap.

**Actions (Advanced):**
- Expansion regions — closed, not to be implemented ([design record](expansion-regions.md)): the iterative form is `for` (§7.17.12, `Actions::ForLoopAction`), which runs in every body position; the parallel form is not SysML v2 — a multiplicity on a performed action usage with a `flow` delivering a collection to its input is not a standard spelling of per-element concurrent performance (SysML v2 §7.17.2, §8.4.13.2; KerML §7.4.7, Annex A.3.6; `Performances.kerml`, `Transfers.kerml`), no OMG corpus model writes one, and the pinned pilot performs no actions. The runtime performs such a node once per token (`runtime/action_frame.go` `beginPerformance`) and refuses a collection delivered to a one-valued pin (`runtime/write_conformance.go` `checkTargetAs`, `ErrMultiplicityViolation`); per-element concurrency is written as distinct nodes under a `fork`
- Exception handlers — closed, not a SysML v2 construct ([design record](exception-handlers.md)): UML's `RaiseExceptionAction`/`ExceptionHandler` have no spelling in §7.17 (no action kind raises, catches or propagates), no metaclass in §8.3.17 or the reflective `SysML.sysml` metamodel, no base type in `Actions.sysml`, and no counterpart in KerML — a `Performance` (`Performances.kerml`) ends but does not fail; no OMG corpus model writes one and the pinned pilot's `SysML.ecore` (175 classes) has none. A failure in SysML v2 is modeled, then handled with the ordinary constructs the runtime executes: an `out` result routed by `decide` (§7.17.3), or a failure signal `send` to an `accept` forked beside the work, whose branch `terminate`s the work (§7.17.7, §7.17.8, §7.17.10 `MonitoredActivity`; `runtime/action_terminate.go`, `runtime/action_executor.go` `acceptMatch`, `stepDecisionNode`); a failure the model does not spell is a typed error at the boundary (`ErrDivisionByZero`, `ErrAcceptDeadlock`, …), never a panic, and an `error` verdict under a verification case
- Structured activities with pin connectors

**State Machines (Advanced):**
- Protocol state machines — **closed by design record and not a SysML v2 construct**; see
  [protocol-state-machines.md](protocol-state-machines.md). The order of *receptions* on a port
  or part is an ordinary exhibited state machine (§7.18.4, `accept … via` §7.17.8), which runs
  today on parts (*Classifier Behaviors*, `state_transition_accept_via_port`). ⚠️ The order it
  declares is enforced only for a directly injected event (`StateExecutor.sendSignal`: dropped
  and reported in `AdvanceReport.Dropped`; the REPL's `%send` refuses it) and for a message a
  model sends to the object that nothing of the object takes (`state_executor.go:dropsMessage`:
  taken off the bus once the object is settled and dropped, reported the same way); a message
  routed to a port stays on the context-wide bus, and a machine exhibited by a port definition
  does not take a model's messages routed to that port. The optional runtime follow-up is not a
  specification gap (port-machine routing; an optional typed error). UML's post-conditions, `ProtocolConformance`, static sequence checking
  and the gating of *operation calls* by state have no SysML v2 spelling and are not tracked as
  missing.

**Object Model:**

**Type System** — the four items once listed here were stale; the pilot's named specialization, redefinition and subsetting constraints are ✅ in [validation-constraints.md](validation-constraints.md) and only the two rows below remain approximate:
- Specialization validation — done. Kind restrictions on specialization (KerML 1.1 §8.3.3.2 class/datatype/structure/behavior: `internal/check/passes/w11a_kerml_specialization.go:W11AKerMLSpecializationPass.Run`, rejection cases k20–k23), binary association and connector specialization (`constraint.go:checkBinaryConnectorEnds`, k24/k26), the default supertype (`w11e_implicit_base.go:implicitBaseChecker.checkDefaultSupertype`, k46 — ⚠️ stricter than the pilot: a two-end association must also reach `Links::BinaryLink`, KerML §8.3.4.7), conjugation (`validateSpecificationSpecificNotConjugated` (pilot `validateSpecializationSpecificNotConjugated`), `validateTypeAtMostOneConjugator`), SysML definition/usage kind typing (`typecheck.go:compatibleTyping`, `one_type.go:checkOneType`, `w8d_occurrence_typing.go`) and variation specialization (`w8d_variability.go:W8DVariabilityPass.Run`). "Generic" types do not exist in KerML or SysML v2 (no type parameters or templates), so nothing is missing under that heading.
- Redefinition conformance — done. Featuring types (`w10b_redefinition.go:checkW10BRedefinition`, p28/p32), end conformance (same, k15), direction (`w8b_redefinition_conformance.go:RedefinitionDirectionPass.Run`, p31), constant and uniqueness (`w8b_redefinition_conformance.go:RedefinitionConformancePass.Run`, k14/s108/p04), lower-bound narrowing as a warning like the pilot (`multiplicity_conformance.go:constraintChecker.checkMultiplicityConformance`, probe `validateRedefinitionMultiplicityConformance` (pilot-only check)), overriding a bound value (`feature_value_overriding.go`), plus the advisory `disjoint-feature-types` warning when two of the feature's effective types are provably disjoint (`disjoint_feature_types.go:checkDisjointFeatureTypes`, `disjoint_feature_types_test.go`), which the pilot does not check; a merely unrelated declared type is silent, as in the pilot.
- Subsetting validation — done. Featuring types (`subsetting_featuring.go:checkSubsettingFeaturingTypes`, k13), upper-bound widening as a warning (`multiplicity_conformance.go`, probe `validateSubsettingMultiplicityConformance` (pilot-only check)), constant/uniqueness (as above), reference and cross subsetting (`w8c_reference_subsetting.go`, `w10b_cross_features.go:checkW10BCrossFeatures`, k09/k16–k19/k42/k54/s88), flow-end subsetting (`w8d_flow_end.go:W8DFlowEndPass.Run`, k38), and a feature-owned multiplicity's featuring types (`multiplicity_domain.go:MultiplicityDomainPass.Run`, probe `validateFeatureMultiplicityDomain`). `validateSubsettingPortionConformance` (pilot-only check) is a no-op in the pinned pilot, so there is nothing to referee; `validateFlowEndImplicitSubsetting` (pilot-only check) is ⚠️ — the pilot warns where OpenSysML rejects the same end as an inaccessible feature.
- ~~Interface realization~~ — removed: UML's `InterfaceRealization` has no SysML v2 counterpart; an `interface def` is a connection between ports (§7.13.3, ends checked by `w10b_ends.go:W10BEndKindPass.Run`, typing by `typecheck.go:compatibleTyping`) and the two sides of an interaction are expressed by port conjugation (`~PortDef`), not by realizing a contract.

**Advanced SysML v2:**
- Allocation execution semantics beyond materializing the allocation and its ends

### What Can't Be Claimed for Spec Compliance

**Intentionally Unspecified (No Normative Semantics):**
- Verification verdict evaluation (VerdictKind/PassIf) — SysML v2 §9.2.17 defines the verdict only through the library: "PassIf returns a pass or fail VerdictKind depending on whether its argument is true or false" (§9.2.17.2.1); the specification states no further evaluation semantics for a verification case body. OpenSysML *does* report a verdict — it runs the case body as it runs an analysis case body and reports the `VerdictKind` that run produced, computed by the library's own `PassIf` calculation where the body calls it, `inconclusive` where the body produces no verdict value and `error` where the run failed — but that reading of the body is this tool's, not a normative one, and it is reported beside the requirement-satisfaction verdicts rather than replacing them; nested subcases are reported individually because the library states no roll-up. See the Verification Case map
- Variability/variation selection — SysML v2 §7.6.7 defines how variations and variants are declared and constrained; it defines no operation that selects a variant. OpenSysML selects the variant a variation usage is bound to (`attribute :>> cut = cut::cutIdeal;`) and errors on an unselected, unknown, or multiply-selected variation; see the Variation and Variant map
- View/viewpoint rendering - SysML v2 §7.26.4: "there are no specific constructs provided in SysML for specifying" how a view is rendered; "conforming tools can provide libraries of rendering usages that reflect the capabilities they provide". OpenSysML *does* render a view — a tree, an interconnection diagram, a state machine, an action flow, case and mixed diagrams, a sequence diagram, a table or a `GridView` relationship matrix, as text, Mermaid, Markdown or delimited records (`%render`, `sysml -render`; see the rendering rows) — but the artifact itself is this tool's output, not a normative form, and the kinds it does not produce are a typed `view.UnsupportedKindError` rather than a substituted rendering. The matrix admits unnamed members through its opt-in exposure path; ordinary renderings continue to omit unnamed exposed members. The viewpoint conformance verdicts OpenSysML reports are likewise tool-defined (see the viewpoint conformance row)
- Case and mixed renderings extend the tool-defined view kinds with use, analysis and verification cases and a shared canvas for structural, state, action and case nodes; their text, Mermaid, DOT and PlantUML writers preserve the relationships represented by each form (`view/case.go` `renderCase`, `view/mixed.go` `renderMixed`; `view/text.go`, `view/mermaid.go`, `view/dot.go`, `view/plantuml.go`; `view/case_mixed_test.go`, `view/testdata/{case,mixed}.*.golden` and the CLI, REPL, LSP and docplan rendering tests). Implemented as a tool-defined rendering extension, claiming nothing normative.
- Allocation execution - SysML v2 §7.22 defines allocation usages as connections and §9.2.8 types them (`Allocation :> BinaryConnection`); neither clause gives an allocation any run-time behavior beyond that of a connection, so materializing the allocation and its ends is all there is to execute

**Implementable But Not Yet Done:**
- None. The last entry here, exception handlers, closed as not a SysML v2 construct: the
  specification that has one is UML's, and "propagation" is a rule about that construct
  ([design record](exception-handlers.md), and the *Actions (Advanced)* bullet above).

**No External Referee Exists (the limit of the evidence, not of the implementation):**
- **Behavioral execution is self-assessed.** The pinned OMG pilot implementation's only execution
  surface is model-level expression evaluation, so token-flow ordering, transition selection,
  concurrency and quiescence are checked against the specification text, the normative library and
  our own traces — not against another implementation. Sensmetry SysIDE is a checker and can
  corroborate static rows only. Every ⚠️ in the Action, State Machine and Classifier Behaviors maps
  should be read with that in mind.
- **The OMG corpora are demonstrations, not a conformance suite.** They were written to show the
  notation off. Agreement over them means we agree where they happen to look; there is no official
  SysML v2 conformance test suite to run.
- **The differential is one-directional.** It finds notation the reference accepts and we reject.
  Notation *we* accept and the reference does not is only visible when a corpus file happens to
  contain it — so our permissiveness is largely unmeasured, and `examples/` still carries
  constructs no pinned production admits (see the ten-class map above).

---

### GridView Relationship Matrix

| Rule | Implementation | Tests | Status |
| --- | --- | --- | --- |
| A standard `GridView` with a resolved positive relationship selector renders the selected relationships as a matrix. SysML metaclass selectors match by identity; refinement and derivation metadata selectors also match specializations. `render asElementTable`, unknown selectors, and selectors nested under `not` retain table rendering. Matrix exposure uses `Model.ExposedMembers` and admits unnamed members through the view's own and inherited filters; ordinary tables and renderings continue to use `Model.ExposedElements`. Relationship edges are extracted once in semantics and query execution delegates to that model API. Rows and columns follow first edge occurrence; cells deduplicate keywords in the fixed order `satisfy`, `verify`, `allocate`, `connect`, `derive`, `refine`, `dependency`. Direct matrix output supports text, Markdown, CSV and TSV; `#matrix` and `#matrix:<target>` use the same rendering. Document Diagram blocks treat matrices as tables for every diagram form. | `semantics/relationship_edges.go` `Model.RelationshipEdgesOf`; `semantics/expose.go` `Model.ExposedMembers`, `Model.ViewExposureConditions`; `resolve/filter.go` `ImportedMembersInto`, `ViewExposureConditions`; `doc/queryexec/related.go` delegates relationship scans; `view/view.go` `Renderer.KindOf`, `Renderer.Render`, `Renderer.RenderExposed`; `view/matrix.go` `matrixShownKinds`, `renderMatrix`, `matrixEdges`; `docrender/markdown.go`, `docrender/html.go` | `semantics/relationship_edges_test.go`; `semantics/expose_test.go`; `view/matrix_test.go` and `view/testdata/matrix.sysml`; `cmd/sysml/render_test.go`; `repl/view_render_test.go`; `lsp/render_test.go`; `docplan/diagram_test.go`; `docrender/diagram_test.go`; `docrender/html_diagram_test.go` | ✅ Implemented (the ordinary exposed-element path is preserved; unnamed-member enumeration is opt-in to matrix rendering) |
