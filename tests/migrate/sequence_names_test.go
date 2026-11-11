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

func TestSequenceRenamedMemberExpressions(t *testing.T) {
	for _, lang := range []string{"", "OCL"} {
		t.Run(lang, func(t *testing.T) {
			r := migrateDocument(t, `
<packagedElement xmi:type="uml:Class" xmi:id="_block" name="Block">
  <ownedAttribute xmi:type="uml:Property" xmi:id="_amount" name="amount">
    <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
  </ownedAttribute>
  <ownedBehavior xmi:type="uml:Interaction" xmi:id="_seq" name="Sequence">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="_copy" name="copy">
      <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      <defaultValue xmi:type="uml:OpaqueExpression" xmi:id="_copyDefault"><language>`+lang+`</language><body>portionOfLife + 1</body></defaultValue>
    </ownedParameter>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="_portion" name="portionOfLife">
      <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      <defaultValue xmi:type="uml:LiteralReal" xmi:id="_portionDefault" value="2.0"/>
    </ownedParameter>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="_amountCopy" name="amountCopy">
      <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      <defaultValue xmi:type="uml:OpaqueExpression" xmi:id="_amountDefault"><body>'self'.amount</body></defaultValue>
    </ownedParameter>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="_qualified" name="qualified">
      <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      <defaultValue xmi:type="uml:OpaqueExpression" xmi:id="_qualifiedDefault"><body>Sequence::portionOfLife</body></defaultValue>
    </ownedParameter>
    <ownedAttribute xmi:type="uml:Property" xmi:id="_self" name="self" type="_block"/>
    <lifeline xmi:type="uml:Lifeline" xmi:id="_line" represents="_self"/>
  </ownedBehavior>
</packagedElement>`, `<sysml:Block xmi:id="_st" base_Class="_block"/>`)
			wantLine(t, r.Notation, "attribute copy : ScalarValues::Real default = portionOfLife2 + 1;")
			wantLine(t, r.Notation, "attribute portionOfLife2 : ScalarValues::Real default = 2.0;")
			wantLine(t, r.Notation, "attribute amountCopy : ScalarValues::Real default = self2.amount;")
			wantLine(t, r.Notation, "attribute qualified : ScalarValues::Real default = Sequence::portionOfLife2;")
			if strings.Contains(string(r.Notation), "default value not migrated") {
				t.Errorf("a reachable default was lost:\n%s", r.Notation)
			}
			for _, d := range errors(t, "renamed-sequence.sysml", r.Notation) {
				t.Errorf("%v", d)
			}
		})
	}
}

func TestSequenceRenamedParameterMessageArgument(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/interaction_use.xmi")
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte(`xmi:id="_suRpm" name="rpm"`), []byte(`xmi:id="_suRpm" name="portionOfLife"`), 1)
	data = bytes.Replace(data, []byte(`<body>rpm</body>`), []byte(`<body>portionOfLife</body>`), 1)
	r, err := migrate.Migrate("interaction_use.xmi", data)
	if err != nil {
		t.Fatal(err)
	}
	wantLine(t, r.Notation, "perform action spinCall ::> Motor::spin { in rpm[1] = portionOfLife2; }")
	wantLine(t, r.Notation, "occurrence warm : 'Spin Up' { attribute :>> portionOfLife2 = 30.0; }")
	for _, d := range errors(t, "renamed-argument.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}
