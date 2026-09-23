package sysmlv1

import (
	"fmt"
	"testing"
)

// A document's view tree is DocGen's: every view-typed property of a view is
// a child section in declaration order, and only a composite (or shared)
// property's view is entered for its own children; a plain reference is a
// leaf, and the exposures on the referencing property are not the view's.
func TestDocGenViewTreeFollowsAggregation(t *testing.T) {
	m, err := Parse([]byte(`<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML"
         xmlns:Document_Profile_="http://www.magicdraw.com/schemas/manual/Document_Profile.xmi">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Class" xmi:id="_doc" name="Doc">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_owned" name="owned" type="_owned" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_linked" name="linked" type="_linked">
        <ownedComment xmi:type="uml:Comment" xmi:id="_c" body="a note"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_shared" name="shared" type="_shared" aggregation="shared"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_owned" name="Owned">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_leaf1" name="leaf" type="_leaf" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_linked" name="Linked">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_leaf2" name="leaf" type="_leaf" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_shared" name="Shared">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p_leaf3" name="leaf" type="_leaf" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_leaf" name="Leaf"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_target" name="Target"/>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_dep_owned" client="_p_owned" supplier="_target"/>
    <packagedElement xmi:type="uml:Dependency" xmi:id="_dep_linked" client="_p_linked" supplier="_target"/>
  </uml:Model>
  <Document_Profile_:Document xmi:id="_st_doc" base_Class="_doc"/>
  <Document_Profile_:view xmi:id="_st_owned" base_Class="_owned"/>
  <Document_Profile_:view xmi:id="_st_linked" base_Class="_linked"/>
  <Document_Profile_:view xmi:id="_st_shared" base_Class="_shared"/>
  <Document_Profile_:view xmi:id="_st_leaf" base_Class="_leaf"/>
  <sysml:Expose xmi:id="_st_expose_owned" base_Dependency="_dep_owned"/>
  <sysml:Expose xmi:id="_st_expose_linked" base_Dependency="_dep_linked"/>
</xmi:XMI>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Documents) != 1 {
		t.Fatalf("%d documents, want 1", len(m.Documents))
	}
	root := m.Documents[0].Root
	names := func(vs []*DocGenView) []string {
		var out []string
		for _, v := range vs {
			out = append(out, v.Class.Name)
		}
		return out
	}
	if got := names(root.Children); len(got) != 3 || got[0] != "Owned" || got[1] != "Linked" || got[2] != "Shared" {
		t.Fatalf("sections = %v, want [Owned Linked Shared]", got)
	}
	owned, linked, shared := root.Children[0], root.Children[1], root.Children[2]
	if got := names(owned.Children); len(got) != 1 || got[0] != "Leaf" {
		t.Errorf("composite view's children = %v, want [Leaf]", got)
	}
	if got := names(shared.Children); len(got) != 1 || got[0] != "Leaf" {
		t.Errorf("shared view's children = %v, want [Leaf]", got)
	}
	if len(linked.Children) != 0 {
		t.Errorf("referenced view's children = %v, want none", names(linked.Children))
	}
	if len(owned.Exposed) != 1 || owned.Exposed[0].Element == nil || owned.Exposed[0].Element.ID != "_target" {
		t.Errorf("composite property's exposure = %+v, want Target", owned.Exposed)
	}
	if len(linked.Exposed) != 0 {
		t.Errorf("referencing property's exposure = %+v, want none", linked.Exposed)
	}
	for _, v := range []*DocGenView{root, owned, linked, shared} {
		if len(v.Malformed) != 0 {
			t.Errorf("%s malformed: %v", v.Class.Name, v.Malformed)
		}
	}
}

// A control flow whose source or target names no node makes the whole chain
// unreadable: the walk refuses it instead of ending cleanly where the edge is lost.
func TestDocGenChainRefusesDanglingFlows(t *testing.T) {
	const method = `<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:Document_Profile_="http://www.magicdraw.com/schemas/manual/Document_Profile.xmi">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Activity" xmi:id="_act" name="Method">
      <node xmi:type="uml:InitialNode" xmi:id="_init"/>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_collect" name="Collect"/>
      <node xmi:type="uml:StructuredActivityNode" xmi:id="_table" name="Table"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_collect"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e2" %s/>
    </packagedElement>
  </uml:Model>
  <Document_Profile_:CollectOwnedElements xmi:id="_st_c" base_Element="_collect"/>
  <Document_Profile_:TableStructure xmi:id="_st_t" base_Element="_table"/>
</xmi:XMI>`
	for _, tc := range []struct{ edge, want string }{
		{`source="_collect" target="_table"`, ""},
		{`source="_collect" target="_missing"`, `ControlFlow _e2's target "_missing" names no node`},
		{`source="_missing" target="_table"`, `ControlFlow _e2's source "_missing" names no node`},
		{`source="_collect"`, `ControlFlow _e2 has no target`},
	} {
		m, err := Parse([]byte(fmt.Sprintf(method, tc.edge)))
		if err != nil {
			t.Fatal(err)
		}
		steps, end := m.DocGenChain(m.Lookup("_act"))
		if end != tc.want {
			t.Errorf("edge %s: chain ended with %q, want %q", tc.edge, end, tc.want)
		}
		if want := 2; tc.want == "" && len(steps) != want {
			t.Errorf("edge %s: %d steps, want %d", tc.edge, len(steps), want)
		}
		if tc.want != "" && steps != nil {
			t.Errorf("edge %s: a refused chain still has steps %v", tc.edge, steps)
		}
	}
}
