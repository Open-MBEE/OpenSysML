package passes

import (
	"strings"
	"testing"
)

// npr7150Model wraps body in a package importing the NPR7150 library.
func npr7150Model(body string) string {
	return "package M {\n\tprivate import NPR7150::*;\n\tprivate import NPR7150::SWERequirements::*;" + body + "\n}"
}

var npr7150Codes = []string{
	CodeNPR7150SoftwareUnclassified,
	CodeNPR7150ClassESafetyCritical,
	CodeNPR7150MappingNoJustification,
	CodeNPR7150MatrixMissingRequirement,
	CodeNPR7150SafetyCriticalNotApplied,
}

// A system containing software states its class, on the usage or on its definition.
func TestNPR7150SoftwareMissingClassification(t *testing.T) {
	src := npr7150Model(`
		#softwareSystem part bare;
		#softwareSystem part classified { :>> softwareClass = SoftwareClass::C; }
		part def GroundTool :> SoftwareSystem { :>> softwareClass = SoftwareClass::D; }
		part tool : GroundTool;
		part untyped : SoftwareSystem;
		part hardware;`)
	w8dWantLines(t, src, CodeNPR7150SoftwareUnclassified, 4, 8)
}

// Appendix D: Class E software cannot be safety-critical.
func TestNPR7150ClassESafetyCritical(t *testing.T) {
	src := npr7150Model(`
		#softwareSystem part prototype { :>> softwareClass = SoftwareClass::E; :>> safetyCritical = true; }
		#softwareSystem part script { :>> softwareClass = SoftwareClass::E; :>> safetyCritical = false; }
		#softwareSystem part fsw { :>> softwareClass = SoftwareClass::B; :>> safetyCritical = true; }`)
	w8dWantLines(t, src, CodeNPR7150ClassESafetyCritical, 4)
}

// A tailored or not-applicable mapping entry carries its justification.
func TestNPR7150MappingMissingJustification(t *testing.T) {
	src := npr7150Model(`
		requirement a : SWE013 { @RequirementMapping { comply = MappingKind::fullyCompliant; } }
		requirement b : SWE141 { @RequirementMapping { comply = MappingKind::notApplicable; } }
		requirement c : SWE032 { @RequirementMapping { comply = MappingKind::tailored; justification = ""; } }
		requirement d : SWE131 { @RequirementMapping { comply = MappingKind::notApplicable; justification = "No IV&V on this project."; } }`)
	w8dWantLines(t, src, CodeNPR7150MappingNoJustification, 5, 6)
}

// Each annotation of a usage is its own entry: a justified one does not excuse another.
func TestNPR7150MappingAnnotationsAreSeparate(t *testing.T) {
	src := npr7150Model(`
		requirement a : SWE141 {
			@RequirementMapping { comply = MappingKind::notApplicable; }
			@RequirementMapping { comply = MappingKind::fullyCompliant; justification = "Covered by the parent project."; }
		}`)
	w8dWantLines(t, src, CodeNPR7150MappingNoJustification, 4)
}

// An entry that states no disposition does not address its requirement in the matrix.
func TestNPR7150MatrixBlankDisposition(t *testing.T) {
	matrix := "\n\t\tpart def Matrix :> RequirementsMappingMatrix { :>> softwareClass = SoftwareClass::E; }\n\t\tpart matrix : Matrix;"
	src := npr7150Model(matrix + npr7150Entries(npr7150ClassE[1:]...) +
		"\n\t\trequirement swe033 : SWE033 { @RequirementMapping { justification = \"Pending.\"; } }")
	diags := only(w8dDiags(t, src), CodeNPR7150MatrixMissingRequirement)
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "1 NPR 7150.2D requirement(s)") || !strings.Contains(diags[0].Message, "SWE033") {
		t.Fatalf("got %v, want one finding naming SWE033", diags)
	}
}

// A disposition bound through an attribute still addresses its requirement in the matrix.
func TestNPR7150MatrixBoundDisposition(t *testing.T) {
	matrix := "\n\t\tpart def Matrix :> RequirementsMappingMatrix { :>> softwareClass = SoftwareClass::E; }\n\t\tpart matrix : Matrix;"
	src := npr7150Model(matrix + npr7150Entries(npr7150ClassE[1:]...) +
		"\n\t\tattribute projectDefault : MappingKind = MappingKind::fullyCompliant;" +
		"\n\t\trequirement swe033 : SWE033 { @RequirementMapping { comply = projectDefault; } }")
	w8dWantLines(t, src, CodeNPR7150MatrixMissingRequirement)
}

// npr7150Entries states a fully compliant mapping entry for each SWE number.
func npr7150Entries(numbers ...string) string {
	var b strings.Builder
	for _, n := range numbers {
		b.WriteString("\n\t\trequirement swe" + n + " : SWE" + n + " { @RequirementMapping { comply = MappingKind::fullyCompliant; } }")
	}
	return b.String()
}

// Table 2 marks exactly these twelve requirements for Class E.
var npr7150ClassE = []string{"033", "013", "042", "139", "121", "125", "020", "176", "022", "205", "148", "156"}

// A matrix expects a mapping entry for every requirement Table 2 marks for its class.
func TestNPR7150MatrixMissingRequirement(t *testing.T) {
	matrix := "\n\t\tpart def Matrix :> RequirementsMappingMatrix { :>> softwareClass = SoftwareClass::E; }\n\t\tpart matrix : Matrix;"
	complete := npr7150Model(matrix + npr7150Entries(npr7150ClassE...))
	w8dWantLines(t, complete, CodeNPR7150MatrixMissingRequirement)

	partial := npr7150Model(matrix + npr7150Entries(npr7150ClassE[2:]...))
	diags := only(w8dDiags(t, partial), CodeNPR7150MatrixMissingRequirement)
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	if got := w8dLine(partial, diags[0].Span); got != 5 {
		t.Errorf("reported on line %d, want the matrix usage on line 5", got)
	}
	for _, want := range []string{"2 NPR 7150.2D requirement(s)", "Class E", "SWE013, SWE033"} {
		if !strings.Contains(diags[0].Message, want) {
			t.Errorf("message %q does not contain %q", diags[0].Message, want)
		}
	}
}

// A matrix for two classes expects the requirements of either, and one stating no class is asked for it.
func TestNPR7150MatrixClasses(t *testing.T) {
	src := npr7150Model(`
		part both : RequirementsMappingMatrix { :>> softwareClass = (SoftwareClass::E, SoftwareClass::F); }
		part none : RequirementsMappingMatrix;` + npr7150Entries(npr7150ClassE...))
	diags := only(w8dDiags(t, src), CodeNPR7150MatrixMissingRequirement)
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "Class E, F") || !strings.Contains(diags[0].Message, "SWE024") {
		t.Fatalf("got %v, want one finding naming Class E, F and SWE024", diags)
	}
	if strings.Contains(diags[0].Message, "SWE134") {
		t.Errorf("SWE134 is not invoked for Class E or F: %s", diags[0].Message)
	}
	w8dWantLines(t, src, CodeNPR7150SoftwareUnclassified, 5)
}

// A safety-critical software requirement is not mapped not applicable beside safety-critical software.
func TestNPR7150SafetyCriticalRequirementNotApplicable(t *testing.T) {
	entries := `
		requirement a : SWE134 { @RequirementMapping { comply = MappingKind::notApplicable; justification = "j"; } }
		requirement b : SWE219 { @RequirementMapping { comply = MappingKind::tailored; justification = "j"; } }
		requirement c : SWE141 { @RequirementMapping { comply = MappingKind::notApplicable; justification = "j"; } }`
	critical := npr7150Model(`
		#softwareSystem part fsw { :>> softwareClass = SoftwareClass::B; :>> safetyCritical = true; }` + entries)
	w8dWantLines(t, critical, CodeNPR7150SafetyCriticalNotApplied, 5)
	benign := npr7150Model(`
		#softwareSystem part fsw { :>> softwareClass = SoftwareClass::B; :>> safetyCritical = false; }` + entries)
	w8dWantLines(t, benign, CodeNPR7150SafetyCriticalNotApplied)
}

// The library's Table 2 rows read back with the per-class counts of the NPR.
func TestNPR7150LibraryMatrix(t *testing.T) {
	a := newNPR7150Audit(NewContext("<t>", newTestIndex(), nil))
	if a == nil {
		t.Fatal("the bundled NPR7150 library is not loaded")
	}
	a.loadMatrix()
	if len(a.requirements) != 100 {
		t.Fatalf("got %d Table 2 requirements, want 100", len(a.requirements))
	}
	counts := map[string]int{}
	critical := 0
	for _, r := range a.requirements {
		for class := range r.classes {
			counts[class]++
		}
		if r.safetyCritical {
			critical++
		}
	}
	want := map[string]int{"A": 100, "B": 100, "C": 92, "D": 64, "E": 12, "F": 65}
	for class, n := range want {
		if counts[class] != n {
			t.Errorf("Class %s: got %d invoked requirements, want %d", class, counts[class], n)
		}
	}
	if critical != 4 {
		t.Errorf("got %d safety-critical software requirements, want 4", critical)
	}
}

// A model that does not use the library, or a workspace package of its name, gets no finding.
func TestNPR7150SilentWithoutLibraryUse(t *testing.T) {
	for _, src := range []string{
		"package M { part def SoftwareSystem; part s : SoftwareSystem; }",
		"package NPR7150 { part def SoftwareSystem; part s : SoftwareSystem; }",
	} {
		for _, d := range w8dDiags(t, src) {
			for _, code := range npr7150Codes {
				if d.Code == code {
					t.Errorf("%s: unexpected %s", src, code)
				}
			}
		}
	}
}
