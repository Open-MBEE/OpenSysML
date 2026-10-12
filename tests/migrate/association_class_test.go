package migrate_test

import "testing"

func TestAssociationClassIsConnectionDefinitionWithFeatures(t *testing.T) {
	r := migrateDocument(t, `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    <packagedElement xmi:type="uml:DataType" xmi:id="_key" name="Key"/>
    <packagedElement xmi:type="uml:AssociationClass" xmi:id="_pair" name="Pair" memberEnd="_aEnd _bEnd">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_tag" name="tag" type="_key"/>
      <ownedEnd xmi:type="uml:Property" xmi:id="_aEnd" name="a" type="_a" association="_pair"/>
      <ownedEnd xmi:type="uml:Property" xmi:id="_bEnd" name="b" type="_b" association="_pair"/>
    </packagedElement>`, "")
	wantClean(t, "association_class.sysml", r)
	for _, line := range []string{
		"connection def Pair {",
		"end a : A;",
		"end b : B;",
		"attribute tag : Key;",
	} {
		wantLine(t, r.Notation, line)
	}
}
