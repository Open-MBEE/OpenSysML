package lower

import (
	"slices"
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
	through, ok := bindings[0].ThroughEnds(attachments, nil, Hop{})
	if !ok || through.Ends[0].Path != "s.src.voltage" || through.Ends[1].Path != "k.dst.voltage" {
		t.Errorf("bind a.voltage = b.voltage through the ends = %q = %q (%v), want s.src.voltage = k.dst.voltage", through.Ends[0].Path, through.Ends[1].Path, ok)
	}
	if through.Decl != bindings[0].Decl || through.Scope != bindings[0].Scope {
		t.Errorf("the rewritten binding lost its declaration or scope")
	}
	if bindings[0].Ends[0].Path != "a.voltage" {
		t.Errorf("ThroughEnds rewrote the original binding's end to %q", bindings[0].Ends[0].Path)
	}
	if _, ok := bindings[1].ThroughEnds(attachments, nil, Hop{Feature: "i"}); ok {
		t.Errorf("bind a.voltage = reading was rewritten, though reading is rooted at no known name")
	}
	through, ok = bindings[1].ThroughEnds(map[string]string{"a": "s.src", "b": "k.dst", "reading": "i.reading"}, nil, Hop{Feature: "i"})
	if !ok || through.Ends[0].Path != "s.src.voltage" || through.Ends[1].Path != "i.reading" {
		t.Errorf("bind a.voltage = reading through the ends and the connector = %q = %q (%v), want s.src.voltage = i.reading", through.Ends[0].Path, through.Ends[1].Path, ok)
	}
	through, ok = bindings[2].ThroughEnds(attachments, nil, Hop{})
	if !ok || through.Ends[0].Path != "s.src" || through.Ends[1].Path != "k.dst" {
		t.Errorf("bind a = b through the ends = %q = %q (%v), want s.src = k.dst", through.Ends[0].Path, through.Ends[1].Path, ok)
	}
	expression := bindings[0]
	expression.Ends[1] = BindingEnd{Expr: expression.Ends[1].Expr}
	if _, ok := expression.ThroughEnds(attachments, nil, Hop{}); ok {
		t.Errorf("an expression end of an anonymous connector was rewritten, though nothing reaches the connector")
	}
	through, ok = expression.ThroughEnds(attachments, nil, Hop{Feature: "w"})
	if !ok || through.Ends[0].Path != "s.src.voltage" || through.Ends[1].Path != "" || !slices.Equal(through.Ends[1].Via, []Hop{{Feature: "w"}}) {
		t.Errorf("an expression end through connector w = %q = via %v (%v), want s.src.voltage = via w", through.Ends[0].Path, through.Ends[1].Via, ok)
	}
	anonymous := &ast.Usage{}
	through, ok = through.ThroughEnds(map[string]string{"s": "p"}, nil, Hop{Anonymous: anonymous})
	if !ok || through.Ends[0].Path != "p.src.voltage" || !slices.Equal(through.Ends[1].Via, []Hop{{Anonymous: anonymous}, {Feature: "w"}}) {
		t.Errorf("an expression end re-rooted through nested connectors = %q = via %v (%v), want p.src.voltage = via the anonymous connector then w", through.Ends[0].Path, through.Ends[1].Via, ok)
	}
	through, ok = bindings[1].ThroughEnds(attachments, map[string]bool{"reading": true}, Hop{Anonymous: anonymous})
	if !ok || through.Ends[0].Path != "s.src.voltage" || through.Ends[1].Path != "reading" ||
		!slices.Equal(through.Ends[1].Via, []Hop{{Anonymous: anonymous}}) || through.Ends[1].HolderPath() != "" {
		t.Errorf("bind a.voltage = reading through an anonymous connector = %q = %q via %v (%v), want s.src.voltage = reading via the anonymous connector", through.Ends[0].Path, through.Ends[1].Path, through.Ends[1].Via, ok)
	}
	through, ok = through.ThroughEnds(map[string]string{"s": "p"}, nil, Hop{Feature: "outer"})
	if !ok || through.Ends[1].Path != "reading" || !slices.Equal(through.Ends[1].Via, []Hop{{Feature: "outer"}, {Anonymous: anonymous}}) {
		t.Errorf("a path end on an anonymous connector re-rooted through connector outer = %q via %v (%v), want reading via outer then the anonymous connector", through.Ends[1].Path, through.Ends[1].Via, ok)
	}
	if _, ok := bindings[0].ThroughEnds(map[string]string{"a": "s.src"}, nil, Hop{}); ok {
		t.Errorf("a binding over an end attached to nothing was rewritten")
	}
}

func TestEndValueBindingsLowerEndBodyValues(t *testing.T) {
	p := parser.New(source.New("ends.sysml", []byte(`package P {
		interface def L {
			end a : Pw;
			end b : ~Pw {
				attribute :>> voltage = 12;
				attribute :>> current default = 2;
				attribute :>> phase := 0;
				attribute :>> label;
			}
			attribute reading = 3;
		}
	}`)))
	file := p.ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("ends.sysml", file)
	scope := idx.DocumentRoot("ends.sysml")
	pkg := file.Members[0].(*ast.Membership).Member.(*ast.Package)
	def := pkg.Members[0].(*ast.Membership).Member.(*ast.Definition)
	bindings := EndValueBindings(def, scope)
	if len(bindings) != 2 {
		t.Fatalf("EndValueBindings lowered %d bindings, want 2: the bound and the default value, not the initial one, the valueless one or the connector's own attribute", len(bindings))
	}
	wants := []struct {
		path     string
		defaults bool
	}{{"b.voltage", false}, {"b.current", true}}
	for i, want := range wants {
		got := bindings[i]
		if got.Ends[0].Path != want.path || got.Ends[1].Path != "" || got.Ends[1].Expr == nil || got.Ends[1].Default != want.defaults {
			t.Errorf("binding %d = %q = expr %T default %v, want %q = its value expression, default %v",
				i, got.Ends[0].Path, got.Ends[1].Expr, got.Ends[1].Default, want.path, want.defaults)
		}
		if got.Decl == nil || got.Decl.Value != got.Ends[1].Expr {
			t.Errorf("binding %d is declared by %v, want the end's attribute", i, got.Decl)
		}
		if got.Origin.Doc != "ends.sysml" || got.Origin.Span != got.Decl.Span() || got.Origin.Kind != "binding" {
			t.Errorf("binding %d origin = %#v, want the end attribute's document, span and kind binding", i, got.Origin)
		}
	}
	if bindings[0].Origin == bindings[1].Origin {
		t.Errorf("the two end value bindings share the origin %#v; a run tells bindings apart by it", bindings[0].Origin)
	}
	through, ok := bindings[0].ThroughEnds(map[string]string{"a": "s.src", "b": "k.dst"}, nil, Hop{Feature: "i"})
	if !ok || through.Ends[0].Path != "k.dst.voltage" || !slices.Equal(through.Ends[1].Via, []Hop{{Feature: "i"}}) {
		t.Errorf("the end value through the ends = %q = via %v (%v), want k.dst.voltage = via i", through.Ends[0].Path, through.Ends[1].Via, ok)
	}
}
