# NASASTD7009 library — design

Status: **implemented.** `internal/workspace/libs/stdlib/OpenSysML Libraries/NASASTD7009.sysml` —
maintained upstream at Open-MBEE/OpenSysML-Extensions-Library and vendored here pinned by
`scripts/extension-libraries-pin.sh` — is bundled with the other OpenSysML extensions, enters the
same conformance, strict-notation and snapshot gates, and is exercised by
`examples/nasa-std-7009-demo/nasa-std-7009-demo.sysml` (`TestExamplesAnalyseCleanly`).
`passes.NASASTD7009Pass` audits models written against it.

## The problem

NASA-STD-7009B, *Standard for Models and Simulations* (Office of the NASA Chief Engineer,
approved 2024-03-05, [NASA Technical Standards System](https://standards.nasa.gov/standard/NASA/NASA-STD-7009)),
exists "to reduce the risks associated with M&S-influenced decisions". It states 43 numbered
`[M&S n]` requirements over the programmatics, development and use of a model or simulation,
most of them records to maintain; a criticality assessment that says how rigorously the Standard
applies, from the consequence of the decision and the influence the M&S results have on it
(Appendix D); two credibility assessments, of the M&S *capability* at the end of development and
of the M&S *results* at the end of use, each by a fixed set of factors with five level
definitions (Appendix E); what a report of M&S results to decision makers carries with the
results (section 4.3.8); and a Requirements Identification Matrix a project fills in (Appendix A).

A project working in SysML v2 runs its analyses and simulations from the model. What it lacks is
the Standard's vocabulary to state, beside each analysis, how critical the decision is and how
credible the model and its results are, and the tables the Standard asks for generated from the
model rather than kept in a spreadsheet beside it. SysML v2 has no profiles; the vocabulary is a
*library* and the matrices are document queries over the same model.

## Design

**The Standard's names and numbers first.** Every concept keeps the Standard's name, numbering
and, where the Standard defines the term, its words (the text is a work of the United States
Government). The library is non-normative: it describes the Standard, it does not replace it,
and nothing in it is stricter than the Standard's text. Following `NPR7123`, each artefact is a
definition specialising the native SysML type, an abstract base usage subsetting the native base
usage, and a `SemanticMetadata` keyword applying both at once:

```sysml
analysis def 'Heater Sizing' {
    @CriticalityAssessment {
        consequence = DecisionConsequence::significant;
        influence = MSInfluence::significant;
        rating = CriticalityRating::red;
    }
}
#capabilityAssessment item heaterCapability {
    #capabilityFactor item modelValidation {
        :>> factor = CapabilityFactor::msValidation;
        :>> level = FactorLevel::level2;
        :>> threshold = FactorLevel::level3;
    }
}
requirement ms40 : MS40 { @Applicability { applicable = Applicable::yes; recordLocation = "…"; } }
```

| NASA-STD-7009B | Library element |
| --- | --- |
| Section 4, the 43 requirements; Appendix A, Table 1 | `MSRequirements` holds every requirement as `requirement def <'M&S n'> MSn :> MSRequirement` (doc = statement; `id`, `section`, `description`, `area`, `rationale`) |
| Section 4, the recommendations; Appendix B, Table 2 | `MSRecommendations`, one `MSRecommendation` definition per item, identified by section and item letter and worded as Table 2 words them |
| Appendix A and B, applicability | `@Applicability { applicable; comments; recordLocation }` on a requirement or recommendation usage; `RequirementsIdentificationRegister` and `RecommendationsAdoptionRegister` queries; `RequirementsIdentificationMatrix` and `RecommendationsAdoptionMatrix :> Document` |
| Appendix D.2, Table 3, decision consequence | `DecisionConsequence` `negligible`…`catastrophic`, each carrying its level, label and the Table 3 cell of every row |
| Appendix D.3, Table 4, M&S influence | `MSInfluence` `negligible`…`controlling` with the level, the description and the Table 4 conditions |
| Appendix D.1, Figure 1, the criticality matrix | `CriticalityRating` (`green`, `yellow`, `red`); the figure twice: as data, the `yellowFrom` and `redFrom` of each `MSInfluence` row, and as the evaluable `calc def CriticalityOf` |
| [M&S 6], the record of the assessed criticality | `@CriticalityAssessment { consequence; influence; rating; method; basis }` on the analysis, model or decision assessed |
| Appendix E.2, the five assessment levels | `FactorLevel` `level0`…`level4` |
| Appendix E.3, Tables 5–11, the capability factors | `CapabilityFactor`, five literals each carrying its title, section, table, question and the evidence of levels 0 to 4 |
| Appendix E.4, Tables 12–20, the results factors | `ResultsFactor`, six literals in the same shape |
| [M&S 48], [M&S 31], the two assessment records | `MSCapabilityAssessment` and `MSResultsAssessment :> Item`, nesting one `CapabilityFactorAssessment` or `ResultsFactorAssessment` (`factor`, `level`, `threshold`, `evidence`) per factor; `CapabilityAssessmentRegister` and `ResultsAssessmentRegister` tabulate the outcome |
| Section 4.3.8, [M&S 32]–[M&S 39], [M&S 50], reporting | `MSResultsReport :> Document` (`uncertaintyEstimate : UncertaintyEstimateKind`, `uncertaintyProcess`, `technicalReviewFindings`, `qualifications`, `riskAcceptance`); `ResultsWarning` with `kind : WarningOccurrence` (the eight occurrences of [M&S 32]) and `impact` |
| [M&S 26] b, placarding | `@Placard { limitExceeded; extent; consequences }` |
| Appendix F, the M&S life cycle | `MSLifeCyclePhase` |

**The criticality matrix is evaluable.** `CriticalityOf(consequence, influence)` returns the cell
of Figure 1, so a model computes its rating instead of copying it:
`sysml -e "NASASTD7009::CriticalityOf(NASASTD7009::DecisionConsequence::moderate, NASASTD7009::MSInfluence::controlling)"`
answers `red`. Appendix D lets a program adjust the matrix; a project that does specialises the
calculation and records its `method`.

**Matrices.** `RequirementsIdentificationMatrix` is a `DocumentQueries::Document` whose section
tabulates `RequirementsIdentificationRegister`: the register walks the descendants of `root`,
selects the `MSRequirement` usages carrying `@Applicability`, and projects Table 1's columns —
Section, Description, Requirement in this Standard, Applicable, Comments — with the requirement
identifier and the record location. The statement is the *definition's* documentation, read
through the member path `type.documentation`, so the Standard's text is stated once.

**Checks.** `passes.NASASTD7009Pass` (constraint tier, source `nasastd7009`) audits a model
against what the Standard states of the elements it classifies. Every finding is a **warning** —
an M&S between development and use is a normal state of the work — and every rule applies only
to an element of the kind the Standard speaks about:

| Code | Rule | NASA-STD-7009B |
| --- | --- | --- |
| `nasastd7009-assessment-missing-factor` | an `MSCapabilityAssessment` or `MSResultsAssessment` usage nests, at any depth, a factor assessment stating a `level` for each of its five or six factors; one finding per record, naming the factors | [M&S 48], [M&S 31] |
| `nasastd7009-report-missing-content` | an `MSResultsReport` usage includes both assessments, an `uncertaintyEstimate` with its `uncertaintyProcess` (none when the estimate is stated not available), the `qualifications` and the `riskAcceptance`; one finding per report | [M&S 50], [M&S 35], [M&S 33], [M&S 34], [M&S 37], [M&S 39] |
| `nasastd7009-report-missing-warning` | a report holding a factor assessed below its `threshold` nests a `ResultsWarning` of kind `unachievedAcceptanceCriteria` | [M&S 32] a, [M&S 43] e |
| `nasastd7009-warning-missing-impact` | a `ResultsWarning` states a non-empty `impact` | [M&S 32] |
| `nasastd7009-criticality-rating-differs-from-matrix` | a `@CriticalityAssessment` whose `rating` is not the cell of Figure 1 states its `method` | §4.1.1.2, Appendix D |
| `nasastd7009-requirement-entry-incomplete` | an `@Applicability` marked not applicable states its `comments`; one marked applicable on a requirement states its `recordLocation` | Appendix A.1, §4.3.8.8 |
| `nasastd7009-matrix-missing-requirement` | the document holding a `RequirementsIdentificationMatrix` usage has an `@Applicability`-annotated usage of every requirement definition; one finding per matrix, naming the missing ids | Table 1 |

Classification follows the typing or the keyword, so `item a : MSCapabilityAssessment`,
`#capabilityAssessment item a` and a usage of a project's own definition specialising it are all
capability assessments, and a report reads the assessments it subsets
(`item capability :> heaterCapability;`) as its own. Library documents are not audited, and a
model that never touches `NASASTD7009` gets no finding; a workspace package that merely reuses
the name is not the vocabulary. A rule the Standard does not state is not added at any severity:
"This Standard levies no requirements with respect to what levels to achieve", so the pass sets
no threshold, demands no criticality assessment of an analysis that states none, and does not
ask for technical reviews, which the Standard does not require.

## How OpenSysML serves the Standard

The Standard speaks about the analyses and simulations a project runs, and OpenSysML runs them
from the model. What each run already records lines up with the records the Standard asks for:

| Record | OpenSysML |
| --- | --- |
| The inputs to the M&S and the setup of the run ([M&S 24], [M&S 25]) | the analysis case's bound parameters in the model; `AnalysisRecords::RecordedRun` records when a run happened, the tool and the command |
| Warning and error messages of the use ([M&S 27]) | the run's diagnostics and verdicts, kept with the recorded run |
| Uncertainty in the inputs and results ([M&S 28], [M&S 29]) | `Simulation` Monte Carlo runs over `RandomFunctions` draws and `Stochastic` branch probabilities |
| Sensitivity analyses ([M&S 30]) | sweeps of an analysis case over its parameters |
| The assessments, the report and the matrices ([M&S 48], [M&S 31], §4.3.8, Appendix A) | this library; `sysml -render-document` writes the report and the matrices as Markdown, HTML or PDF, and `-validate` reports the gaps as warnings with the requirement each comes from |

## Alternatives considered

- **Factor levels as integers.** Rejected: Appendix E.2 calls the 0–4 index an illustration of a
  five-tier labelling a program may relabel, and says levels are not to be compared or combined
  across factors; an enumeration a project can read the level definitions off keeps that.
- **One definition per factor.** Rejected: the factors are a closed list the two requirements
  enumerate, and a literal carrying its level definitions lets one `factor` feature state which
  is assessed and the pass name what is missing.
- **The criticality matrix only as a calculation.** Rejected: a pass does not evaluate
  calculations, and a tool reading the figure needs it as data; the rows carry it and the
  calculation is checked against them cell by cell (`TestNASASTD7009CriticalityRating` reads the
  rows, the library's own evaluation agrees on all 25 cells).
- **Checks as errors, or unconditional.** Rejected, as for `NPR7123`: the Standard is applied by
  criticality and tailored with the Technical Authority's approval.

## Known limitations

- Most requirements ask that a record be *maintained*; the library records where
  (`recordLocation`) and the pass checks the entry, not the record's content.
- Table 2 omits section 4.2.2 item b and so letters the following items one earlier than the
  section does; the library keeps the section's lettering and all nine items.
- Sections 4.2.3 and 4.2.5 give [M&S 15]/[M&S 16] and [M&S 17]/[M&S 18] one rationale each; both
  definitions of a pair carry it.
- The rule that a validation level above 1 needs the conditions of level 1, and a
  process/product management level of 3 or 4 those of level 2, is a judgement about evidence and
  is documented on the factors, not checked.
- The pass audits the document that holds the matrix; entries a project keeps in a second file
  are not seen by the matrix rule, so keep the matrix beside its entries.
- The definitions of section 3.2 and NASA-HDBK-7009's guidance are not transcribed.
