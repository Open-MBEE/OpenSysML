package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestToBindingsNormalizesBindingSpellings(t *testing.T) {
	p := parser.New(source.New("binding.sysml", []byte(`package P {
		part def Owner {
			binding bind x = y;
			bind x = y;
			binding named bind x = y;
			binding namedOf of x = y;
			binding namedOnly = x;
			bind incomplete;
			binding incompleteOf of x;
			binding [1] config.host = serverAddress;
		}
	}`)))
	file := p.ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("binding.sysml", file)
	scope := idx.DocumentRoot("binding.sysml")
	var bindings []Binding
	for _, member := range file.Members {
		membership, ok := member.(*ast.Membership)
		if !ok {
			continue
		}
		pkg, ok := membership.Member.(*ast.Package)
		if !ok {
			continue
		}
		for _, nested := range pkg.Members {
			ownerMembership, ok := nested.(*ast.Membership)
			if !ok {
				continue
			}
			owner, ok := ownerMembership.Member.(*ast.Definition)
			if ok {
				bindings = append(bindings, ToBindings(owner, scope)...)
			}
		}
	}
	if len(bindings) != 6 {
		t.Fatalf("lowered %d bindings, want 6", len(bindings))
	}
	wants := map[[2]string]int{
		{"x", "y"}:                       4,
		{"namedOnly", "x"}:               1,
		{"config.host", "serverAddress"}: 1,
	}
	for _, binding := range bindings {
		paths := [2]string{binding.Ends[0].Path, binding.Ends[1].Path}
		if wants[paths] == 0 {
			t.Errorf("binding paths = %q, want one of [x, y], [namedOnly, x] or [config.host, serverAddress]", paths)
			continue
		}
		wants[paths]--
	}
	for paths, count := range wants {
		if count != 0 {
			t.Errorf("binding paths %q occurred %d extra times", paths, count)
		}
	}
}

func TestToBindingsKeepsMultipleContributors(t *testing.T) {
	p := parser.New(source.New("binding-multiple.sysml", []byte(`package P {
		part def Sys {
			part edges : Edge[*];
			part leftEdge : Edge;
			part rightEdge : Edge;
			binding [1] bind [0..1] edges = [0..1] leftEdge;
			binding [1] bind [0..1] edges = [0..1] rightEdge;
		}
	}`)))
	file := p.ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("binding-multiple.sysml", file)
	scope := idx.DocumentRoot("binding-multiple.sysml")
	var bindings []Binding
	for _, member := range file.Members {
		membership, ok := member.(*ast.Membership)
		if !ok {
			continue
		}
		pkg, ok := membership.Member.(*ast.Package)
		if !ok {
			continue
		}
		for _, nested := range pkg.Members {
			ownerMembership, ok := nested.(*ast.Membership)
			if !ok {
				continue
			}
			owner, ok := ownerMembership.Member.(*ast.Definition)
			if ok {
				bindings = append(bindings, ToBindings(owner, scope)...)
			}
		}
	}
	if len(bindings) != 2 {
		t.Fatalf("lowered %d bindings, want 2", len(bindings))
	}
	for _, binding := range bindings {
		if got := [2]string{binding.Ends[0].Path, binding.Ends[1].Path}; got != [2]string{"edges", "leftEdge"} &&
			got != [2]string{"edges", "rightEdge"} {
			t.Errorf("binding paths = %q, want edges/leftEdge or edges/rightEdge", got)
		}
	}
}
