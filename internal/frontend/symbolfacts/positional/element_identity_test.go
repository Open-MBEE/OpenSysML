package positional_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const identityModel = `
package D {
  private import ScalarValues::*;
  part def V;
  part outer : V { part : V; part named : V; }
  part def M {
    port p;
    state sm { entry; then a; state a;
      transition t first a accept when 1 > 0 if 2 > 1 then a;
      transition u first a accept sig : Integer via p then a;
    }
    attribute z : Integer = 1 + 2;
  }
  connection def C;
  part holder { part a { part b; } part c; connection : C connect a.b to c; }
}
`

// identityContext indexes one document over the standard library with the
// positional and element-form ids a service's symbol context carries.
func identityContext(t *testing.T) *symbolfacts.Context {
	t.Helper()
	base, _ := libs.SharedLibrary()
	idx := symbols.NewOverlay(base)
	file := source.New("model.sysml", []byte(identityModel))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("fixture does not parse: %v", p.Diagnostics)
	}
	idx.AddDocument("model.sysml", root)
	idx.ExpandWildcardImports()
	sc := symbolfacts.NewContext(idx)
	sc.Positional = symbolfacts.NewPositional(idx, func() []symbolfacts.PositionalDocument {
		return []symbolfacts.PositionalDocument{{File: file, Root: idx.DocumentRoot("model.sysml")}}
	})
	reader := export.NewElementIDReader([]export.ElementIDDocument{{File: file, Root: root}})
	sc.ElementIDs = func() *export.ElementIDReader { return reader }
	return sc
}

// reflect follows a path of reflective features from the symbol id names.
func reflect(t *testing.T, sc *symbolfacts.Context, id string, path ...string) *symbols.Symbol {
	t.Helper()
	syms := sc.Lookup(id)
	if len(syms) != 1 {
		t.Fatalf("Lookup(%s) = %v", id, syms)
	}
	sym := syms[0]
	for _, feature := range path {
		elements, ok := sc.Semantics.ReflectiveElements(sym, feature)
		if !ok || len(elements) == 0 {
			t.Fatalf("%s.%s derives nothing", id, feature)
		}
		sym = elements[0]
	}
	return sym
}

// Every element reflection reaches is named by one of the ids the tree already
// has — qualified name, positional id, element form @id — and that id resolves
// back to the very symbol in the semantic model it was read from.
func TestElementIdentityIsTheTreesIDAndRoundTrips(t *testing.T) {
	sc := identityContext(t)
	cases := []struct {
		name string
		path []string
		want string
	}{
		{"D::outer::named", nil, "D::outer::named"},
		{"D::outer::@0", nil, "D::outer::@0"},
		{"D::M::sm::t", []string{"triggerAction"}, "D__M__sm__t___40trigger"},
		{"D::M::sm::t", []string{"triggerAction", "payloadParameter"}, "D__M__sm__t___40trigger___400"},
		{"D::M::sm::t", []string{"triggerAction", "payloadArgument"}, "D__M__sm__t___40trigger___400_pvalue"},
		{"D::M::sm::t", []string{"guardExpression"}, "D__M__sm__t_pguard"},
		{"D::M::sm::t", []string{"guardExpression", "argument"}, "D__M__sm__t_pguard_pa0"},
		{"D::M::sm::u", []string{"triggerAction", "payloadParameter"}, "D__M__sm__u___40trigger__sig"},
		{"D::M::sm::u", []string{"triggerAction", "receiverArgument"}, "D__M__sm__u___40trigger___401_pvalue"},
		{"D::M::z", []string{"valuation", "value"}, "D__M__z_pvalue"},
		{"D::M::z", []string{"valuation", "value", "parameter"}, "D__M__z_pvalue_pin0"},
		{"D::M::z", []string{"valuation", "value", "result"}, "D__M__z_pvalue_pout"},
		{"D::holder::@2", []string{"connectorEnd"}, "D__holder___402_pend0"},
		{"D::holder::@2", []string{"connectorEnd", "ownedElement"}, "D__holder___402_pend0_pchain"},
	}
	for _, tc := range cases {
		sym := reflect(t, sc, tc.name, tc.path...)
		got := sc.ElementIdentity(sym)
		if got != tc.want {
			t.Errorf("ElementIdentity(%s%v) = %q, want %q", tc.name, tc.path, got, tc.want)
			continue
		}
		back := sc.ResolveElement(got, sc.Semantics)
		if len(back) != 1 || back[0] != sym {
			t.Errorf("ResolveElement(%s) = %v, want the one symbol it was read from", got, back)
		}
	}
}

func TestResolveElementReadsEveryIDFormOfANamedElement(t *testing.T) {
	sc := identityContext(t)
	named := sc.Lookup("D::outer::named")
	for _, id := range []string{"D::outer::named", "D__outer__named"} {
		if got := sc.ResolveElement(id, sc.Semantics); len(got) != 1 || got[0] != named[0] {
			t.Errorf("ResolveElement(%s) = %v, want D::outer::named", id, got)
		}
	}
	for _, id := range []string{"", "D::outer::@7", "D__outer___407", "D__M__sm__t___40trigger_pnowhere", "not an id!"} {
		if got := sc.ResolveElement(id, sc.Semantics); len(got) != 0 {
			t.Errorf("ResolveElement(%q) = %v, want nothing", id, got)
		}
	}
}

// Without the element form's ids a synthesized element has no identity, and no
// semantic model to walk resolves a form id; neither is read as another element.
func TestElementIdentityNeedsTheFormForSynthesizedElements(t *testing.T) {
	sc := identityContext(t)
	value := reflect(t, sc, "D::M::z", "valuation", "value")
	if got := sc.ResolveElement("D__M__z_pvalue", nil); len(got) != 0 {
		t.Errorf("ResolveElement without a model = %v, want nothing", got)
	}
	sc.ElementIDs = nil
	if got := sc.ElementIdentity(value); got != "" {
		t.Errorf("ElementIdentity without the form = %q, want none", got)
	}
	if got := sc.ElementIdentity(reflect(t, sc, "D::outer::@0")); got != "D::outer::@0" {
		t.Errorf("positional identity without the form = %q", got)
	}
}
