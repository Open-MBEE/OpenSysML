package migrate_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/mtip"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

func TestSequenceInheritedOccurrenceName(t *testing.T) {
	r := migrateDocument(t, `
<packagedElement xmi:type="uml:Class" xmi:id="_block" name="Block">
  <ownedBehavior xmi:type="uml:Interaction" xmi:id="_seq" name="Sequence">
    <ownedAttribute xmi:type="uml:Property" xmi:id="_self" name="self" type="_block"/>
    <ownedAttribute xmi:type="uml:Property" xmi:id="_other" name="other" type="_block"/>
    <lifeline xmi:type="uml:Lifeline" xmi:id="_line" represents="_self"/>
    <lifeline xmi:type="uml:Lifeline" xmi:id="_otherLine" represents="_other"/>
    <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sent" covered="_line" message="_message"/>
    <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_received" covered="_otherLine" message="_message"/>
    <message xmi:type="uml:Message" xmi:id="_message" name="send" messageSort="asynchCall" sendEvent="_sent" receiveEvent="_received"/>
  </ownedBehavior>
</packagedElement>`, `<sysml:Block xmi:id="_st" base_Class="_block"/>`)
	wantLine(t, r.Notation, "ref part self2 : Block {")
	wantLine(t, r.Notation, "message 'send' from self2.sendSent to other.sendReceived;")
	if strings.Contains(string(r.Notation), "ref part self :") {
		t.Error("sequence declares the inherited name self")
	}
	ws := model.NewWorkspace()
	ws.Open("sequence.sysml", r.Notation, 1)
	for _, d := range ws.Diagnostics("sequence.sysml") {
		if d.Severity == diag.SeverityError || d.Code == "name-conflict" {
			t.Errorf("%v", d)
		}
	}
}

func TestSequenceInheritedNameLayout(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/usecase_sequence.xmi")
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data, []byte(`name="stage"`), []byte(`name="self"`))
	data = bytes.Replace(data, []byte(`xmi:id="_lStage" name="self"`), []byte(`xmi:id="_lStage"`), 1)
	for _, id := range []string{"_lStage", "_pStage"} {
		t.Run(id, func(t *testing.T) {
			layout := &mtip.Export{Diagrams: []mtip.Diagram{{ID: "_diag_seq", Placements: []mtip.Placement{
				{ID: id, X: 200, Y: 20, Width: 120, Height: 50},
			}}}}
			r, err := migrate.MigrateOptions("sequence.xmi", data, migrate.Options{Layout: layout})
			if err != nil {
				t.Fatal(err)
			}
			wantLine(t, r.Notation, "ref part self2 : 'Sample Stage' {")
			wantLine(t, r.Notation, "message move from operator.moveSent to self2.moveReceived;")
			if id == "_lStage" {
				wantLine(t, r.Notation, "metadata DiagramLayout::Layout about self2 { x = 200; y = 20; width = 120; height = 50; }")
			} else if r.Report.Layout == nil || r.Report.Layout.PlacementsUnexposed != 1 {
				t.Errorf("represented property has no separate sequence symbol: %+v", r.Report.Layout)
			}
			if strings.Contains(string(r.Notation), "Layout about self {") {
				t.Error("layout refers to the inherited self instead of the lifeline")
			}
		})
	}
}
