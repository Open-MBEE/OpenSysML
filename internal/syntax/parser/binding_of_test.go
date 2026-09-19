package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// `binding b of f = v` names the feature the binding binds, so the `of` target
// is a connector end and not a typing: typing it would type-check the bound
// feature as the binding's type.
func TestBindingOfTargetIsAConnectorEnd(t *testing.T) {
	const code = `package P { binding linkAToB of [1] featureA = [1] featureB; }`

	p := New(source.New("test.sysml", []byte(code)))
	file := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("offset %d: %s", d.Span.Offset, d.Message)
	}

	binding := findBindingUsage(t, file)
	if binding.Ident.Name != "linkAToB" {
		t.Errorf("binding name = %q, want linkAToB", binding.Ident.Name)
	}
	for _, rel := range binding.Relationships {
		t.Errorf("binding carries a %v relationship; ends are connector ends", rel.Kind)
	}
	if binding.Value != nil {
		t.Errorf("binding carries a value; ends are connector ends")
	}
	ends := binding.ConnectorEnds
	if len(ends) != 2 {
		t.Fatalf("binding has %d ends, want 2", len(ends))
	}
	for i, want := range []string{"featureA", "featureB"} {
		if got := ast.QualifiedText(ends[i].AttachedTarget()); got != want {
			t.Errorf("end %d target = %q, want %q", i, got, want)
		}
		if ends[i].Multiplicity == nil {
			t.Errorf("end %d has no multiplicity, want [1]", i)
		}
		if _, named := ends[i].DeclaredName(); named {
			t.Errorf("end %d is named, want anonymous", i)
		}
	}
}

func TestAnonymousBindingSimpleEnds(t *testing.T) {
	const code = `package P {
		binding bind b = a;
		bind b = a;
	}`

	sf := source.New("test.sysml", []byte(code))
	file := New(sf).ParseFile()
	var bindings []*ast.Usage
	var collect func([]ast.Node)
	collect = func(nodes []ast.Node) {
		for _, node := range nodes {
			switch v := node.(type) {
			case *ast.Membership:
				collect([]ast.Node{v.Member})
			case *ast.Package:
				collect(v.Members)
			case *ast.Usage:
				if v.Kind == ast.UsageBinding {
					bindings = append(bindings, v)
				}
				collect(v.Members)
			}
		}
	}
	collect(file.Members)
	if len(bindings) != 2 {
		t.Fatalf("parsed %d bindings, want 2", len(bindings))
	}
	for _, binding := range bindings {
		if binding.Ident.Name != "" {
			t.Errorf("binding ident = %q, want empty", binding.Ident.Name)
		}
		if len(binding.Relationships) != 0 || binding.Value != nil {
			t.Fatalf("binding stores ends outside ConnectorEnds")
		}
		if len(binding.ConnectorEnds) != 2 {
			t.Fatalf("binding has %d ends, want 2", len(binding.ConnectorEnds))
		}
		for i, want := range []string{"b", "a"} {
			target, ok := binding.ConnectorEnds[i].Target.(*ast.QualifiedName)
			if !ok || len(target.Parts) != 1 || target.Parts[0].Text != want {
				t.Errorf("end %d target = %#v, want qualified name %s", i, binding.ConnectorEnds[i].Target, want)
				continue
			}
			if sf.Text(target.Parts[0].Span) != want {
				t.Errorf("end %d target span = %#v, want source span for %s", i, target, want)
			}
		}
	}
}

// A binding end reads like a succession's: `[mult] name ::> feature` names the
// end, and the name never becomes the binding's own name.
func TestBindingNamedEnds(t *testing.T) {
	cases := []struct {
		file, code   string
		bindingName  string
		endNames     [2]string
		endTargets   [2]string
		endMultCount int
	}{
		{"t.sysml", `part def D { bind e1 ::> a = e2 references b; }`, "", [2]string{"e1", "e2"}, [2]string{"a", "b"}, 0},
		{"t.sysml", `part def D { bind [1] e1 ::> a = [1] e2 ::> b; }`, "", [2]string{"e1", "e2"}, [2]string{"a", "b"}, 2},
		{"t.sysml", `part def D { bind e3 ::> a = b; }`, "", [2]string{"e3", ""}, [2]string{"a", "b"}, 0},
		{"t.sysml", `part def D { bind a = e2 ::> b; }`, "", [2]string{"", "e2"}, [2]string{"a", "b"}, 0},
		{"t.sysml", `part def D { binding x bind e1 ::> a = e2 ::> b; }`, "x", [2]string{"e1", "e2"}, [2]string{"a", "b"}, 0},
		{"t.kerml", `class D { binding e1 ::> a = e2 ::> b; }`, "", [2]string{"e1", "e2"}, [2]string{"a", "b"}, 0},
		{"t.kerml", `class D { binding of e1 ::> a = e2 ::> b; }`, "", [2]string{"e1", "e2"}, [2]string{"a", "b"}, 0},
		{"t.kerml", `class D { binding x of e1 references a = e2 references b; }`, "x", [2]string{"e1", "e2"}, [2]string{"a", "b"}, 0},
		{"t.kerml", `class D { binding [1] e1 ::> a = [1] e2 ::> b; }`, "", [2]string{"e1", "e2"}, [2]string{"a", "b"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			p := New(source.New(tc.file, []byte(tc.code)))
			file := p.ParseFile()
			for _, d := range p.Diagnostics {
				t.Errorf("offset %d: %s", d.Span.Offset, d.Message)
			}
			binding := findBindingUsage(t, file)
			if binding.Ident.Name != tc.bindingName {
				t.Errorf("binding name = %q, want %q", binding.Ident.Name, tc.bindingName)
			}
			if len(binding.ConnectorEnds) != 2 {
				t.Fatalf("binding has %d ends, want 2", len(binding.ConnectorEnds))
			}
			mults := 0
			for i, end := range binding.ConnectorEnds {
				name, named := end.DeclaredName()
				if named != (tc.endNames[i] != "") || name.Name != tc.endNames[i] {
					t.Errorf("end %d name = %q (named=%v), want %q", i, name.Name, named, tc.endNames[i])
				}
				if got := ast.QualifiedText(end.AttachedTarget()); got != tc.endTargets[i] {
					t.Errorf("end %d attached target = %q, want %q", i, got, tc.endTargets[i])
				}
				if end.Multiplicity != nil {
					mults++
				}
			}
			if mults != tc.endMultCount {
				t.Errorf("%d ends carry a multiplicity, want %d", mults, tc.endMultCount)
			}
		})
	}
}

func findBindingUsage(t *testing.T, root *ast.RootNamespace) *ast.Usage {
	t.Helper()

	var find func(nodes []ast.Node) *ast.Usage
	find = func(nodes []ast.Node) *ast.Usage {
		for _, n := range nodes {
			switch v := n.(type) {
			case *ast.Membership:
				if found := find([]ast.Node{v.Member}); found != nil {
					return found
				}
			case *ast.Package:
				if found := find(v.Members); found != nil {
					return found
				}
			case *ast.Definition:
				if found := find(v.Members); found != nil {
					return found
				}
			case *ast.Usage:
				if v.Kind == ast.UsageBinding {
					return v
				}
				if found := find(v.Members); found != nil {
					return found
				}
			}
		}
		return nil
	}

	found := find(root.Members)
	if found == nil {
		t.Fatalf("no binding usage parsed")
	}
	return found
}
