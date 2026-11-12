package migrate_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

func sequenceExpressionDocument(body string) []byte {
	return document(`
<packagedElement xmi:type="uml:Class" xmi:id="_block" name="Block">
  <ownedAttribute xmi:type="uml:Property" xmi:id="_amount" name="amount">`+realHref+`</ownedAttribute>
  <ownedBehavior xmi:type="uml:Interaction" xmi:id="_seq" name="Sequence">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="_copy" name="copy">`+realHref+`
      <defaultValue xmi:type="uml:OpaqueExpression" xmi:id="_value"><body>`+body+`</body></defaultValue>
    </ownedParameter>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="_portion" name="portionOfLife">`+realHref+`</ownedParameter>
    <ownedAttribute xmi:type="uml:Property" xmi:id="_self" name="self" type="_block"/>
    <lifeline xmi:type="uml:Lifeline" xmi:id="_line" represents="_self"/>
  </ownedBehavior>
</packagedElement>`, `<sysml:Block xmi:id="_st" base_Class="_block"/>`)
}

func checkSequenceExpression(t *testing.T, data []byte, want string) {
	t.Helper()
	r, err := migrate.Migrate("sequence-expression.xmi", data)
	if err != nil {
		t.Fatal(err)
	}
	wantLine(t, r.Notation, "attribute copy : ScalarValues::Real default = "+want+";")
	if strings.Contains(string(r.Notation), "default value not migrated") {
		t.Errorf("reachable expression was lost:\n%s", r.Notation)
	}
	for _, d := range errors(t, "sequence-expression.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

func TestSequenceExpressionThroughRenamedRoot(t *testing.T) {
	for _, prefix := range []string{"", "$::"} {
		t.Run(prefix, func(t *testing.T) {
			data := sequenceExpressionDocument(prefix + "Occurrences::Block::Sequence::portionOfLife + " + prefix + "Occurrences::Block::Sequence::portionOfLife")
			data = bytes.Replace(data, []byte(`name="Model"`), []byte(`name="Occurrences"`), 1)
			checkSequenceExpression(t, data, prefix+"OccurrencesModel::Block::Sequence::portionOfLife2 + "+prefix+"OccurrencesModel::Block::Sequence::portionOfLife2")
		})
	}
}
