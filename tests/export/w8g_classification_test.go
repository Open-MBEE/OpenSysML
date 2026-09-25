package export_test

import "testing"

// TestKeywordDecidedMetaclasses pins the metaclass each spelling builds where
// several spellings share one `ast.UsageKind`: the pinned grammar names them
// DataType (KerML.xtext:788), Function (:924) and ReferenceUsage
// (SysML.xtext:632 DefaultReferenceUsage).
func TestKeywordDecidedMetaclasses(t *testing.T) {
	g := turtleOf(t, "kinds", `package K {
    datatype D {
        attribute x : Real;
    }
    function F {
        in x : Real;
    }
    part def P {
        ref d : D;
        attribute a : Real;
        calc def C { in x : Real; }
    }
}`)
	for _, tc := range []struct{ subject, metaclass string }{
		{"urn:sysmlv2:element:K__D", "DataType"},
		{"urn:sysmlv2:element:K__F", "Function"},
		{"urn:sysmlv2:element:K__F__x", "ReferenceUsage"},
		{"urn:sysmlv2:element:K__P__d", "ReferenceUsage"},
		{"urn:sysmlv2:element:K__P__a", "AttributeUsage"},
		{"urn:sysmlv2:element:K__P__C", "CalculationDefinition"},
	} {
		wantType(t, g, tc.subject, tc.metaclass)
	}
}

// TestKindlessParameterIsAReferenceUsage is the downstream half of the aligned
// reading: a parameter with no kind keyword is the same ReferenceUsage a
// kindless member is, not a part or attribute usage.
func TestKindlessParameterIsAReferenceUsage(t *testing.T) {
	g := turtleOf(t, "params", `package Q {
    calc def Total {
        in a : Real;
        return : Real = a;
    }
    part def R {
        attribute m : Real;
        ref n : Real;
    }
}`)
	wantType(t, g, "urn:sysmlv2:element:Q__Total__a", "ReferenceUsage")
	wantType(t, g, "urn:sysmlv2:element:Q__R__m", "AttributeUsage")
	wantType(t, g, "urn:sysmlv2:element:Q__R__n", "ReferenceUsage")
}

// TestKeywordMetaclassesRoundTrip checks the notation survives the new
// metaclasses: the written keyword is recorded beside them, so reading the graph
// back gives the declaration as written.
func TestKeywordMetaclassesRoundTrip(t *testing.T) {
	const src = `package K {
    datatype D;
    part def P {
        ref d : D;
    }
}
`
	checkRoundTrip(t, src)
}
