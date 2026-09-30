package model

import (
	"regexp"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
)

// caseCollidingDocumentModel declares two documents whose names meet letter
// case aside, one referring to the other, so a set tags both files.
const caseCollidingDocumentModel = `
package Reports {
	private import DocumentQueries::*;

	ref shouting : WEEKLY;

	part def Weekly :> Document {
		attribute redefines title = "Weekly";
		part intro : Paragraph {
			part see : Ref {
				ref redefines target = shouting;
			}
		}
	}
	part def WEEKLY :> Document {
		attribute redefines title = "WEEKLY";
	}
}
`

// A document rendered on its own links a sibling by the file a set writes it
// to, tag included, so a preview's links land on the set's files.
func TestRenderDocumentMarkdownLinksSiblingsByPlannedFiles(t *testing.T) {
	ws := openDoc(t, "reports.sysml", caseCollidingDocumentModel)
	markdown, err := ws.RenderDocumentMarkdown("Reports::Weekly", docrender.MarkdownOptions{})
	if err != nil {
		t.Fatalf("RenderDocumentMarkdown: %v", err)
	}
	files, err := DocumentFiles([]string{"Reports::Weekly", "Reports::WEEKLY"}, ".md")
	if err != nil {
		t.Fatalf("DocumentFiles: %v", err)
	}
	want := regexp.QuoteMeta("](" + files["Reports::WEEKLY"] + ")")
	if !regexp.MustCompile(`Reports-WEEKLY~[0-9a-f]+\.md`).MatchString(files["Reports::WEEKLY"]) {
		t.Fatalf("planned file %q is not tagged", files["Reports::WEEKLY"])
	}
	if !regexp.MustCompile(want).MatchString(markdown) {
		t.Errorf("markdown does not link %s:\n%s", files["Reports::WEEKLY"], markdown)
	}
}
