package lower

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// metadataStateGraph lowers state usage m of a model importing StateMachines
// through the name-resolution tier, the way the runtime does; prelude declares
// package-level members the body refers to.
func metadataStateGraph(t *testing.T, prelude, body string) (*StateGraph, error) {
	t.Helper()
	src := "package M {\n private import StateMachines::*;\n" + prelude + "\n state m {\n" + body + "\n }\n}\n"
	p := parser.New(source.New("m.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("m.sysml", root)
	idx.ExpandWildcardImports()
	pkg, ok := idx.DocumentRoot("m.sysml").LookupLocal("M")
	if !ok {
		t.Fatal("package M not indexed")
	}
	u, ok := pkg.Scope.LookupLocal("m")
	if !ok {
		t.Fatal("state m not indexed")
	}
	return ToStateGraphWithEndpoints(u.Decl, u.Scope, NewLibraryStateTypes(resolve.New(idx)))
}

func pseudostateShape(g *StateGraph) []string {
	shape := make([]string, 0, len(g.Pseudostates))
	for _, ps := range g.Pseudostates {
		shape = append(shape, ps.Kind.String()+":"+ps.Name)
	}
	return shape
}

func deferredShape(g *StateGraph) []string {
	shape := make([]string, 0)
	for _, s := range g.States {
		for _, trigger := range s.Defer {
			switch ev := trigger.(type) {
			case *ast.AcceptEvent:
				shape = append(shape, "accept:"+qualifiedNameText(ev.SignalType))
			case *ast.CallEvent:
				shape = append(shape, "call:"+qualifiedNameText(ev.Operation))
			case *ast.QualifiedName:
				shape = append(shape, "target:"+qualifiedNameText(ev))
			case *ast.Usage:
				shape = append(shape, "target:"+qualifiedNameText(typingTarget(ev)))
			default:
				shape = append(shape, reflect.TypeOf(trigger).String())
			}
		}
	}
	return shape
}

func qualifiedNameText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, 0, len(qn.Parts))
	for _, part := range qn.Parts {
		parts = append(parts, part.Text)
	}
	return strings.Join(parts, "::")
}

// TestMetadataPseudostatesMatchKeywordForms: the metadata-spelled state
// notation lowers to the same pseudostates and deferred triggers the keyword
// spellings produce.
func TestMetadataPseudostatesMatchKeywordForms(t *testing.T) {
	cases := []struct {
		name    string
		prelude string
		old     string
		new     string
	}{
		{
			name: "choice",
			old:  "state a; state b; choice pick;",
			new:  "state a; state b; #choice state pick;",
		},
		{
			name: "junction",
			old:  "state a; state b; junction j;",
			new:  "state a; state b; #junction state j;",
		},
		{
			name: "shallow history",
			old:  "state a { entry; then aa; state aa; } shallow history h;",
			new:  "state a { entry; then aa; state aa; } #shallowHistory state h;",
		},
		{
			name: "deep history",
			old:  "state a { entry; then aa; state aa; } deep history h;",
			new:  "state a { entry; then aa; state aa; } #deepHistory state h;",
		},
		{
			name:    "deferred signal",
			prelude: "item def Ping;",
			old:     "state a { defer Ping; }",
			new:     "state a { #deferred ref : Ping; }",
		},
		{
			name:    "deferred call",
			prelude: "private import ScalarValues::*; action def setSpeed { attribute v : Integer; }",
			old:     "state a { defer setSpeed(v); }",
			new:     "state a { #deferred ref : setSpeed; }",
		},
		{
			name:    "named deferred ref",
			prelude: "item def Ping;",
			old:     "state a { defer Ping; }",
			new:     "state a { #deferred ref p : Ping; }",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gOld, err := metadataStateGraph(t, tc.prelude, tc.old)
			if err != nil {
				t.Fatalf("old form lowers: %v", err)
			}
			gNew, err := metadataStateGraph(t, tc.prelude, tc.new)
			if err != nil {
				t.Fatalf("new form lowers: %v", err)
			}
			if got, want := pseudostateShape(gNew), pseudostateShape(gOld); !reflect.DeepEqual(got, want) {
				t.Errorf("pseudostates are %v, want %v", got, want)
			}
			if got, want := deferredShape(gNew), deferredShape(gOld); !reflect.DeepEqual(got, want) {
				t.Errorf("deferred triggers are %v, want %v", got, want)
			}
		})
	}
}
