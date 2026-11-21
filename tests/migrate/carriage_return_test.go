package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// KerML §8.2.2.3 Table 4 has no escape for U+000D, so a v1 string or name holding one is
// written with the character itself: `\r` is a pilot extension a strict check rejects.
func TestCarriageReturnIsWrittenWithoutANonstandardEscape(t *testing.T) {
	const members = `
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Motor">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_label" name="la&#13;bel">
        <defaultValue xmi:type="uml:LiteralString" xmi:id="_lv" value="on&#13;off&#10;'&quot;\"/>
      </ownedAttribute>
    </packagedElement>`
	for _, tc := range []struct {
		name string
		opts migrate.Options
		mode diag.ConformanceMode
	}{
		{"default", migrate.Options{}, diag.ConformanceDefault},
		{"strict", migrate.Options{Strict: true}, diag.ConformanceStrict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := migrateDocumentOptions(t, members, `<sysml:Block xmi:id="_st" base_Class="_b"/>`, tc.opts)
			wantLine(t, r.Notation, "attribute 'la\rbel' default = \"on\roff\\n'\\\"\\\\\";")
			if strings.Contains(string(r.Notation), `\r`) {
				t.Errorf("the notation spells the escape `\\r`, which Table 4 does not list:\n%q", r.Notation)
			}
			ws := model.NewWorkspace()
			ws.SetConformanceMode(tc.mode)
			ws.Open("t.sysml", r.Notation, 1)
			for _, d := range ws.Diagnostics("t.sysml") {
				t.Errorf("%v", d)
			}
		})
	}
}

// A line comment ends at a carriage return, so a strict migration's comment naming
// a made-up name holding one spells it `\r` there, where no escape is read.
func TestCarriageReturnDoesNotEndAStrictMigrationComment(t *testing.T) {
	v1 := strings.ReplaceAll(actorUseCaseModel, `name="Operator"`, `name="Oper&#13;ator"`)
	if v1 == actorUseCaseModel {
		t.Fatal("the model names no Operator")
	}
	r := migrateDocumentOptions(t, v1, "", migrate.Options{Strict: true})
	wantLine(t, r.Notation, "actor 'oper\rator' :> 'Oper\rator';\n        // names the migration made up: 'oper\\rator'\n")
	wantLine(t, r.Notation, "// names the migration made up: 'Oper\\rator to Control Instrument'\n")
	ws := model.NewWorkspace()
	ws.SetConformanceMode(diag.ConformanceStrict)
	ws.Open("t.sysml", r.Notation, 1)
	for _, d := range ws.Diagnostics("t.sysml") {
		t.Errorf("%v", d)
	}
}
