### NPR7150 Extension Library (`OpenSysML Libraries/NPR7150.sysml`; an OpenSysML extension, non-normative)

NASA NPR 7150.2D is a process standard, not a modeling language; the library carries its
vocabulary and the software assurance tasks of NASA-STD-8739.8B (see
`docs/project/npr7150-library.md`) and `passes.NPR7150Pass` audits a model against the facts the
NPR states of that vocabulary, as warnings at the constraint tier. The rows claim agreement with
the NPR's text, not with SysML v2 semantics.

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| A `SoftwareSystem` usage states a `softwareClass`, on the usage or on its definition (NPR 7150.2D §3.5.1, SWE-020); a `RequirementsMappingMatrix` usage states the class or classes it is for | `passes/npr7150.go` `checkSoftwareSystem`, `checkMatrix` | `passes/npr7150_test.go` `TestNPR7150SoftwareMissingClassification`, `TestNPR7150MatrixClasses` | 🧩 Extension (OpenSysML library) |
| A `SoftwareSystem` of Class E does not state `safetyCritical = true` (Appendix D: Class E software cannot be safety-critical software) | `passes/npr7150.go` `checkSoftwareSystem` | `TestNPR7150ClassESafetyCritical` | 🧩 Extension (OpenSysML library) |
| A `@RequirementMapping` marked `tailored` or `notApplicable` states a non-empty `justification` (Appendix C, §2.2.5) | `passes/npr7150.go` `check`, `mapping` | `TestNPR7150MappingMissingJustification` | 🧩 Extension (OpenSysML library) |
| A `RequirementsMappingMatrix` usage's document holds a `@RequirementMapping`-annotated usage of every definition of `NPR7150::SWERequirements` that Table 2 marks 'X' (`@ClassApplicability`) for one of the matrix's classes; the finding is one per matrix, naming the missing ids | `passes/npr7150.go` `checkMatrix`, `loadMatrix` | `TestNPR7150MatrixMissingRequirement`, `TestNPR7150MatrixClasses`, `TestNPR7150LibraryMatrix` | 🧩 Extension (OpenSysML library) |
| A document stating a `SoftwareSystem` with `safetyCritical = true` does not map a safety-critical software requirement (`@SafetyCriticalSoftwareCondition`: SWE-023, SWE-134, SWE-219, SWE-220; §3.7.2 to §3.7.5) `notApplicable` | `passes/npr7150.go` `checkSafetyCritical` | `TestNPR7150SafetyCriticalRequirementNotApplicable` | 🧩 Extension (OpenSysML library) |
| Library documents are not audited, and a model that never uses the library — or a workspace package merely named `NPR7150` — gets no finding | `passes/npr7150.go` `Run`, `newNPR7150Audit` | `TestNPR7150SilentWithoutLibraryUse` | 🧩 Extension (OpenSysML library) |
