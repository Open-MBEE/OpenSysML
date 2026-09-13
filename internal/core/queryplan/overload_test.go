package queryplan_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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

var importOrders = []string{
	"private import A::*; private import B::*;",
	"private import B::*; private import A::*;",
}

func TestCompileSelectsQueryOverloadByArgumentType(t *testing.T) {
	const src = `
		package A { %[1]s calc def Pick :> Query { in source : Element; in depth : Integer; Descendants(source = source, maxDepth = depth) } }
		package B { %[1]s calc def Pick :> Query { in source : Element; in depth : String; WhereType(source = source, type = depth) } }
		package Fixture {
			%[1]s %[2]s
			calc def ByDepth :> Query { in s : Element; Pick(source = s, depth = 2) }
			calc def ByType :> Query { in s : Element; Pick(source = s, depth = "PartUsage") }
		}
	`
	for _, imports := range importOrders {
		content := fmt.Sprintf(src, overloadImports, imports)
		program, err := compileOverloaded(t, content, "ByDepth")
		if err != nil {
			t.Fatalf("%s: ByDepth: %v", imports, err)
		}
		if got := dependencyOf(t, program, "Fixture::ByDepth"); got != "A::Pick" {
			t.Fatalf("%s: ByDepth calls %s, want A::Pick", imports, got)
		}
		program, err = compileOverloaded(t, content, "ByType")
		if err != nil {
			t.Fatalf("%s: ByType: %v", imports, err)
		}
		if got := dependencyOf(t, program, "Fixture::ByType"); got != "B::Pick" {
			t.Fatalf("%s: ByType calls %s, want B::Pick", imports, got)
		}
	}
}

// Selections memoized before the checker's typing was installed do not outlive it:
// untyped, the arguments leave the overloads tied; typed, they select one.
func TestCompileFollowsTheArgumentTypingInstalledLast(t *testing.T) {
	const src = `
		package A { %[1]s calc def Pick :> Query { in source : Element; in depth : Integer; Descendants(source = source, maxDepth = depth) } }
		package B { %[1]s calc def Pick :> Query { in source : Element; in depth : String; WhereType(source = source, type = depth) } }
		package Fixture {
			%[1]s private import A::*; private import B::*;
			calc def ByType :> Query { in s : Element; Pick(source = s, depth = "PartUsage") }
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
		package A { %[1]s calc def Pick :> Query { in source : Element; OwnedElements(source = source) } }
		package B { %[1]s calc def Pick :> Query { in source : Element; in depth : Integer; Descendants(source = source, maxDepth = depth) } }
		package Fixture {
			%[1]s %[2]s
			calc def Deep :> Query { in s : Element; Pick(source = s, depth = 2) }
		}
	`
	for _, imports := range importOrders {
		program, err := compileOverloaded(t, fmt.Sprintf(src, overloadImports, imports), "Deep")
		if err != nil {
			t.Fatalf("%s: %v", imports, err)
		}
		if got := dependencyOf(t, program, "Fixture::Deep"); got != "B::Pick" {
			t.Fatalf("%s: Deep calls %s, want B::Pick", imports, got)
		}
	}
}

func TestCompileRejectsAmbiguousQueryInvocation(t *testing.T) {
	const src = `
		package A { %[1]s calc def Pick :> Query { in source : Element; OwnedElements(source = source) } }
		package B { %[1]s calc def Pick :> Query { in source : Element; Descendants(source = source, maxDepth = 1) } }
		package Fixture {
			%[1]s %[2]s
			calc def Either :> Query { in s : Element; Pick(source = s) }
		}
	`
	for _, imports := range importOrders {
		_, err := compileOverloaded(t, fmt.Sprintf(src, overloadImports, imports), "Either")
		var planning *queryplan.Error
		if !errors.As(err, &planning) || planning.Kind != queryplan.ErrorAmbiguousInvocation {
			t.Fatalf("%s: error = %v, want %s", imports, err, queryplan.ErrorAmbiguousInvocation)
		}
		if !slices.Equal(planning.Path, []string{"A::Pick", "B::Pick"}) &&
			!slices.Equal(planning.Path, []string{"B::Pick", "A::Pick"}) {
			t.Fatalf("%s: ambiguous between %v, want A::Pick and B::Pick", imports, planning.Path)
		}
	}
}

func TestCompileColumnSelectsTheLibraryColumnAmongOverloads(t *testing.T) {
	const src = `
		package Other { private import ScalarValues::*; calc def Column { in name : String; in expression : Integer; in width : Integer; return : Integer = width; } }
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
