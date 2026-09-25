package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// Prefix metadata ahead of `subject` reaches the editor as one syntax error at
// the `#` carrying its quick fix, and the recovered member is still indexed.
func TestMisplacedMemberPrefixMetadataIsASyntaxError(t *testing.T) {
	ws := NewWorkspace()
	uri := "file:///misplaced.sysml"
	src := "package P {\n\tmetadata def M;\n\tpart def T;\n\trequirement def R {\n\t\t#M subject s : T;\n\t}\n}\n"
	ws.Open(uri, []byte(src), 1)
	defer ws.Close(uri)

	var errs []diag.Diagnostic
	for _, d := range ws.Diagnostics(uri) {
		if d.Severity == diag.SeverityError {
			errs = append(errs, d)
		}
	}
	if len(errs) != 1 {
		t.Fatalf("errors = %+v, want the one placement error", errs)
	}
	d := errs[0]
	if d.Code != "syntax" || d.Message != "prefix metadata follows 'subject': write `subject #M s`" {
		t.Errorf("diagnostic = %+v", d)
	}
	if got := src[d.Span.Offset:d.Span.End()]; got != "#M" {
		t.Errorf("span covers %q, want the prefix run", got)
	}
	if len(d.Fixes) != 1 || len(d.Fixes[0].Edits) == 0 {
		t.Fatalf("fixes = %+v, want the one that moves the run", d.Fixes)
	}

	syms := ws.LookupQualified("P::R::s")
	if len(syms) != 1 {
		t.Fatalf("P::R::s matched %d symbols, want the recovered subject", len(syms))
	}
	if _, ok := syms[0].Decl.(*ast.SubjectMember); !ok {
		t.Errorf("P::R::s declared by %T, want *ast.SubjectMember", syms[0].Decl)
	}
}
