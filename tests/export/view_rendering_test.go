package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

const viewRenderingModel = `package Views {
    rendering def Tree;
    rendering tree : Tree;
    view def ByReference {
        render tree;
    }
    view def ByDeclaration {
        render rendering t2 : Tree;
    }
}
`

// A view's `render tree;` is a RenderingUsage named by the rendering it
// references (SysML RenderingUsage::namingFeature): the graph names it
// Views::ByReference::tree with sysml:name and no declaredName, and the API
// element form carries the same name; it reads back as written.
func TestViewRenderingReferenceIsNamedByTheRendering(t *testing.T) {
	turtle, back := graphOnlyRoundTrip(t, "m.sysml", []byte(viewRenderingModel))
	graph := string(turtle)
	for _, want := range []string{
		"elmt:Views__ByReference__tree\n    a sysml:RenderingUsage ;\n    sysml:name \"tree\" ;\n    sysml:qualifiedName \"Views::ByReference::tree\" ;",
		"elmt:Views__ByReference__tree_om\n    a sysml:ViewRenderingMembership ;",
		"sysml:referencedFeature elmt:Views__tree ;",
		"elmt:Views__ByDeclaration__t2\n    a sysml:RenderingUsage ;",
		"sysml:declaredName \"t2\" ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	if strings.Contains(graph, "Views::ByReference::@0") {
		t.Errorf("the rendering reference is named by position:\n%s", graph)
	}
	notation := string(back)
	for _, want := range []string{"        render tree;\n", "        render rendering t2 : Tree;\n"} {
		if !strings.Contains(notation, want) {
			t.Errorf("the notation should contain %q:\n%s", want, notation)
		}
	}
	// The name is derived, so a graph written without sysml:name still agrees
	// with its text and reads back verbatim.
	unnamed, err := convert.Convert("m.ttl", withoutTriples(t, turtle, "sysml:name"), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation without sysml:name: %v", err)
	}
	if string(unnamed) != viewRenderingModel {
		t.Errorf("without sysml:name the notation changed\n--- want ---\n%s\n--- got ---\n%s", viewRenderingModel, unnamed)
	}

	doc, err := convert.Convert("m.sysml", []byte(viewRenderingModel), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(doc, &elements); err != nil {
		t.Fatal(err)
	}
	var renderings []map[string]any
	for _, element := range elements {
		if element["@type"] == "RenderingUsage" && element["qualifiedName"] != "Views::tree" {
			renderings = append(renderings, element)
		}
	}
	if len(renderings) != 2 {
		t.Fatalf("api-json holds %d view renderings, want 2", len(renderings))
	}
	for _, element := range renderings {
		switch element["qualifiedName"] {
		case "Views::ByReference::tree":
			if element["name"] != "tree" || element["declaredName"] != nil {
				t.Errorf("`render tree;` = name %v, declaredName %v; want the name tree and no declared name", element["name"], element["declaredName"])
			}
		case "Views::ByDeclaration::t2":
			if element["declaredName"] != "t2" {
				t.Errorf("`render rendering t2` declaredName = %v, want t2", element["declaredName"])
			}
		default:
			t.Errorf("unexpected view rendering %v", element["qualifiedName"])
		}
	}
}
