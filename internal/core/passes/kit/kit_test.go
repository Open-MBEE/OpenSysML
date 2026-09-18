package kit

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

type testUnion struct {
	docs  []string
	about int
}

func (u *testUnion) Regather(_ *Context, _ *Gathers, doc string, _ map[string]bool) {
	u.docs = append(u.docs, doc)
}

func (u *testUnion) RegatherAbout(_ *Context, _ *Gathers, _ map[string]bool) {
	u.about++
}

func testContext() *Context {
	idx := symbols.NewIndex()
	root := parser.New(source.New("a.sysml", []byte("package P {}"))).ParseFile()
	idx.AddDocument("a.sysml", root)
	return NewContext("a.sysml", source.KindSysML, idx, nil, Options{}, semantics.NewModel)
}

func TestGathersUnionLifecycle(t *testing.T) {
	ctx := testContext()
	g := NewGathers()
	u := &testUnion{}
	if got := g.UnionOf(ctx, "test", func() Union { return u }); got != u {
		t.Fatalf("UnionOf returned %T, want existing union", got)
	}
	if got := g.Union("test"); got != u {
		t.Fatalf("Union returned %T, want test union", got)
	}
	if got := g.UnionOf(ctx, "test", func() Union { t.Fatal("rebuilt union"); return nil }); got != u {
		t.Fatal("UnionOf did not return the existing union")
	}
	if !g.Has("a.sysml") || !g.Gathered("a.sysml") {
		t.Fatal("workspace document was not gathered")
	}
	if len(u.docs) != 1 || u.docs[0] != "a.sysml" || u.about != 1 {
		t.Fatalf("initial gather = %#v, about=%d", u.docs, u.about)
	}
	if got := g.Regather(ctx, map[string]bool{"a.sysml": true}); len(got) != 0 {
		t.Fatalf("Regather changed keys = %v, want none", got)
	}
	ctx.Index.RemoveDocument("a.sysml")
	g.Regather(ctx, map[string]bool{"a.sysml": true, AboutGather: true})
	g.Reset()
	if g.Union("test") != nil || g.Gathered("a.sysml") {
		t.Fatal("Reset retained gather state")
	}
}

func TestGathersGatherAndHelpers(t *testing.T) {
	ctx := testContext()
	g := NewGathers()
	if got := NewGathers().Regather(ctx, map[string]bool{}); got != nil {
		t.Fatalf("empty Regather = %v, want nil", got)
	}
	var seen bool
	g.Gather(ctx, "a.sysml", func(root *symbols.Scope) { seen = root != nil })
	if !seen {
		t.Fatal("Gather did not visit the document root")
	}
	if got := SortedKeys(map[string]bool{"b": true, "a": true}); len(got) != 2 || got[0] != "a" {
		t.Fatalf("SortedKeys = %v", got)
	}
	s := CountSet[string]{"old": 1}
	changed := map[string]bool{}
	Move(s, map[string]bool{"old": true}, map[string]bool{"new": true}, func(k string) string { return k }, changed)
	if s.Has("old") || !s.Has("new") || !changed["old"] || !changed["new"] {
		t.Fatalf("Move result set=%v changed=%v", s, changed)
	}
	Move(s, map[string]bool{"new": true}, nil, func(k string) string { return k }, nil)
	s["held"] = 2
	Move(s, map[string]bool{"held": true}, nil, func(k string) string { return k }, nil)
}

func TestContextStateAndModel(t *testing.T) {
	ctx := testContext()
	if ctx.Resolver() == nil || ctx.Model() == nil {
		t.Fatal("Context did not initialize resolver and model")
	}
	ctx.SetFailures([]source.Span{{Offset: 2, Len: 3}})
	if !ctx.DownstreamSpan(source.Span{Offset: 2, Len: 3}) {
		t.Fatal("DownstreamSpan did not find a contained failure")
	}
	var nilCtx *Context
	if nilCtx.DownstreamSpan(source.Span{}) || nilCtx.DownstreamOfFailure(nil) {
		t.Fatal("nil context unexpectedly reported a failure")
	}
	if (&Context{}).DownstreamOfFailure(nil) {
		t.Fatal("nil reference unexpectedly reported a failure")
	}
	if ctx.DownstreamOfFailure(&ast.QualifiedName{}) {
		t.Fatal("unrelated reference unexpectedly reported a failure")
	}
	if (&Context{}).DownstreamSpan(source.Span{}) {
		t.Fatal("nil context state unexpectedly reported a failure")
	}
	other := NewContext("other.sysml", source.KindSysML, symbols.NewIndex(), []diag.Diagnostic{{}}, Options{}, semantics.NewModel)
	other.Share(ctx.Resolver(), ctx.Model(), ctx.Gathers())
	if other.Resolver() != ctx.Resolver() || other.Model() != ctx.Model() || other.Gathers() != ctx.Gathers() {
		t.Fatal("Share did not reuse context state")
	}
}

func TestContextRequiresModelConstructor(t *testing.T) {
	ctx := NewContext("test.sysml", source.KindSysML, symbols.NewIndex(), nil, Options{}, nil)
	defer func() {
		if recover() != "kit: Context built without a model constructor" {
			t.Fatal("Model did not panic with the required message")
		}
	}()
	ctx.Model()
}

func TestWalkerHelpers(t *testing.T) {
	scope := symbols.NewScope(nil, nil)
	sym := &symbols.Symbol{Name: "x", OwnerScope: scope}
	if ReferenceScope(nil) != nil {
		t.Fatal("nil symbol unexpectedly had a reference scope")
	}
	if DeclarationScope(sym) != scope || ReferenceScope(sym) != scope {
		t.Fatal("scope helpers returned the wrong scope")
	}
	sym.Scope = scope
	if ReferenceScope(sym) != scope {
		t.Fatal("ReferenceScope ignored the symbol scope")
	}
	if MultiplicityOf(nil) != nil || UnnamedMetadataBody(nil, nil) != nil {
		t.Fatal("nil helper inputs were not handled")
	}
	if DeclarationScope(nil) != nil {
		t.Fatal("nil symbol unexpectedly had a declaration scope")
	}
	if UnnamedMetadataBody(scope, &ast.PrefixMetadata{}) != nil {
		t.Fatal("empty metadata unexpectedly had a body")
	}
	ForEachBodySymbol(nil, func(*symbols.Symbol) { t.Fatal("nil body was visited") })
	ForEachBodySymbol(scope, func(*symbols.Symbol) { t.Fatal("empty body was visited") })
	if !IsReference(&ast.QualifiedName{}) || IsReference(nil) {
		t.Fatal("reference helper returned the wrong result")
	}
	if ChainSteps(nil) != nil {
		t.Fatal("nil chain did not return nil")
	}
	if ChainSteps(&ast.FeatureReference{}) != nil {
		t.Fatal("unnamed feature reference unexpectedly had chain steps")
	}
	(&Walker{}).Walk(nil, func(*symbols.Symbol) { t.Fatal("nil scope was visited") })
	if MemberSymbols(nil, nil) != nil {
		t.Fatal("nil member scope unexpectedly had symbols")
	}
	if MultiplicityOf(&symbols.Symbol{Decl: &ast.Definition{}}) != nil {
		t.Fatal("empty definition unexpectedly had multiplicity")
	}
	if got := PassLevel(99).String(); got != "unknown" {
		t.Fatalf("unknown level string = %q", got)
	}
	if got := LevelSyntax.String(); got != "syntax" {
		t.Fatalf("syntax level string = %q", got)
	}
}
