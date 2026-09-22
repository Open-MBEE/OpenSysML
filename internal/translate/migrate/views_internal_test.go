package migrate

import (
	"strings"
	"testing"
)

// viewModel wraps members beside the block Pump (_pump) in package Sys (_sys),
// the «Stakeholder» Operator (_op) and the «Viewpoint» Ops (_vp); further
// stereotype applications follow the model.
func viewModel(members, stereotypes string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="Model">
    <packagedElement xmi:type="uml:Package" xmi:id="_sys" name="Sys">
      <packagedElement xmi:type="uml:Class" xmi:id="_pump" name="Pump"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_op" name="Operator"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_vp" name="Ops"/>
    ` + members + `
  </uml:Model>
  <sysml:Block xmi:id="_sb" base_Class="_pump"/>
  <sysml:Stakeholder xmi:id="_ss" base_Classifier="_op"/>
  <sysml:Viewpoint xmi:id="_sv" base_Class="_vp" stakeholder="_op" purpose="Run the pump."/>
  ` + stereotypes + `
</xmi:XMI>`
}

func TestViewForms(t *testing.T) {
	for _, tc := range []struct {
		name, members, stereotypes string
		want                       []string
		id                         string
		verdict                    Verdict
	}{
		{"a viewpoint carries its purpose and stakeholders",
			``, ``,
			[]string{"viewpoint Ops {\n    doc /* Run the pump. */\n    subject;\n    stakeholder operator : Operator;\n}"}, "_vp", Mapped},
		{"a concern tag frames a concern",
			`<packagedElement xmi:type="uml:Class" xmi:id="_vp2" name="Safety"/>`,
			`<sysml:Viewpoint xmi:id="_sv2" base_Class="_vp2" concern="Is it safe?"/>`,
			[]string{"viewpoint Safety {\n    frame concern {\n        doc /* Is it safe? */\n    }\n}"}, "_vp2", Mapped},
		{"a view exposes a package with its contents and an element by itself",
			`<packagedElement xmi:type="uml:Class" xmi:id="_v" name="Overview"/>
			 <packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_v" supplier="_sys _pump"/>`,
			`<sysml:View xmi:id="_s1" base_Class="_v"/><sysml:Expose xmi:id="_s2" base_Dependency="_d"/>`,
			[]string{"view Overview {\n    expose Sys::**;\n    expose Sys::Pump;\n}"}, "_v", Mapped},
		{"a view conforms to a viewpoint by generalization",
			`<packagedElement xmi:type="uml:Class" xmi:id="_v" name="Overview">
			   <generalization xmi:type="uml:Generalization" xmi:id="_g" general="_vp"/>
			 </packagedElement>`,
			`<sysml:View xmi:id="_s1" base_Class="_v"/><sysml:Conform xmi:id="_s2" base_Generalization="_g"/>`,
			[]string{"view Overview {\n    satisfy Ops;\n}"}, "_g", Mapped},
		{"a view conforms to a viewpoint by dependency and by tag, once",
			`<packagedElement xmi:type="uml:Class" xmi:id="_v" name="Overview"/>
			 <packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_v" supplier="_vp"/>`,
			`<sysml:View xmi:id="_s1" base_Class="_v" viewpoint="_vp"/><sysml:Conform xmi:id="_s2" base_Dependency="_d"/>`,
			[]string{"view Overview {\n    satisfy Ops;\n}"}, "_d", Mapped},
		{"a view whose tag names an absent viewpoint is approximated",
			`<packagedElement xmi:type="uml:Class" xmi:id="_v" name="Overview"/>`,
			`<sysml:View xmi:id="_s1" base_Class="_v" viewpoint="_gone"/>`,
			[]string{"view Overview;"}, "_v", Approximated},
		{"a view property typed by a view subsets it",
			`<packagedElement xmi:type="uml:Class" xmi:id="_v" name="Overview">
			   <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="detail" type="_v2" aggregation="composite"/>
			 </packagedElement>
			 <packagedElement xmi:type="uml:Class" xmi:id="_v2" name="Detail"/>`,
			`<sysml:View xmi:id="_s1" base_Class="_v"/><sysml:View xmi:id="_s2" base_Class="_v2"/>`,
			[]string{"view Overview {\n    view detail :> Detail;\n}"}, "_p", Mapped},
		{"a view nested in a block is subset by its siblings alone",
			`<packagedElement xmi:type="uml:Class" xmi:id="_v" name="Overview">
			   <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="detail" type="_v2" aggregation="composite"/>
			 </packagedElement>
			 <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Book">
			   <nestedClassifier xmi:type="uml:Class" xmi:id="_v2" name="Detail"/>
			   <ownedAttribute xmi:type="uml:Property" xmi:id="_p2" name="own" type="_v2" aggregation="composite"/>
			 </packagedElement>`,
			`<sysml:View xmi:id="_s1" base_Class="_v"/><sysml:View xmi:id="_s2" base_Class="_v2"/><sysml:Block xmi:id="_s3" base_Class="_b"/>`,
			[]string{"view Overview {\n    view detail;\n}", "view own :> Detail;"}, "_p", Approximated},
		{"a viewpoint nested in a block cannot be satisfied from outside it",
			`<packagedElement xmi:type="uml:Class" xmi:id="_v" name="Overview">
			   <generalization xmi:type="uml:Generalization" xmi:id="_g" general="_vp2"/>
			 </packagedElement>
			 <packagedElement xmi:type="uml:Class" xmi:id="_b" name="Book">
			   <nestedClassifier xmi:type="uml:Class" xmi:id="_vp2" name="Inner"/>
			 </packagedElement>`,
			`<sysml:View xmi:id="_s1" base_Class="_v"/><sysml:Viewpoint xmi:id="_s2" base_Class="_vp2"/><sysml:Block xmi:id="_s3" base_Class="_b"/><sysml:Conform xmi:id="_s4" base_Generalization="_g"/>`,
			[]string{"the viewpoint Book::Inner is a feature of the part def Book, which only its members can name"}, "_g", Unmapped},
		{"a view nested in a package-level view is reached by a feature chain",
			`<packagedElement xmi:type="uml:Class" xmi:id="_v" name="Overview">
			   <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="detail" type="_v3" aggregation="composite"/>
			 </packagedElement>
			 <packagedElement xmi:type="uml:Class" xmi:id="_v2" name="Outer">
			   <nestedClassifier xmi:type="uml:Class" xmi:id="_v3" name="Inner"/>
			 </packagedElement>`,
			`<sysml:View xmi:id="_s1" base_Class="_v"/><sysml:View xmi:id="_s2" base_Class="_v2"/><sysml:View xmi:id="_s3" base_Class="_v3"/>`,
			[]string{"view detail :> Outer.Inner;"}, "_p", Mapped},
		{"an expose of a diagram exposes the diagram's view",
			`<packagedElement xmi:type="uml:Class" xmi:id="_v" name="Overview"/>
			 <packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_v" supplier="_bdd"/>
			 <xmi:Extension extender="Tool"><ownedDiagram xmi:type="uml:Diagram" xmi:id="_bdd" name="Pump BDD" ownerOfDiagram="_sys"/></xmi:Extension>`,
			`<sysml:View xmi:id="_s1" base_Class="_v"/><sysml:Expose xmi:id="_s2" base_Dependency="_d"/>`,
			[]string{"view Overview {\n    expose Sys::'Pump BDD';\n}", "view 'Pump BDD' {\n        render Views::asTextualNotation;\n    }"}, "_d", Mapped},
		{"an expose whose supplier is an href to a diagram exposes the diagram's view",
			`<packagedElement xmi:type="uml:Class" xmi:id="_v" name="Overview"/>
			 <packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_v"><supplier href="#_bdd"/></packagedElement>
			 <xmi:Extension extender="Tool"><ownedDiagram xmi:type="uml:Diagram" xmi:id="_bdd" name="Pump BDD" ownerOfDiagram="_sys"/></xmi:Extension>`,
			`<sysml:View xmi:id="_s1" base_Class="_v"/><sysml:Expose xmi:id="_s2" base_Dependency="_d"/>`,
			[]string{"view Overview {\n    expose Sys::'Pump BDD';\n}"}, "_d", Mapped},
		{"an expose of a diagram held by an unwritten owner names the view where it is written",
			`<packagedElement xmi:type="uml:Class" xmi:id="_v" name="Overview"/>
			 <packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_v" supplier="_bdd"/>
			 <xmi:Extension extender="Tool"><ownedDiagram xmi:type="uml:Diagram" xmi:id="_bdd" name="Pump BDD"/></xmi:Extension>`,
			`<sysml:View xmi:id="_s1" base_Class="_v"/><sysml:Expose xmi:id="_s2" base_Dependency="_d"/>`,
			[]string{"view Overview {\n    expose 'Pump BDD';\n}", "view 'Pump BDD' {\n    render Views::asTextualNotation;\n}"}, "_d", Mapped},
		{"a diagram named like a view of its package is written beside it under another name",
			`<packagedElement xmi:type="uml:Class" xmi:id="_v" name="Overview"/>
			 <packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_v" supplier="_ov"/>
			 <xmi:Extension extender="Tool"><ownedDiagram xmi:type="uml:Diagram" xmi:id="_ov" name="Overview" ownerOfDiagram="_m"/></xmi:Extension>`,
			`<sysml:View xmi:id="_s1" base_Class="_v"/><sysml:Expose xmi:id="_s2" base_Dependency="_d"/>`,
			[]string{"view Overview {\n    expose 'Overview 2';\n}", "view 'Overview 2' {\n    render Views::asTextualNotation;\n}"}, "_ov", Approximated},
		{"an expose from a block is refused",
			`<packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_pump" supplier="_sys"/>`,
			`<sysml:Expose xmi:id="_s2" base_Dependency="_d"/>`,
			[]string{"the client Sys::Pump is not a view"}, "_d", Unmapped},
		{"a conform to a block is refused",
			`<packagedElement xmi:type="uml:Class" xmi:id="_v" name="Overview"/>
			 <packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_v" supplier="_pump"/>`,
			`<sysml:View xmi:id="_s1" base_Class="_v"/><sysml:Conform xmi:id="_s2" base_Dependency="_d"/>`,
			[]string{"Sys::Pump becomes a part def, which a view cannot satisfy"}, "_d", Unmapped},
		{"a view package holds its members",
			`<packagedElement xmi:type="uml:Package" xmi:id="_v" name="Handbook">
			   <packagedElement xmi:type="uml:Class" xmi:id="_ch" name="Chapter"/>
			 </packagedElement>`,
			`<sysml:View xmi:id="_s1" base_Package="_v"/>`,
			[]string{"view Handbook {\n    part def Chapter;\n}"}, "_v", Approximated},
		{"a nested view model holds its members and satisfies its viewpoint",
			`<packagedElement xmi:type="uml:Model" xmi:id="_v" name="Handbook">
			   <packagedElement xmi:type="uml:Class" xmi:id="_ch" name="Chapter"/>
			 </packagedElement>`,
			`<sysml:View xmi:id="_s1" base_Package="_v" viewpoint="_vp"/>`,
			[]string{"view Handbook {\n    satisfy Ops;\n    part def Chapter;\n}"}, "_v", Approximated},
		{"a concernList naming a block frames nothing and leaves the block mapped",
			`<packagedElement xmi:type="uml:Class" xmi:id="_vp2" name="Safety"/>`,
			`<sysml:Viewpoint xmi:id="_sv2" base_Class="_vp2" concernList="_pump"/>`,
			[]string{"part def Pump;", "viewpoint Safety;"}, "_pump", Mapped},
		{"a concernList naming a block is reported on the viewpoint",
			`<packagedElement xmi:type="uml:Class" xmi:id="_vp2" name="Safety"/>`,
			`<sysml:Viewpoint xmi:id="_sv2" base_Class="_vp2" concernList="_pump"/>`,
			[]string{"viewpoint Safety;"}, "_vp2", Approximated},
		{"an instance of a view or viewpoint has no definition to specialize",
			`<packagedElement xmi:type="uml:Class" xmi:id="_v" name="Overview"/>
			 <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_i" name="snapshot" classifier="_v _vp"/>`,
			`<sysml:View xmi:id="_s1" base_Class="_v"/>`,
			[]string{"the instance's classifier Overview is written as a view usage, which an individual cannot specialize; the instance's classifier Ops is written as a viewpoint usage, which an individual cannot specialize"}, "_i", Unmapped},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Migrate("views.xmi", []byte(viewModel(tc.members, tc.stereotypes)))
			if err != nil {
				t.Fatal(err)
			}
			got := string(r.Notation)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("notation lacks %q:\n%s", w, got)
				}
			}
			if n := strings.Count(got, "satisfy Ops;"); n > 1 {
				t.Errorf("Ops is satisfied %d times:\n%s", n, got)
			}
			var found bool
			for _, e := range r.Report.Entries {
				if e.ID != tc.id {
					continue
				}
				found = true
				if e.Verdict != tc.verdict {
					t.Errorf("verdict %s, want %s (%s)", e.Verdict, tc.verdict, e.Note)
				}
			}
			if !found {
				t.Errorf("%s is missing from the report", tc.id)
			}
			if strings.Contains(got, "individual view") || strings.Contains(got, "individual viewpoint") {
				t.Errorf("an individual specializes a usage:\n%s", got)
			}
		})
	}
}

func TestRootViewPackage(t *testing.T) {
	// A «View» package at the document root is a view like any other; a root
	// Model is written at the top level whatever it is stereotyped.
	for _, tc := range []struct {
		name, root, want string
		verdict          Verdict
	}{
		{"package", "uml:Package", "view Handbook {\n    part def Chapter;\n    view Overview {\n        expose Chapter;\n    }\n}", Approximated},
		{"model", "uml:Model", "part def Chapter;\nview Overview {\n    expose Chapter;\n}", Mapped},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML">
  <` + tc.root + ` xmi:type="` + tc.root + `" xmi:id="_v" name="Handbook">
    <packagedElement xmi:type="uml:Class" xmi:id="_ch" name="Chapter"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_ov" name="Overview"/>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_d" client="_ov" supplier="_ch"/>
  </` + tc.root + `>
  <sysml:View xmi:id="_s1" base_Package="_v"/>
  <sysml:Block xmi:id="_s2" base_Class="_ch"/>
  <sysml:View xmi:id="_s3" base_Class="_ov"/>
  <sysml:Expose xmi:id="_s4" base_Dependency="_d"/>
</xmi:XMI>`
			r, err := Migrate("root.xmi", []byte(src))
			if err != nil {
				t.Fatal(err)
			}
			if got := string(r.Notation); !strings.Contains(got, tc.want) {
				t.Errorf("notation lacks %q:\n%s", tc.want, got)
			}
			for _, e := range r.Report.Entries {
				if e.ID == "_v" && e.Verdict != tc.verdict {
					t.Errorf("verdict %s, want %s (%s)", e.Verdict, tc.verdict, e.Note)
				}
			}
		})
	}
}
