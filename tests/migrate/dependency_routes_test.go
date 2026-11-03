package migrate_test

import (
	"bytes"
	"testing"
)

// A «Refine» drawn on an activity diagram is routed on the ActionFlowView the
// diagram is written as: the dependency notation lets one be drawn between any
// two element nodes a view shows, so the connection it is written as is pinned.
func TestGoldenRefineOnActivityDiagramIsRouted(t *testing.T) {
	r := migrateStream(t, "refine_on_activity")
	checkGolden(t, "testdata/xmi/refine_on_activity.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/refine_on_activity.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "refine_on_activity.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// A SysML Requirement Diagram is written as a tree rendering, which draws the
// requirements, their nesting and the derive and satisfy edges between them.
func TestGoldenRequirementDiagramIsATree(t *testing.T) {
	r := migrateStream(t, "requirement_diagram")
	checkGolden(t, "testdata/xmi/requirement_diagram.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/requirement_diagram.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "requirement_diagram.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}
