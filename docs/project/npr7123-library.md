# NPR7123 library — design

Status: **implemented.** `internal/workspace/libs/stdlib/OpenSysML Libraries/NPR7123.sysml` —
maintained upstream at Open-MBEE/OpenSysML-Extensions-Library and vendored here pinned by
`scripts/extension-libraries-pin.sh` — is bundled with the other OpenSysML extensions, enters the
same conformance, strict-notation and snapshot gates, and is exercised by
`examples/npr7123-demo/npr7123-demo.sysml` (`TestExamplesAnalyseCleanly`). `passes.NPR7123Pass`
audits models written against it.

## The problem

NASA NPR 7123.1D, *NASA Systems Engineering Processes and Requirements* (Office of the Chief
Engineer, effective 2023-07-05, [NODIS](https://nodis3.gsfc.nasa.gov/displayDir.cfm?Internal_ID=N_PR_7123_001D_&page_name=main)),
is the procedural requirement every NASA program and project's systems engineering is assessed
against. It names 17 *common technical processes* in three groups, each with one `[SE-nn]`
requirement; the life-cycle reviews a project passes through, with entrance and success criteria
(Appendix G) and the minimum work products each review presents at a stated maturity
(§5.2.2.2, Table 5-1, Appendix F); the Technology Readiness Levels (Appendix E); the Systems
Engineering Management Plan that records how the processes are tailored (§6, SE-58); the
Technical Authority, waivers and deviations of chapter 2; and in Appendix H the *compliance
matrix* — one row per active requirement with `FC`, `T` or `NA` and a justification — that a
project fills in and the Center reviews.

A project working in SysML v2 already has the processes, the reviews, the products and the
requirements in its model. What it lacks is the NPR's vocabulary to classify them with, and the
table the NPR asks for generated from the model rather than kept in a spreadsheet beside it.
SysML v2 has no profiles; the vocabulary is a *library* and the compliance matrix is a document
query over the same model.

## Design

**The NPR's names and numbers first.** Every concept keeps the NPR's name, numbering and, where
the NPR defines the term, its words (the text is a work of the United States Government). The
library is non-normative: it describes the NPR, it does not replace it, and nothing in it is
stricter than the NPR's text. Following `MOSA` and `OOSEM`, each artefact is a definition
specialising the native SysML type, an abstract base usage subsetting the native base usage, and
a `SemanticMetadata` keyword applying both at once:

```sysml
#technicalPlanning action plan;                       // a TechnicalProcess, in technicalProcesses
#lifeCycleReview action pdr { :>> kind = ReviewKind::PDR; }
#verificationAndValidationPlan item vvPlan :> reviewedProducts {
    :>> maturity = WorkProductMaturity::baseline;
}
```

| NPR 7123.1D | Library element |
| --- | --- |
| §3.2.2–3.2.18, the 17 common technical processes, three groups | `TechnicalProcess :> Action` with `group : TechnicalProcessGroup` (`systemDesign`, `productRealization`, `technicalManagement`), `requirementId`, `section`, `tailoring`; one definition, base usage and keyword per process (`StakeholderExpectationsDefinition` … `DecisionAnalysis`), each doc carrying the NPR's "is used to…" text and its SE id |
| Ch. 2, Technical Authority, tailoring, waivers and deviations | `TechnicalAuthority :> Part` with `TechnicalAuthorityKind`; `@Waiver`, `@Deviation` (the NPR's own distinction: tailoring seeks relief by waiver or deviation, customizing selects how a requirement is implemented and needs neither) |
| §5.1, life cycle and key decision points | `LifeCyclePhase` and `KeyDecisionPoint`, kept thin: NPR 7123.1D illustrates the life cycles and defers them to NPR 7120.5 |
| §5.2.2, life-cycle and technical reviews | `TechnicalReview :> Action` (`entranceCriteria`, `successCriteria`, `reviewedProducts`, `items` for RFAs/RIDs/actions), `LifeCycleReview` (`kind : ReviewKind`, `phase`, `keyDecisionPoint`), `PeerReview`; `ReviewKind` enumerates every Appendix G review by acronym |
| Appendix G, Tables G-1…G-21, entrance and success criteria | `ReviewCriteria::<acronym>::Entrance<n>` / `Success<n>` requirement definitions (`EntranceCriterion`, `SuccessCriterion`, `number`), one per criterion; a project subsets or redefines them — G.1.2 expects tailoring |
| §5.2.2.2, Table 5-1, Appendix F, work products and maturity | `WorkProduct :> Item` with `maturity : WorkProductMaturity` (`initial`, `final`, `update`; `preliminary`, `baseline`, `approve`; `approach`); one item definition per Table 5-1 product carrying a `@MaturityExpectation { review; maturity; required; requirementId }` per table cell, `required = true` for the §5.2.2.2 minimum products |
| Appendix E, Technology Readiness Levels | `TechnologyReadinessLevel` `trl1`…`trl9` with the definition and success-criteria text; `@TechnologyReadiness { level; assessedAt; basis }` |
| Ch. 6, the SEMP; SE-58 | `SEMP :> Document, WorkProduct`; a project nests one usage per process under it, recording the tailoring |
| Appendix H, Table H-1; Appendix J | `SERequirements` holds every active requirement as `requirement def <'SE-nn'> SEnn :> SERequirement` (doc = statement; `id`, `section`, `rationale`); SE-01…05, SE-49 and SE-50 are deleted and not modelled |
| Appendix H, compliance | `ComplyKind` (`fullyCompliant` "FC", `tailored` "T", `notApplicable` "NA"), `@Compliance { comply; justification }` on a requirement usage, `ComplianceEntries`/`ComplianceRegister` queries, `ComplianceMatrix :> Document` |
| §6.2.1 TPMs, MOEs, leading indicators | the standard `ParametersOfInterestMetadata` (`#moe`, `#mop`), re-exported rather than redefined |

**Compliance matrix.** `ComplianceMatrix` is a `DocumentQueries::Document` whose one section
tabulates `ComplianceRegister`: a model specialises it and binds `root` to the usage holding its
compliance entries (`calc rows : ComplianceRegister { in :>> root = complianceEntries; }`). The
register walks the descendants of `root`, selects the usages carrying `@Compliance`, and projects
Table H-1's columns — Req ID, NPR Section, Requirement Statement, Rationale, Comply?,
Justification. The statement is the *definition's* documentation, read through the member path
`type.documentation`: a member-path segment naming no declared member reads the element-valued
metaclass feature of that name (`internal/doc/queryexec/memberpath.go`), which is what makes the
documentation of `SE07` reachable from a usage `requirement se07 : SE07`.

**Checks.** `passes.NPR7123Pass` (constraint tier, source `npr7123`) audits a model against what
the NPR states of the elements it classifies. Every finding is a **warning** — a project between
reviews is a normal state of the work — and every rule applies only to an element of the kind the
NPR speaks about:

| Code | Rule | NPR 7123.1D |
| --- | --- | --- |
| `npr7123-review-missing-entrance-or-success-criteria` | a `LifeCycleReview` usage nests an `EntranceCriterion` and a `SuccessCriterion` requirement usage; one stating neither, or only one kind, is reported once | §5.2.2.1, SE-34 |
| `npr7123-review-missing-minimum-product` | a review of a kind with minimum products states each under `reviewedProducts`, at or above the required maturity (initial/preliminary < approve/baseline < final/update); an absent product is reported on the review, one below maturity on the product | §5.2.2.2, SE-35…SE-56, SE-66…SE-69; Table 5-1 |
| `npr7123-compliance-missing-justification` | a `@Compliance` marked `tailored` or `notApplicable` states a non-empty `justification` | Table H-1 |
| `npr7123-compliance-matrix-missing-requirement` | the document holding a `ComplianceMatrix` usage has a `@Compliance`-annotated usage of every active SE requirement definition; one finding per matrix, naming the missing ids | Table H-1 |
| `npr7123-semp-missing-process` | a `SEMP` usage nests, at any depth, one usage of each of the 17 process definitions; a SEMP a review lists under `reviewedProducts` is a stand-in and is not audited | §6.2.3, SE-58 |

Classification follows the typing or the keyword, so `action pdr : LifeCycleReview`,
`#lifeCycleReview action pdr` and a usage of a project's own `action def PDR :> LifeCycleReview`
are all life-cycle reviews. Library documents are not audited, and a model that never touches
`NPR7123` gets no finding; a workspace package that merely reuses the name is not the vocabulary.
A rule the NPR does not state is not added at any severity: the pass does not, for instance,
demand a TRL, a Technical Authority or a review of every kind.

**A Center implementation layers on top.** NPR 7123.1D §1.2 and Appendix H expect each Center to
implement the NPR with its own procedures. A Center writes a specialising library package rather
than editing this one: its review definitions specialise `LifeCycleReview` and fix the `kind`;
its tailored criteria subset or redefine the `ReviewCriteria` definitions; its templates
specialise the work-product definitions with the Center's names; and its compliance matrix
specialises `ComplianceMatrix`, adding columns for the Center's own requirements. The pass keeps
reading the NPR facts through the specialisations, so a Center's models are audited against the
NPR without a Center-specific pass.

## How OpenSysML serves the NPR

What the NPR asks a project to *show* — the SEMP's tailoring of each process (SE-58), the
reviews' criteria and products (§5.2.2), and the compliance matrix (Appendix H) — is generated
from the model: `sysml -render-document` writes the compliance matrix as Markdown, HTML or PDF,
`-validate` reports the gaps as warnings with the NPR section each comes from, and every element
is reachable through the model API, the REPL and the language server.

## Alternatives considered

- **One requirement definition per Table 5-1 cell.** Rejected: the maturity a product reaches at a
  review is a fact about the product, so it is an annotation on the product definition, and the
  minimum products of §5.2.2.2 are the cells marked required, not a parallel list.
- **Entrance and success criteria as plain documentation on the review kind.** Rejected: G.1.2
  says projects tailor them, and a requirement definition per criterion is what a project can
  subset, redefine, verify and trace.
- **Statements read through a `documentation` column on the usage.** Rejected: a usage's
  documentation is its own; the NPR's statement lives once, on the definition, and the member path
  `type.documentation` reads it there without copying it into every entry.
- **Checks as errors, or unconditional.** Rejected, as for MOSA and OOSEM: a project before its
  SRR has no baselined requirements yet and is not wrong.

## Known limitations

- The life cycles themselves — phases, KDPs, which review closes which phase — are NPR 7120.5's
  and NPR 7120.8's; `LifeCyclePhase` and `KeyDecisionPoint` are placeholders a program library
  fills in.
- Table H-1 prints SE-59…SE-64 in one row with one rationale; the library gives each its own
  statement from §6.2.5–6.2.9 and the part of the shared rationale that speaks to it.
- The pass audits the document that holds the compliance matrix; entries a project keeps in a
  second file are not seen by the matrix rule, so keep the matrix beside its entries.
- Center implementations, the program-level SRR/SDR tables of Appendix G applied to programs
  rather than projects, and the contracted-project requirements of chapter 4 (SE-24…SE-31) are
  vocabulary only: the NPR states no model-checkable fact about them.
