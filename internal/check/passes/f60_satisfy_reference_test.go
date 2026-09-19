package passes

import (
	"strings"
	"testing"
)

// F60: a `satisfy requirement` usage is a requirement usage (SysML v2 §8.3.19),
// so a connector end may reference one. Corpus: "Requirements Examples/
// RequirementDerivationExample.sysml:32" (`end r1 ::> req1;`), which the pilot
// accepts.
func TestF60ConnectorEndReferencesSatisfyUsage(t *testing.T) {
	src := "requirement def Req; part def System; connection def D { end e1 : Req; } " +
		"part system : System; part ctx { satisfy requirement req : Req by system; " +
		"connection : D { end e1 ::> req; } }"
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Errorf("expected no type diagnostics, got %v", diags)
	}
}

// F60 negative: `satisfy` still requires a requirement usage as its reference
// target, so a satisfy of a part usage stays rejected.
func TestF60SatisfyOfNonRequirementRejected(t *testing.T) {
	src := "part def P; part p : P; part ctx { satisfy p; }"
	diags := typeDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "satisfy target must be a requirement usage") {
		t.Errorf("got %q", diags[0].Message)
	}
}
