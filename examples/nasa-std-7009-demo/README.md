# NASA-STD-7009 demo

[`nasa-std-7009-demo.sysml`](nasa-std-7009-demo.sysml) works a small fictional use of a model —
the *Lunar Night Survival Heater*, a lumped thermal model sizing the heater that keeps an
electronics box alive through the lunar night — through NASA-STD-7009B, the Standard for Models
and Simulations, using the bundled [`NASASTD7009`](../../docs/project/nasastd7009-library.md)
library. Each fact is stated where the Standard expects it:

```sysml
analysis def 'Heater Sizing' {
    @CriticalityAssessment {
        consequence = DecisionConsequence::significant;
        influence = MSInfluence::significant;
        rating = CriticalityRating::red;
    }
    return heaterPower : Real = margin * HeaterPower(conductance, radiativeCoupling, survivalTemperature, sinkTemperature);
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

| Package | What it holds |
| --- | --- |
| `Thermal Model` | the `HeaterPower` calculation, the `Heater Sizing` analysis that uses it, and the `@CriticalityAssessment` of the decision it informs ([M&S 6]) |
| `Capability Assessment` | the `#capabilityAssessment` with the five Appendix E.3 factors, their levels, thresholds and evidence ([M&S 48]) |
| `Results Assessment` | the `#resultsAssessment` with the six Appendix E.4 factors ([M&S 31]), and an earlier, partial one |
| `Results Report` | the `MSResultsReport` section 4.3.8 asks for: both assessments, the uncertainty estimate and its process, the qualifications, the risk acceptance and a `#resultsWarning` for the factor below its threshold; and a draft |
| `Matrix Entries` | one `@Applicability`-annotated usage per requirement, with where its record is kept, and the recommendations the project adopts |
| `Requirements Matrix` | a `RequirementsIdentificationMatrix` and a `RecommendationsAdoptionMatrix` whose tables are Tables 1 and 2 generated from the entries |

## From the command line

Run the analysis:

```bash
./bin/sysml examples/nasa-std-7009-demo/nasa-std-7009-demo.sysml \
    -e "'Lunar Night Survival Heater'::'Thermal Model'::heaterSizing.heaterPower"
```

```text
  = [15.430738802500002]
```

Analyse the model:

```bash
./bin/sysml examples/nasa-std-7009-demo/nasa-std-7009-demo.sysml -validate
```

The model has no errors, and `-validate` also runs the NASA-STD-7009B checks, which report the
gaps the model leaves open on purpose (each is marked with a `doc` comment):

| Warning | Gap |
| --- | --- |
| `nasastd7009-criticality-rating-differs-from-matrix` | `Harness Loss Estimate` rates a moderate consequence with a controlling influence yellow where Figure 1 rates it red, and records no method |
| `nasastd7009-assessment-missing-factor` | `earlyResults` records two of the six results factors ([M&S 31]) |
| `nasastd7009-report-missing-content` | `draftReport` states no uncertainty process, qualifications or risk acceptance ([M&S 34], [M&S 37], [M&S 39]) |
| `nasastd7009-report-missing-warning` | `draftReport` assesses validation below its threshold with no warning of unachieved acceptance criteria ([M&S 32] a) |
| `nasastd7009-warning-missing-impact` | `sinkTemperatureLimit` states no `impact` ([M&S 32]) |
| `nasastd7009-requirement-entry-incomplete` | `ms42` is applicable with no `recordLocation` (§4.3.8.8) |
| `nasastd7009-matrix-missing-requirement` | no entry covers [M&S 51] |

Record the missing fact and the warning goes away.

Render the report of M&S results or the Requirements Identification Matrix (Markdown by default;
`-doc-form html` or `pdf` for the others):

```bash
./bin/sysml examples/nasa-std-7009-demo/nasa-std-7009-demo.sysml \
    -render-document "'Lunar Night Survival Heater'::'Results Report'::'Heater Sizing Report'"
./bin/sysml examples/nasa-std-7009-demo/nasa-std-7009-demo.sysml \
    -render-document "'Lunar Night Survival Heater'::'Requirements Matrix'::'Heater Requirements Identification Matrix'"
```

The report renders the two assessment outcomes, its explicit warning with the impact
(`ReportedWarnings`) and, in one table, the uncertainty estimate and its process, the review
findings, the qualifications and the risk acceptance (`ReportStatements`).

The matrix is Table 1 over the project's entries, 42 rows for the 43 requirements ([M&S 51] is
the deliberate gap); abridged:

```markdown
| Section | Description | Req ID | Requirement in this Standard | Applicable | Comments | Record Location |
| --- | --- | --- | --- | --- | --- | --- |
| 4.1.1.1 | General M\&S Programmatics | M\&S 40 | A record of the intended use of M\&S shall be maintained. | yes |  | models/thermal/README.md, section Intended use |
| 4.1.1.2 | General M\&S Programmatics | M\&S 6 | A record of the assessed criticality associated with M\&S use shall be maintained. | yes |  | this model, heaterSizing |
| … | | | | | | |
```

The requirement statement is read from the library's requirement definition through the member
path `type.documentation`, so the Standard's text is stated once, in the library.
