package resolve

import "testing"

// A parameter named with a keyword — quoted as KerML requires, or bare as the
// parser tolerates — declares that name, so references to it resolve.
func TestResolveKeywordNamedParameter(t *testing.T) {
	for _, src := range []string{
		"part def Anything; calc def Cast { in 'type' : Anything; return : Anything = 'type'; }",
		"part def Anything; calc def Cast { in type : Anything; return : Anything = 'type'; }",
		"part def Anything; action def Classify { in ref state : Anything; out result : Anything = 'state'; }",
		"part def Anything; action def Classify { in ref 'type' : Anything; out result : Anything = 'type'; }",
	} {
		r := resolveDoc(t, "<t>", src)
		if len(r.Diagnostics) != 0 {
			t.Errorf("%s: expected no diagnostics, got %v", src, r.Diagnostics)
		}
	}
}

// The keyword name is the parameter's own, so an unrelated name still fails.
func TestResolveKeywordNamedParameterUnresolved(t *testing.T) {
	r := resolveDoc(t, "<t>", "part def Anything; calc def Cast { in 'type' : Anything; return : Anything = 'kind'; }")
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved-reference diagnostic")
	}
}
