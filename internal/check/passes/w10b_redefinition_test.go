package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// spanText returns the source text a diagnostic covers, so a test can assert
// the location the reference names rather than an offset.
func spanText(src string, d diag.Diagnostic) string {
	if d.Span.Offset < 0 || d.Span.End() > len(src) {
		return ""
	}
	return src[d.Span.Offset:d.Span.End()]
}

// Two package-level features cannot redefine one another: both are featured by
// Anything (validation/invalid/Redefinition_OwningType_Invalid).
func TestW10BPackageLevelRedefinitionIsReported(t *testing.T) {
	const src = `package P {
		part def Wheel;
		part wheel : Wheel;
		part wheel1 redefines wheel;
	}`
	diags := only(constraintDiags(t, src), "redefinition-package-level")
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	if diags[0].Message != msgRedefinePackageLevel {
		t.Errorf("message = %q, want %q", diags[0].Message, msgRedefinePackageLevel)
	}
	if diags[0].Severity != diag.SeverityError {
		t.Errorf("severity = %v, want an error", diags[0].Severity)
	}
	// The diagnostic sits on the redefined reference, as in the reference.
	if got := spanText(src, diags[0]); got != "wheel" {
		t.Errorf("span text = %q, want %q", got, "wheel")
	}
}

// A package-level feature redefined from inside a type is well-formed: the
// featuring types differ (validation/valid/Redefinition_OwningType).
func TestW10BPackageLevelRedefinitionFromTypeIsClean(t *testing.T) {
	const src = `package P {
		part def Wheel;
		part wheel : Wheel;
		part def Vehicle {
			part redefines wheel;
		}
	}`
	if diags := only(constraintDiags(t, src), "redefinition-package-level"); len(diags) != 0 {
		t.Fatalf("redefining from inside a type is legal, got %v", diags)
	}
}

// A redefinition may not share its featuring type with the redefined feature,
// which an alias hop does not hide.
func TestW10BSameFeaturingTypeIsReported(t *testing.T) {
	const src = `package P {
		part def Engine;
		part def B {
			alias eng1 for Vehicle::eng;
		}
		part def Vehicle specializes B {
			part eng : Engine;
			part smallEng : Engine redefines eng1;
		}
	}`
	diags := only(constraintDiags(t, src), "redefinition-same-featuring")
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	if diags[0].Message != msgRedefineSameFeaturing {
		t.Errorf("message = %q, want %q", diags[0].Message, msgRedefineSameFeaturing)
	}
	if got := spanText(src, diags[0]); got != "eng1" {
		t.Errorf("span text = %q, want %q", got, "eng1")
	}
}

// Redefining a feature inherited from a supertype is well-formed.
func TestW10BInheritedRedefinitionIsClean(t *testing.T) {
	const src = `package P {
		part def Engine;
		part def Vehicle { part eng : Engine; }
		part def SmallVehicle :> Vehicle { part smallEng : Engine redefines eng; }
	}`
	if diags := only(constraintDiags(t, src), "redefinition-same-featuring"); len(diags) != 0 {
		t.Fatalf("inherited redefinition is legal, got %v", diags)
	}
}

// Within an Association or a Connector an end is redefined only by an end (KerML 1.1
// §8.3.3.3.8), whatever notation declares the owner; the pinned pilot reports every case.
func TestW10BEndFeatureRedefinitionIsReported(t *testing.T) {
	const kermlTypes = `class T; feature x : T; feature y : T;
		assoc AS { end feature e1 : T; end feature e2 : T; }
		interaction IA { end feature s : T; end feature t : T; }`
	const sysmlTypes = `part def A { port p : P; attribute v : ScalarValues::Integer; } port def P;
		part x : A; part y : A;
		connection def Conn { end one : A; end two : A; }
		interface def IF { end port p1 : P; end port p2 : P; }
		allocation def AL { end src : A; end tgt : A; }
		flow def FD { end s : A; end t : A; }`
	cases := []struct{ name, file, src string }{
		{"kerml assoc", "k.kerml", kermlTypes + " assoc AS2 :> AS { feature yy :>> e1; }"},
		{"kerml assoc struct", "k.kerml", kermlTypes + " assoc struct ASS :> AS { feature ww :>> e1; }"},
		{"kerml interaction", "k.kerml", kermlTypes + " interaction IA2 :> IA { feature zz :>> s; }"},
		{"kerml connector", "k.kerml", kermlTypes + " connector c : AS from x to y { feature c1 :>> e1; }"},
		{"kerml flow", "k.kerml", kermlTypes + " flow f : IA from x to y { feature f1 :>> s; }"},
		{"sysml connection def", "s.sysml", sysmlTypes + " connection def Conn2 :> Conn { constant ref q :>> one; }"},
		{"sysml interface def", "s.sysml", sysmlTypes + " interface def IF2 :> IF { constant ref q :>> p1; }"},
		{"sysml allocation def", "s.sysml", sysmlTypes + " allocation def AL2 :> AL { constant ref q :>> src; }"},
		{"sysml flow def", "s.sysml", sysmlTypes + " flow def FD2 :> FD { constant ref q :>> s; }"},
		{"sysml connection usage", "s.sysml", sysmlTypes + " connection c : Conn connect x to y { constant ref q :>> one; }"},
		{"sysml interface usage", "s.sysml", sysmlTypes + " interface i : IF connect x.p to y.p { constant ref q :>> p1; }"},
		{"sysml allocation usage", "s.sysml", sysmlTypes + " allocation al : AL allocate x to y { constant ref q :>> src; }"},
		{"sysml flow usage", "s.sysml", sysmlTypes + " flow f : FD from x.v to y.v { constant ref q :>> s; }"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := only(diagsIn(t, tc.file, "package RedefinitionEnd { "+tc.src+" }", "constraint"), "redefinition-not-end")
			if len(diags) != 1 {
				t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
			}
			if diags[0].Message != msgRedefineEndFeature {
				t.Errorf("message = %q, want %q", diags[0].Message, msgRedefineEndFeature)
			}
			if diags[0].Severity != diag.SeverityError {
				t.Errorf("severity = %v, want an error", diags[0].Severity)
			}
		})
	}
}

// Outside an Association or a Connector the owner condition of KerML 1.1 §8.3.3.3.8 is not
// met, so a non-end may redefine an inherited end; the pinned pilot is silent on every case.
func TestW10BNonEndRedefiningEndOutsideAssociationOrConnectorIsClean(t *testing.T) {
	cases := []struct{ name, file, src string }{
		{"kerml feature typed by assoc struct", "k.kerml",
			"class T; assoc struct AS { end feature e1 : T; end feature e2 : T; } feature k : AS { feature zz :>> e1; }"},
		{"kerml feature typed by assoc", "k.kerml",
			"class T; assoc AS { end feature e1 : T; end feature e2 : T; } feature k : AS { feature zz :>> e1; }"},
		{"kerml step typed by interaction", "k.kerml",
			"class T; interaction IA { end feature s : T; end feature t : T; } step k : IA { feature zz :>> s; }"},
		{"kerml class", "k.kerml",
			"class T; class C { end feature e : T; } class D :> C { feature d :>> e; }"},
		{"sysml part typed by connection def", "s.sysml",
			"part def A; connection def Conn { end one : A; end two : A; } part k : Conn { constant ref kk :>> one; }"},
		{"sysml part typed by interface def", "s.sysml",
			"port def P; interface def IF { end port p1 : P; end port p2 : P; } part k : IF { constant ref kk :>> p1; }"},
		{"sysml item typed by flow def", "s.sysml",
			"part def A; flow def FD { end s : A; end t : A; } item k : FD { constant ref kk :>> s; }"},
		{"sysml part def", "s.sysml",
			"part def A; part def D { end part e : A; } part def E :> D { constant part :>> e; }"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diags := only(diagsIn(t, tc.file, "package RedefinitionEnd { "+tc.src+" }", "constraint"), "redefinition-not-end"); len(diags) != 0 {
				t.Fatalf("owner is neither an association nor a connector, got %v", diags)
			}
		})
	}
}

// The newly legal shapes as fixtures analyzed against the library, in both
// notations, so a regression in the owner condition is attributable to the model.
func TestW10BRedefinitionEndOwnerFixturesAreClean(t *testing.T) {
	for _, file := range []string{"redefinition_end_owner.sysml", "redefinition_end_owner.kerml"} {
		if diags := libraryFixtureDiags(t, file); len(diags) != 0 {
			t.Errorf("%s: expected no diagnostics, got %v", file, diags)
		}
	}
}

// An end feature redefined by an end feature is well-formed.
func TestW10BEndRedefiningEndIsClean(t *testing.T) {
	const src = `package RedefinitionEnd {
		part def A { end part e; }
		part def B :> A { end part :>> e; }
	}`
	if diags := only(constraintDiags(t, src), "redefinition-not-end"); len(diags) != 0 {
		t.Fatalf("an end redefining an end is legal, got %v", diags)
	}
}

// Malformed input must not panic and must not invent a redefinition target.
func TestW10BRedefinitionMalformedInput(t *testing.T) {
	const src = `package P {
		part wheel1 redefines ;
		part def { part :>> ; }
	}`
	for _, code := range []string{"redefinition-package-level", "redefinition-same-featuring", "redefinition-not-end"} {
		if diags := only(constraintDiags(t, src), code); len(diags) != 0 {
			t.Fatalf("%s fired on malformed input: %v", code, diags)
		}
	}
}

// A feature declared at the document root is package-level too: the root
// namespace is no type, so two root features cannot redefine one another.
func TestW10BRootLevelRedefinitionIsReported(t *testing.T) {
	for name, src := range map[string]string{
		"sysml": `part def Engine;
part eng : Engine;
part eng2 : Engine :>> eng;`,
		"kerml": `class Engine;
feature eng : Engine;
feature eng2 : Engine :>> eng;`,
	} {
		t.Run(name, func(t *testing.T) {
			diags := only(constraintDiags(t, src), "redefinition-package-level")
			if len(diags) != 1 {
				t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
			}
			if diags[0].Message != msgRedefinePackageLevel {
				t.Errorf("message = %q, want %q", diags[0].Message, msgRedefinePackageLevel)
			}
			if got := spanText(src, diags[0]); got != "eng" {
				t.Errorf("span text = %q, want %q", got, "eng")
			}
		})
	}
}

// A root-level feature redefined from inside a type has a featuring type on
// one side only, so the pair is well-formed.
func TestW10BRootLevelRedefinitionFromTypeIsClean(t *testing.T) {
	const src = `part def Engine;
part eng : Engine;
part def Car {
	part :>> eng;
}`
	if diags := constraintDiags(t, src); len(diags) != 0 {
		t.Fatalf("redefining a root feature from inside a type is legal, got %v", diags)
	}
}
