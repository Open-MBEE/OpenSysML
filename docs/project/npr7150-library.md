# NPR7150 library — design

Status: **implemented.** `internal/workspace/libs/stdlib/OpenSysML Libraries/NPR7150.sysml` —
maintained upstream at Open-MBEE/OpenSysML-Extensions-Library and vendored here pinned by
`scripts/extension-libraries-pin.sh` — is bundled with the other OpenSysML extensions, enters the
same conformance, strict-notation and snapshot gates, and is exercised by
`examples/npr7150-demo/npr7150-demo.sysml` (`TestExamplesAnalyseCleanly`). `passes.NPR7150Pass`
audits models written against it.

## The problem

NASA NPR 7150.2D, *NASA Software Engineering Requirements* (Office of the Chief Engineer,
[NODIS](https://nodis3.gsfc.nasa.gov/displayDir.cfm?Internal_ID=N_PR_7150_002D_&page_name=main)),
is the procedural requirement the software of every NASA program and project is assessed
against. It classifies software into six classes, A to F (Appendix D); states its requirements
as numbered `[SWE-nnn]` statements in chapters 2 to 5; and in Appendix C gives the *Requirements
Mapping Matrix* (Table 2): for each project-level requirement, the classes it is invoked for and
the authority that approves tailoring it. A project fills in its own copy of that matrix
(SWE-125), saying for each invoked requirement whether it complies, tailors or does not apply.
NASA-STD-8739.8B, *Software Assurance and Software Safety Standard*, is written against the same
numbers: its Table 1 lists, per SWE requirement, the tasks software assurance performs.

A project working in SysML v2 already has its software systems and its requirements in the
model. What it lacks is the NPR's vocabulary to classify them with, and the matrix the NPR asks
for generated from the model rather than kept in a spreadsheet beside it. As for
[NPR 7123.1D](npr7123-library.md), the vocabulary is a *library* and the mapping matrix is a
document query over the same model.

## Design

`library package NPR7150` follows the NPR7123 library's shape: a definition per concept, a base
usage each classified element subsets, and a semantic-metadata keyword that does both.

```sysml
#softwareSystem part fsw { :>> softwareClass = SoftwareClass::B; :>> safetyCritical = true; }
requirement swe134 : SWE134 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
```

| NPR concept | Library element |
| --- | --- |
| Software classes A to F (Appendix D) | `enum def SoftwareClass`, each literal documented with the Appendix D definition |
| A system or subsystem containing software (SWE-020, SWE-205) | `part def SoftwareSystem` with `softwareClass` and `safetyCritical`; keyword `#softwareSystem` |
| A numbered requirement | `requirement def <'SWE-013'> SWE013 :> SWERequirement` carrying the statement as documentation, its `id` and `section` |
| Table 2: the requirements invoked per class | the 100 definitions of `NPR7150::SWERequirements`, each with one `@ClassApplicability` per class marked 'X', the `authority` for Classes A to E and the `classFAuthority` |
| The institutional requirements of chapter 2 and the rest not in Table 2 | the 30 definitions of `NPR7150::InstitutionalRequirements` |
| "If a project has safety-critical software" (§3.7) | `@SafetyCriticalSoftwareCondition` on SWE-023, SWE-134, SWE-219 and SWE-220 |
| Bi-directional traceability (SWE-052, Table 1) | `enum def TraceabilityRelation` and `@TraceabilityExpectation` |
| A project's mapping to a requirement | `@RequirementMapping { comply; justification; approvedBy }` (`#mapsRequirement`) with `MappingKind` fully compliant, tailored or not applicable |
| The project's Requirements Mapping Matrix (SWE-125) | `part def RequirementsMappingMatrix :> Document` with the classes it is for, and `calc def RequirementsMappingRegister` generating its rows; `calc def AssuranceTaskRegister` lists the mapped NASA-STD-8739.8B tasks |
| The software classification record (SWE-176) | `calc def SoftwareClassificationRegister` |
| NASA-STD-8739.8B Table 1 | the 203 definitions of `NPR7150::SoftwareAssurance`, `requirement def <'SWE-134-SA-1'> SA134_1 :> SATask`, each with the `sweId` it is keyed to, the lettered item where the standard splits a requirement (SWE-065 a to d) and its task number |

The statements are transcribed from the NPR's chapters; applicability and authority from
Appendix C. Where the two differ in wording (SWE-039, SWE-052, SWE-094, SWE-131, SWE-141,
SWE-157) the chapter text is the one kept.

## The checks

`passes.NPR7150Pass` runs at the constraint tier and reports warnings only, for a document that
uses the bundled library:

| Code | Fact |
| --- | --- |
| `npr7150-software-missing-classification` | a `SoftwareSystem`, or a `RequirementsMappingMatrix`, states no `softwareClass` (§3.5.1, SWE-020; Appendix C) |
| `npr7150-class-e-safety-critical` | a Class E `SoftwareSystem` is marked safety-critical (Appendix D) |
| `npr7150-mapping-missing-justification` | a `@RequirementMapping` is tailored or not applicable with no `justification` (Appendix C, §2.2.5) |
| `npr7150-mapping-matrix-missing-requirement` | the document of a `RequirementsMappingMatrix` has no mapping entry for a requirement Table 2 marks for one of its classes |
| `npr7150-safety-critical-requirement-not-applicable` | a §3.7 safety-critical software requirement is mapped not applicable in a document stating safety-critical software |

The pass reads Table 2 off the library's annotations rather than a table of its own, and only
for a document that states a mapping matrix or safety-critical software with mapping entries.

A mapping entry's disposition is the `MappingKind` literal its `comply` states: written out
(`comply = MappingKind::tailored`), held by an attribute (`comply = projectDefault`) or read
through a feature chain (`comply = config.kind`), following each attribute's value and the
features it redefines. One that ends in `null` or in no value states none, and its requirement
counts as not addressed. A `justification` is read the same way, and one that ends in no text
is not a justification.

## How OpenSysML serves the NPR

What the NPR asks a project to *show* — its software classification (SWE-020, SWE-176) and its
Requirements Mapping Matrix (SWE-125, Appendix C) — is generated from the model:
`sysml -render-document` writes the matrix as Markdown, HTML or PDF, `-validate` reports the gaps
as warnings with the NPR section each comes from, and every element is reachable through the
model API, the REPL and the language server. [`examples/npr7150-demo`](https://github.com/Open-MBEE/OpenSysML/tree/develop/examples/npr7150-demo)
works a small project through it.

## Alternatives considered

- **Applicability as six Boolean attributes on the requirement.** Rejected: an annotation per
  invoked class is what a filter or a query selects on, and it leaves a class the NPR does not
  invoke simply unstated.
- **The NASA-STD-8739.8B tasks as documentation on the SWE requirement.** Rejected: a task is
  something software assurance plans, performs and maps separately, so each is a requirement
  definition keyed to its SWE number.
- **A separate library for NASA-STD-8739.8B.** Rejected: its Table 1 has no identifiers of its
  own; every row is addressed by an NPR 7150.2 requirement number.
- **Checks as errors, or unconditional.** Rejected, as for NPR7123: a project that has not yet
  classified its software is not wrong.

## Known limitations

- Table 2 prints SWE-015 as invoked for Class F but leaves its Class F authority blank; the
  library transcribes the row as printed, so its `classFAuthority` is unstated.
- The pass audits the document that holds the mapping matrix; entries a project keeps in a second
  file are not seen by the matrix rule, so keep the matrix beside its entries.
- The NASA-STD-8739.8B tasks and the institutional requirements are vocabulary only: no check
  expects a project to map them.
