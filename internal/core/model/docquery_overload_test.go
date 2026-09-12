package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/docrender"
)

// overloadedDocumentModel renders a document whose tables call Pick, declared
// once per key type; only the argument's type tells the two apart.
const overloadedDocumentModel = `
package A {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	calc def Pick :> Query { in source : Element; in key : Integer; Descendants(source = source, maxDepth = key) }
}
package B {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	calc def Pick :> Query { in source : Element; in key : String; WhereType(source = Descendants(source = source, maxDepth = 3), type = key) }
}
package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	%s

	part def Subsystem {
		attribute mass : Real;
	}

	part telescope {
		part optics : Subsystem {
			attribute redefines mass = 8.5;
		}
	}

	calc def ByDepth :> Query {
		in root : Element;
		Project(source = Pick(source = root, key = 1), properties = ("name", "mass"))
	}

	calc def ByType :> Query {
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
// whichever order the declarations were imported.
func TestRenderDocumentMarkdownSelectsQueryOverloadsByArgumentType(t *testing.T) {
	for _, imports := range []string{
		"private import A::*; private import B::*;",
		"private import B::*; private import A::*;",
	} {
		ws := openDoc(t, "report.sysml", fmt.Sprintf(overloadedDocumentModel, imports))
		markdown, err := ws.RenderDocumentMarkdown("Observatory::MassReport", docrender.MarkdownOptions{})
		if err != nil {
			t.Fatalf("%s: RenderDocumentMarkdown: %v", imports, err)
		}
		for _, want := range []string{"*Nearest parts*", "*Part usages*"} {
			if !strings.Contains(markdown, want) {
				t.Errorf("%s: markdown missing %q:\n%s", imports, want, markdown)
			}
		}
		if got := strings.Count(markdown, "| optics | 8.5 |"); got != 2 {
			t.Errorf("%s: optics row rendered %d times, want once per table:\n%s", imports, got, markdown)
		}
	}
}
