package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// conformanceDiags returns the feature-conformance findings of a KerML source.
func conformanceDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>.kerml", []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<t>.kerml", root)
	var out []diag.Diagnostic
	for _, d := range Analyze("<t>.kerml", root, nil, idx) {
		switch d.Code {
		case "redefinition-direction-conformance",
			"subsetting-uniqueness-conformance",
			"subsetting-constancy-conformance":
			out = append(out, d)
		}
	}
	return out
}

// codes returns the diagnostic codes, so a case can assert what did and did not
// fire without depending on order.
func codes(diags []diag.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Code)
	}
	return out
}

// Redefinition_DirectionConformance_invalid.kerml.xt: `out :>> z` redefines an
// `in` feature.
func TestW8BDirectionConformanceOpposingDirection(t *testing.T) {
	diags := conformanceDiags(t, `
		package P {
			class C { in z; }
			class D specializes C { out :>> z; }
		}
	`)
	if !hasCode(diags, "redefinition-direction-conformance") {
		t.Fatalf("expected a direction violation, got %v", codes(diags))
	}
	if len(diags) != 1 {
		t.Fatalf("got %d conformance diagnostics, want one: %v", len(diags), codes(diags))
	}
}

func TestW8BDirectionConformanceThroughConjugation(t *testing.T) {
	// D1 reaches C by a conjugation, which reverses z to `out`, so `in` breaks.
	diags := conformanceDiags(t, `
		package P {
			class C { in z; }
			class C1 conjugates C;
			class D1 specializes C1 { in :>> z; }
		}
	`)
	if !hasCode(diags, "redefinition-direction-conformance") {
		t.Fatalf("expected a direction violation through conjugation, got %v", codes(diags))
	}
}

func TestW8BDirectionConformanceAdmitsAnyDirectionForInout(t *testing.T) {
	// A redefined `inout` admits any declared direction (KerML 8.3.3.3.8).
	diags := conformanceDiags(t, `
		package P {
			class C { inout y; inout z; in x; }
			class D specializes C { out :>> y; in :>> z; in :>> x; }
		}
	`)
	if len(diags) != 0 {
		t.Fatalf("expected no violation, got %v", codes(diags))
	}
}

// Redefinition_DirectionConformance_invalid.kerml.xt: a redefined `in`, `out`
// or `inout` is not conformed to by a redefinition declaring no direction.
func TestW8BDirectionConformanceRejectsUndeclaredDirection(t *testing.T) {
	for _, dir := range []string{"in", "out", "inout"} {
		t.Run(dir, func(t *testing.T) {
			diags := conformanceDiags(t, `
				package P {
					class C { `+dir+` x; }
					class D specializes C { feature :>> x; }
				}
			`)
			if len(diags) != 1 || diags[0].Code != "redefinition-direction-conformance" {
				t.Fatalf("got %v, want one direction violation", codes(diags))
			}
		})
	}
}

// A feature with no direction may be redefined with or without one.
func TestW8BDirectionConformanceUndirectedTargetAdmitsAny(t *testing.T) {
	diags := conformanceDiags(t, `
		package P {
			class C { feature x; feature y; }
			class D specializes C { feature :>> x; in :>> y; }
		}
	`)
	if len(diags) != 0 {
		t.Fatalf("expected no violation, got %v", codes(diags))
	}
}

// directionDiags returns the direction-conformance findings of a SysML source.
func directionDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>.sysml", []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<t>.sysml", root)
	var out []diag.Diagnostic
	for _, d := range Analyze("<t>.sysml", root, nil, idx) {
		if d.Code == "redefinition-direction-conformance" {
			out = append(out, d)
		}
	}
	return out
}

// A usage body redefining an `in` parameter without restating the direction
// declares none, which does not conform; `in :>> …` does.
func TestW8BDirectionConformanceUsageBodyMustRestateDirection(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		action def Burn { in durationS : Real; }
		action burn : Burn { :>> durationS = 3.0; }
		action burns : Burn { in :>> durationS = 3.0; }
	}`
	diags := directionDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("got %d direction diagnostics, want one: %v", len(diags), diags)
	}
	if got := spanText(src, diags[0]); got != "durationS" {
		t.Errorf("reported at %q, want the redefined name", got)
	}
}

// A step's first parameter redefines its behavior's first parameter by position as well
// as what its clause names, so an `out` written first under `in input; out output;` conflicts.
func TestW8BDirectionConformancePositionalParameterRedefinition(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		action def Step { in input : Integer; out output : Integer; }
		action def Chain {
			action stepA : Step { out :>> output; }
			action stepB : Step { in :>> input; out :>> output; }
		}
	}`
	diags := directionDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("got %d direction diagnostics, want one: %v", len(diags), diags)
	}
	if got := spanText(src, diags[0]); got != "out :>> output;" {
		t.Errorf("reported at %q, want the conflicting parameter declaration", got)
	}
}

func TestW8BDirectionConformanceRunsAlongsideTypeErrors(t *testing.T) {
	root := parser.New(source.New("<t>.sysml", []byte(`
		package P {
			port def OutPort {
				out ref anout;
				in ref anin;
			}
			part p {
				port aport : OutPort {
					in ref redefines anout;
					out ref redefines anin;
				}
			}
			port def Bad {
				part nested;
			}
		}
	`))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>.sysml", root)
	var directions []diag.Diagnostic
	for _, d := range Analyze("<t>.sysml", root, nil, idx) {
		if d.Code == "redefinition-direction-conformance" {
			directions = append(directions, d)
		}
	}
	if len(directions) != 2 {
		t.Fatalf("got %d direction diagnostics, want 2: %v", len(directions), directions)
	}
}

// Feature_nonunique_invalid.kerml.xt: a nonunique feature may not subset or
// redefine a unique one.
func TestW8BUniquenessConformance(t *testing.T) {
	diags := conformanceDiags(t, `
		package P {
			classifier A { feature x; }
			classifier B specializes A { feature x1 nonunique subsets x; }
			classifier C specializes A { feature x2 nonunique redefines x; }
		}
	`)
	if len(diags) != 2 {
		t.Fatalf("expected a violation on each of the two features, got %v", codes(diags))
	}
	for _, d := range diags {
		if d.Code != "subsetting-uniqueness-conformance" {
			t.Fatalf("unexpected diagnostic %q", d.Code)
		}
	}
}

func TestW8BUniquenessConformanceAllowsNonuniqueTarget(t *testing.T) {
	diags := conformanceDiags(t, `
		package P {
			classifier A { feature x nonunique; }
			classifier B specializes A { feature x1 nonunique subsets x; }
		}
	`)
	if len(diags) != 0 {
		t.Fatalf("a nonunique target imposes nothing, got %v", codes(diags))
	}
}

// Subsetting_constant_invalid.kerml.xt: a variable feature may not subset or
// redefine a constant one.
func TestW8BConstancyConformance(t *testing.T) {
	diags := conformanceDiags(t, `
		package P {
			class A {
				const feature f;
				var feature g :> f;
			}
			class B :> A {
				var feature h :>> f;
				var feature i :> g;
			}
		}
	`)
	if got := len(diags); got != 2 {
		t.Fatalf("expected the two declared violations, got %d: %v", got, codes(diags))
	}
	for _, d := range diags {
		if d.Code != "subsetting-constancy-conformance" {
			t.Fatalf("unexpected diagnostic %q", d.Code)
		}
	}
}

func TestW8BConstancyConformanceAllowsConstantRestriction(t *testing.T) {
	diags := conformanceDiags(t, `
		package P {
			class A { const feature f; }
			class B :> A { const feature h :>> f; feature k :> f; }
		}
	`)
	if len(diags) != 0 {
		t.Fatalf("a constant or unmodified restriction conforms, got %v", codes(diags))
	}
}

// The rule reads the subsetted feature's own isConstant (KerML 8.3.3.3.9): merely subsetting
// a constant feature makes none constant, and a subsetting cycle declares nothing.
func TestW8BConstancyConformanceIsNotInherited(t *testing.T) {
	diags := conformanceDiags(t, `
		package P {
			class A {
				const feature c;
				feature d :> c;
				var feature e :> d;
				feature a :> b;
				feature b :> a;
				var feature v :> a;
			}
		}
	`)
	if len(diags) != 0 {
		t.Fatalf("constancy is the subsetted feature's own, got %v", codes(diags))
	}
}

// constancyDiags returns the constancy-conformance findings of a SysML source
// analysed with the standard library, which the usages' variability derives from.
func constancyDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	var out []diag.Diagnostic
	for _, d := range w8cLibraryDiagnostics(t, "<t>.sysml", src) {
		if d.Code == "subsetting-constancy-conformance" || d.Severity == diag.SeverityError {
			out = append(out, d)
		}
	}
	return out
}

// A SysML usage owned by an occurrence definition or usage, not a portion nor a composite
// action, varies by Usage::mayTimeVary (SysML v2 §8.3.6.4), so it must be constant to subset one.
func TestW8BConstancyConformanceSysMLUsages(t *testing.T) {
	diags := constancyDiags(t, `
		package P {
			private import ScalarValues::*;
			part def A {
				constant attribute c : Real;
				attribute d :> c;
				derived attribute e :> c;
				ref attribute r :> c;
				constant part cp : A;
				part dp :> cp;
			}
			part def B :> A {
				attribute f :>> c;
			}
			action def Act {
				in constant attribute x : Real;
				in attribute y : Real :> x;
			}
			calc def Cal {
				in constant x : Real;
				return y : Real :> x;
			}
			part p : A {
				attribute g :> c;
			}
		}
	`)
	if got := len(diags); got != 8 {
		t.Fatalf("expected eight violations (d, e, r, dp, f, y, y, g), got %d: %v", got, codes(diags))
	}
	for _, d := range diags {
		if d.Code != "subsetting-constancy-conformance" {
			t.Fatalf("unexpected diagnostic %q: %s", d.Code, d.Message)
		}
	}
}

// An end usage that may vary in time is constant unwritten (SysML v2 §8.2.2.6.2): an end
// conforms to anything, a plain usage subsetting or redefining an end does not.
func TestW8BConstancyConformanceSysMLEnds(t *testing.T) {
	diags := constancyDiags(t, `
		package P {
			part def A { constant ref part r : A; }
			part def B :> A { end part z :>> r; }
			connection def Conn :> Connections::BinaryConnection {
				end one : A;
				end two : A;
				ref part w :> one;
			}
			connection def Conn2 :> Conn { end three :>> one; }
			part k : Conn { end ref za :>> one; }
		}
	`)
	if got := len(diags); got != 1 {
		t.Fatalf("expected the one violation at w :> one, got %d: %v", got, codes(diags))
	}
	if diags[0].Code != "subsetting-constancy-conformance" {
		t.Fatalf("unexpected diagnostic %q: %s", diags[0].Code, diags[0].Message)
	}
}

// Where mayTimeVary is false the premise fails and the rule stays silent: a portion, a usage
// owned by a package or an attribute definition, a composite action, a usage declared constant.
func TestW8BConstancyConformanceSysMLNonVaryingUsages(t *testing.T) {
	diags := constancyDiags(t, `
		package P {
			private import ScalarValues::*;
			part def A {
				constant item x : A;
				timeslice t :> A::x;
				snapshot s :> A::x;
				constant attribute c : Real;
				constant attribute k :> c;
			}
			part a : A { timeslice t2 :> x; }
			part def B :> A { constant attribute e :>> c; }
			attribute def D { attribute c2 : Real; attribute d2 :> c2; }
			action def Act { action ca; action da :> ca; }
			attribute c3 : Real;
			attribute d3 :> c3;
		}
	`)
	if len(diags) != 0 {
		t.Fatalf("no usage here both varies in time and subsets a constant one, got %v", codes(diags))
	}
}

// MetadataTests_MetadataFeature_invalid.kerml.xt: a metadata annotation body
// restates features of the annotated type, so a name it does not offer — even
// one that exists in the surrounding namespace — redefines nothing.
func TestW8BMetadataBodyMustRedefineOwningTypeFeature(t *testing.T) {
	src := `package P {
		metadata def A { feature x; feature u { feature v; } }
		feature bad;
		feature a {
			@A {
				x = 1;
				u { v = 1; bad; }
			}
		}
	}`
	root := parser.New(source.New("<t>.kerml", []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<t>.kerml", root)
	var got []diag.Diagnostic
	for _, d := range Analyze("<t>.kerml", root, nil, idx) {
		if d.Code == "metadata-owning-type-feature" {
			got = append(got, d)
		}
	}
	if len(got) != 1 {
		t.Fatalf("expected one violation, on `bad`, got %d: %v", len(got), got)
	}
	if got[0].Span.Offset == 0 {
		t.Fatalf("diagnostic must be located at the offending declaration: %+v", got[0])
	}
}

// conformanceDiagsSysML returns the feature-conformance findings of a SysML source.
func conformanceDiagsSysML(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>.sysml", []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<t>.sysml", root)
	var out []diag.Diagnostic
	for _, d := range Analyze("<t>.sysml", root, nil, idx) {
		switch d.Code {
		case "subsetting-uniqueness-conformance", "subsetting-constancy-conformance":
			out = append(out, d)
		}
	}
	return out
}

// A subsetting the feature has implicitly is a Subsetting all the same: a
// composite action in an action definition implicitly subsets the unique
// Actions::Action::subactions, so declaring it nonunique breaks uniqueness
// conformance, reported once at the declaration however it subsets (#726).
func TestW8BUniquenessConformanceOfAnImplicitSubsetting(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      int
	}{
		{"implicit", "package N { action def A { action b[*] nonunique; } }", 1},
		{"written and implicit", "package N { action def A { action b[*] nonunique :> subactions; } }", 1},
		{"unique", "package N { action def A { action b[*]; } }", 0},
		// A composite part in a part implicitly subsets Item::subitems, unique too.
		{"composite part in a part", "package N { part def A { part b[*] nonunique; } }", 1},
		// A message is an action: in a part it implicitly subsets Part::ownedActions, unique.
		{"message in a part", "package N { abstract flow def F; part def C { message m : F[*] nonunique; } }", 1},
		// A reference part or an attribute has no implicit subsetting of a unique feature.
		{"reference part", "package N { part def A { ref part b[*] nonunique; } }", 0},
		{"attribute", "package N { part def A { attribute b[*] nonunique; } }", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := conformanceDiagsSysML(t, tc.src)
			if len(diags) != tc.want {
				t.Fatalf("got %v, want %d uniqueness finding(s)", codes(diags), tc.want)
			}
			for _, d := range diags {
				if d.Code != "subsetting-uniqueness-conformance" {
					t.Fatalf("unexpected diagnostic %q", d.Code)
				}
			}
		})
	}
}

// A parameter redefines the general behavior's parameter at its position without `:>>`
// (KerML 7.4.7.2); that implied Redefinition is held to the constancy rule like a written one.
func TestW8BConstancyConformanceImplicitParameters(t *testing.T) {
	src := `package IP {
    private import ScalarValues::*;
    action def Gen { in constant attribute limit : Real; }
    action def Spec :> Gen { in attribute cap : Real; }
    action def SpecOk :> Gen { in constant attribute cap : Real; }
    action def SpecExplicit :> Gen { in attribute cap :>> limit; }
    action def SpecSubsets :> Gen { in attribute cap :> limit; }
}`
	got := codes(constancyDiags(t, src))
	const want = "subsetting-constancy-conformance"
	if len(got) != 3 || got[0] != want || got[1] != want || got[2] != want {
		t.Fatalf("one finding each for Spec, SpecExplicit and SpecSubsets (written `:>` and implied `:>>` share the target), got %v", got)
	}
}

// A clause naming the parameter at its own position is one redefinition, so a
// direction conflict there is reported once, at the clause.
func TestW8BDirectionConformanceWrittenPositionalTargetReportedOnce(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		action def A { in x : Integer; }
		action a : A { out :>> x; }
	}`
	diags := directionDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("got %d direction diagnostics, want one: %v", len(diags), diags)
	}
	if got := spanText(src, diags[0]); got != "x" {
		t.Errorf("reported at %q, want the redefined name", got)
	}
}

// A parameter subsetting its positional counterpart still redefines it by
// position, so an opposing direction is reported from that redefinition.
func TestW8BDirectionConformancePositionalCounterpartSubsetted(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		action def A { in x : Integer; }
		action a : A { out y :> x; }
	}`
	if diags := directionDiags(t, src); len(diags) != 1 {
		t.Fatalf("got %d direction diagnostics, want one: %v", len(diags), diags)
	}
}
