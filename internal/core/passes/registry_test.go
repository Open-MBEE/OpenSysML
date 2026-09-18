package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

type stubPass struct {
	level PassLevel
	diags []diag.Diagnostic
}

func (s stubPass) Level() PassLevel { return s.level }

func (s stubPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	return s.diags
}

func TestRegistryRunsPassesInLevelOrder(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubPass{level: LevelNameResolution, diags: []diag.Diagnostic{{Severity: diag.SeverityWarning, Source: "b"}}})
	reg.Register(stubPass{level: LevelSyntax, diags: []diag.Diagnostic{{Severity: diag.SeverityWarning, Source: "a"}}})
	got := reg.Run(NewContext("t", nil, nil), "t", nil)
	if len(got) != 2 {
		t.Fatalf("got %d diagnostics, want 2", len(got))
	}
	if got[0].Source != "a" || got[1].Source != "b" {
		t.Fatalf("passes ran out of level order: %q then %q", got[0].Source, got[1].Source)
	}
}

func TestRegistrySkipsHigherLevelAfterError(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubPass{level: LevelSyntax, diags: []diag.Diagnostic{{Severity: diag.SeverityError, Source: "syntax"}}})
	reg.Register(stubPass{level: LevelType, diags: []diag.Diagnostic{{Source: "type"}}})
	got := reg.Run(NewContext("t", nil, nil), "t", nil)
	if len(got) != 1 {
		t.Fatalf("got %d diagnostics, want 1 (type pass must be skipped)", len(got))
	}
	if got[0].Source != "syntax" {
		t.Fatalf("got source %q, want syntax", got[0].Source)
	}
}

type elementScopedStub struct{ stubPass }

func (elementScopedStub) ElementScoped() {}

func TestRegistryRunsElementScopedPassAfterError(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubPass{level: LevelSyntax, diags: []diag.Diagnostic{{Severity: diag.SeverityError, Source: "syntax"}}})
	reg.Register(stubPass{level: LevelType, diags: []diag.Diagnostic{{Source: "type"}}})
	reg.Register(elementScopedStub{stubPass{level: LevelType, diags: []diag.Diagnostic{{Source: "element-scoped"}}}})
	got := reg.Run(NewContext("t", nil, nil), "t", nil)
	if len(got) != 2 || got[1].Source != "element-scoped" {
		t.Fatalf("want syntax plus element-scoped only, got %v", got)
	}
}

// subjectPass is element-scoped and reports about each subject the tiers below
// left judgeable, which is what per-element gating has to decide.
type subjectPass struct {
	subjects []ast.Node
}

func (subjectPass) Level() PassLevel { return LevelType }

func (subjectPass) ElementScoped() {}

func (p subjectPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, s := range p.subjects {
		if ctx.DownstreamOfFailure(s) {
			continue
		}
		out = append(out, diag.Diagnostic{Span: s.Span(), Source: "subject"})
	}
	return out
}

func TestRegistryGatesElementScopedPassPerElement(t *testing.T) {
	bad := &ast.ErrorNode{NodeBase: ast.NodeBase{NodeSpan: source.Span{Offset: 10, Len: 5}}}
	good := &ast.ErrorNode{NodeBase: ast.NodeBase{NodeSpan: source.Span{Offset: 40, Len: 5}}}
	reg := NewRegistry()
	reg.Register(stubPass{level: LevelNameResolution, diags: []diag.Diagnostic{{
		Severity: diag.SeverityError, Span: source.Span{Offset: 11, Len: 3}, Source: "nameres",
	}}})
	reg.Register(subjectPass{subjects: []ast.Node{bad, good}})
	got := reg.Run(NewContext("t", nil, nil), "t", nil)
	if len(got) != 2 {
		t.Fatalf("got %v, want the nameres error plus one subject diagnostic", got)
	}
	if got[1].Span.Offset != good.Span().Offset {
		t.Fatalf("reported subject at offset %d, want the one that resolved (%d)",
			got[1].Span.Offset, good.Span().Offset)
	}
}

func TestRegistryDoesNotGateOnSameLevelFailure(t *testing.T) {
	subject := &ast.ErrorNode{NodeBase: ast.NodeBase{NodeSpan: source.Span{Offset: 10, Len: 5}}}
	reg := NewRegistry()
	reg.Register(stubPass{level: LevelType, diags: []diag.Diagnostic{{
		Severity: diag.SeverityError, Span: source.Span{Offset: 11, Len: 3}, Source: "type",
	}}})
	reg.Register(subjectPass{subjects: []ast.Node{subject}})
	if got := reg.Run(NewContext("t", nil, nil), "t", nil); len(got) != 2 {
		t.Fatalf("got %v, want both: a same-level failure gates nothing", got)
	}
}

func TestRegistryGatesOnParseFailure(t *testing.T) {
	subject := &ast.ErrorNode{NodeBase: ast.NodeBase{NodeSpan: source.Span{Offset: 10, Len: 5}}}
	parse := []diag.Diagnostic{{Severity: diag.SeverityError, Span: source.Span{Offset: 10, Len: 5}, Source: "syntax"}}
	reg := NewRegistry()
	reg.Register(subjectPass{subjects: []ast.Node{subject}})
	if got := reg.Run(NewContext("t", nil, parse), "t", nil); len(got) != 0 {
		t.Fatalf("got %v, want nothing: the subject did not parse", got)
	}
}

func TestRegistrySameLevelNeverSkips(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubPass{level: LevelNameResolution, diags: []diag.Diagnostic{{Severity: diag.SeverityError, Source: "a"}}})
	reg.Register(stubPass{level: LevelNameResolution, diags: []diag.Diagnostic{{Source: "b"}}})
	got := reg.Run(NewContext("t", nil, nil), "t", nil)
	if len(got) != 2 {
		t.Fatalf("got %d diagnostics, want 2 (same-level passes never skip)", len(got))
	}
}
