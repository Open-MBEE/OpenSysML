package migrate

import (
	"strings"
	"testing"
)

// A diagram held by an extension at the document root, naming no owner, is
// written at the top level beside the root packages, not inside the first one.
func TestRootDiagramBesideRootPackages(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:diagram="http://example.org/diagram">
  <uml:Package xmi:type="uml:Package" xmi:id="_a" name="A">
    <packagedElement xmi:type="uml:Class" xmi:id="_pump" name="Pump"/>
  </uml:Package>
  <uml:Package xmi:type="uml:Package" xmi:id="_b" name="B">
    <packagedElement xmi:type="uml:Class" xmi:id="_valve" name="Valve"/>
  </uml:Package>
  <xmi:Extension extender="Tool">` + diagram("_d", "Overview", "", "Class Diagram", "_pump", "_valve") + `</xmi:Extension>
</xmi:XMI>`
	r, err := Migrate("roots.xmi", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out := string(r.Notation)
	want := "view Overview {\n    expose A::Pump;\n    expose B::Valve;\n    render Views::asTreeDiagram;\n}"
	if !strings.Contains(out, want) {
		t.Errorf("output lacks a top-level view:\n%s", out)
	}
	if strings.Contains(out, "    view Overview") {
		t.Errorf("view written inside a root package:\n%s", out)
	}
	var found int
	for _, e := range r.Report.Entries {
		if e.ID != "_d" {
			continue
		}
		found++
		if e.Verdict != Approximated || e.Target != "Overview" || !strings.Contains(e.Note, "written at the top level") {
			t.Errorf("entry = %+v", e)
		}
	}
	if found != 1 {
		t.Errorf("_d is reported %d times, want once", found)
	}
}
