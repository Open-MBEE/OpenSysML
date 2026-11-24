package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// The symbol table spells a document's root Namespace off its first written top-level element,
// as rootNamespaceIDs does, whether that element is a package, an alias or an import.
func TestSymbolRootNamespaceIDFollowsTheFirstWrittenElement(t *testing.T) {
	for _, text := range []string{
		"alias A for P;\npackage P {}\n",
		"package P {}\nalias A for P;\n",
		"import Q::*;\npackage P {}\npackage Q {}\n",
		"package P {}\npackage Q {}\n",
		"comment /* c */\npackage P {}\n",
	} {
		sf := source.New("root.sysml", []byte(text))
		root := parser.New(sf).ParseFile()
		graph, res, encoders, err := modelToRDF([]ModelDocument{{File: sf, Root: root}}, IDQualifiedName)
		if err != nil {
			t.Fatal(err)
		}
		ids, err := identityIDsOf(res, encoders, []bool{false})
		if err != nil {
			t.Fatal(err)
		}
		if graph, err = withRootNamespace(graph); err != nil {
			t.Fatal(err)
		}
		var exported []string
		for _, subject := range graph.Subjects() {
			if graph.Type(subject) == rdf.SysML+"Namespace" {
				exported = append(exported, rdf.LocalName(subject.Value))
			}
		}
		model := encoders[0].ids.model
		got, ok := NewSymbolIDs(ids, res, model).Of(model.RootNamespace(sf.Name()))
		if !ok || len(exported) != 1 || got != exported[0] {
			t.Errorf("%q: the symbol root id is %q (%v), the conversion writes %v", text, got, ok, exported)
		}
	}
}
