package migrate

import (
	"strings"
	"testing"
)

func TestPlainClassesAndTheirPropertiesUseOccurrenceForms(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML"
         xmlns:Custom="http://www.example.org/profiles/Custom">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="Model">
    <packagedElement xmi:type="uml:Interface" xmi:id="_link" name="Link"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_plain" name="Plain"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_linked" name="Linked"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_system" name="System">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_child" name="child" type="_plain" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_peer" name="peer" type="_plain"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_connector" name="connector" type="_link"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_block" name="Block"/>
  </uml:Model>
  <sysml:Block xmi:id="_block_st" base_Class="_block"/>
  <sysml:Block xmi:id="_system_st" base_Class="_system"/>
  <Custom:HyperlinkOwner xmi:id="_hyperlink" base_Element="_linked"/>
</xmi:XMI>`

	r, err := Migrate("occurrences.xmi", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"occurrence def Plain;",
		"occurrence def Linked {",
		"part def Block;",
		"occurrence child : Plain;",
		"ref occurrence peer : Plain;",
		"port 'connector' : Link;",
	} {
		if !strings.Contains(string(r.Notation), want) {
			t.Errorf("notation lacks %q:\n%s", want, r.Notation)
		}
	}
	for id, want := range map[string]Verdict{
		"_plain": Mapped, "_linked": Mapped, "_child": Mapped, "_peer": Mapped, "_connector": Mapped,
	} {
		var found bool
		for _, entry := range r.Report.Entries {
			if entry.ID == id {
				found = true
				if entry.Verdict != want {
					t.Errorf("%s: verdict = %s, note = %q; want %s", id, entry.Verdict, entry.Note, want)
				}
				break
			}
		}
		if !found {
			t.Errorf("migration report has no entry for %s", id)
		}
	}
}
