package migrate

import (
	"strings"
	"testing"
)

// diagramModel wraps members beside package Sys (_sys) holding block Pump
// (_pump) with attribute rate (_rate), and a tool extension holding diagrams.
func diagramModel(members, diagrams string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML"
         xmlns:diagram="http://example.org/diagram">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="Model">
    <packagedElement xmi:type="uml:Package" xmi:id="_sys" name="Sys">
      <packagedElement xmi:type="uml:Class" xmi:id="_pump" name="Pump">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_rate" name="rate"/>
      </packagedElement>
    </packagedElement>
    ` + members + `
    <xmi:Extension extender="Tool">` + diagrams + `</xmi:Extension>
  </uml:Model>
  <sysml:Block xmi:id="_sb" base_Class="_pump"/>
</xmi:XMI>`
}

// diagram serializes a diagram of kind showing the ids listed.
func diagram(id, name, owner, kind string, shown ...string) string {
	var b strings.Builder
	b.WriteString(`<ownedDiagram xmi:type="uml:Diagram" xmi:id="` + id + `" name="` + name + `"`)
	if owner != "" {
		b.WriteString(` ownerOfDiagram="` + owner + `"`)
	}
	b.WriteString(`><xmi:Extension><diagramRepresentation><diagram:DiagramRepresentationObject type="` + kind + `"><diagramContents>`)
	for _, s := range shown {
		b.WriteString(`<usedElements>` + s + `</usedElements>`)
	}
	b.WriteString(`</diagramContents></diagram:DiagramRepresentationObject></diagramRepresentation></xmi:Extension></ownedDiagram>`)
	return b.String()
}

func TestDiagramViews(t *testing.T) {
	for _, tc := range []struct {
		name, members, diagrams string
		want                    []string
		verdict                 Verdict
		note                    string
	}{
		{"a diagram of a package exposes the members it shows, not the package",
			``, diagram("_d", "Pumps", "_sys", "SysML Block Definition Diagram", "_pump", "_rate"),
			[]string{"view Pumps {\n        expose Pump;\n        expose Sys::Pump::rate;\n        render Views::asTreeDiagram;\n    }"}, Mapped, ""},
		{"a diagram of a block is written in its part def",
			``, diagram("_d", "Pump IBD", "_pump", "SysML Internal Block Diagram", "_rate"),
			[]string{"part def Pump {\n        ref rate;\n        view 'Pump IBD' {\n            expose rate;\n            render Views::asInterconnectionDiagram;\n        }\n    }"}, Mapped, ""},
		{"a diagram naming no owner is written where its extension is held",
			``, diagram("_d", "Loose", "", "Class Diagram", "_pump"),
			[]string{"view Loose {\n    expose Sys::Pump;\n    render Views::asTreeDiagram;\n}"}, Approximated, "the diagram names no owner; written at the top level"},
		{"a diagram naming an unknown owner is written where its extension is held",
			``, diagram("_d", "Lost", "_gone", "Generic Table", "_pump"),
			[]string{"view Lost {\n    expose Sys::Pump;\n    render Views::asElementTable;\n}"}, Approximated, "ownerOfDiagram _gone resolves to no element"},
		{"shown ids that resolve to nothing are counted, not exposed",
			``, diagram("_d", "Dangling", "_sys", "SysML Package Diagram", "_pump", "_nope", "_neither"),
			[]string{"view Dangling {\n        expose Pump;\n        render Views::asTreeDiagram;\n    }"}, Approximated, "2 of 3 shown ids resolve to no element"},
		{"a diagram showing nothing is an empty view",
			``, diagram("_d", "Empty", "_sys", "SysML Activity Diagram"),
			[]string{"view Empty {\n        render Views::asTextualNotation;\n    }"}, Approximated, "the diagram shows nothing; the view exposes nothing"},
		{"a diagram owned by an element with no body is written in the nearest ancestor that has one",
			`<packagedElement xmi:type="uml:Enumeration" xmi:id="_mode" name="Mode">
			   <ownedLiteral xmi:type="uml:EnumerationLiteral" xmi:id="_on" name="on"/>
			 </packagedElement>`,
			diagram("_d", "Modes", "_mode", "Generic Table", "_on"),
			[]string{"view Modes {\n    expose Mode::on;\n    render Views::asElementTable;\n}"}, Approximated, "its owner Enumeration Mode has no v2 body; written at the top level"},
		{"a diagram of a kind the tool made up is textual",
			``, diagram("_d", "Board", "_sys", "Kanban Board"),
			[]string{"render Views::asTextualNotation;"}, Approximated, "the diagram shows nothing"},
		{"a diagram named like a member of its package is renamed past it",
			``, diagram("_d", "Pump", "_sys", "SysML Block Definition Diagram", "_pump"),
			[]string{"view 'Pump 2' {\n        expose Pump;\n        render Views::asTreeDiagram;\n    }"}, Approximated, "written as Pump 2 since a member of its owner is also named Pump"},
		{"a diagram named like an earlier diagram of its owner is numbered",
			``, diagram("_d1", "Overview", "_sys", "SysML Package Diagram") + diagram("_d", "Overview", "_sys", "SysML Package Diagram"),
			[]string{"view Overview {", "view 'Overview 2' {"}, Approximated, "written as Overview 2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Migrate("diagrams.xmi", []byte(diagramModel(tc.members, tc.diagrams)))
			if err != nil {
				t.Fatal(err)
			}
			got := string(r.Notation)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("notation lacks %q:\n%s", w, got)
				}
			}
			var found int
			for _, e := range r.Report.Entries {
				if e.ID != "_d" {
					continue
				}
				found++
				if e.Verdict != tc.verdict {
					t.Errorf("verdict %s, want %s (%s)", e.Verdict, tc.verdict, e.Note)
				}
				if !strings.Contains(e.Note, tc.note) {
					t.Errorf("note %q lacks %q", e.Note, tc.note)
				}
			}
			if found != 1 {
				t.Errorf("_d is reported %d times, want once", found)
			}
		})
	}
}

// TestDiagramWithoutHost covers a diagram nothing written can hold: its owner
// is a profile, which the migrator skips, and so is every ancestor.
func TestDiagramWithoutHost(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:diagram="http://example.org/diagram">
  <uml:Profile xmi:type="uml:Profile" xmi:id="_p" name="Custom">
    <packagedElement xmi:type="uml:Stereotype" xmi:id="_st" name="Marked"/>
    <xmi:Extension extender="Tool">` + diagram("_d", "Profile Diagram", "_p", "Profile Diagram", "_st") + `</xmi:Extension>
  </uml:Profile>
</xmi:XMI>`
	r, err := Migrate("profile.xmi", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(r.Notation), "view") {
		t.Errorf("a view is written for a diagram nothing holds:\n%s", r.Notation)
	}
	var found bool
	for _, e := range r.Report.Entries {
		if e.ID != "_d" {
			continue
		}
		found = true
		if e.Verdict != Unmapped {
			t.Errorf("verdict %s, want unmapped (%s)", e.Verdict, e.Note)
		}
		if !strings.Contains(e.Note, "nor any ancestor of it is written") {
			t.Errorf("note %q does not say nothing holds it", e.Note)
		}
	}
	if !found {
		t.Errorf("_d is missing from the report")
	}
}
