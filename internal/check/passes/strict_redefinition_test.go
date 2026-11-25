package passes

import (
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// strictRedefinitionFixture declares a pipe typed by a type with no `length`;
// the name resolves through an import, so only strict mode rejects it.
const strictRedefinitionFixture = "../../../tests/testdata/passes/strict_redefinition_resolves_to_nothing.sysml"

func strictRedefinitionDiags(t *testing.T, name, src string, mode diag.ConformanceMode) []diag.Diagnostic {
	t.Helper()
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx := newTestIndexFromDoc(name, root)
	idx.ExpandWildcardImports()
	return AnalyzeWithOptions(name, source.KindOf(name), root, nil, idx, Options{Conformance: mode})
}

func TestStrictRedefinitionResolvesToNothing(t *testing.T) {
	data, err := os.ReadFile(strictRedefinitionFixture)
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	got := strictRedefinitionDiags(t, "model.sysml", src, diag.ConformanceStrict)
	var found []diag.Diagnostic
	for _, d := range got {
		if d.Code == CodeRedefinitionResolvesToNothing {
			found = append(found, d)
		}
	}
	if len(found) != 1 {
		t.Fatalf("got %d %s diagnostics, want one: %+v", len(found), CodeRedefinitionResolvesToNothing, got)
	}
	d := found[0]
	if d.Severity != diag.SeverityError {
		t.Errorf("severity = %v, want error", d.Severity)
	}
	const want = "member length redefines nothing resolvable in HotLegPipe"
	if d.Message != want {
		t.Errorf("message = %q, want %q", d.Message, want)
	}
	wantOffset := strings.Index(src, "attribute :>> length") + len("attribute :>> ")
	if d.Span.Offset != wantOffset || d.Span.Len != len("length") {
		t.Errorf("span = %+v, want offset %d length %d", d.Span, wantOffset, len("length"))
	}
	pos := source.New("model.sysml", []byte(src)).Lines().PosAt(d.Span.Offset)
	if pos.Line != 10 || pos.Col != 27 {
		t.Errorf("position = %d:%d, want 10:27", pos.Line, pos.Col)
	}
}

// The finding exists only under strict conformance: the same model is accepted
// in the default mode, so the committed corpora do not move.
func TestStrictRedefinitionIsADefaultModeNoOp(t *testing.T) {
	data, err := os.ReadFile(strictRedefinitionFixture)
	if err != nil {
		t.Fatal(err)
	}
	got := strictRedefinitionDiags(t, "model.sysml", string(data), diag.ConformanceDefault)
	if len(got) != 0 {
		t.Fatalf("default mode reported %+v, want silence", got)
	}
}

// A redefinition of a member the type actually declares or inherits is clean
// under strict conformance, whether its target is written bare or qualified.
func TestStrictRedefinitionAcceptsDeclaredAndInheritedMembers(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"inherited", `package P {
			part def Base { attribute length; }
			part def Derived :> Base { attribute :>> length; }
		}`},
		{"qualified", `package P {
			part def Base { attribute length; }
			part def Derived :> Base { attribute :>> Base::length; }
		}`},
		{"owned", `package P {
			part def Holder {
				attribute length;
				attribute :>> length;
			}
		}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := strictRedefinitionDiags(t, "model.sysml", tc.src, diag.ConformanceStrict)
			for _, d := range got {
				if d.Code == CodeRedefinitionResolvesToNothing {
					t.Errorf("unexpected %s: %s", d.Code, d.Message)
				}
			}
		})
	}
}

// A typed usage that redefines or subsets a usage carries that usage's
// members, so nested redefinitions of them are legal.
func TestStrictRedefinitionAcceptsMembersOfRedefinedTyping(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"redefines", `package C {
			part def Wheel { attribute radius; }
			part def Car { part wheel : Wheel { attribute pressure; } }
			part def SportsCar :> Car { part :>> wheel : Wheel { attribute :>> pressure; } }
		}`},
		{"subsets", `package C {
			part def Wheel { attribute radius; }
			part def Car { part wheel : Wheel { attribute pressure; } }
			part def SportsCar :> Car { part spare subsets Car::wheel : Wheel { attribute :>> pressure; } }
		}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := strictRedefinitionDiags(t, "model.sysml", tc.src, diag.ConformanceStrict)
			for _, d := range got {
				if d.Code == CodeRedefinitionResolvesToNothing {
					t.Errorf("unexpected %s: %s", d.Code, d.Message)
				}
			}
		})
	}
}

// A redefinition in a definition may name an enclosing-namespace feature,
// including one reached by a private import in that namespace.
func TestStrictRedefinitionAcceptsImportedTargetInDefinition(t *testing.T) {
	const src = `package P {
		private import ISQBase::*;
		item def Curve { attribute radius; }
		item def PlanarCurve :> Curve { attribute :>> length [1]; }
	}`
	got := strictRedefinitionDiags(t, "model.sysml", src, diag.ConformanceStrict)
	for _, d := range got {
		if d.Code == CodeRedefinitionResolvesToNothing {
			t.Errorf("unexpected %s: %s", d.Code, d.Message)
		}
	}
}

// A usage that subsets a feature of an enclosing type stands in for the
// original feature, so its nested redefinitions may name that feature's kin.
func TestStrictRedefinitionAcceptsTargetOfSubsettedType(t *testing.T) {
	const src = `package P {
		part def Base { part source; }
		part def Derived :> Base { part thisThing :>> source; }
		part holders : Derived [0..*];
		part holder : Derived subsets holders {
			part thisThing :>> Derived::thisThing, Base::source;
		}
	}`
	got := strictRedefinitionDiags(t, "model.sysml", src, diag.ConformanceStrict)
	for _, d := range got {
		if d.Code == CodeRedefinitionResolvesToNothing {
			t.Errorf("unexpected %s: %s", d.Code, d.Message)
		}
	}
}

// A body nested in an occurrence (a snapshot or timeslice) redefines features
// of the type that occurrence instantiates, not of the snapshot itself.
func TestStrictRedefinitionAcceptsSnapshotOfEnclosingType(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		occurrence def Callout { attribute crewOnSite : Integer; }
		individual callout17 : Callout {
			snapshot arrival { attribute :>> crewOnSite = 2; }
			timeslice approach { attribute :>> crewOnSite = 5; }
		}
	}`
	got := strictRedefinitionDiags(t, "model.sysml", src, diag.ConformanceStrict)
	for _, d := range got {
		if d.Code == CodeRedefinitionResolvesToNothing {
			t.Errorf("unexpected %s: %s", d.Code, d.Message)
		}
	}
}

// A member of an enclosing type is not a member of the declaring type: the
// narrow context must still report a targetless redefinition.
func TestStrictRedefinitionDoesNotBorrowAnEnclosingTypeName(t *testing.T) {
	const src = `package P {
		part def Outer {
			attribute length;
			part def Inner { attribute diameter; }
			part pipe : Inner {
				attribute :>> length = 7;
			}
		}
	}`
	got := strictRedefinitionDiags(t, "model.sysml", src, diag.ConformanceStrict)
	var found []diag.Diagnostic
	for _, d := range got {
		if d.Code == CodeRedefinitionResolvesToNothing {
			found = append(found, d)
		}
	}
	if len(found) != 1 {
		t.Fatalf("got %d %s diagnostics, want one: %+v", len(found), CodeRedefinitionResolvesToNothing, got)
	}
	if !strings.Contains(found[0].Message, "in Inner") {
		t.Errorf("message = %q, want the declaring type Inner", found[0].Message)
	}
}

// A target the redefining feature cannot access is already reported by the
// inheritance rule; the strict check must not add a contradictory second finding.
func TestStrictRedefinitionDefersToTheInheritanceRule(t *testing.T) {
	const src = `package P {
		part def A { attribute diameter; }
		part def B { attribute length; }
		part p : A {
			attribute :>> B::length = 1;
		}
	}`
	got := strictRedefinitionDiags(t, "model.sysml", src, diag.ConformanceStrict)
	var related []diag.Diagnostic
	for _, d := range got {
		if d.Code == CodeRedefinitionResolvesToNothing || d.Code == "redefinition-no-inherited" {
			related = append(related, d)
		}
	}
	if len(related) != 1 || related[0].Code != "redefinition-no-inherited" {
		t.Fatalf("got %+v, want only redefinition-no-inherited", related)
	}
}

// A targetless redefinition in a snapshot reports the occurrence's type, not
// the snapshot usage that declares the clause.
func TestStrictRedefinitionSnapshotFailureNamesTheOccurrenceType(t *testing.T) {
	const src = `package P {
		private import ISQBase::*;
		occurrence def Callout { attribute diameter; }
		occurrence callout17 : Callout {
			snapshot arrival {
				attribute :>> length = 10.0 [SI::m];
			}
		}
	}`
	got := strictRedefinitionDiags(t, "model.sysml", src, diag.ConformanceStrict)
	var found []diag.Diagnostic
	for _, d := range got {
		if d.Code == CodeRedefinitionResolvesToNothing {
			found = append(found, d)
		}
	}
	if len(found) != 1 {
		t.Fatalf("got %d %s diagnostics, want one: %+v", len(found), CodeRedefinitionResolvesToNothing, got)
	}
	if !strings.Contains(found[0].Message, "in Callout") {
		t.Errorf("message = %q, want the occurrence type Callout", found[0].Message)
	}
}

// A chain redefinition is judged by where its prefix is declared, not by the
// members of the type owning the clause; the strict check leaves it alone.
func TestStrictRedefinitionLeavesChainsToTheirOwnRule(t *testing.T) {
	const src = `package P {
		private import ScalarValues::Integer;
		part def Vehicle { attribute fuel : Integer default 0; }
		part vehicle1 : Vehicle;
		requirement def Full {
			subject vehicle : Vehicle;
			require constraint { vehicle.fuel > 0 };
		}
		satisfy requirement check : Full by vehicle1 {
			attribute redefines vehicle1.fuel = 1300;
		}
	}`
	got := strictRedefinitionDiags(t, "model.sysml", src, diag.ConformanceStrict)
	for _, d := range got {
		if d.Code == CodeRedefinitionResolvesToNothing {
			t.Errorf("unexpected %s: %s", d.Code, d.Message)
		}
	}
}

// A name that resolves to no element is the name-resolution tier's finding;
// the strict redefinition check does not report it a second time.
func TestStrictRedefinitionLeavesUnresolvedNamesToNameResolution(t *testing.T) {
	const src = `package P {
		part def HotLegPipe { attribute diameter; }
		part def Holder {
			part pipe : HotLegPipe {
				attribute :>> length;
			}
		}
	}`
	root := parser.New(source.New("model.sysml", []byte(src))).ParseFile()
	idx := newTestIndexFromDoc("model.sysml", root)
	ctx := NewContextWithOptions("model.sysml", source.KindSysML, idx, nil, Options{Conformance: diag.ConformanceStrict})
	got := StrictRedefinitionPass{}.Run(ctx, "model.sysml", root)
	if len(got) != 0 {
		t.Fatalf("got %+v, want the unresolved name left to name resolution", got)
	}
}
