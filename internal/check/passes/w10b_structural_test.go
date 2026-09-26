package passes

import (
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A second entry/do/exit action, a second return parameter and a composite
// usage owned by a port each draw the reference's diagnostic, on the extra
// member rather than on the declaration.
func TestW10BStructuralReportsTheExtraMember(t *testing.T) {
	const src = `package P {
		action def Act;
		part def Fuel;
		state def S {
			entry action e1 : Act;
			entry action e2 : Act;
			do action d1 : Act;
			do action d2 : Act;
			exit action x1 : Act;
			exit action x2 : Act;
		}
		calc def C {
			return a;
			return b;
		}
		port def BadPort {
			part fuel : Fuel;
			ref part other : Fuel;
		}
		part holder {
			port bad : BadPort {
				part nested : Fuel;
			}
		}
	}`
	byMessage := map[string]int{}
	for _, d := range typeDiags(t, src) {
		switch d.Message {
		case msgOnlyOneEntryAction, msgOnlyOneDoAction, msgOnlyOneExitAction,
			msgOnlyOneReturn, msgPortDefComposite, msgPortUsageComposite:
			byMessage[d.Message]++
			if d.Severity != diag.SeverityError {
				t.Errorf("%q severity = %v, want an error", d.Message, d.Severity)
			}
		}
	}
	for msg, want := range map[string]int{
		msgOnlyOneEntryAction: 1,
		msgOnlyOneDoAction:    1,
		msgOnlyOneExitAction:  1,
		msgOnlyOneReturn:      1,
		msgPortDefComposite:   1,
		msgPortUsageComposite: 1,
	} {
		if byMessage[msg] != want {
			t.Errorf("got %d %q diagnostics, want %d", byMessage[msg], msg, want)
		}
	}
}

// One of each subaction, one return, and referential port members — including
// the reference-subsetting form the training corpus uses — stay silent.
func TestW10BStructuralClean(t *testing.T) {
	const src = `package P {
		action def Act;
		part def Fuel;
		attribute def Temp;
		state def S {
			entry action e1 : Act;
			do action d1 : Act;
			exit action x1 : Act;
		}
		calc def C {
			return a;
		}
		port def GoodPort {
			attribute t : Temp;
			ref part fuel : Fuel;
			in item i : Fuel;
		}
		part holder {
			part fuel : Fuel;
			port good : GoodPort {
				ref part f2 : Fuel;
				port sub : GoodPort;
			}
		}
	}`
	for _, d := range typeDiags(t, src) {
		switch d.Message {
		case msgOnlyOneEntryAction, msgOnlyOneDoAction, msgOnlyOneExitAction,
			msgOnlyOneReturn, msgPortDefComposite, msgPortUsageComposite:
			t.Errorf("unexpected %q at offset %d", d.Message, d.Span.Offset)
		}
	}
}

func TestW10BReturnParametersAcrossFunctionKinds(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		duplicate string
		single    string
	}{
		{
			name:      "constraint_definition",
			file:      "constraint.sysml",
			duplicate: "package P { constraint def K { return r1; return r2; } }",
			single:    "package P { constraint def K { return r1; } }",
		},
		{
			name:      "constraint_usage",
			file:      "constraint.sysml",
			duplicate: "package P { constraint c { return r1; return r2; } }",
			single:    "package P { constraint c { return r1; } }",
		},
		{
			name:      "predicate",
			file:      "predicate.kerml",
			duplicate: "package P { predicate Pred { return r1; return r2; } }",
			single:    "package P { predicate Pred { return r1; } }",
		},
		{
			name:      "function",
			file:      "function.kerml",
			duplicate: "package P { function F { return r1; return r2; } }",
			single:    "package P { function F { return r1; } }",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := typeDiagsForFile(t, tt.file, tt.duplicate)
			var got []diag.Diagnostic
			for _, d := range diags {
				if d.Code == "only-one-return-parameter" {
					got = append(got, d)
				}
			}
			if len(got) != 1 {
				t.Fatalf("got %d duplicate-return diagnostics, want one: %v", len(got), diags)
			}
			if got[0].Message != msgOnlyOneReturn {
				t.Errorf("message = %q, want %q", got[0].Message, msgOnlyOneReturn)
			}
			if got[0].Severity != diag.SeverityError {
				t.Errorf("severity = %v, want an error", got[0].Severity)
			}
			if want := strings.Index(tt.duplicate, "return r2"); got[0].Span.Offset != want {
				t.Errorf("diagnostic offset = %d, want second return at %d", got[0].Span.Offset, want)
			}

			for _, d := range typeDiagsForFile(t, tt.file, tt.single) {
				if d.Code == "only-one-return-parameter" {
					t.Errorf("single return produced a duplicate-return diagnostic at offset %d", d.Span.Offset)
				}
			}
		})
	}
}

func typeDiagsForFile(t *testing.T, file, src string) []diag.Diagnostic {
	t.Helper()
	p := parser.New(source.New(file, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parser diagnostics for %s: %v", file, p.Diagnostics)
	}
	idx := symbols.NewIndex()
	idx.AddDocument(file, root)
	var out []diag.Diagnostic
	for _, d := range Analyze(file, root, nil, idx) {
		if d.Source == "type" {
			out = append(out, d)
		}
	}
	return out
}

func TestW10BPortEventOccurrenceIsReferential(t *testing.T) {
	const src = `package P {
		part holder {
			port base;
			port p {
				event occurrence changed;
			}
		}
	}`
	for _, d := range typeDiags(t, src) {
		if d.Message == msgPortUsageComposite {
			t.Fatalf("unexpected composite port-usage diagnostic at offset %d", d.Span.Offset)
		}
	}
}

func TestW10BRedefinedPortMayNotOwnCompositeUsages(t *testing.T) {
	const src = `package P {
		part def Holder { port base; }
		part holder : Holder {
			port redefines base {
				part child;
			}
		}
	}`
	found := false
	for _, d := range typeDiags(t, src) {
		if d.Message == msgPortUsageComposite {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("a redefined port with a composite nested part must be reported")
	}
}

// A redefinition names a feature of the owner's generals, never a sibling
// (KerML 8.2.3.5.2), so the port here redefines nothing and the name-resolution
// tier reports it; the type tier does not run behind that error.
func TestW10BSiblingRedefinitionIsUnresolved(t *testing.T) {
	const src = `package P {
		part holder {
			port base;
			port redefines base {
				part child;
			}
		}
	}`
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	for _, d := range Analyze("<t>", root, nil, idx) {
		if strings.HasPrefix(d.Message, "unresolved reference: base") {
			return
		}
	}
	t.Error("a redefinition of a sibling must be reported unresolved")
}

// A variant port under a port owner has no owning type, so it must be referential;
// the pilot reports each composite one down a chain of variations and no other.
func TestW10BVariantPortMustBeReferential(t *testing.T) {
	const src = `package P {
		port def PD;
		part def D;
		port def Q {
			port p1 : PD;
			variation port vp : PD {
				variant port a : PD;
				variant abstract port b : PD;
				variant port c : PD { port n : PD; }
				variant ref port ok1 : PD;
				variant in port ok2 : PD;
				variant end port ok3 : PD;
				variant p1;
			}
			variation ref part vpart : D {
				variant variation port vq : PD {
					variant port d : PD;
				}
			}
		}
		part holder {
			port q : Q {
				variation port vp2 : ~PD {
					variant port e : ~PD;
				}
			}
			part x {
				variation port vp3 : PD {
					variant port fine : PD;
				}
			}
		}
	}`
	var got []int
	for _, d := range typeDiags(t, src) {
		switch d.Message {
		case msgVariantPortComposite:
			got = append(got, 1+strings.Count(src[:d.Span.Offset], "\n"))
		case msgPortDefComposite, msgPortUsageComposite:
			t.Errorf("unexpected %q at offset %d", d.Message, d.Span.Offset)
		}
	}
	sort.Ints(got)
	if want := []int{7, 8, 9, 16, 17, 24}; !slices.Equal(got, want) {
		t.Errorf("variant port diagnostics on lines %v, want %v", got, want)
	}
}

// A variant is reached through its variation's owner alone, so a composite variant
// port nested in a variation port is reported once, like the pilot; under a part
// owner no variant is reported, and a variant that is no port is no nested usage.
func TestW10BNestedVariantPortsReportOnce(t *testing.T) {
	const src = `package P {
		port def PD;
		part def D;
		port def Q {
			variation port vp : PD {
				variant variation port a : PD {
					variant port b : PD;
					variant ref port ok : PD;
				}
				variant port c : PD;
				variant part x : D;
			}
		}
		part def Owner {
			variation port vp : PD {
				variant variation port a : PD {
					variant port b : PD;
				}
				variant port c : PD;
				variant part x : D;
			}
		}
	}`
	var got []int
	for _, d := range typeDiags(t, src) {
		switch d.Message {
		case msgVariantPortComposite:
			got = append(got, 1+strings.Count(src[:d.Span.Offset], "\n"))
		case msgPortDefComposite, msgPortUsageComposite:
			t.Errorf("unexpected %q at offset %d", d.Message, d.Span.Offset)
		}
	}
	sort.Ints(got)
	if want := []int{6, 7, 10}; !slices.Equal(got, want) {
		t.Errorf("variant port diagnostics on lines %v, want %v", got, want)
	}
}

// Only a port that restates `variation` is a variation to the pilot: a variant under
// a plain redefinition of one is reported as misplaced instead, never as composite.
func TestW10BVariantPortUnderRedefinedVariation(t *testing.T) {
	const src = `package P {
		port def PD;
		port def Q { variation port vp : PD { variant ref port a : PD; } }
		port def R :> Q { port :>> vp { variant port b : PD; } }
		port def S :> Q { variation port :>> vp { variant port c : PD; } }
		part holder { port q : Q { port :>> vp { variant port d : PD; } } }
	}`
	var got []int
	for _, d := range typeDiags(t, src) {
		if d.Message == msgVariantPortComposite {
			got = append(got, 1+strings.Count(src[:d.Span.Offset], "\n"))
		}
	}
	if want := []int{5}; !slices.Equal(got, want) {
		t.Errorf("variant port diagnostics on lines %v, want %v", got, want)
	}
}

// Malformed input must not panic the pass.
func TestW10BStructuralMalformed(t *testing.T) {
	for _, src := range []string{
		"package P { state def S { entry ; entry ; } }",
		"package P { port def X { part",
		"package P { calc def C { return ; return ; } ",
	} {
		typeDiags(t, src)
	}
}
