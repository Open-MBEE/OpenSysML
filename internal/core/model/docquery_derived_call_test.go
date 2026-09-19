package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
)

// callValuedDocumentModel renders a table over an attribute whose value is a
// call, which only the runtime's declared reader can evaluate.
const callValuedDocumentModel = `
package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	calc def Twice { in x : Real; return : Real = x * 2.0; }

	part def Subsystem {
		attribute mass : Real = Twice(4.25);
	}

	part telescope {
		part optics : Subsystem;
	}

	calc def Parts :> Query {
		in root : Element;
		Project(source = Descendants(source = root, maxDepth = 1), properties = ("name", "mass"))
	}

	part def MassReport :> Document {
		attribute redefines title = "Telescope Mass Report";

		part parts : Table {
			attribute redefines caption = "Parts";
			calc rows : Parts {
				in root = telescope;
			}
		}
	}
}
`

// A rendered document reads a call-valued attribute through the workspace's
// semantic model, whose argument typing selects the call.
func TestRenderDocumentMarkdownReadsCallValuedAttributes(t *testing.T) {
	ws := openDoc(t, "report.sysml", callValuedDocumentModel)
	markdown, err := ws.RenderDocumentMarkdown("Observatory::MassReport", docrender.MarkdownOptions{})
	if err != nil {
		t.Fatalf("RenderDocumentMarkdown: %v", err)
	}
	if !strings.Contains(markdown, "| optics | 8.5 |") {
		t.Errorf("markdown lacks the optics row with its computed mass:\n%s", markdown)
	}
}
