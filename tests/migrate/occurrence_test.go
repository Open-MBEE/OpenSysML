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

func TestMixedPartAndOccurrenceIndividualClassifiers(t *testing.T) {
	for _, tc := range []struct {
		name         string
		classifiers  string
		generalizers string
	}{
		{name: "occurrence first", classifiers: "_plain _engine", generalizers: "Plain, Engine"},
		{name: "part first", classifiers: "_engine _plain", generalizers: "Engine, Plain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_plain" name="Plain"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_engine" name="Engine">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_x" name="x">`+integerHref+`</ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_motor" name="motor" classifier="`+tc.classifiers+`">
      <slot xmi:id="_slot" definingFeature="_x">
        <value xmi:type="uml:LiteralInteger" xmi:id="_value" value="1"/>
      </slot>
    </packagedElement>`, `<sysml:Block xmi:id="_block" base_Class="_engine"/>`)
			wantLine(t, r.Notation, "individual part def motor :> "+tc.generalizers+" {")
			wantLine(t, r.Notation, "attribute :>> x = 1;")
			wantNote(t, r, "_motor", migrate.Mapped, "")
			wantNote(t, r, "_slot", migrate.Mapped, "")
			wantClean(t, "mixed-individual.sysml", r)
		})
	}
}

func TestOccurrenceSlotHoldsPartIndividual(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_vehicle" name="Vehicle"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
      <generalization xmi:type="uml:Generalization" xmi:id="_car_general" general="_vehicle"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_garage" name="Garage">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_vehicle_usage" name="vehicle" type="_vehicle" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_my_car" name="myCar" classifier="_car"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_garage_instance" name="g" classifier="_garage">
      <slot xmi:id="_slot" definingFeature="_vehicle_usage">
        <value xmi:type="uml:InstanceValue" xmi:id="_value" instance="_my_car"/>
      </slot>
    </packagedElement>`,
		`<sysml:Block xmi:id="_car_block" base_Class="_car"/><sysml:Block xmi:id="_garage_block" base_Class="_garage"/>`)
	wantLine(t, r.Notation, "occurrence vehicle : Vehicle;")
	wantLine(t, r.Notation, "individual part def myCar :> Car;")
	wantLine(t, r.Notation, "individual occurrence :>> vehicle : myCar;")
	wantNote(t, r, "_slot", migrate.Mapped, "")
	wantNote(t, r, "_garage_instance", migrate.Mapped, "")
	wantClean(t, "occurrence-slot.sysml", r)
}

func TestOccurrenceDefaultUsesPartIndividualAsType(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_vehicle" name="Vehicle"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_car" name="Car">
      <generalization xmi:type="uml:Generalization" xmi:id="_car_general" general="_vehicle"/>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_my_car" name="myCar" classifier="_car"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_garage" name="Garage">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_vehicle_usage" name="vehicle" type="_vehicle" aggregation="composite">
        <defaultValue xmi:type="uml:InstanceValue" xmi:id="_default" instance="_my_car"/>
      </ownedAttribute>
    </packagedElement>`,
		`<sysml:Block xmi:id="_car_block" base_Class="_car"/><sysml:Block xmi:id="_garage_block" base_Class="_garage"/>`)
	wantLine(t, r.Notation, "occurrence vehicle : Vehicle, myCar;")
	wantNoLine(t, r.Notation, "default = myCar")
	wantNote(t, r, "_vehicle_usage", migrate.Approximated, "the default value, the individual myCar, is written as a type of the usage")
	wantClean(t, "occurrence-default.sysml", r)
}

func TestOccurrenceIndividualDoesNotTypePartSlot(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_vehicle" name="Vehicle"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_engine" name="Engine"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_garage" name="Garage">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_engine_usage" name="engine" type="_engine" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_plain" name="plain" classifier="_vehicle"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_garage_instance" name="g" classifier="_garage">
      <slot xmi:id="_slot" definingFeature="_engine_usage">
        <value xmi:type="uml:InstanceValue" xmi:id="_value" instance="_plain"/>
      </slot>
    </packagedElement>`,
		`<sysml:Block xmi:id="_engine_block" base_Class="_engine"/><sysml:Block xmi:id="_garage_block" base_Class="_garage"/>`)
	wantNote(t, r, "_slot", migrate.Unmapped, "is an individual occurrence def, which cannot type a part")
	wantNoLine(t, r.Notation, "individual part :>> engine")
	wantClean(t, "plain-individual-part-slot.sysml", r)
}
