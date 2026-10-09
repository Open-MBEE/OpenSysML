package migrate_test

import "testing"

func TestPropertyCommonPartsAreWrittenOnTheUsage(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_parent" name="Parent">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_base" name="base">`+realHref+`</ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_subset" name="subsetTarget">`+realHref+`</ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_child" name="Child">
      <generalization xmi:type="uml:Generalization" xmi:id="_general" general="_parent"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_derived" name="derived" redefinedProperty="_base" subsettedProperty="_subset">
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_lower" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_upper" value="*"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_default" value="2.5"/>
        `+realHref+`
      </ownedAttribute>
    </packagedElement>`, "")
	wantClean(t, "property_common.sysml", r)
	for _, part := range []string{
		"attribute 'derived' : ScalarValues::Real",
		":>> base",
		":> subsetTarget",
		"default = 2.5",
		"[0..*]",
	} {
		wantLine(t, r.Notation, part)
	}
}

func TestUntypedPropertiesAreReferencesOutsideConstraintDefinitions(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_owner" name="Owner">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_plain" name="plain"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_constraint" name="Constraint">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_parameter" name="parameter"/>
    </packagedElement>`,
		`<sysml:ConstraintBlock xmi:id="_constraintStereotype" base_Class="_constraint"/>`)
	wantClean(t, "property_untyped.sysml", r)
	wantLine(t, r.Notation, "ref plain;")
	wantLine(t, r.Notation, "in attribute parameter[1];")
}
