package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestSyntaxPassSurfacesParseDiagnostics(t *testing.T) {
	parseDiags := []diag.Diagnostic{
		{Severity: diag.SeverityError, Span: source.Span{Offset: 3, Len: 2}, Message: "expected '}'", Code: "syntax", Source: "syntax"},
	}
	ctx := NewContext("t", nil, parseDiags)
	p := SyntaxPass{}
	if p.Level() != LevelSyntax {
		t.Fatalf("Level() = %v, want LevelSyntax", p.Level())
	}
	got := p.Run(ctx, "t", nil)
	if len(got) != 1 || got[0].Message != "expected '}'" || got[0].Source != "syntax" {
		t.Fatalf("got %+v, want the single parse diagnostic", got)
	}
}

func TestSyntaxPassEmptyWhenNoParseDiagnostics(t *testing.T) {
	got := SyntaxPass{}.Run(NewContext("t", nil, nil), "t", nil)
	if len(got) != 0 {
		t.Fatalf("got %d diagnostics, want 0", len(got))
	}
}
