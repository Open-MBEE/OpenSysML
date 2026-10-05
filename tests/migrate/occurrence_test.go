package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

func TestPlainClassSpecializesBlockAsOccurrenceDefinition(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_block" name="Block">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_base" name="base">`+realHref+`</ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_plain" name="Plain">
      <generalization xmi:type="uml:Generalization" xmi:id="_general" general="_block"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_derived" name="derived" redefinedProperty="_base">`+realHref+`</ownedAttribute>
    </packagedElement>`,
		`<sysml:Block xmi:id="_stereotype" base_Class="_block"/>`)
	wantClean(t, "occurrence_generalization.sysml", r)
	wantLine(t, r.Notation, "occurrence def Plain :> Block {")
	wantLine(t, r.Notation, "attribute 'derived' : ScalarValues::Real :>> base;")
	wantNote(t, r, "_plain", migrate.Mapped, "")
}

func TestBlockSpecializesPlainClassAsPartDefinition(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_plain" name="Plain"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_block" name="Block">
      <generalization xmi:type="uml:Generalization" xmi:id="_general" general="_plain"/>
    </packagedElement>`,
		`<sysml:Block xmi:id="_stereotype" base_Class="_block"/>`)
	wantClean(t, "part_generalization.sysml", r)
	wantLine(t, r.Notation, "occurrence def Plain;")
	wantLine(t, r.Notation, "part def Block :> Plain;")
	wantNote(t, r, "_block", migrate.Mapped, "")
}

func TestPortTypedByPlainClassUsesReferentialOccurrencePayload(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_plain" name="Plain"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_system" name="System">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_port" name="link" type="_plain" aggregation="composite"/>
    </packagedElement>`,
		`<sysml:Block xmi:id="_stereotype" base_Class="_system"/>`)
	wantClean(t, "port_occurrence_payload.sysml", r)
	wantLine(t, r.Notation, "occurrence def Plain;")
	wantLine(t, r.Notation, "port link {\n        ref occurrence link : Plain;")
	wantNote(t, r, "_plain", migrate.Mapped, "")
}
