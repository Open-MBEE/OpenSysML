# NPR 7150.2 demo

[`npr7150-demo.sysml`](npr7150-demo.sysml) works a small fictional project — the *Regolith Sample
Imager*, a lunar-surface instrument with a sample-illumination laser — through NASA NPR 7150.2D
using the bundled [`NPR7150`](../../docs/project/npr7150-library.md) library. Each system containing
software carries the NPR's keyword and states its software class, and each requirement the NPR
invokes for that class has a mapping entry:

```sysml
#softwareSystem part imagerFlightSoftware {
    :>> softwareClass = SoftwareClass::C;
    :>> safetyCritical = true;
}
requirement swe013 : SWE013 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
requirement swe141 : SWE141 {
    @RequirementMapping {
        comply = MappingKind::notApplicable;
        justification = "The project is Category 3 and was not selected for software IV&V.";
        approvedBy = "Center SMA Technical Authority";
    }
}
```

| Package | What it holds |
| --- | --- |
| `Software Classification` | the `#softwareSystem`s of the project with their `softwareClass` (SWE-020) and safety-critical determination (SWE-205) |
| `Mapping Entries` | one `@RequirementMapping`-annotated usage per requirement Table 2 of Appendix C marks for Class C — fully compliant, tailored or not applicable with its justification and approval |
| `Software Assurance` | the six NASA-STD-8739.8B Table 1 tasks software assurance performs for SWE-134 |
| `Requirements Mapping Matrix` | a `RequirementsMappingMatrix` document for Class C whose tables are the project's mapping matrix and its software classification record, generated from the model |

## From the command line

Analyse the model:

```bash
./bin/sysml examples/npr7150-demo/npr7150-demo.sysml -validate
```

The model has no errors, and `-validate` also runs the NPR 7150.2D checks, which report the five
gaps the model leaves open on purpose (each is marked with a comment):

| Warning | Gap |
| --- | --- |
| `npr7150-software-missing-classification` | `calibrationScripts` states no software class (SWE-020) |
| `npr7150-class-e-safety-critical` | `laserInterlockPrototype` is Class E and safety-critical (Appendix D) |
| `npr7150-safety-critical-requirement-not-applicable` | `swe219` is mapped not applicable although `imagerFlightSoftware` is safety-critical (§3.7.4) |
| `npr7150-mapping-missing-justification` | `swe220` is tailored with no `justification` |
| `npr7150-mapping-matrix-missing-requirement` | no mapping entry covers SWE-058 |

```text
examples/npr7150-demo/npr7150-demo.sysml:39:4: warning[npr7150-software-missing-classification]: This system containing software states no software class: NPR 7150.2D section 3.5.1 (SWE-020) has the p
			#softwareSystem part calibrationScripts {
   ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/npr7150-demo/npr7150-demo.sysml:43:4: warning[npr7150-class-e-safety-critical]: This system is Class E and safety-critical: NPR 7150.2D Appendix D states that Class E software cannot be safet
			#softwareSystem part laserInterlockPrototype {
   ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/npr7150-demo/npr7150-demo.sysml:85:4: warning[npr7150-safety-critical-requirement-not-applicable]: This mapping entry marks SWE219 not applicable, but `imagerFlightSoftware` is safety-critica
			requirement swe219 : SWE219 { @RequirementMapping { comply = MappingKind::notApplicable; justification = "The flight software reuses a heritage executive tested to statement coverage."; } }
   ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/npr7150-demo/npr7150-demo.sysml:87:4: warning[npr7150-mapping-missing-justification]: This mapping entry is marked tailored with no justification: NPR 7150.2D Appendix C has a requirement tai
			requirement swe220 : SWE220 { @RequirementMapping { comply = MappingKind::tailored; } }
   ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
examples/npr7150-demo/npr7150-demo.sysml:198:3: warning[npr7150-mapping-matrix-missing-requirement]: This Requirements Mapping Matrix has no mapping entry for 1 NPR 7150.2D requirement(s) Table 2 mark
		part mappingMatrix : 'Imager Flight Software Mapping Matrix';
  ^~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
✓ comment
✓ package 'Regolith Sample Imager'
✓ examples/npr7150-demo/npr7150-demo.sysml: no errors
```

Record the missing fact — the class, a class of D or higher, a mapping other than not applicable,
a `justification`, the SWE-058 entry — and the warning goes away.

Render the mapping matrix (Markdown by default; `-doc-form html` or `pdf` for the others):

```bash
./bin/sysml examples/npr7150-demo/npr7150-demo.sysml \
    -render-document "'Regolith Sample Imager'::'Requirements Mapping Matrix'::'Imager Flight Software Mapping Matrix'"
```

The first table is the project's Requirements Mapping Matrix, 91 rows for the 92 requirements
Table 2 marks for Class C (SWE-058 is the deliberate gap); the requirement text is abridged here:

```markdown
| Section | SWE \# | Requirement Text | Class A-E Authority | Class F Authority | Comply? | Justification | Approved By |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 3.1.2 | SWE-033 | The project manager shall assess options for software acquisition versus… | center | cio | fullyCompliant |  |  |
| 3.1.3 | SWE-013 | The project manager shall develop, maintain, and execute software plans,… | center | cio | fullyCompliant |  |  |
| … | | | | | | | |
| 3.7.4 | SWE-219 | If a project has safety-critical software, the project manager shall ensure… | center | cio | notApplicable | The flight software reuses a heritage executive tested to statement coverage. |  |
| 3.7.5 | SWE-220 | If a project has safety-critical software, the project manager shall ensure all… | center | cio | tailored |  |  |
```

The second lists the NASA-STD-8739.8B tasks mapped for SWE-134:

```markdown
| SWE \# | Task | Task Text | Comply? | Justification | Approved By |
| --- | --- | --- | --- | --- | --- |
| SWE-134 | SWE-134-SA-1 | Analyze the software requirements and the software design and work with the… | fullyCompliant |  |  |
| … | | | | | |
```

The third is the software classification record SWE-176 asks for:

```markdown
| name | softwareClass | safetyCritical | documentation |
| --- | --- | --- | --- |
| imagerFlightSoftware | C | true | Commands the imager and its sample-illumination laser: … |
| groundDataPipeline | C | false | Processes the imager's science data on the ground: Class C. |
| grainSizeEstimator | D |  |  |
| calibrationScripts |  |  | Deliberate gap: no software class is stated. |
| laserInterlockPrototype | E | true | Deliberate gap: Class E software cannot be safety-critical. |
```

The requirement text, section and tailoring authority are read from the library's requirement
definition, so the NPR's text is stated once, in the library.
