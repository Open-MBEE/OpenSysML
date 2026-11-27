### NPR7123 Extension Library (`OpenSysML Libraries/NPR7123.sysml`; an OpenSysML extension, non-normative)

NASA NPR 7123.1D is a process standard, not a modeling language; the library carries its
vocabulary (see `docs/project/npr7123-library.md`) and `passes.NPR7123Pass` audits a model
against the facts the NPR states of that vocabulary, as warnings at the constraint tier. The
rows claim agreement with the NPR's text, not with SysML v2 semantics.

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| A life-cycle review usage states entrance criteria and success criteria (NPR 7123.1D §5.2.2.1, SE-34): an `EntranceCriterion` and a `SuccessCriterion` requirement usage nested in its body; one stating neither or only one kind is reported once | `passes/npr7123.go` `checkReview` | `passes/npr7123_test.go` `TestNPR7123ReviewMissingCriteria` | 🧩 Extension (OpenSysML library) |
| A life-cycle review of a kind with §5.2.2.2 minimum products (`@MaturityExpectation { required = true }` on the library's work-product definitions, transcribed from Table 5-1) states each product under `reviewedProducts`, at or above the maturity the section asks for (Appendix F order: initial/preliminary < approve/baseline < final/update) | `passes/npr7123.go` `checkReview`, `npr7123MaturityRank` | `TestNPR7123ReviewMissingMinimumProduct` | 🧩 Extension (OpenSysML library) |
| A `@Compliance` marked `tailored` or `notApplicable` states a non-empty `justification` (Table H-1) | `passes/npr7123.go` `checkCompliance` | `TestNPR7123ComplianceMissingJustification` | 🧩 Extension (OpenSysML library) |
| A `ComplianceMatrix` usage's document holds a `@Compliance`-annotated usage of every active `[SE-nn]` definition of `NPR7123::SERequirements` (Table H-1 covers every active requirement); the finding is one per matrix, naming the missing ids | `passes/npr7123.go` `checkMatrix` | `TestNPR7123ComplianceMatrixMissingRequirement` | 🧩 Extension (OpenSysML library) |
| A `SEMP` usage nests, at any depth, one usage of each of the 17 common technical process definitions (§6.2.3, SE-58); a SEMP a review states under `reviewedProducts` is a stand-in and is not audited | `passes/npr7123.go` `checkSEMP`, `reviewedProduct` | `TestNPR7123SEMPMissingProcess` | 🧩 Extension (OpenSysML library) |
| Library documents are not audited, and a model that never uses the library — or a workspace package merely named `NPR7123` — gets no finding | `passes/npr7123.go` `Run`, `newNPR7123Audit` | `TestNPR7123NeedsTheLibrary` | 🧩 Extension (OpenSysML library) |
| A document-query member path whose segment names no declared member reads the element-valued metaclass feature of that name, so `type.documentation` reads the documentation of a usage's definition (what `NPR7123::ComplianceRegister` needs for the Requirement Statement column) | `queryexec/memberpath.go` `memberPathStep` | `queryexec/memberpath_test.go` `TestExecuteMemberPathReadsMetaclassFeatureSegment` | 🧩 Extension (OpenSysML library) |
