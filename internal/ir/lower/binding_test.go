package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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

func TestToBindingsKeepsTheFeatureAnIndexedEndSelectsFrom(t *testing.T) {
	p := parser.New(source.New("binding-indexed.sysml", []byte(`package P {
		part def Owner {
			bind gauge = tank.levels#(2);
			bind xs#(1) = ys#(i + 1);
		}
	}`)))
	file := p.ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("binding-indexed.sysml", file)
	scope := idx.DocumentRoot("binding-indexed.sysml")
	var bindings []Binding
	for _, member := range file.Members {
		if pkg, ok := member.(*ast.Membership).Member.(*ast.Package); ok {
			for _, nested := range pkg.Members {
				if owner, ok := nested.(*ast.Membership).Member.(*ast.Definition); ok {
					bindings = append(bindings, ToBindings(owner, scope)...)
				}
			}
		}
	}
	if len(bindings) != 2 {
		t.Fatalf("lowered %d bindings, want 2", len(bindings))
	}
	wants := [][2]struct {
		path    string
		indexed bool
	}{
		{{"gauge", false}, {"tank.levels", true}},
		{{"xs", true}, {"ys", true}},
	}
	for i, binding := range bindings {
		for end, want := range wants[i] {
			got := binding.Ends[end]
			if got.Path != want.path || (got.Index != nil) != want.indexed {
				t.Errorf("binding %d end %d = path %q indexed %v, want path %q indexed %v",
					i, end, got.Path, got.Index != nil, want.path, want.indexed)
			}
			if _, isIndex := got.Expr.(*ast.IndexExpr); isIndex != want.indexed {
				t.Errorf("binding %d end %d expr = %T, want the end as written", i, end, got.Expr)
			}
		}
	}
}

func TestBindingThroughEndsReRootsEndPaths(t *testing.T) {
	p := parser.New(source.New("through.sysml", []byte(`package P {
		interface def L {
			end a : Pw;
			end b : ~Pw;
			attribute reading;
			bind a.voltage = b.voltage;
			bind a.voltage = reading;
			bind a = b;
		}
	}`)))
	file := p.ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("through.sysml", file)
	scope := idx.DocumentRoot("through.sysml")
	var def *ast.Definition
	for _, member := range file.Members {
		pkg, ok := member.(*ast.Membership).Member.(*ast.Package)
		if !ok {
			continue
		}
		def = pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
	}
	bindings := ToBindings(def, scope)
	if len(bindings) != 3 {
		t.Fatalf("ToBindings lowered %d bindings, want 3", len(bindings))
	}
	attachments := map[string]string{"a": "s.src", "b": "k.dst"}
	through, ok := bindings[0].ThroughEnds(attachments)
	if !ok || through.Ends[0].Path != "s.src.voltage" || through.Ends[1].Path != "k.dst.voltage" {
		t.Errorf("bind a.voltage = b.voltage through the ends = %q = %q (%v), want s.src.voltage = k.dst.voltage", through.Ends[0].Path, through.Ends[1].Path, ok)
	}
	if through.Decl != bindings[0].Decl || through.Scope != bindings[0].Scope {
		t.Errorf("the rewritten binding lost its declaration or scope")
	}
	if bindings[0].Ends[0].Path != "a.voltage" {
		t.Errorf("ThroughEnds rewrote the original binding's end to %q", bindings[0].Ends[0].Path)
	}
	if _, ok := bindings[1].ThroughEnds(attachments); ok {
		t.Errorf("bind a.voltage = reading was rewritten, though reading is the connector's own feature")
	}
	through, ok = bindings[2].ThroughEnds(attachments)
	if !ok || through.Ends[0].Path != "s.src" || through.Ends[1].Path != "k.dst" {
		t.Errorf("bind a = b through the ends = %q = %q (%v), want s.src = k.dst", through.Ends[0].Path, through.Ends[1].Path, ok)
	}
	expression := bindings[0]
	expression.Ends[1] = BindingEnd{Expr: expression.Ends[1].Expr}
	if _, ok := expression.ThroughEnds(attachments); ok {
		t.Errorf("a binding whose right end is an expression over no feature path was rewritten")
	}
	if _, ok := bindings[0].ThroughEnds(map[string]string{"a": "s.src"}); ok {
		t.Errorf("a binding over an end attached to nothing was rewritten")
	}
}
