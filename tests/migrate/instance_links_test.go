package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

func TestInstanceSpecificationLink(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    <packagedElement xmi:type="uml:Association" xmi:id="_association" name="AB" memberEnd="_aEnd _bEnd">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_label" name="label">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#String"/>
      </ownedAttribute>
      <ownedEnd xmi:type="uml:Property" xmi:id="_aEnd" name="a" type="_a" association="_association"/>
      <ownedEnd xmi:type="uml:Property" xmi:id="_bEnd" name="b" type="_b" association="_association"/>
    </packagedElement>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_ia" name="IA" classifier="_a"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_ib" name="IB" classifier="_b"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_link" name="L" classifier="_association">
      <slot xmi:type="uml:Slot" xmi:id="_slotA" definingFeature="_aEnd">
        <value xmi:type="uml:InstanceValue" xmi:id="_valueA" instance="_ia"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_slotB" definingFeature="_bEnd">
        <value xmi:type="uml:InstanceValue" xmi:id="_valueB" instance="_ib"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_slotLabel" definingFeature="_label">
        <value xmi:type="uml:LiteralString" xmi:id="_valueLabel" value="connected"/>
      </slot>
    </packagedElement>`, "")
	wantClean(t, "instance_links.sysml", r)
	for _, line := range []string{
		"occurrence IA : A;",
		"occurrence IB : B;",
		"connection L : AB connect IA to IB {",
		`attribute :>> label = "connected";`,
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_link", migrate.Mapped, "")
}

// A link of an association written as its member-end properties has no
// connection def to be typed by: it is an untyped connection between the
// instances, joined in the order of the association's member ends.
func TestInstanceSpecificationLinkWithoutConnectionDefinitionIsUntyped(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_aEnd" name="b" type="_b" association="_association"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bEnd" name="a" type="_a" association="_association"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Association" xmi:id="_association" memberEnd="_aEnd _bEnd"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_ia" name="IA" classifier="_a"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_ib" name="IB" classifier="_b"/>
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_link" name="L" classifier="_association">
      <slot xmi:type="uml:Slot" xmi:id="_slotA" definingFeature="_aEnd">
        <value xmi:type="uml:InstanceValue" xmi:id="_valueA" instance="_ib"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_slotB" definingFeature="_bEnd">
        <value xmi:type="uml:InstanceValue" xmi:id="_valueB" instance="_ia"/>
      </slot>
    </packagedElement>`, "")
	wantClean(t, "untyped_link.sysml", r)
	wantLine(t, r.Notation, "connection L connect IB to IA;")
	wantNote(t, r, "_link", migrate.Approximated, "the link is written as an untyped connection: its association is written as its member-end properties, so there is no connection def to type it by")
}
