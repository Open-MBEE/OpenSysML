package migrate

import (
	"strings"
	"testing"
)

// Sibling associations sharing a name are declared under the distinct names
// their references and report rows use, and an anonymous association a diagram
// shows is still written as its member-end properties.
func TestSameNamedAssociationsDeclareDistinctNames(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:diagram="http://example.org/diagram">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="Model">
    <packagedElement xmi:type="uml:Package" xmi:id="_sys" name="Sys">
      <packagedElement xmi:type="uml:Class" xmi:id="_pump" name="Pump">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_outlet" name="outlet" type="_valve" association="_loose"/>
      </packagedElement>
      <packagedElement xmi:type="uml:Class" xmi:id="_valve" name="Valve"/>
      <packagedElement xmi:type="uml:Association" xmi:id="_feeds1" name="Feeds" memberEnd="_f1a _f1b">
        <ownedEnd xmi:type="uml:Property" xmi:id="_f1a" name="source" type="_pump" association="_feeds1"/>
        <ownedEnd xmi:type="uml:Property" xmi:id="_f1b" name="sink" type="_valve" association="_feeds1"/>
      </packagedElement>
      <packagedElement xmi:type="uml:Association" xmi:id="_feeds2" name="Feeds" memberEnd="_f2a _f2b">
        <ownedEnd xmi:type="uml:Property" xmi:id="_f2a" name="source" type="_valve" association="_feeds2"/>
        <ownedEnd xmi:type="uml:Property" xmi:id="_f2b" name="sink" type="_pump" association="_feeds2"/>
      </packagedElement>
      <packagedElement xmi:type="uml:Association" xmi:id="_loose" memberEnd="_outlet _la">
        <ownedEnd xmi:type="uml:Property" xmi:id="_la" type="_pump" association="_loose"/>
      </packagedElement>
    </packagedElement>
    <xmi:Extension extender="Tool">` + diagram("_d", "Links", "", "Class Diagram", "_feeds2", "_loose") + `</xmi:Extension>
  </uml:Model>
</xmi:XMI>`
	r, err := Migrate("assoc.xmi", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out := string(r.Notation)
	for _, want := range []string{
		"connection def Feeds {",
		"connection def 'Feeds 2' {",
		"expose Sys::'Feeds 2';",
		"ref part outlet : Valve;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "connection def") != 2 {
		t.Errorf("want exactly the two named associations as connection defs:\n%s", out)
	}
	for _, e := range r.Report.Entries {
		switch e.ID {
		case "_feeds2":
			if e.Target != "Sys::'Feeds 2'" {
				t.Errorf("_feeds2 target = %q, want Sys::'Feeds 2'", e.Target)
			}
		case "_loose":
			if e.Target != "" || !strings.Contains(e.Note, "member-end properties") {
				t.Errorf("_loose entry = %+v", e)
			}
		}
	}
}
