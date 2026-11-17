package symbolfacts

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestNamespaceFactsIncludeEveryDocumentFragment(t *testing.T) {
	idx, _ := modelWithLibrary(t, `
package P { part def A; package Nested { part def X; } }
package Parts { part def Engine; }
part def Duplicate { part a; }
`)
	p := parser.New(source.New("other.sysml", []byte(`
package P { part def B; package Nested { part def Y; } }
package Parts { part def Vehicle; }
part def Duplicate { part b; }
`)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatal(p.Diagnostics)
	}
	idx.AddDocument("other.sysml", root)
	idx.ExpandWildcardImports()
	sc := NewContext(idx)
	for _, tc := range []struct {
		id   string
		want []string
	}{
		{"P", []string{"P::A", "P::B", "P::Nested"}},
		{"P::Nested", []string{"P::Nested::X", "P::Nested::Y"}},
		{"Parts", []string{"Parts::Engine", "Parts::Vehicle"}},
		{"Duplicate", []string{"Duplicate::a"}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			matches := LookupNamed(idx, tc.id)
			if len(matches) < 2 {
				t.Fatalf("fixture has %d matches, want multiple fragments", len(matches))
			}
			got := Of(matches[0], sc).ChildIds
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("children = %v, want %v", got, tc.want)
			}
			if tc.id != "Duplicate" {
				if other := Of(matches[1], sc).ChildIds; !reflect.DeepEqual(got, other) {
					t.Errorf("fragments differ: %v vs %v", got, other)
				}
			}
		})
	}
	for _, name := range []string{"model.sysml", "other.sysml"} {
		if got := Root(sc, idx.DocumentRoot(name)).ChildIds; !reflect.DeepEqual(got, []string{"P", "Parts", "Duplicate"}) {
			t.Errorf("document %s root children = %v", name, got)
		}
	}
}

func TestNamespaceFactsKeepLibraryAndModelChildrenSeparate(t *testing.T) {
	idx, sc := modelWithLibrary(t, "package Parts { part def Vehicle; }")
	for _, sym := range LookupNamed(idx, "Parts") {
		info := Of(sym, sc)
		hasVehicle, hasPart := false, false
		for _, id := range info.ChildIds {
			hasVehicle = hasVehicle || id == "Parts::Vehicle"
			hasPart = hasPart || id == "Parts::Part"
		}
		if idx.Library(sym) {
			if !hasPart || hasVehicle {
				t.Errorf("library children = %v", info.ChildIds)
			}
		} else if !hasVehicle || hasPart {
			t.Errorf("model children = %v", info.ChildIds)
		}
	}
}
