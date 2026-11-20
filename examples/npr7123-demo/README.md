# NPR 7123.1 demo

[`npr7123-demo.sysml`](npr7123-demo.sysml) works a small fictional project — the *Dust Lidar
Pathfinder*, a lunar-surface dust lidar on a lander — through NASA NPR 7123.1D using the bundled
[`NPR7123`](../../docs/project/npr7123-library.md) library. Each artefact carries the NPR's
keyword, which both types it and adds it to the NPR's base usage, and each NPR fact is stated
where the NPR expects it:

```sysml
#semp part pathfinderSEMP : SEMP {
    :>> maturity = WorkProductMaturity::baseline;
    #technicalPlanning action planTechnicalEffort {
        :>> tailoring = "Fully applied; the SEMP is this document.";
    }
}
#lifeCycleReview action pdr {
    :>> kind = ReviewKind::PDR;
    requirement pdrEntrance1 : ReviewCriteria::PDR::Entrance1 :> entranceCriteria;
    #verificationAndValidationPlan item vvPlanAtPDR :> reviewedProducts {
        :>> maturity = WorkProductMaturity::preliminary;
    }
}
requirement se07 : SE07 { @Compliance { comply = ComplyKind::fullyCompliant; } }
```

| Package | What it holds |
| --- | --- |
| `Technical Planning` | the `#semp` SEMP addressing all 17 common technical processes with its tailoring (SE-58), an earlier SEMP outline, the `#mop` mass and power margins and the `#moe` dust detection rate |
| `Work Products` | the §5.2.2.2 minimum products the project has produced so far — the stakeholder expectations, concept of operations, requirement set, design solution, V&V plan and their companions — each at its current `WorkProductMaturity` |
| `Life-Cycle Reviews` | the `#lifeCycleReview`s SRR, PDR and CDR with tailored Appendix G entrance and success criteria, the minimum products of §5.2.2.2 under `reviewedProducts`, a `#peerReview`, and the lidar transmitter's `@TechnologyReadiness` (TRL 6 at PDR) |
| `Compliance Entries` | one `@Compliance`-annotated usage per active SE requirement — `FC`, `T` or `NA` with its justification — plus the `#technicalAuthority` and the `@Deviation` the tailored entries cite |
| `Compliance Matrix` | a `ComplianceMatrix` document whose table is Table H-1 generated from the entries |

## From the command line

Analyse the model:

```bash
./bin/sysml examples/npr7123-demo/npr7123-demo.sysml -validate
```

The model has no errors, and `-validate` also runs the NPR 7123.1D checks, which report the five
gaps the model leaves open on purpose (each is marked with a `doc` comment):

| Warning | Gap |
| --- | --- |
| `npr7123-semp-missing-process` | `sempOutline` addresses only the four system design processes (SE-58) |
| `npr7123-review-missing-minimum-product` | `vvPlanAtPDR` is `preliminary` where §5.2.2.2 (SE-68) asks for `baseline`; the `cdr` states no design solution (SE-46) |
| `npr7123-review-missing-entrance-or-success-criteria` | the `cdr` states no entrance or success criteria (SE-34) |
| `npr7123-compliance-missing-justification` | `se65` is tailored with no `justification` |
| `npr7123-compliance-matrix-missing-requirement` | no compliance entry covers SE-31 |

```text
examples/npr7123-demo/npr7123-demo.sysml:75:3: warning[npr7123-semp-missing-process]: This SEMP does not address 13 of the 17 common technical processes: ProductImplementation, ProductIntegration, Pro
		#semp part sempOutline : SEMP {
  ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/npr7123-demo/npr7123-demo.sysml:180:4: warning[npr7123-review-missing-minimum-product]: This VerificationAndValidationPlan is stated at maturity preliminary, below the baseline NPR 7123.1D se
			#verificationAndValidationPlan item vvPlanAtPDR :> reviewedProducts {
   ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/npr7123-demo/npr7123-demo.sysml:186:3: warning[npr7123-review-missing-minimum-product]: This CDR states no DesignSolution: NPR 7123.1D section 5.2.2.2 (SE-46) has the technical team provide i
		#lifeCycleReview action cdr {
  ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/npr7123-demo/npr7123-demo.sysml:186:3: warning[npr7123-review-missing-entrance-or-success-criteria]: This life-cycle review states no entrance criteria and no success criteria: NPR 7123.1D se
		#lifeCycleReview action cdr {
  ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/npr7123-demo/npr7123-demo.sysml:398:4: warning[npr7123-compliance-missing-justification]: This compliance entry is marked tailored with no justification: NPR 7123.1D Table H-1 asks for the ju
			requirement se65 : SE65 {
   ^~~~~~~~~~~~~~~~~~~~~~~~~
examples/npr7123-demo/npr7123-demo.sysml:441:3: warning[npr7123-compliance-matrix-missing-requirement]: This compliance matrix has no compliance entry for 1 active NPR 7123.1D requirement(s): SE31. Ta
		part complianceMatrix : 'Pathfinder Compliance Matrix';
  ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
✓ package 'Dust Lidar Pathfinder'
✓ examples/npr7123-demo/npr7123-demo.sysml: no errors
```

Record the missing fact — the processes, a `baseline` maturity, the criteria, a `justification`,
the SE-31 entry — and the warning goes away.

Render the compliance matrix (Markdown by default; `-doc-form html` or `pdf` for the others):

```bash
./bin/sysml examples/npr7123-demo/npr7123-demo.sysml \
    -render-document "'Dust Lidar Pathfinder'::'Compliance Matrix'::'Pathfinder Compliance Matrix'"
```

The document's one table is Table H-1 over the project's entries, 61 rows for the 62 active
requirements (SE-31 is the deliberate gap); the statement and rationale columns are abridged here:

```markdown
| Req ID | NPR Section | Requirement Statement | Rationale | Comply? | Justification |
| --- | --- | --- | --- | --- | --- |
| SE-06 | 6.1.5 | The ETA shall approve the SEMP, waiver or deviation authorizations, an… | This requirement ensures that the ETA has reviewed and approved of key… | fullyCompliant |  |
| SE-07 | 3.2.2.1 | Program/Project Managers shall identify and implement an ETA-approved… | This requirement ensures that the program/project and the ETA identifi… | fullyCompliant |  |
| SE-08 | 3.2.3.1 | Program/Project Managers shall identify and implement an ETA-approved… | This requirement ensures that the program/project and the ETA identifi… | fullyCompliant |  |
| SE-09 | 3.2.4.1 | Program/Project Managers shall identify and implement an ETA-approved… | This requirement ensures that the program/project and the ETA identifi… | fullyCompliant |  |
| … | | | | | |
| SE-65 | 5.2.1.3 | The technical team shall develop and document an approach to Human Sys… | The HSI approach ensures that system aspects that interface with human… | tailored |  |
| SE-69 | 5.2.2.2.g(1) | The technical team shall provide the following minimum products at the… | At ORR the project should have at least preliminary results from the V… | fullyCompliant |  |
```

The requirement statement is read from the library's requirement definition through the member
path `type.documentation`, so the NPR's text is stated once, in the library.
