package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// typeDiags parses src, indexes it, runs the full default registry, and returns
// only diagnostics whose Source is "type".
func typeDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	// A bare resource set: these fixtures assert the type rules, not the
	// library's contribution to them.
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	all := Analyze("<t>", root, nil, idx)
	var out []diag.Diagnostic
	for _, d := range all {
		if d.Source == "type" {
			out = append(out, d)
		}
	}
	return out
}

func TestTypeCheckSpecializesSameKindOK(t *testing.T) {
	diags := typeDiags(t, "part def Vehicle; part def Car specializes Vehicle;")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestTypeCheckSpecializesCrossKindError(t *testing.T) {
	diags := typeDiags(t, "attribute def Mass; part def Car specializes Mass;")
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if diags[0].Code != "specialization-kind" {
		t.Fatalf("expected code %q, got %q", "specialization-kind", diags[0].Code)
	}
}

func TestTypeCheckTypingWantsMatchingDef(t *testing.T) {
	diags := typeDiags(t, "attribute def Mass; part def Car { part p : Mass; }")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
}

func TestTypeCheckTypingMatchingDefOK(t *testing.T) {
	diags := typeDiags(t, "part def Engine; part def Car { part e : Engine; }")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestTypeCheckUnresolvedTargetSkipped(t *testing.T) {
	diags := typeDiags(t, "part def Car specializes Missing;")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics (gated), got %v", diags)
	}
}

func TestTypeCheckTypingByAliasOK(t *testing.T) {
	src := `
		attribute def PowerValue;
		alias PowerAlias for PowerValue;
		calc def Test {
			attribute p : PowerAlias;
		}
	`
	diags := typeDiags(t, src)
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics for alias typing, got %v", diags)
	}
}

// `satisfy <name>` references an existing requirement usage (a reference
// subsetting), so a requirement usage target is legal there.
func TestTypeCheckSatisfyRequirementUsageOK(t *testing.T) {
	src := `
		package P {
			requirement vehicleSpecification;
			part def Vehicle;
			part v : Vehicle;
			part ctx {
				satisfy vehicleSpecification by v;
			}
		}
	`
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics for satisfy of a requirement usage, got %v", diags)
	}
}

// A viewpoint usage is a requirement usage, so `satisfy <viewpointUsage>` is legal.
func TestTypeCheckSatisfyViewpointUsageOK(t *testing.T) {
	src := `
		package P {
			viewpoint perspective;
			view def StructureView {
				satisfy perspective;
			}
		}
	`
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics for satisfy of a viewpoint usage, got %v", diags)
	}
}

// Satisfying something that is not a requirement usage stays an error.
func TestTypeCheckSatisfyNonRequirementUsageError(t *testing.T) {
	src := `
		package P {
			attribute a;
			part ctx {
				satisfy a;
			}
		}
	`
	diags := typeDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic for satisfy of a non-requirement, got %v", diags)
	}
}

// The declaration form is a requirement usage, so a requirement definition —
// or a concern or viewpoint definition, which are requirement definitions —
// types it. This is the whole parser fixture parse/satisfy_reference.sysml at
// the tier the typing is judged, since a parser golden cannot see it.
func TestTypeCheckSatisfyDeclarationTypedByRequirementDefOK(t *testing.T) {
	src := `
		package SatisfyReference {
			requirement def VehicleSpecification;
			requirement vehicleSpecification;
			viewpoint perspective;
			part vehicle;

			part context {
				satisfy vehicleSpecification by vehicle;
				satisfy requirement viewpointConformance by vehicle;
				satisfy requirement conformance : VehicleSpecification by vehicle;
			}

			view def StructureView {
				satisfy perspective;
			}
		}
	`
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

// Typing the declaration form with something that is not a requirement
// definition stays an error.
func TestTypeCheckSatisfyDeclarationTypedByPartDefError(t *testing.T) {
	src := `
		package P {
			part def Vehicle;
			part vehicle;
			part ctx {
				satisfy requirement r : Vehicle by vehicle;
			}
		}
	`
	diags := typeDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
}

// An alias for a requirement usage is a legal satisfy target.
func TestTypeCheckSatisfyAliasOfRequirementUsageOK(t *testing.T) {
	src := `
		package P {
			requirement vehicleSpecification;
			alias VS for vehicleSpecification;
			part v;
			part ctx {
				satisfy VS by v;
			}
		}
	`
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics for satisfy through an alias, got %v", diags)
	}
}

// A non-Boolean loop or branch condition is a type error, and it is reported at
// typecheck: the executor evaluates conditions it was told are Boolean.
func TestTypeCheckNonBooleanControlFlowConditions(t *testing.T) {
	cases := map[string]string{
		"while": `
			package P {
				action a {
					action driver {
						while 5 { }
					}
				}
			}
		`,
		"until": `
			package P {
				action a {
					action driver {
						loop { } until 5;
					}
				}
			}
		`,
		"if": `
			package P {
				action a {
					action driver {
						if 5 { }
					}
				}
			}
		`,
		// A condition nested in a loop body is checked too, so a body the
		// executor now runs cannot carry an unchecked condition.
		"nested in a loop body": `
			package P {
				action a {
					action driver {
						while true {
							if 5 { }
						}
					}
				}
			}
		`,
		// And a condition nested in a branch body, the other direction of nesting.
		"nested in a branch body": `
			package P {
				action a {
					action driver {
						if true {
							while 5 { }
						}
					}
				}
			}
		`,
	}

	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			diags := typeDiags(t, src)
			if len(diags) != 1 {
				t.Fatalf("expected one type diagnostic for a non-Boolean %s condition, got %v", name, diags)
			}
			if !strings.Contains(diags[0].Message, "must be Boolean") {
				t.Errorf("diagnostic does not report the condition's type: %q", diags[0].Message)
			}
		})
	}
}

// A Boolean condition in any of the loop forms is accepted, including one that
// reads a name the loop's own body declares.
func TestTypeCheckBooleanControlFlowConditionsOK(t *testing.T) {
	src := `
		package P {
			action a {
				attribute total = 0;
				attribute steps = (1, 2);
				action driver {
					while total < 5 {
						attribute bump = 1;
						assign total := total + bump;
						if total == 2 { assign total := total + 1; } else { assign total := total - 1; }
					}
					loop { assign total := total + 1; } until total > 9;
					for s in steps { assign total := total + s; }
				}
			}
		}
	`
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics for the loop forms, got %v", diags)
	}
}

// TestTypeCheckConjugatedTyping covers SysML v2 §7.12.3: `~` names the
// conjugated definition of a port definition, so its target must be a port and
// only a port usage or a connector end may be typed by it.
func TestTypeCheckConjugatedTyping(t *testing.T) {
	if diags := typeDiags(t, `port def P { in item i; }
		part def Craft { port p : ~P; }
		interface def I { end a : P; end b : ~P; }`); len(diags) != 0 {
		t.Errorf("expected no type diagnostics, got %v", diags)
	}

	notAPort := typeDiags(t, "part def Q; part def Craft { port p : ~Q; }")
	if len(notAPort) == 0 {
		t.Errorf("expected a diagnostic for conjugating a part definition")
	} else if !strings.Contains(notAPort[0].Message, "conjugated port definition") {
		t.Errorf("diagnostic = %q, want it to name conjugation", notAPort[0].Message)
	}

	notAPortUsage := typeDiags(t, "port def P; part def Craft { part p : ~P; }")
	if len(notAPortUsage) == 0 {
		t.Errorf("expected a diagnostic for a part usage typed by a conjugated port")
	}
}
