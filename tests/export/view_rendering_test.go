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

// A graph written before members were named by their naming feature keyed a
// later declaration of the same name by that name (`P::A::a`) and the
// effectively named member before it by position: its references still read
// back, the member being matched by the name that graph gave it.
func TestOlderGraphKeyingALaterDeclarationByItsNameReadsBack(t *testing.T) {
	const src = `package P {
    action def M;
    action a;
    action def A {
        perform P::a;
        action a : M {
            action b :> P::a;
        }
    }
}
`
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(withoutSourceText(t, turtle))
	for _, want := range []string{`sysml:qualifiedName "P::A::a"`, `sysml:qualifiedName "P::A::@1"`, `sysml:qualifiedName "P::A::@1::b"`} {
		if !strings.Contains(graph, want) {
			t.Fatalf("the graph should state %s:\n%s", want, graph)
		}
	}
	older := strings.NewReplacer("P__A__a", "P__A___400", `"P::A::a"`, `"P::A::@0"`, "    sysml:name \"a\" ;\n", "").Replace(graph)
	older = strings.NewReplacer("P__A___401", "P__A__a", `"P::A::@1`, `"P::A::a`).Replace(older)
	back, err := convert.Convert("m.ttl", []byte(older), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the older graph: %v\n%s", err, older)
	}
	for _, want := range []string{"        perform P::a;\n", "        action a : M {\n", "            action b subsets P::a;\n"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the notation should contain %q:\n%s", want, back)
		}
	}
}
