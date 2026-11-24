package passes

import (
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// strictRedefinitionFixture is the model the strict redefinition check must
// reject: `length` resolves through the import, but HotLegPipe declares no such
// member, so `:>> length` has nothing to redefine in the pipe's type.
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
