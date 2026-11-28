package passes

import (
	"cmp"
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
		"\n\t\trequirement swe033 : SWE033 { @RequirementMapping { justification = \"Pending.\"; } }" +
		"\n\t\trequirement again : SWE033 { @RequirementMapping { comply = null; } }")
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
		"\n\t\tattribute softwareDefault : MappingKind = projectDefault;" +
		"\n\t\trequirement swe033 : SWE033 { @RequirementMapping { comply = softwareDefault; } }")
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

// A disposition bound through attributes that ends in no value leaves the row's Comply? cell empty.
func TestNPR7150MatrixBoundBlankDisposition(t *testing.T) {
	matrix := "\n\t\tpart def Matrix :> RequirementsMappingMatrix { :>> softwareClass = SoftwareClass::E; }\n\t\tpart matrix : Matrix;"
	for name, attributes := range map[string]string{
		"null":    "\n\t\tattribute pending : MappingKind = null;",
		"unbound": "\n\t\tattribute pending : MappingKind;",
		"chained": "\n\t\tattribute undecided : MappingKind = null;\n\t\tattribute pending : MappingKind = undecided;",
	} {
		t.Run(name, func(t *testing.T) {
			src := npr7150Model(matrix + npr7150Entries(npr7150ClassE[1:]...) + attributes +
				"\n\t\trequirement swe033 : SWE033 { @RequirementMapping { comply = pending; } }")
			diags := only(w8dDiags(t, src), CodeNPR7150MatrixMissingRequirement)
			if len(diags) != 1 || !strings.Contains(diags[0].Message, "1 NPR 7150.2D requirement(s)") || !strings.Contains(diags[0].Message, "SWE033") {
				t.Fatalf("got %v, want one finding naming SWE033", diags)
			}
		})
	}
}

// A disposition bound through attributes is held to the same rules as one written as a literal.
func TestNPR7150BoundDispositionIsAudited(t *testing.T) {
	src := npr7150Model(`
		#softwareSystem part fsw { :>> softwareClass = SoftwareClass::B; :>> safetyCritical = true; }
		attribute waived : MappingKind = MappingKind::notApplicable;
		attribute relaxed : MappingKind = MappingKind::tailored;
		attribute projectDefault : MappingKind = relaxed;
		requirement a : SWE134 { @RequirementMapping { comply = waived; justification = "j"; } }
		requirement b : SWE032 { @RequirementMapping { comply = projectDefault; } }
		requirement c : SWE141 { @RequirementMapping { comply = waived; } }`)
	w8dWantLines(t, src, CodeNPR7150SafetyCriticalNotApplied, 8)
	w8dWantLines(t, src, CodeNPR7150MappingNoJustification, 9, 10)
}

// A disposition read through a feature chain is the literal the chain ends in, whether the
// feature states it, redefines it or inherits it.
func TestNPR7150ChainedDisposition(t *testing.T) {
	matrix := "\n\t\tpart def Matrix :> RequirementsMappingMatrix { :>> softwareClass = SoftwareClass::E; }\n\t\tpart matrix : Matrix;"
	for name, tc := range map[string]struct {
		config  string
		comply  string
		missing int
	}{
		"stated":     {config: "\n\t\tpart cfg { attribute kind : MappingKind = MappingKind::fullyCompliant; }"},
		"redefined":  {config: "\n\t\tpart def Config { attribute kind : MappingKind; }\n\t\tpart cfg : Config { :>> kind = MappingKind::fullyCompliant; }"},
		"inherited":  {config: "\n\t\tpart def Config { attribute kind : MappingKind = MappingKind::fullyCompliant; }\n\t\tpart cfg : Config;"},
		"nested":     {config: "\n\t\tpart cfg { part sw { attribute kind : MappingKind = MappingKind::fullyCompliant; } }\n\t\tattribute projectKind : MappingKind = cfg.sw.kind;\n\t\tpart alias { attribute kind : MappingKind = projectKind; }", comply: "alias.kind"},
		"redefining": {config: "\n\t\tpart def Config { attribute kind : MappingKind = MappingKind::fullyCompliant; }\n\t\tpart cfg : Config { attribute effective :>> kind; }", comply: "cfg.effective"},
		"sibling":    {config: "\n\t\tpart def Config { attribute policy : MappingKind; attribute kind : MappingKind = policy; }\n\t\tpart cfg : Config { :>> policy = MappingKind::fullyCompliant; }"},
		"unset":      {config: "\n\t\tpart def Config { attribute policy : MappingKind default MappingKind::fullyCompliant; attribute kind : MappingKind = policy; }\n\t\tpart cfg : Config { :>> policy = null; }", missing: 1},
		"empty":      {config: "\n\t\tattribute def Config { attribute kind : MappingKind = MappingKind::fullyCompliant; }\n\t\tattribute cfg : Config[0..1] = null;", missing: 1},
		"blank":      {config: "\n\t\tpart cfg { attribute kind : MappingKind; }", missing: 1},
		"null":       {config: "\n\t\tpart def Config { attribute kind : MappingKind = MappingKind::fullyCompliant; }\n\t\tpart cfg : Config { :>> kind = null; }", missing: 1},
	} {
		t.Run(name, func(t *testing.T) {
			comply := cmp.Or(tc.comply, "cfg.kind")
			src := npr7150Model(matrix + npr7150Entries(npr7150ClassE[1:]...) + tc.config +
				"\n\t\trequirement swe033 : SWE033 { @RequirementMapping { comply = " + comply + "; } }")
			if diags := only(w8dDiags(t, src), CodeNPR7150MatrixMissingRequirement); len(diags) != tc.missing {
				t.Fatalf("got %v, want %d finding(s)", diags, tc.missing)
			}
		})
	}
}

// A disposition read through a feature chain is held to the same rules as one written as a literal.
func TestNPR7150ChainedDispositionIsAudited(t *testing.T) {
	src := npr7150Model(`
		#softwareSystem part fsw { :>> softwareClass = SoftwareClass::B; :>> safetyCritical = true; }
		part def Config { attribute kind : MappingKind; }
		part waived : Config { :>> kind = MappingKind::notApplicable; }
		part relaxed { attribute kind : MappingKind = MappingKind::tailored; }
		requirement a : SWE134 { @RequirementMapping { comply = waived.kind; justification = "j"; } }
		requirement b : SWE032 { @RequirementMapping { comply = relaxed.kind; } }
		requirement c : SWE141 { @RequirementMapping { comply = waived.kind; } }`)
	w8dWantLines(t, src, CodeNPR7150SafetyCriticalNotApplied, 8)
	w8dWantLines(t, src, CodeNPR7150MappingNoJustification, 9, 10)
}

// Every MappingKind literal read through a feature chain addresses its requirement.
func TestNPR7150ChainedDispositionLiterals(t *testing.T) {
	matrix := "\n\t\tpart def Matrix :> RequirementsMappingMatrix { :>> softwareClass = SoftwareClass::E; }\n\t\tpart matrix : Matrix;"
	for _, literal := range []string{"fullyCompliant", "tailored", "notApplicable"} {
		for name, config := range map[string]string{
			"stated":    "\n\t\tpart cfg { attribute kind : MappingKind = MappingKind::" + literal + "; }",
			"redefined": "\n\t\tpart def Config { attribute kind : MappingKind; }\n\t\tpart cfg : Config { :>> kind = MappingKind::" + literal + "; }",
		} {
			t.Run(literal+"/"+name, func(t *testing.T) {
				src := npr7150Model(matrix + npr7150Entries(npr7150ClassE[1:]...) + config +
					"\n\t\trequirement swe033 : SWE033 { @RequirementMapping { comply = cfg.kind; } }")
				diags := w8dDiags(t, src)
				if missing := only(diags, CodeNPR7150MatrixMissingRequirement); len(missing) != 0 {
					t.Fatalf("got %v, want the requirement addressed", missing)
				}
				unjustified := len(only(diags, CodeNPR7150MappingNoJustification))
				if want := literal != "fullyCompliant"; (unjustified == 1) != want {
					t.Fatalf("got %d missing-justification finding(s), want one: %t", unjustified, want)
				}
			})
		}
	}
}

// A justification held by an attribute, or read through a feature chain, is one the entry gives;
// one that ends in no text is not.
func TestNPR7150BoundJustification(t *testing.T) {
	src := npr7150Model(`
		attribute rationale : ScalarValues::String = "Approved by the Technical Authority";
		attribute relayed : ScalarValues::String = rationale;
		part record { attribute text : ScalarValues::String = "Heritage executive"; }
		part def Record { attribute text : ScalarValues::String = "Waived"; }
		part filed : Record { attribute note :>> text; }
		attribute empty : ScalarValues::String = "";
		attribute unset : ScalarValues::String;
		requirement a : SWE134 { @RequirementMapping { comply = MappingKind::tailored; justification = rationale; } }
		requirement b : SWE032 { @RequirementMapping { comply = MappingKind::tailored; justification = relayed; } }
		requirement c : SWE141 { @RequirementMapping { comply = MappingKind::tailored; justification = record.text; } }
		requirement d : SWE033 { @RequirementMapping { comply = MappingKind::tailored; justification = filed.note; } }
		requirement e : SWE013 { @RequirementMapping { comply = MappingKind::tailored; justification = empty; } }
		requirement f : SWE042 { @RequirementMapping { comply = MappingKind::tailored; justification = unset; } }`)
	w8dWantLines(t, src, CodeNPR7150MappingNoJustification, 15, 16)
}
