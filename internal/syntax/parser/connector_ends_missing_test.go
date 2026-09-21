package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A binary connector clause whose first end is missing reports the gap at the
// `to`/`then` keyword itself, not the keyword misread as an end's name, and
// still parses the second end so no spurious declaration-tail error follows.
func TestConnectorFirstEndMissingReportsAtKeyword(t *testing.T) {
	src := "part def C { connection c : I connect  to ; }"
	_, diags := parseOneMemberWithDiags(t, src)

	if len(diags) != 2 {
		t.Fatalf("diagnostics = %v, want 2", diags)
	}
	if diags[0].Message != "expected a connector end before 'to'" {
		t.Errorf("diagnostic 0 = %q, want the missing end reported at 'to'", diags[0].Message)
	}
	if got, want := diags[0].Span.Offset, strings.Index(src, "to"); got != want {
		t.Errorf("diagnostic 0 offset = %d, want %d (the 'to' token)", got, want)
	}
	if diags[1].Message != "expected a name" {
		t.Errorf("diagnostic 1 = %q, want the missing second end reported at ';'", diags[1].Message)
	}
	if got, want := diags[1].Span.Offset, strings.Index(src, ";"); got != want {
		t.Errorf("diagnostic 1 offset = %d, want %d (the ';' token)", got, want)
	}
}

// `connect to a;` states one missing end and nothing else: recovery reads `a`
// as the second end rather than compounding the error.
func TestConnectorFirstEndMissingYieldsOneDiagnostic(t *testing.T) {
	_, diags := parseOneMemberWithDiags(t, "part def C { connection c connect to a; }")

	if len(diags) != 1 {
		t.Fatalf("diagnostics = %v, want exactly 1", diags)
	}
	if diags[0].Message != "expected a connector end before 'to'" {
		t.Errorf("diagnostic = %q, want the missing end reported at 'to'", diags[0].Message)
	}
}

// A connector end may genuinely be named `to`: `connect to to b` names its
// first end `to` and parses without a diagnostic.
func TestConnectorEndNamedToParses(t *testing.T) {
	member, diags := parseOneMemberWithDiags(t, "connection c connect to to b;")
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %v, want none", diags)
	}
	u, ok := member.(*ast.Usage)
	if !ok {
		t.Fatalf("expected *ast.Usage, got %T", member)
	}
	if len(u.ConnectorEnds) != 2 {
		t.Fatalf("expected 2 ends, got %d", len(u.ConnectorEnds))
	}
}

// A first end whose name starts with the delimiter keyword and continues past
// it — a chain, a qualified name or a references clause — is an end, not the
// delimiter: the try-parse tells it apart from a missing end.
func TestConnectorFirstEndNamedKeywordParses(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"chained end named to", "connection c connect to.port to target;"},
		{"qualified end named to", "connection c connect to::p to b;"},
		{"end named to with references clause", "connection c connect to references x to b;"},
		{"from end named to chained", "connector c from to.p to q;"},
		{"succession end named then", "action a { succession s first then.a then b; }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := parseOneMemberWithDiags(t, tc.src)
			if len(diags) != 0 {
				t.Fatalf("%q: diagnostics = %v, want none", tc.src, diags)
			}
		})
	}
}
