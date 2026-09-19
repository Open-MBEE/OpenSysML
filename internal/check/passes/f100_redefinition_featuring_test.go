package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// constraintDiagsKerML is constraintDiags for KerML notation fixtures.
func constraintDiagsKerML(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>.kerml", []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<t>.kerml", root)
	all := Analyze("<t>.kerml", root, nil, idx)
	var out []diag.Diagnostic
	for _, d := range all {
		if d.Source == "constraint" {
			out = append(out, d)
		}
	}
	return out
}

// F100: a qualified redefinition target reachable through the feature's
// featuring context is legal even when it is not an inherited member of the
// lexical owner (KerML 1.0 §8.3.3.3; TimeVaryingCarDriver.kerml:93).

func TestF100RedefinitionFeaturedByInheritedContext(t *testing.T) {
	src := `
		package P2 {
			struct Vehicle {
				feature speed;
			}
			struct Car specializes Vehicle {
				feature ctx;
				feature speed2 :>> Vehicle::speed featured by ctx;
			}
		}
	`
	diags := constraintDiagsKerML(t, src)
	if hasCode(diags, "redefinition-no-inherited") {
		t.Fatalf("ctx is featured within Car, which inherits speed; got %v", diags)
	}
}

func TestF100RedefinitionSnapshotStyleContexts(t *testing.T) {
	// Featuring contexts that redefine a common feature (the variable-feature
	// snapshot encoding) make the target accessible; the pilot accepts this.
	src := `
		package P3 {
			struct Occ {
				feature snapshots;
			}
			struct A specializes Occ {
				member feature x featured by A_snap {
					member feature A_snap :>> Occ::snapshots featured by A;
				}
			}
			struct B specializes Occ {
				member feature bx : A featured by B_snap {
					member feature B_snap :>> Occ::snapshots featured by B;
				}
				member feature x1 :>> A::x featured by BA_snap {
					member feature BA_snap :>> Occ::snapshots featured by B::bx;
				}
			}
		}
	`
	diags := constraintDiagsKerML(t, src)
	if hasCode(diags, "redefinition-no-inherited") {
		t.Fatalf("x is reachable through the snapshot featuring contexts; got %v", diags)
	}
}

func TestF100RedefinitionFeaturedByUnrelatedContextRejected(t *testing.T) {
	// The featuring context's owner does not inherit speed, so the target is
	// inaccessible; the pilot rejects this ("Must be an accessible feature").
	src := `
		package P1 {
			struct Vehicle {
				feature speed;
			}
			struct Car {
				feature ctx;
				feature speed2 :>> Vehicle::speed featured by ctx;
			}
		}
	`
	diags := constraintDiagsKerML(t, src)
	if !hasCode(diags, "redefinition-no-inherited") {
		t.Fatalf("expected redefinition-no-inherited diagnostic, got %v", diags)
	}
}

func TestF100RedefinitionSnapshotContextsWithoutCommonTargetRejected(t *testing.T) {
	// Featuring contexts that redefine no common feature do not bridge; the
	// pilot rejects this ("Must be an accessible feature").
	src := `
		package P8 {
			struct A {
				member feature x featured by A_snap {
					member feature A_snap featured by A;
				}
			}
			struct B {
				member feature bx : A featured by B_snap {
					member feature B_snap featured by B;
				}
				member feature x1 :>> A::x featured by BA_snap {
					member feature BA_snap featured by B::bx;
				}
			}
		}
	`
	diags := constraintDiagsKerML(t, src)
	if !hasCode(diags, "redefinition-no-inherited") {
		t.Fatalf("expected redefinition-no-inherited diagnostic, got %v", diags)
	}
}

func TestF100RedefinitionSnapshotContextUnrelatedTypingRejected(t *testing.T) {
	// The bridging context's own featuring chain does not conform (bx : C, not
	// A), so the target stays inaccessible; the pilot rejects this.
	src := `
		package P7 {
			struct Occ {
				feature snapshots;
			}
			struct A specializes Occ {
				member feature x featured by A_snap {
					member feature A_snap :>> Occ::snapshots featured by A;
				}
			}
			struct C;
			struct B specializes Occ {
				member feature bx : C featured by B_snap {
					member feature B_snap :>> Occ::snapshots featured by B;
				}
				member feature x1 :>> A::x featured by BA_snap {
					member feature BA_snap :>> Occ::snapshots featured by B::bx;
				}
			}
		}
	`
	diags := constraintDiagsKerML(t, src)
	if !hasCode(diags, "redefinition-no-inherited") {
		t.Fatalf("expected redefinition-no-inherited diagnostic, got %v", diags)
	}
}

func TestF100RedefinitionUnresolvedFeaturedByNoPanic(t *testing.T) {
	// An unresolved featured-by target is a nameres error; the constraint tier
	// is skipped and nothing panics.
	src := `
		package P {
			struct Vehicle {
				feature speed;
			}
			struct Car {
				feature speed2 :>> Vehicle::speed featured by NoSuchContext;
			}
		}
	`
	diags := constraintDiagsKerML(t, src)
	if hasCode(diags, "redefinition-no-inherited") {
		t.Fatalf("constraint tier should be gated by the nameres error, got %v", diags)
	}
}
