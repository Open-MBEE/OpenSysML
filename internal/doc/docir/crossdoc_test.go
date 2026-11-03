package docir

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
)

// crossDocumentFixture declares two documents where one references the
// other's content block and root.
const crossDocumentFixture = `
	part def Appendix :> Document {
		attribute redefines title = "Appendix";
		part tables : Section {
			attribute redefines title = "Detail Tables";
			part body : Paragraph {
				attribute redefines text = "detail";
			}
		}
	}
	part def Report :> Document {
		attribute redefines title = "Report";
		part intro : Paragraph {
			part see : Ref {
				ref redefines target = Appendix::tables;
			}
			part whole : Ref {
				ref redefines target = Appendix;
			}
		}
	}
`

// TestEvaluateCrossDocumentRefs locks the IR of references into another
// document: the target document travels with the run, a content-block
// reference carries its anchor, a root reference none.
func TestEvaluateCrossDocumentRefs(t *testing.T) {
	fixture := loadEvaluationFixture(t, crossDocumentFixture)
	document := fixture.mustEvaluate(t, "Report")
	runs := document.Content()[0].Runs()
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(runs))
	}
	for i, want := range []struct {
		text     string
		target   string
		document string
	}{
		{"Detail Tables", "tables", "Observatory::Appendix"},
		{"Appendix", "", "Observatory::Appendix"},
	} {
		if runs[i].Kind() != RunRef {
			t.Fatalf("run %d kind = %s", i, runs[i].Kind())
		}
		if runs[i].Text() != want.text || runs[i].Target() != want.target || runs[i].TargetDocument() != want.document {
			t.Errorf("run %d = %q %q %q, want %q %q %q",
				i, runs[i].Text(), runs[i].Target(), runs[i].TargetDocument(),
				want.text, want.target, want.document)
		}
	}
}

// TestEvaluateSetEmitsCrossDocumentAnchors checks that evaluating documents as
// a set stamps the anchor another document's reference requires onto the
// target content block, while separate evaluation does not.
func TestEvaluateSetEmitsCrossDocumentAnchors(t *testing.T) {
	fixture := loadEvaluationFixture(t, crossDocumentFixture)
	plans := []*docplan.Plan{fixture.plan(t, "Appendix"), fixture.plan(t, "Report")}
	outcomes, err := EvaluateSet(plans, fixture.context(), queryexec.Options{}, nil)
	if err != nil {
		t.Fatalf("evaluate set: %v", err)
	}
	if len(outcomes) != 2 {
		t.Fatalf("documents = %d, want 2", len(outcomes))
	}
	for _, outcome := range outcomes {
		if outcome.Err != nil {
			t.Fatalf("evaluate %s: %v", outcome.Name, outcome.Err)
		}
	}
	appendix := outcomes[0].Document
	if appendix.Name() != "Observatory::Appendix" {
		t.Fatalf("first document = %s", appendix.Name())
	}
	if anchor := appendix.Content()[0].Anchor(); anchor != "tables" {
		t.Errorf("set-evaluated anchor = %q, want %q", anchor, "tables")
	}
	alone := fixture.mustEvaluate(t, "Appendix")
	if anchor := alone.Content()[0].Anchor(); anchor != "" {
		t.Errorf("separately evaluated anchor = %q, want none", anchor)
	}
}

// crossDocumentListFixture references another document's content block from
// a Markdown list item rather than from a paragraph run.
const crossDocumentListFixture = `
	part def Appendix :> Document {
		attribute redefines title = "Appendix";
		part tables : Section {
			attribute redefines title = "Detail Tables";
			part body : Paragraph {
				attribute redefines text = "detail";
			}
		}
	}
	part def Report :> Document {
		attribute redefines title = "Report";
		rep language "markdown" /* - see [](sysml:Appendix::tables) */
	}
`

// TestEvaluateSetEmitsAnchorsForListItemRefs checks that a reference inside a
// static list item requests its target's anchor like a paragraph run does.
func TestEvaluateSetEmitsAnchorsForListItemRefs(t *testing.T) {
	fixture := loadEvaluationFixture(t, crossDocumentListFixture)
	plans := []*docplan.Plan{fixture.plan(t, "Appendix"), fixture.plan(t, "Report")}
	outcomes, err := EvaluateSet(plans, fixture.context(), queryexec.Options{}, nil)
	if err != nil {
		t.Fatalf("evaluate set: %v", err)
	}
	for _, outcome := range outcomes {
		if outcome.Err != nil {
			t.Fatalf("evaluate %s: %v", outcome.Name, outcome.Err)
		}
	}
	if anchor := outcomes[0].Document.Content()[0].Anchor(); anchor != "tables" {
		t.Errorf("anchor requested from a list item = %q, want %q", anchor, "tables")
	}
	items := outcomes[1].Document.Content()[0].Items()
	if len(items) != 1 || len(items[0].Runs()) < 2 || items[0].Runs()[1].Target() != "tables" {
		t.Fatalf("list item runs = %+v, want a reference run targeting %q", items, "tables")
	}
}
