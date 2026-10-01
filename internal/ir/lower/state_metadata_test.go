package lower

import (
	"errors"
	"reflect"
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

// TestMetadataPseudostatesMatchKeywordForms: the metadata-spelled state
// notation lowers to the same pseudostates the keyword spellings produce.
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
		})
	}
}

// TestPseudostateMetadataRootedSpelling: `#$::StateMachines::choice` names the
// library package from the root, past a `StateMachines` member that hides it —
// the spelling the fixes and migrator write when the name is taken.
func TestPseudostateMetadataRootedSpelling(t *testing.T) {
	src := "package StateMachines {}\n" +
		"package M {\n state m {\n #$::StateMachines::choice state pick;\n }\n}\n"
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
	usage, ok := u.Decl.(*ast.Usage)
	if !ok || len(usage.Members) != 1 {
		t.Fatalf("state m has members %v, want one", usage.Members)
	}
	member, ok := unwrapMembership(usage.Members[0]).(*ast.Usage)
	if !ok {
		t.Fatalf("member is %T, want a usage", usage.Members[0])
	}
	kind, ok := PseudostateMetadata(resolve.New(idx), u.Scope, member)
	if !ok || kind != ast.PseudostateChoice {
		t.Fatalf("PseudostateMetadata = %v, %v, want choice", kind, ok)
	}
}

// TestPseudostateIsNotATransitionSource: a transition whose source succession
// hangs on a pseudostate is refused the same either spelling of it.
func TestPseudostateIsNotATransitionSource(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"keyword", "entry; then a; state a; choice pick; transition then ready; state ready;"},
		{"metadata", "entry; then a; state a; #StateMachines::choice state pick; transition then ready; state ready;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, err := metadataStateGraph(t, "", tc.body)
			var src *TransitionSourceError
			if !errors.As(err, &src) {
				t.Fatalf("error is %v (graph %v), want a TransitionSourceError", err, g)
			}
		})
	}
}

// TestParallelRegionOfOnlyAPseudostate: a parallel region holding a pseudostate
// and no substate needs no initial state, either spelling of the pseudostate.
func TestParallelRegionOfOnlyAPseudostate(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"keyword", "entry; then regions; state regions parallel { state r1 { choice pick; } state r2 { entry; then s2; state s2; } }"},
		{"metadata", "entry; then regions; state regions parallel { state r1 { #StateMachines::choice state pick; } state r2 { entry; then s2; state s2; } }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := metadataStateGraph(t, "", tc.body); err != nil {
				t.Fatalf("lowering: %v", err)
			}
		})
	}
}

// TestMetadataLocalAliasSpelling: an alias declared in the annotated usage's
// own body types the annotation as the library metadata it targets, which the
// graph reads the same as the spelling that names the metadata directly.
func TestMetadataLocalAliasSpelling(t *testing.T) {
	gOld, err := metadataStateGraph(t, "", "state a; state b; choice pick;")
	if err != nil {
		t.Fatalf("keyword form lowers: %v", err)
	}
	gNew, err := metadataStateGraph(t, "",
		"state a; state b; #localChoice state pick { alias localChoice for StateMachines::ChoiceMetadata; }")
	if err != nil {
		t.Fatalf("alias form lowers: %v", err)
	}
	if got, want := pseudostateShape(gNew), pseudostateShape(gOld); !reflect.DeepEqual(got, want) {
		t.Fatalf("pseudostates are %v, want %v", got, want)
	}
	if got := pseudostateShape(gNew); !reflect.DeepEqual(got, []string{"choice:pick"}) {
		t.Fatalf("pseudostates are %v, want [choice:pick]", got)
	}
}
