package runtime

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// qualifiedImportModel declares one package of values and the façades that
// re-export them through every import form, so a qualified name written from
// outside reaches a value only the way the checker says it does.
const qualifiedImportModel = `
	package A {
		private import ScalarValues::*;
		attribute x : Integer = 1;
		attribute <s> shortNamed : Integer = 2;
		package Deep { attribute d : Integer = 3; }
		part def Base { attribute inherited : Integer = 4; }
		part def Derived :> Base;
		enum def Color { red; green; }
		variation attribute cut {
			variant attribute cutIdeal;
			variant attribute cutShallow;
		}
	}
	package Wild { public import A::*; }
	package Single { public import A::x; }
	package Facade { public import Wild::*; }
	package Recursive { public import A::**; }
	package Aliased { alias ax for A::x; }
	package Twice {
		private import ScalarValues::*;
		attribute t : Integer = 1;
		attribute t : Integer = 2;
	}
	package Priv {
		private import A::*;
		attribute own = x;
	}
	package test {
		private import ScalarValues::*;
		calc def Two {
			in n : Integer;
			out a = n + 1;
			out b = n * 2;
		}
		calc c : Two { in n = 5; }
	}
	package probes {
		attribute viaWild = Wild::x;
		attribute viaFacade = Facade::x;
		attribute viaRecursive = Recursive::d;
		attribute viaCalc = test::c::a;
		attribute viaPrivate = Priv::x;
		attribute viaSingleOther = Single::shortNamed;
		attribute missing = Wild::nope;
		attribute twice = Twice::t;
	}
`

// qualifiedImportRuntime builds the model over the standard library, runs the
// checker over it, and returns the runtime with the messages the checker
// reported inside package probes (Twice's duplicate members are reported too).
func qualifiedImportRuntime(t *testing.T) (*Context, *symbols.Scope, []string) {
	t.Helper()
	file := parseAndBuild(t, qualifiedImportModel)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", file)
	ctx.model.resolver.ResolveDocument("<test>", file)
	root := idx.DocumentRoot("<test>")
	probes, ok := root.LookupLocal("probes")
	if !ok || probes.Decl == nil {
		t.Fatal("package probes not indexed")
	}
	within := probes.Decl.Span()
	var checked []string
	for _, d := range ctx.model.resolver.Diagnostics {
		if d.Span.Offset >= within.Offset && d.Span.End() <= within.End() {
			checked = append(checked, d.Message)
		}
	}
	return ctx, root, checked
}

// probeValue evaluates the value of the named attribute of package probes in
// the scope it was written in, as its declaration is read.
func probeValue(t *testing.T, ctx *Context, root *symbols.Scope, name string) (Value, error) {
	t.Helper()
	pkg, ok := root.LookupLocal("probes")
	if !ok || pkg.Scope == nil {
		t.Fatal("package probes not indexed")
	}
	sym, ok := pkg.Scope.LookupLocal(name)
	if !ok {
		t.Fatalf("attribute %s not found", name)
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		t.Fatalf("attribute %s binds no value", name)
	}
	return ctx.EvalWithScope(usage.Value, sym.OwnerScope)
}

// qualifiedImportRejections are the probes the checker rejects, each with the
// error the evaluator classifies it as and the message the checker reports.
var qualifiedImportRejections = map[string]struct {
	err error
	msg string
}{
	"viaPrivate":     {ErrUnresolvedReference, "unresolved reference: Priv::x"},
	"viaSingleOther": {ErrUnresolvedReference, "unresolved reference: Single::shortNamed"},
	"missing":        {ErrUnresolvedReference, "unresolved reference: Wild::nope"},
	"twice":          {ErrAmbiguousReference, "ambiguous reference: Twice::t (2 candidates)"},
}

// TestQualifiedNameThroughImportEvaluates evaluates qualified names whose
// segments cross an import, as the checker resolves them: a public import, of
// every member or of one, a façade of a façade, a recursive import, an alias,
// a short name, and the owned and inherited members those were already reaching.
func TestQualifiedNameThroughImportEvaluates(t *testing.T) {
	ctx, root, checked := qualifiedImportRuntime(t)
	if len(checked) != len(qualifiedImportRejections) {
		t.Errorf("checker reported %q, want only the rejected probes", checked)
	}
	for name, want := range map[string]string{
		"viaWild": "1", "viaFacade": "1", "viaRecursive": "3", "viaCalc": "6",
	} {
		val, err := probeValue(t, ctx, root, name)
		if err != nil {
			t.Errorf("probes::%s: %v", name, err)
		} else if got := FormatValue(val); got != want {
			t.Errorf("probes::%s = %s, want %s", name, got, want)
		}
	}

	cases := []struct {
		src  string
		want string
	}{
		{"Wild::x", "1"},
		{"Single::x", "1"},
		{"Facade::x", "1"},
		{"Facade::Deep::d", "3"},
		{"Recursive::x", "1"},
		{"Recursive::d", "3"},
		{"Aliased::ax", "1"},
		{"Wild::s", "2"},
		{"Wild::shortNamed", "2"},
		{"Wild::Color::red", "Color::red"},
		{"Wild::Derived::inherited", "4"},
		{"A::Derived::inherited", "4"},
		{"Priv::own", "1"},
		{"test::c::a", "6"},
	}
	for _, tc := range cases {
		val, err := evalIn(t, ctx, root, tc.src)
		if err != nil {
			t.Errorf("eval %s: %v", tc.src, err)
			continue
		}
		if got := FormatValue(val); got != tc.want {
			t.Errorf("eval %s = %s, want %s", tc.src, got, tc.want)
		}
	}
}

// TestQualifiedNameThroughImportRejectedAsChecked rejects at evaluation exactly
// the qualified names the checker rejects, with the checker's message: a private
// import, a single-member import naming another member, a missing name, and a
// name several members answer to.
func TestQualifiedNameThroughImportRejectedAsChecked(t *testing.T) {
	ctx, root, checked := qualifiedImportRuntime(t)
	for name, want := range qualifiedImportRejections {
		t.Run(name, func(t *testing.T) {
			if !slices.Contains(checked, want.msg) {
				t.Errorf("checker reported %q, want %q among them", checked, want.msg)
			}
			_, err := probeValue(t, ctx, root, name)
			if !errors.Is(err, want.err) {
				t.Fatalf("probes::%s: err = %v, want %v", name, err, want.err)
			}
			if err.Error() != want.msg {
				t.Errorf("probes::%s: err = %q, want the checker's %q", name, err, want.msg)
			}
		})
	}
	// A name typed at the prompt is rejected the same way, at whichever depth
	// the import stops answering.
	for _, src := range []string{"Priv::x", "Facade::Deep::nope"} {
		_, err := evalIn(t, ctx, root, src)
		if !errors.Is(err, ErrUnresolvedReference) || err.Error() != "unresolved reference: "+src {
			t.Errorf("eval %s: err = %v, want unresolved reference: %s", src, err, src)
		}
	}
	_, err := evalIn(t, ctx, root, "Twice::t + 0")
	if !errors.Is(err, ErrAmbiguousReference) || err.Error() != "ambiguous reference: Twice::t (2 candidates)" {
		t.Errorf("eval Twice::t + 0: err = %v, want ambiguous reference: Twice::t (2 candidates)", err)
	}
}

// TestGlobalQualifiedNameThroughImport reads and rejects `$::`-rooted names as the
// checker does; the name is built since the expression parser does not read `$::`.
func TestGlobalQualifiedNameThroughImport(t *testing.T) {
	ctx, root, _ := qualifiedImportRuntime(t)
	globalName := func(parts ...string) *ast.FeatureReference {
		qn := &ast.QualifiedName{Global: true}
		for _, p := range parts {
			qn.Parts = append(qn.Parts, ast.NameSegment{Text: p})
		}
		return &ast.FeatureReference{Name: qn}
	}
	val, err := ctx.EvalWithScope(globalName("Wild", "x"), root)
	if err != nil || FormatValue(val) != "1" {
		t.Errorf("$::Wild::x = (%v, %v), want 1", val, err)
	}
	_, err = ctx.EvalWithScope(globalName("Wild", "nope"), root)
	if !errors.Is(err, ErrUnresolvedReference) || err.Error() != "unresolved reference: $::Wild::nope" {
		t.Errorf("$::Wild::nope: err = %v, want unresolved reference: $::Wild::nope", err)
	}
}

// TestQualifiedNameThroughImportKeepsTypedErrors keeps the typed errors a
// variation and an enumeration report for a name that is no variant or literal,
// whether reached through an import or directly, and a calc usage's outputs
// answered from its evaluation.
func TestQualifiedNameThroughImportKeepsTypedErrors(t *testing.T) {
	ctx, root, _ := qualifiedImportRuntime(t)
	cases := []struct {
		src   string
		want  error
		names []string
	}{
		{"Wild::Color::purple", ErrNotALiteral, []string{"purple", "red", "green"}},
		{"A::Color::purple", ErrNotALiteral, []string{"purple", "red", "green"}},
		{"Wild::cut::nope", ErrNotAVariant, []string{"nope", "cutIdeal", "cutShallow"}},
		{"test::c::nope", ErrUnknownOutput, []string{"nope", "a, b"}},
	}
	for _, tc := range cases {
		_, err := evalIn(t, ctx, root, tc.src)
		if !errors.Is(err, tc.want) {
			t.Errorf("eval %s: err = %v, want %v", tc.src, err, tc.want)
			continue
		}
		for _, name := range tc.names {
			if !strings.Contains(err.Error(), name) {
				t.Errorf("eval %s: error %q does not name %s", tc.src, err, name)
			}
		}
	}
}

// TestLibraryQuantityThroughFacadeResolves reaches a library quantity through
// the ISQ and SI façades, which re-export the ISQ part packages: the name
// resolves to the declaration its home package answers with, so the evaluator
// answers with that declaration — a valueless `[*]` quantity, read as empty —
// rather than a missing member.
func TestLibraryQuantityThroughFacadeResolves(t *testing.T) {
	ctx, root, _ := qualifiedImportRuntime(t)
	for _, src := range []string{"ISQSpaceTime::speed", "ISQ::speed", "SI::speed"} {
		val, err := evalIn(t, ctx, root, src)
		if err != nil || elementCount(&val) != 0 {
			t.Errorf("%s = (%s, %v), want the empty sequence the valueless [*] quantity holds", src, FormatValue(val), err)
		}
	}
	val, err := evalIn(t, ctx, root, "TrigFunctions::pi")
	if err != nil || !strings.HasPrefix(FormatValue(val), "3.14159") {
		t.Errorf("TrigFunctions::pi = (%v, %v), want the library constant", val, err)
	}
}

// TestQualifiedNameEvaluatedInSeveralScopes evaluates one parsed A::x in two
// scopes that each hold their own A::x, in both orders: each evaluation answers
// with its scope's value rather than the first scope's.
func TestQualifiedNameEvaluatedInSeveralScopes(t *testing.T) {
	file := parseAndBuild(t, `
		package One { package A { attribute x = 1; } }
		package Two { package A { attribute x = 2; } }
	`)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", file)
	ctx.model.resolver.ResolveDocument("<test>", file)
	root := idx.DocumentRoot("<test>")
	scopes := map[string]*symbols.Scope{}
	for _, name := range []string{"One", "Two"} {
		pkg, ok := root.LookupLocal(name)
		if !ok || pkg.Scope == nil {
			t.Fatalf("package %s not indexed", name)
		}
		scopes[name] = pkg.Scope
	}
	p := parser.New(source.New("<expr>", []byte("A::x")))
	expr := p.ParseExpression()
	if expr == nil || len(p.Diagnostics) > 0 {
		t.Fatalf("parse A::x: %v", p.Diagnostics)
	}
	for _, order := range [][]string{{"One", "Two"}, {"Two", "One"}} {
		for _, name := range order {
			want := map[string]string{"One": "1", "Two": "2"}[name]
			val, err := ctx.EvalWithScope(expr, scopes[name])
			if err != nil || FormatValue(val) != want {
				t.Errorf("A::x in %s (after %v) = (%v, %v), want %s", name, order, val, err, want)
			}
		}
	}
	if n := len(ctx.model.resolver.Diagnostics); n != 0 {
		t.Errorf("evaluation reported %d resolver diagnostics: %v", n, ctx.model.resolver.Diagnostics)
	}
}
