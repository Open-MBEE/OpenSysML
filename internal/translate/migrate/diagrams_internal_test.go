package migrate

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/mtip"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

func TestDiagramLayoutAttributesQualifyExposedNames(t *testing.T) {
	x := exposures{names: map[string]bool{"height": true, "points": true, "text": true, "width": true}}
	for _, tc := range []struct {
		prefix, definition, name, value, want string
	}{
		{"DiagramLayout::", "Layout", "height", "40", ":>> DiagramLayout::Layout::height = 40"},
		{"DiagramLayout::", "Route", "points", "(1, 2)", ":>> DiagramLayout::Route::points = (1, 2)"},
		{"DiagramLayout::", "Canvas", "height", "80", ":>> DiagramLayout::Canvas::height = 80"},
		{"DiagramLayout::", "Note", "height", "20", ":>> DiagramLayout::Note::height = 20"},
		{"DiagramLayout::", "Style", "text", `"red"`, `:>> DiagramLayout::Style::text = "red"`},
		{"DiagramLayout::", "Picture", "width", "60", ":>> DiagramLayout::Picture::width = 60"},
		{"$::DiagramLayout::", "Layout", "scale", "60", "scale = 60"},
	} {
		if got := diagramLayoutAttribute(tc.prefix, tc.definition, tc.name, tc.value, x); got != tc.want {
			t.Errorf("diagramLayoutAttribute(%q, %q, %q) = %q, want %q", tc.definition, tc.name, tc.value, got, tc.want)
		}
	}
}

func TestExposureNameUsesFinalUnquotedSegment(t *testing.T) {
	for _, tc := range []struct {
		ref, want string
	}{
		{"Scale::height", "height"},
		{"Scale::'Height::Sample'", "Height::Sample"},
		{"Scale::'John\\'s Height'", "John's Height"},
		{"'a\\\\b'::'tab\\there'", "tab\there"},
		{"Scale::*", ""},
		{"Scale::**", ""},
	} {
		if got := exposureName(tc.ref); got != tc.want {
			t.Errorf("exposureName(%q) = %q, want %q", tc.ref, got, tc.want)
		}
	}
}

// diagramModel wraps members beside package Sys (_sys) holding block Pump
// (_pump) with attribute rate (_rate), a tool extension holding diagrams, and
// the stereotype applications applied.
func diagramModel(members, diagrams string, applied ...string) string {
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
  ` + strings.Join(applied, "\n  ") + `
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

func TestPrivateLayoutPlacementOutsideOwnerIsUnexposed(t *testing.T) {
	members := `<packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_x" name="x" visibility="private" type="_pump" aggregation="composite"/>
    </packagedElement>`
	data := diagramModel(members, diagram("_d", "Outside", "_sys", "SysML Block Definition Diagram"),
		`<sysml:Block xmi:id="_s_a" base_Class="_a"/>`)
	layout := &mtip.Export{Diagrams: []mtip.Diagram{{
		ID: "_d",
		Placements: []mtip.Placement{
			{ID: "_x", X: 10, Y: 10, Width: 20, Height: 20},
		},
		Unsupported: map[string]int{},
	}}}
	r, err := MigrateOptions("outside.xmi", []byte(data), Options{Layout: layout, LayoutSource: "outside.layout.xml"})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	if l := r.Report.Layout; l == nil || l.PlacementsWritten != 0 || l.PlacementsUnexposed != 1 {
		t.Fatalf("layout summary: %+v", l)
	}
	if strings.Contains(string(r.Notation), "metadata DiagramLayout::Layout about") {
		t.Errorf("private placement written outside its owner:\n%s", r.Notation)
	}
	if !strings.Contains(string(r.Notation), "private part x") || strings.Contains(string(r.Notation), "public part x") {
		t.Errorf("private feature visibility was not preserved:\n%s", r.Notation)
	}
	for _, entry := range r.Report.Entries {
		if entry.ID == "_d" {
			if !strings.Contains(entry.Note, "private to") || !strings.Contains(entry.Note, "no name visible in the view denotes it") {
				t.Errorf("diagram note = %q, want private-member explanation", entry.Note)
			}
			return
		}
	}
	t.Fatal("diagram report entry not found")
}

func TestPrivateLayoutPlacementInEnclosingOwnerKeepsExistingExposure(t *testing.T) {
	members := `<packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_x" name="x" visibility="private" type="_pump" aggregation="composite"/>
      <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B"/>
    </packagedElement>`
	data := diagramModel(members, diagram("_d", "Inside", "_b", "SysML Block Definition Diagram", "_x"),
		`<sysml:Block xmi:id="_s_a" base_Class="_a"/>`,
		`<sysml:Block xmi:id="_s_b" base_Class="_b"/>`)
	layout := &mtip.Export{Diagrams: []mtip.Diagram{{
		ID: "_d",
		Placements: []mtip.Placement{
			{ID: "_x", X: 10, Y: 10, Width: 20, Height: 20},
		},
		Unsupported: map[string]int{},
	}}}
	r, err := MigrateOptions("inside.xmi", []byte(data), Options{Layout: layout, LayoutSource: "inside.layout.xml"})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	if l := r.Report.Layout; l == nil || l.PlacementsWritten != 1 || l.PlacementsUnexposed != 0 {
		t.Fatalf("layout summary: %+v", l)
	}
	notation := string(r.Notation)
	if strings.Contains(notation, "private part x") ||
		!strings.Contains(notation, "metadata DiagramLayout::Layout about") {
		t.Errorf("in-scope exposure behavior changed:\n%s", r.Notation)
	}
	if strings.Contains(notation, "private alias") {
		t.Errorf("owner with no shadowed layout target wrote an alias:\n%s", notation)
	}
}

func TestPrivateLayoutAliasAvoidsImportedViewName(t *testing.T) {
	members := `<packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_x" name="x" visibility="private" type="_pump" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a_b" name="b" type="_b" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_b_x" name="x"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_taken" name="x (A)"/>`
	data := diagramModel(members, diagram("_d", "Inside", "_a", "SysML Block Definition Diagram", "_x", "_b_x", "_taken"),
		`<sysml:Block xmi:id="_s_a" base_Class="_a"/>`,
		`<sysml:Block xmi:id="_s_b" base_Class="_b"/>`)
	layout := &mtip.Export{Diagrams: []mtip.Diagram{{
		ID: "_d",
		Placements: []mtip.Placement{
			{ID: "_x", X: 10, Y: 10, Width: 20, Height: 20},
		},
		Unsupported: map[string]int{},
	}}}
	r, err := MigrateOptions("exposed_alias_name.xmi", []byte(data), Options{Layout: layout, LayoutSource: "exposed_alias_name.layout.xml"})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	notation := string(r.Notation)
	if !strings.Contains(notation, "expose 'x (A)'") {
		t.Fatalf("view did not expose the element named 'x (A)':\n%s", notation)
	}
	if !strings.Contains(notation, "private alias 'x (A) 2' for x;") {
		t.Fatalf("private layout alias did not avoid the imported view name:\n%s", notation)
	}
}

// A name the owner's body already refers to unqualified (here the type of
// part c, the top-level block 'x (A)') is not free for an alias: declared in
// the owner, the alias would capture that reference.
func TestPrivateLayoutAliasAvoidsNameTheOwnerRefersTo(t *testing.T) {
	members := `<packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_x" name="x" visibility="private" type="_pump" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a_c" name="c" type="_taken" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_b_x" name="x"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_taken" name="x (A)"/>`
	data := diagramModel(members, diagram("_d", "Inside", "_a", "SysML Block Definition Diagram", "_x", "_b_x"),
		`<sysml:Block xmi:id="_s_a" base_Class="_a"/>`,
		`<sysml:Block xmi:id="_s_b" base_Class="_b"/>`,
		`<sysml:Block xmi:id="_s_taken" base_Class="_taken"/>`)
	layout := &mtip.Export{Diagrams: []mtip.Diagram{{
		ID: "_d",
		Placements: []mtip.Placement{
			{ID: "_x", X: 10, Y: 10, Width: 20, Height: 20},
		},
		Unsupported: map[string]int{},
	}}}
	r, err := MigrateOptions("referenced_alias_name.xmi", []byte(data), Options{Layout: layout, LayoutSource: "referenced_alias_name.layout.xml"})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	notation := string(r.Notation)
	if !strings.Contains(notation, "part c : 'x (A)';") {
		t.Fatalf("part c is not typed by the top-level block by its simple name:\n%s", notation)
	}
	if !strings.Contains(notation, "private alias 'x (A) 2' for x;") || strings.Contains(notation, "private alias 'x (A)' for x;") {
		t.Fatalf("private layout alias took the name the owner's body refers to:\n%s", notation)
	}
	if !strings.Contains(notation, "metadata DiagramLayout::Layout about 'x (A) 2' {") {
		t.Fatalf("layout placement does not name the alias:\n%s", notation)
	}
}

// A view's name written bare from a nested scope is recorded against every
// scope it resolves past, like any other simple reference, so an alias planned
// in one of them later cannot capture it.
func TestViewRefRecordsTheNameItResolvesPast(t *testing.T) {
	pkg := &sysmlv1.Element{Type: "Package", Name: "P"}
	owner := &sysmlv1.Element{Type: "Class", Name: "A", Parent: pkg}
	pkg.Children = []*sysmlv1.Element{owner}
	m := &migration{referenced: map[*sysmlv1.Element]map[string]bool{}}
	v := &view{name: "x (A)", host: pkg}
	if got := m.viewRef(v, owner); got != "'x (A)'" {
		t.Fatalf("viewRef = %q, want the bare name", got)
	}
	if !m.referredPast(owner, "x (A)") || m.referredPast(pkg, "x (A)") {
		t.Fatalf("referred past A = %t, past P = %t; want true, false", m.referredPast(owner, "x (A)"), m.referredPast(pkg, "x (A)"))
	}
	top := &view{name: "Top"}
	if got := m.viewRef(top, owner); got != "Top" || !m.referredPast(owner, "Top") || !m.referredPast(pkg, "Top") {
		t.Fatalf("top-level viewRef = %q, referred past A = %t, past P = %t", got, m.referredPast(owner, "Top"), m.referredPast(pkg, "Top"))
	}
}

func TestPrivateLayoutAliasHoleIsWrittenInAncestorOwner(t *testing.T) {
	owner := &sysmlv1.Element{Type: "Class", Name: "A"}
	target := &sysmlv1.Element{Type: "Property", Name: "x", Parent: owner}
	m := &migration{
		w:                     &writer{},
		strict:                true,
		scope:                 owner,
		privateAliasOwners:    map[*sysmlv1.Element]bool{owner: true},
		privateAliasHoles:     map[*sysmlv1.Element]bool{},
		privateAliasesByOwner: map[*sysmlv1.Element][]*privateLayoutAlias{owner: {{owner: owner, target: target, name: "x (A)"}}},
	}
	m.w.block("part def A", func() {
		m.w.block("part def B", func() {
			m.w.line("view Nested;")
		})
		m.views(owner)
	})
	m.w.fill()
	notation := m.w.String()
	viewAt := strings.Index(notation, "view Nested;")
	aliasAt := strings.Index(notation, "private alias 'x (A)' for x;")
	if viewAt < 0 || aliasAt < viewAt {
		t.Fatalf("ancestor owner alias was not deferred until after nested views:\n%s", notation)
	}
}

// machineMembers is a state machine Modes (_sm) whose region enters Idle
// (_idle) and leaves it for Run (_run) on signal Go.
const machineMembers = `<packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go"/>
    <packagedElement xmi:type="uml:StateMachine" xmi:id="_sm" name="Modes">
      <region xmi:type="uml:Region" xmi:id="_r">
        <subvertex xmi:type="uml:Pseudostate" xmi:id="_sm_init"/>
        <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
        <subvertex xmi:type="uml:State" xmi:id="_run" name="Run"/>
        <transition xmi:type="uml:Transition" xmi:id="_t_init" source="_sm_init" target="_idle"/>
        <transition xmi:type="uml:Transition" xmi:id="_t_go" source="_idle" target="_run">
          <trigger xmi:type="uml:Trigger" xmi:id="_tr_go" event="_ev_go"/>
        </transition>
      </region>
    </packagedElement>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_ev_go" signal="_go"/>`

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
			[]string{"part def Pump {\n        attribute rate;\n        view 'Pump IBD' {\n            expose rate;\n            render Views::asInterconnectionDiagram;\n        }\n    }"}, Mapped, ""},
		{"a diagram naming no owner is written where its extension is held",
			``, diagram("_d", "Loose", "", "Class Diagram", "_pump"),
			[]string{"view Loose {\n    expose Sys::Pump;\n    render Views::asTreeDiagram;\n}"}, Approximated, "the diagram names no owner; written in package Model, which holds it"},
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
			[]string{"view Modes {\n    expose Mode::on;\n    render Views::asElementTable;\n}"}, Approximated, "its owner Enumeration Mode has no v2 body; written in package Model"},
		{"a diagram of a kind the tool made up is textual",
			``, diagram("_d", "Board", "_sys", "Kanban Board"),
			[]string{"render Views::asTextualNotation;"}, Approximated, "the diagram shows nothing"},
		{"a diagram named like a member of its package is renamed past it",
			``, diagram("_d", "Pump", "_sys", "SysML Block Definition Diagram", "_pump"),
			[]string{"view 'Pump 2' {\n        expose Pump;\n        render Views::asTreeDiagram;\n    }"}, Approximated, "written as Pump 2 since a member of its owner is also named Pump"},
		{"a diagram of an activity that is an operation's method is written in the operation's body",
			`<packagedElement xmi:type="uml:Class" xmi:id="_valve" name="Valve">
			   <ownedOperation xmi:type="uml:Operation" xmi:id="_open" name="Open" method="_opening"/>
			   <ownedBehavior xmi:type="uml:Activity" xmi:id="_opening" name="Opening" specification="_open">
			     <node xmi:type="uml:InitialNode" xmi:id="_o_init"/>
			   </ownedBehavior>
			 </packagedElement>`,
			diagram("_d", "Opening", "_opening", "SysML Activity Diagram", "_opening", "_pump"),
			[]string{"action open {\n        view Opening {\n            expose open;\n            expose Sys::Pump;\n            render Views::asTextualNotation;\n        }\n    }"}, Mapped,
			"its owner Activity Valve::Opening is written as the body of action Model::Valve::open, whose method it is"},
		{"a diagram of an opaque behavior that is an operation's method is written in the operation's body",
			`<packagedElement xmi:type="uml:Class" xmi:id="_valve" name="Valve">
			   <ownedOperation xmi:type="uml:Operation" xmi:id="_shut" name="Shut" method="_shutting"/>
			   <ownedBehavior xmi:type="uml:OpaqueBehavior" xmi:id="_shutting" name="Shutting" specification="_shut">
			     <language>JavaScript</language><body>1;</body>
			   </ownedBehavior>
			 </packagedElement>`,
			diagram("_d", "Shutting", "_shutting", "SysML Activity Diagram", "_pump"),
			[]string{"action shut {\n        view Shutting {\n            expose Sys::Pump;\n            render Views::asTextualNotation;\n        }\n        rep language \"JavaScript\" /* 1;*/"}, Mapped,
			"its owner OpaqueBehavior Valve::Shutting is written as the body of action Model::Valve::shut, whose method it is"},
		{"a diagram owned by an action node is written in the body of the node's activity",
			`<packagedElement xmi:type="uml:Activity" xmi:id="_fill" name="Fill">
			   <node xmi:type="uml:InitialNode" xmi:id="_f_init"/>
			   <node xmi:type="uml:OpaqueAction" xmi:id="_pour" name="pour"><language>JavaScript</language><body>1;</body></node>
			   <edge xmi:type="uml:ControlFlow" xmi:id="_f_e" source="_f_init" target="_pour"/>
			 </packagedElement>`,
			diagram("_d", "Pouring", "_pour", "SysML Activity Diagram", "_pour", "_pump"),
			[]string{"action def Fill {\n    view Pouring : StandardViewDefinitions::ActionFlowView {\n        expose Fill;\n        expose Sys::Pump;\n        render Views::asInterconnectionDiagram;\n    }"}, Approximated,
			"its owner OpaqueAction Fill::pour has no v2 body; written in action def Model::Fill"},
		{"a diagram owned by a node of a method activity is written in the operation's body",
			`<packagedElement xmi:type="uml:Class" xmi:id="_valve" name="Valve">
			   <ownedOperation xmi:type="uml:Operation" xmi:id="_open" name="Open" method="_opening"/>
			   <ownedBehavior xmi:type="uml:Activity" xmi:id="_opening" name="Opening" specification="_open">
			     <node xmi:type="uml:InitialNode" xmi:id="_o_init"/>
			     <node xmi:type="uml:OpaqueAction" xmi:id="_turn" name="turn"><language>JavaScript</language><body>1;</body></node>
			     <edge xmi:type="uml:ControlFlow" xmi:id="_o_e" source="_o_init" target="_turn"/>
			   </ownedBehavior>
			 </packagedElement>`,
			diagram("_d", "Turning", "_turn", "SysML Activity Diagram", "_turn"),
			[]string{"action open {\n        view Turning : StandardViewDefinitions::ActionFlowView {\n            expose open;\n            render Views::asInterconnectionDiagram;\n        }"}, Approximated,
			"its owner OpaqueAction Valve::Opening::turn has no v2 body; written in action Model::Valve::open"},
		{"a member of a method behavior is exposed under the operation that holds its body",
			`<packagedElement xmi:type="uml:Class" xmi:id="_valve" name="Valve">
			   <ownedOperation xmi:type="uml:Operation" xmi:id="_open" name="Open" method="_opening"/>
			   <ownedBehavior xmi:type="uml:Activity" xmi:id="_opening" name="Opening" specification="_open">
			     <ownedAttribute xmi:type="uml:Property" xmi:id="_status" name="status"/>
			     <node xmi:type="uml:InitialNode" xmi:id="_o_init"/>
			   </ownedBehavior>
			 </packagedElement>`,
			diagram("_d", "Valves", "_sys", "SysML Block Definition Diagram", "_status"),
			[]string{"action open {\n        attribute status;", "view Valves {\n        expose Valve::open::status;\n        render Views::asTreeDiagram;\n    }"}, Mapped, ""},
		{"a diagram of a method activity named like one of its members is numbered, the member keeping its name",
			`<packagedElement xmi:type="uml:Class" xmi:id="_valve" name="Valve">
			   <ownedOperation xmi:type="uml:Operation" xmi:id="_open" name="Open" method="_opening"/>
			   <ownedBehavior xmi:type="uml:Activity" xmi:id="_opening" name="Opening" specification="_open">
			     <ownedAttribute xmi:type="uml:Property" xmi:id="_status" name="status"/>
			     <node xmi:type="uml:InitialNode" xmi:id="_o_init"/>
			     <node xmi:type="uml:OpaqueAction" xmi:id="_turn" name="turn"><language>JavaScript</language><body>1;</body></node>
			     <edge xmi:type="uml:ControlFlow" xmi:id="_o_e" source="_o_init" target="_turn"/>
			   </ownedBehavior>
			 </packagedElement>`,
			diagram("_d1", "status", "_opening", "SysML Activity Diagram", "_status") + diagram("_d", "turn", "_opening", "SysML Activity Diagram", "_turn"),
			[]string{"action open {\n        view 'status 2' {\n            expose Valve::open::status;\n            render Views::asTextualNotation;", "view 'turn 2' : StandardViewDefinitions::ActionFlowView {\n            expose open;\n            render Views::asInterconnectionDiagram;", "attribute status;", "action turn {"}, Approximated,
			"written as turn 2"},
		{"a shown association end exposes the connection def declaring it, not the end by itself",
			`<packagedElement xmi:type="uml:Class" xmi:id="_tank" name="Tank">
			   <ownedAttribute xmi:type="uml:Property" xmi:id="_t_end" name="pump" type="_pump" association="_feeds"/>
			 </packagedElement>
			 <packagedElement xmi:type="uml:Association" xmi:id="_feeds" name="Feeds" memberEnd="_t_end _a_end">
			   <ownedEnd xmi:type="uml:Property" xmi:id="_a_end" type="_pump" association="_feeds"/>
			 </packagedElement>`,
			diagram("_d", "Feeding", "_sys", "SysML Block Definition Diagram", "_a_end"),
			[]string{"connection def Feeds {\n    end pump2 : Sys::Pump;\n    end pump : Sys::Pump;\n    metadata MigrationMetadata::SynthesizedName about pump;\n}", "view Feeding {\n        expose Feeds;\n        render Views::asTreeDiagram;\n    }"}, Mapped, ""},
		{"a member named like a library package shadows it, so a shown primitive exposes the library's from the global namespace",
			`<packagedElement xmi:type="uml:Class" xmi:id="_svs" name="ScalarValues"/>
			 <packagedElement xmi:type="uml:Class" xmi:id="_tank" name="Tank">
			   <ownedAttribute xmi:type="uml:Property" xmi:id="_level" name="level">
			     <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
			   </ownedAttribute>
			 </packagedElement>`,
			diagram("_d", "Levels", "_tank", "SysML Block Definition Diagram", "_level",
				"http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"),
			[]string{"occurrence def ScalarValues;", "attribute level : $::ScalarValues::Real;", "view Levels {\n        expose level;\n        expose $::ScalarValues::Real;\n        render Views::asTreeDiagram;\n    }"}, Mapped, ""},
		{"a diagram of a user stereotype is written in its metadata def",
			`<packagedElement xmi:type="uml:Profile" xmi:id="_marks" name="Marks">
			   <packagedElement xmi:type="uml:Stereotype" xmi:id="_review" name="Review">
			     <ownedAttribute xmi:type="uml:Property" xmi:id="_status" name="status"/>
			   </packagedElement>
			 </packagedElement>`,
			diagram("_d", "Reviews", "_review", "Profile Diagram", "_status", "_pump"),
			[]string{"metadata def Review {\n        attribute status;\n        view Reviews {\n            expose status;\n            expose Sys::Pump;\n            render Views::asTreeDiagram;\n        }\n    }"}, Mapped, ""},
		{"a diagram without a representation exposes nothing, whatever other tool content lists",
			``, `<ownedDiagram xmi:type="uml:Diagram" xmi:id="_d" name="Bare" ownerOfDiagram="_sys"><xmi:Extension>
			   <legend type="_pump"><usedElements>_rate</usedElements></legend><history><usedObjects href="#_pump"/></history>
			 </xmi:Extension></ownedDiagram>`,
			[]string{"view Bare {\n        render Views::asTextualNotation;\n    }"}, Approximated,
			"no diagram representation is serialized: what the diagram is and shows is unknown, and the view exposes nothing"},
		{"a diagram named like an earlier diagram of its owner is numbered",
			``, diagram("_d1", "Overview", "_sys", "SysML Package Diagram") + diagram("_d", "Overview", "_sys", "SysML Package Diagram"),
			[]string{"view Overview {", "view 'Overview 2' {"}, Approximated, "written as Overview 2"},
		{"a state machine diagram exposes the state def whose graph it draws, its states drawn by the graph",
			machineMembers,
			diagram("_d", "Modes", "_sm", "SysML State Machine Diagram", "_idle", "_run", "_t_go", "_sm_init"),
			[]string{"state def Modes {\n    view Modes : StandardViewDefinitions::StateTransitionView {\n        expose Model::Modes;\n        render Views::asInterconnectionDiagram;\n    }\n    entry; then Idle;\n    state Idle;\n    state Run;\n    transition 'Idle accept Go then Run' first Idle accept Go then Run;\n    metadata MigrationMetadata::SynthesizedName about 'Idle accept Go then Run';\n}"},
			Mapped, "the view exposes state def Model::Modes, whose graph the rendering draws with the 4 shown nodes and edges of it"},
		{"a diagram of a composite state is drawn as the graph of the state def its machine is written as",
			`<packagedElement xmi:type="uml:StateMachine" xmi:id="_sm" name="Modes">
			   <region xmi:type="uml:Region" xmi:id="_r">
			     <subvertex xmi:type="uml:State" xmi:id="_busy" name="Busy">
			       <region xmi:type="uml:Region" xmi:id="_r2">
			         <subvertex xmi:type="uml:State" xmi:id="_read" name="Read"/>
			       </region>
			     </subvertex>
			   </region>
			 </packagedElement>`,
			diagram("_d", "Busy", "_busy", "SysML State Machine Diagram", "_read"),
			[]string{"state def Modes {\n    view 'Busy 2' : StandardViewDefinitions::StateTransitionView {\n        expose Modes;\n        render Views::asInterconnectionDiagram;\n    }\n    /* the region has no initial pseudostate: nothing enters it */\n    state Busy parallel {\n        state region {\n            /* the region has no initial pseudostate: nothing enters it */\n            state Read;\n        }\n        metadata MigrationMetadata::SynthesizedName about region;\n    }\n}"},
			Approximated, "the view exposes state def Model::Modes, whose graph the rendering draws with the 1 shown nodes and edges of it; its owner State Modes::<Region>::Busy has no v2 body; written in state def Model::Modes"},
		{"a state shown by another diagram is exposed as the member its state def declares",
			machineMembers,
			diagram("_d", "Vertices", "_sys", "SysML Block Definition Diagram", "_idle", "_sm_init"),
			[]string{"view Vertices {\n        expose Modes::Idle;\n        render Views::asTreeDiagram;\n    }"},
			Approximated, "1 of 2 shown elements are not written and not exposed"},
		{"a diagram of a classifier behavior draws the graph of the state def the behavior is written as",
			`<packagedElement xmi:type="uml:Class" xmi:id="_ctl" name="Controller" classifierBehavior="_sm">
			   <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Modes">
			     <region xmi:type="uml:Region" xmi:id="_r">
			       <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
			     </region>
			   </ownedBehavior>
			 </packagedElement>`,
			diagram("_d", "Modes", "_sm", "SysML State Machine Diagram", "_idle"),
			[]string{"occurrence def Controller {\n    state def Modes {\n        view Modes : StandardViewDefinitions::StateTransitionView {\n            expose Controller::Modes;\n            render Views::asInterconnectionDiagram;\n        }", "exhibit state modes : Modes;"}, Mapped,
			"the view exposes state def Model::Controller::Modes, whose graph the rendering draws with the 1 shown nodes and edges of it"},
		{"an activity diagram of a package draws no graph",
			``, diagram("_d", "Flows", "_sys", "SysML Activity Diagram", "_pump"),
			[]string{"view Flows {\n        expose Pump;\n        render Views::asTextualNotation;\n    }"}, Mapped, ""},
		{"an empty activity diagram is not a graph view of its activity",
			`<packagedElement xmi:type="uml:Activity" xmi:id="_fill" name="Fill">
			   <node xmi:type="uml:InitialNode" xmi:id="_f_init"/>
			   <node xmi:type="uml:OpaqueAction" xmi:id="_pour" name="pour"><language>JavaScript</language><body>1;</body></node>
			   <edge xmi:type="uml:ControlFlow" xmi:id="_f_e" source="_f_init" target="_pour"/>
			 </packagedElement>`,
			diagram("_d", "Sketch", "_fill", "SysML Activity Diagram"),
			[]string{"action def Fill {\n    view Sketch {\n        render Views::asTextualNotation;\n    }\n    first start then pour;"}, Approximated,
			"the diagram shows nothing; the view exposes nothing"},
		{"a state machine diagram showing none of the graph is not a graph view of the state def",
			machineMembers,
			diagram("_d", "Signals", "_sm", "SysML State Machine Diagram", "_go"),
			[]string{"state def Modes {\n    view Signals {\n        expose Go;\n        render Views::asTextualNotation;\n    }"}, Mapped, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Migrate("diagrams.xmi", []byte(diagramModel(tc.members, tc.diagrams)))
			if err != nil {
				t.Fatal(err)
			}
			got := string(r.Notation)
			for _, w := range tc.want {
				if !hasBlock(got, w) {
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

func TestDiagramNotesDoNotNameOmittedVertices(t *testing.T) {
	members := `<packagedElement xmi:type="uml:StateMachine" xmi:id="_sm" name="Modes">
      <region xmi:type="uml:Region" xmi:id="_region">
        <subvertex xmi:type="uml:Pseudostate" xmi:id="_pick" name="Pick" kind="choice"/>
        <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
      </region>
    </packagedElement>`
	for _, tc := range []struct {
		name     string
		strict   bool
		anchored bool
	}{
		{name: "strict", strict: true},
		{name: "non-strict", anchored: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model, err := sysmlv1.Parse([]byte(diagramModel(
				members,
				diagram("_d", "Modes", "_sm", "SysML State Machine Diagram", "_pick"),
			)))
			if err != nil {
				t.Fatal(err)
			}
			d := &model.Diagrams[0]
			d.Drawn = true
			d.Symbols = []*sysmlv1.Symbol{
				{ID: "note", Class: "Note", Bounds: &sysmlv1.Bounds{X: 1, Y: 2, Width: 30, Height: 10}, Text: "Review"},
				{ID: "anchor", Class: "NoteAnchor", Ends: [2]string{"note", "pick"}},
				{ID: "pick", Class: "State", ElementID: "_pick"},
			}
			r := FromModelOptions("diagrams.xmi", model, Options{Strict: tc.strict})
			got := string(r.Notation)
			if tc.strict {
				// A strict migration writes no layout, DiagramLayout being OpenSysML's.
				if strings.Contains(got, "DiagramLayout") {
					t.Errorf("strict notation names DiagramLayout:\n%s", got)
				}
				return
			}
			anchored := strings.Contains(got, "metadata DiagramLayout::Note about")
			if anchored != tc.anchored {
				t.Errorf("note anchored = %t, want %t:\n%s", anchored, tc.anchored, got)
			}
			if !tc.anchored && !strings.Contains(got, "@DiagramLayout::Note {") {
				t.Errorf("note is not retained free of the omitted vertex:\n%s", got)
			}
		})
	}
}

// TestSimulationConfigurationDiagram covers a diagram whose owner is a run
// configuration, written as an action def with its settings and target.
func TestSimulationConfigurationDiagram(t *testing.T) {
	r, err := Migrate("diagrams.xmi", []byte(diagramModel(
		`<packagedElement xmi:type="uml:Class" xmi:id="_cfg" name="Trial"/>
		 <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_rig" name="rig" classifier="_pump"/>`,
		diagram("_d", "Setup", "_cfg", "Simulation Configuration Diagram", "_cfg", "_pump"),
		`<SimulationProfile:SimulationConfig xmlns:SimulationProfile="http://www.magicdraw.com/schemas/SimulationProfile.xmi" xmi:id="_sc" base_Class="_cfg" executionTarget="_rig" numberOfRuns="2"/>`)))
	if err != nil {
		t.Fatal(err)
	}
	got := string(r.Notation)
	want := "action def Trial {\n    @Simulation::Configuration {\n        runs = 2;\n    }\n    part target :> rig;\n    view Setup {\n        expose Trial;\n        expose Sys::Pump;\n        render Views::asTextualNotation;\n    }\n}"
	if !hasBlock(got, want) {
		t.Errorf("notation lacks %q:\n%s", want, got)
	}
	for _, e := range r.Report.Entries {
		if e.ID == "_d" && e.Verdict != Mapped {
			t.Errorf("verdict %s (%s), want mapped", e.Verdict, e.Note)
		}
	}
}

// TestDiagramWithoutHost covers a diagram nothing written can hold: its owner
// is the modeling tool's own profile, which the migrator skips, and so is every ancestor.
func TestDiagramWithoutHost(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:diagram="http://example.org/diagram">
  <uml:Profile xmi:type="uml:Profile" xmi:id="_p" name="Customization"
               URI="http://www.magicdraw.com/spec/Customization/190/UML">
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

// TestExposeOfUnhostedDiagramFailsPerClient covers an «Expose» with two view
// clients and two suppliers, one a block and one a diagram nothing written can
// hold: the block is exposed from both views and every pair of the diagram fails.
func TestExposeOfUnhostedDiagramFailsPerClient(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML"
         xmlns:diagram="http://example.org/diagram">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="Model">
    <packagedElement xmi:type="uml:Class" xmi:id="_pump" name="Pump"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_v1" name="Overview"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_v2" name="Detail"/>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_v1 _v2" supplier="_pump _pd"/>
  </uml:Model>
  <uml:Profile xmi:type="uml:Profile" xmi:id="_p" name="Customization"
               URI="http://www.magicdraw.com/spec/Customization/190/UML">
    <packagedElement xmi:type="uml:Stereotype" xmi:id="_st" name="Marked"/>
    <xmi:Extension extender="Tool">` + diagram("_pd", "Profile Diagram", "_p", "Profile Diagram", "_st") + `</xmi:Extension>
  </uml:Profile>
  <sysml:Block xmi:id="_s0" base_Class="_pump"/>
  <sysml:View xmi:id="_s1" base_Class="_v1"/>
  <sysml:View xmi:id="_s2" base_Class="_v2"/>
  <sysml:Expose xmi:id="_s3" base_Dependency="_d"/>
</xmi:XMI>`
	r, err := Migrate("expose.xmi", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := string(r.Notation)
	for _, w := range []string{"view Overview {\n    expose Pump;\n}", "view Detail {\n    expose Pump;\n}"} {
		if !hasBlock(got, w) {
			t.Errorf("notation lacks %q:\n%s", w, got)
		}
	}
	if strings.Contains(got, "expose 'Profile Diagram'") || strings.Contains(got, "view 'Profile Diagram'") {
		t.Errorf("the unhosted diagram is exposed or written:\n%s", got)
	}
	var found bool
	for _, e := range r.Report.Entries {
		if e.ID != "_d" {
			continue
		}
		found = true
		if e.Verdict != Approximated {
			t.Errorf("verdict %s, want approximated (%s)", e.Verdict, e.Note)
		}
		for _, w := range []string{"2 of 4 relationships written", "the exposed Diagram 'Profile Diagram' is not written as a view"} {
			if !strings.Contains(e.Note, w) {
				t.Errorf("note %q lacks %q", e.Note, w)
			}
		}
	}
	if !found {
		t.Errorf("_d is missing from the report")
	}
}

func TestLayoutClauseWording(t *testing.T) {
	for _, tc := range []struct {
		written, unexposed, dangling, onEdges, total int
		want                                         string
	}{
		{1, 0, 0, 0, 1, "1 of 1 shown elements positioned"},
		{1, 1, 1, 0, 3, "1 of 3 shown elements positioned (1 not exposed, 1 resolving to no element)"},
		{0, 2, 0, 0, 2, "0 of 2 shown elements positioned (2 not exposed)"},
		{2, 0, 0, 2, 4, "2 of 4 shown elements positioned (2 lying on connections)"},
	} {
		got := layoutClause(tc.written, tc.unexposed, tc.dangling, tc.onEdges, tc.total)
		if got != tc.want {
			t.Errorf("layoutClause = %q, want %q", got, tc.want)
		}
	}
}

func TestRouteClauseWording(t *testing.T) {
	for _, tc := range []struct {
		reasons map[string]int
		total   int
		want    string
	}{
		{map[string]int{routeWritten: 1}, 1, "1 of 1 connectors routed"},
		{map[string]int{routeWritten: 1, routeNotWritten: 1, routeDangling: 1}, 3, "1 of 3 connectors routed (1 not written, 1 resolving to no element)"},
		{map[string]int{routeNotDrawn: 2, routeNoMember: 1, routeNotExposed: 1}, 4, "0 of 4 connectors routed (2 not drawn, 1 not exposed, 1 no v2 member)"},
	} {
		got := routeClause(tc.reasons, tc.total)
		if got != tc.want {
			t.Errorf("routeClause(%v) = %q, want %q", tc.reasons, got, tc.want)
		}
	}
}

func TestExposuresHideAllButSimplyExposedNames(t *testing.T) {
	x := exposures{names: map[string]bool{}, by: map[string][]string{}}
	for _, ref := range []string{"GUI", "Sub::GUI", "Panel", "Sub::panel", "'Control Panel'", "Scale::'John\\'s Height'"} {
		name := exposureName(ref)
		x.names[name] = true
		x.by[name] = append(x.by[name], ref)
	}
	want := columnNames{"GUI": true, "panel": true, "John's Height": true}
	if got := x.hides(); !reflect.DeepEqual(got, want) {
		t.Errorf("hides() = %v, want %v", got, want)
	}
}

func TestExposureNameReadsBackWrittenNames(t *testing.T) {
	for _, name := range []string{"GUI", "Control Panel", "John's Height", `back\slash`, "line\nbreak", "if"} {
		ref := "Owner::" + writeName(name)
		if got := exposureName(ref); got != name {
			t.Errorf("exposureName(%s) = %q, want %q", ref, got, name)
		}
	}
}

func TestExposuresHideEscapedNamesExposedTwice(t *testing.T) {
	x := exposures{names: map[string]bool{}, by: map[string][]string{}}
	for _, ref := range []string{writeName("John's Height"), "Box::" + writeName("John's Height"), writeName("Bob's Width")} {
		name := exposureName(ref)
		x.names[name] = true
		x.by[name] = append(x.by[name], ref)
	}
	want := columnNames{"John's Height": true}
	if got := x.hides(); !reflect.DeepEqual(got, want) {
		t.Errorf("hides() = %v, want %v", got, want)
	}
}
