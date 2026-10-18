package modeldoc_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/modeldoc"
)

// overloadedDocumentModel renders a document whose tables call Pick, declared
// once per key type; only the argument's type tells the two apart.
const overloadedDocumentModel = `
calc def A :> %[1]s {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	calc def Pick :> Query { in source : Element; in key : Integer; Descendants(source = source, maxDepth = key) }
}
calc def B :> %[2]s {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	calc def Pick :> Query { in source : Element; in key : String; WhereType(source = Descendants(source = source, maxDepth = 3), type = key) }
}
package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Subsystem {
		attribute mass : Real;
	}

	part telescope {
		part optics : Subsystem {
			attribute redefines mass = 8.5;
		}
	}

	calc def ByDepth :> %[3]s {
		in root : Element;
		Project(source = Pick(source = root, key = 1), properties = ("name", "mass"))
	}

	calc def ByType :> %[3]s {
		in root : Element;
		Project(source = Pick(source = root, key = "PartUsage"), properties = ("name", "mass"))
	}

	part def MassReport :> Document {
		attribute redefines title = "Telescope Mass Report";

		part nearest : Table {
			attribute redefines caption = "Nearest parts";
			calc rows : ByDepth {
				in root = telescope;
			}
		}

		part parts : Table {
			attribute redefines caption = "Part usages";
			calc rows : ByType {
				in root = telescope;
			}
		}
	}
}
`

// A rendered document selects each query overload by its argument's type, in
// whichever way A and B chain under Query to be the queries' one general.
func TestRenderDocumentMarkdownSelectsQueryOverloadsByArgumentType(t *testing.T) {
	for _, chain := range []struct{ a, b, top string }{{"DocumentQueries::Query", "A", "B"}, {"B", "DocumentQueries::Query", "A"}} {
		generals := chain.a + "/" + chain.b + "/" + chain.top
		ws := openDoc(t, "report.sysml", fmt.Sprintf(overloadedDocumentModel, chain.a, chain.b, chain.top))
		markdown, err := modeldoc.RenderDocumentMarkdown(ws, "Observatory::MassReport", docrender.MarkdownOptions{})
		if err != nil {
			t.Fatalf("%s: RenderDocumentMarkdown: %v", generals, err)
		}
		for _, want := range []string{"*Nearest parts*", "*Part usages*"} {
			if !strings.Contains(markdown, want) {
				t.Errorf("%s: markdown missing %q:\n%s", generals, want, markdown)
			}
		}
		if got := strings.Count(markdown, "| optics | 8.5 |"); got != 2 {
			t.Errorf("%s: optics row rendered %d times, want once per table:\n%s", generals, got, markdown)
		}
	}
}
