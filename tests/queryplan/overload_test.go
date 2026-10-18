package queryplan_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const overloadImports = `
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
`

// overloadModel is a parsed fixture with its resolver and semantic model, its
// calls selected as the checker selects them once typed is called.
type overloadModel struct {
	index    *symbols.Index
	resolver *resolve.Resolver
	model    *semantics.Model
}

func parseOverloaded(t *testing.T, content string) overloadModel {
	t.Helper()
	index := libs.NewModelIndex()
	p := parser.New(source.New("queries.sysml", []byte(content)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	index.AddDocument("queries.sysml", root)
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	return overloadModel{index: index, resolver: resolver, model: model}
}

func (m overloadModel) typed() overloadModel {
	m.model.SetArgumentTyper(passes.NewArgumentTyper(m.resolver, m.model))
	return m
}

func (m overloadModel) compile(t *testing.T, query string) (*queryplan.Program, error) {
	t.Helper()
	matches := symbols.PreferDeclared(m.index.LookupQualified("Fixture::" + query))
	if len(matches) != 1 {
		t.Fatalf("lookup %s: got %d symbols", query, len(matches))
	}
	return queryplan.Compile(m.index, m.model, m.resolver, matches[0])
}

// compileOverloaded plans Fixture::<query> from content, selecting calls as the
// checker does: by argument type, not by the first declaration a name finds.
func compileOverloaded(t *testing.T, content, query string) (*queryplan.Program, error) {
	t.Helper()
	return parseOverloaded(t, content).typed().compile(t, query)
}

func dependencyOf(t *testing.T, program *queryplan.Program, query string) string {
	t.Helper()
	for _, definition := range program.Definitions() {
		if definition.Name() == query {
			deps := definition.Dependencies()
			if len(deps) != 1 {
				t.Fatalf("%s dependencies = %v, want one", query, deps)
			}
			return deps[0]
		}
	}
	t.Fatalf("%s not compiled", query)
	return ""
}

// generalChains specialize A and B into one chain under Query, in either order, so a
// query specializing the chain's top inherits both Pick through its single general.
var generalChains = []struct{ a, b, top string }{{"DocumentQueries::Query", "A", "B"}, {"B", "DocumentQueries::Query", "A"}}

func TestCompileSelectsQueryOverloadByArgumentType(t *testing.T) {
	const src = `
		calc def A :> %[2]s { %[1]s calc def Pick :> Query { in source : Element; in depth : Integer; Descendants(source = source, maxDepth = depth) } }
		calc def B :> %[3]s { %[1]s calc def Pick :> Query { in source : Element; in depth : String; WhereType(source = source, type = depth) } }
		package Fixture {
			%[1]s
			calc def ByDepth :> %[4]s { in s : Element; Pick(source = s, depth = 2) }
			calc def ByType :> %[4]s { in s : Element; Pick(source = s, depth = "PartUsage") }
		}
	`
	for _, chain := range generalChains {
		content := fmt.Sprintf(src, overloadImports, chain.a, chain.b, chain.top)
		program, err := compileOverloaded(t, content, "ByDepth")
		if err != nil {
			t.Fatalf("%s: ByDepth: %v", chain, err)
		}
		if got := dependencyOf(t, program, "Fixture::ByDepth"); got != "A::Pick" {
			t.Fatalf("%s: ByDepth calls %s, want A::Pick", chain, got)
		}
		program, err = compileOverloaded(t, content, "ByType")
		if err != nil {
			t.Fatalf("%s: ByType: %v", chain, err)
		}
		if got := dependencyOf(t, program, "Fixture::ByType"); got != "B::Pick" {
			t.Fatalf("%s: ByType calls %s, want B::Pick", chain, got)
		}
	}
}

// Selections memoized before the checker's typing was installed do not outlive it:
// untyped, the arguments leave the overloads tied; typed, they select one.
func TestCompileFollowsTheArgumentTypingInstalledLast(t *testing.T) {
	const src = `
		calc def A :> DocumentQueries::Query { %[1]s calc def Pick :> Query { in source : Element; in depth : Integer; Descendants(source = source, maxDepth = depth) } }
		calc def B :> A { %[1]s calc def Pick :> Query { in source : Element; in depth : String; WhereType(source = source, type = depth) } }
		package Fixture {
			%[1]s
			calc def ByType :> B { in s : Element; Pick(source = s, depth = "PartUsage") }
		}
	`
	m := parseOverloaded(t, fmt.Sprintf(src, overloadImports))
	var planning *queryplan.Error
	if _, err := m.compile(t, "ByType"); !errors.As(err, &planning) || planning.Kind != queryplan.ErrorAmbiguousInvocation {
		t.Fatalf("untyped: error = %v, want %s between A::Pick and B::Pick", err, queryplan.ErrorAmbiguousInvocation)
	}
	program, err := m.typed().compile(t, "ByType")
	if err != nil {
		t.Fatalf("typed: %v", err)
	}
	if got := dependencyOf(t, program, "Fixture::ByType"); got != "B::Pick" {
		t.Fatalf("typed: ByType calls %s, want B::Pick", got)
	}
}

func TestCompileSelectsQueryOverloadByArgumentNames(t *testing.T) {
	const src = `
		calc def A :> %[2]s { %[1]s calc def Pick :> Query { in source : Element; OwnedElements(source = source) } }
		calc def B :> %[3]s { %[1]s calc def Pick :> Query { in source : Element; in depth : Integer; Descendants(source = source, maxDepth = depth) } }
		package Fixture {
			%[1]s
			calc def Deep :> %[4]s { in s : Element; Pick(source = s, depth = 2) }
		}
	`
	for _, chain := range generalChains {
		program, err := compileOverloaded(t, fmt.Sprintf(src, overloadImports, chain.a, chain.b, chain.top), "Deep")
		if err != nil {
			t.Fatalf("%s: %v", chain, err)
		}
		if got := dependencyOf(t, program, "Fixture::Deep"); got != "B::Pick" {
			t.Fatalf("%s: Deep calls %s, want B::Pick", chain, got)
		}
	}
}

func TestCompileRejectsAmbiguousQueryInvocation(t *testing.T) {
	const src = `
		calc def A :> %[2]s { %[1]s calc def Pick :> Query { in source : Element; OwnedElements(source = source) } }
		calc def B :> %[3]s { %[1]s calc def Pick :> Query { in source : Element; Descendants(source = source, maxDepth = 1) } }
		package Fixture {
			%[1]s
			calc def Either :> %[4]s { in s : Element; Pick(source = s) }
		}
	`
	for _, chain := range generalChains {
		_, err := compileOverloaded(t, fmt.Sprintf(src, overloadImports, chain.a, chain.b, chain.top), "Either")
		var planning *queryplan.Error
		if !errors.As(err, &planning) || planning.Kind != queryplan.ErrorAmbiguousInvocation {
			t.Fatalf("%s: error = %v, want %s", chain, err, queryplan.ErrorAmbiguousInvocation)
		}
		if !slices.Equal(planning.Path, []string{"A::Pick", "B::Pick"}) &&
			!slices.Equal(planning.Path, []string{"B::Pick", "A::Pick"}) {
			t.Fatalf("%s: ambiguous between %v, want A::Pick and B::Pick", chain, planning.Path)
		}
	}
}

func TestCompileColumnSelectsTheLibraryColumnAmongOverloads(t *testing.T) {
	const src = `
		package Other { private import ScalarValues::*; calc def Column { in name : String; in expression : Integer; in width : Integer[1]; return : Integer = width; } }
		package Fixture {
			private import Other::*;
			%s
			calc def Wide :> Query {
				in s : Element;
				Project(source = s, columns = Column("twice", 2))
			}
		}
	`
	program, err := compileOverloaded(t, fmt.Sprintf(src, overloadImports), "Wide")
	if err != nil {
		t.Fatalf("Wide: %v", err)
	}
	expression := program.Definitions()[0].Expression()
	if expression.Operation() != queryplan.OperationProject {
		t.Fatalf("Wide operation = %s, want Project", expression.Operation())
	}
	var columns *queryplan.Expression
	for _, argument := range expression.Arguments() {
		if argument.Name == "columns" {
			columns = &argument.Value
		}
	}
	if columns == nil || len(columns.Arguments()) != 1 ||
		columns.Arguments()[0].Value.Operation() != queryplan.OperationColumn ||
		columns.Arguments()[0].Value.Target() != "twice" {
		t.Fatalf("Wide columns = %+v, want one Column twice", columns)
	}
}
