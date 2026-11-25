package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

func TestSequenceNamedArgumentsUseSourceParameterNames(t *testing.T) {
	for _, tc := range []struct {
		name, arguments, bindings string
	}{
		{"named", `<argument xmi:type="uml:LiteralReal" xmi:id="_a" name="portionOfLife" value="30.0"/>`,
			"attribute :>> portionOfLife2 = 30.0;"},
		{"mixed", `<argument xmi:type="uml:LiteralReal" xmi:id="_b" name="portionOfLife2" value="40.0"/>
<argument xmi:type="uml:LiteralReal" xmi:id="_a" value="30.0"/>`,
			"attribute :>> portionOfLife22 = 40.0; attribute :>> portionOfLife2 = 30.0;"},
		{"missing", `<argument xmi:type="uml:LiteralReal" xmi:id="_a" name="absent" value="30.0"/>`, ""},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := migrateDocument(t, `
<packagedElement xmi:type="uml:Class" xmi:id="_block" name="Block">
  <ownedBehavior xmi:type="uml:Interaction" xmi:id="_target" name="Target">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="_p" name="portionOfLife">
      <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
    </ownedParameter>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="_p2" name="portionOfLife2">
      <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
    </ownedParameter>
  </ownedBehavior>
  <ownedBehavior xmi:type="uml:Interaction" xmi:id="_caller" name="Caller">
    <fragment xmi:type="uml:InteractionUse" xmi:id="_use" name="run" refersTo="_target">`+tc.arguments+`</fragment>
  </ownedBehavior>
</packagedElement>`, `<sysml:Block xmi:id="_st" base_Class="_block"/>`)
			wantLine(t, r.Notation, "attribute portionOfLife2 : ScalarValues::Real;")
			wantLine(t, r.Notation, "attribute portionOfLife22 : ScalarValues::Real;")
			if tc.bindings != "" {
				wantLine(t, r.Notation, "occurrence run : Target { "+tc.bindings+" }")
				for _, entry := range r.Report.Entries {
					if entry.ID == "_use" && strings.Contains(entry.Note, "has no parameter") {
						t.Errorf("a source-named argument was dropped: %s", entry.Note)
					}
				}
			} else {
				wantLine(t, r.Notation, "occurrence run : Target;")
				if tc.name == "missing" {
					wantNote(t, r, "_use", migrate.Approximated, "has no parameter of Target to bind to and is dropped")
				}
			}
			for _, d := range errors(t, "named-arguments.sysml", r.Notation) {
				t.Errorf("%v", d)
			}
		})
	}
}
