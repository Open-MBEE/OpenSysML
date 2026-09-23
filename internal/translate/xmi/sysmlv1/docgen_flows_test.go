package sysmlv1

import (
	"fmt"
	"testing"
)

// Object flows carry data between pins, not the document's order: an object
// flow beside the control chain, even a malformed one, leaves the chain as it is.
func TestDocGenChainIgnoresObjectFlows(t *testing.T) {
	const method = `<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:Document_Profile_="http://www.magicdraw.com/schemas/manual/Document_Profile.xmi">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Activity" xmi:id="_act" name="Method">
      <node xmi:type="uml:InitialNode" xmi:id="_init"/>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_collect" name="Collect">
        <result xmi:type="uml:OutputPin" xmi:id="_collect_out"/>
      </node>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_filter" name="Filter">
        <argument xmi:type="uml:InputPin" xmi:id="_filter_in"/>
      </node>
      <node xmi:type="uml:StructuredActivityNode" xmi:id="_table" name="Table"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_collect"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_collect" target="_filter"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_filter" target="_table"/>
      %s
    </packagedElement>
  </uml:Model>
  <Document_Profile_:CollectOwnedElements xmi:id="_st_c" base_Element="_collect"/>
  <Document_Profile_:FilterByNames xmi:id="_st_f" base_Element="_filter"/>
  <Document_Profile_:TableStructure xmi:id="_st_t" base_Element="_table"/>
</xmi:XMI>`
	for _, tc := range []struct{ name, edges string }{
		{"no object flow", ``},
		{"pin to pin", `<edge xmi:type="uml:ObjectFlow" xmi:id="_o1" source="_collect_out" target="_filter_in"/>`},
		{"node to node", `<edge xmi:type="uml:ObjectFlow" xmi:id="_o1" source="_collect" target="_table"/>`},
		{"dangling", `<edge xmi:type="uml:ObjectFlow" xmi:id="_o1" source="_collect_out" target="_missing"/>`},
		{"no target", `<edge xmi:type="uml:ObjectFlow" xmi:id="_o1" source="_collect_out"/>`},
	} {
		m, err := Parse([]byte(fmt.Sprintf(method, tc.edges)))
		if err != nil {
			t.Fatal(err)
		}
		steps, end := m.DocGenChain(m.Lookup("_act"))
		if end != "" {
			t.Errorf("%s: chain refused: %s", tc.name, end)
			continue
		}
		var got []string
		for _, s := range steps {
			got = append(got, s.Kind)
		}
		if want := "[CollectOwnedElements FilterByNames TableStructure]"; fmt.Sprint(got) != want {
			t.Errorf("%s: steps %v, want %s", tc.name, got, want)
		}
	}
}

// A viewpoint whose method tag names no activity is malformed, and the view
// says so; a viewpoint that declares no method at all has none.
func TestDocGenViewKeepsWhyMethodIsMissing(t *testing.T) {
	const model = `<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML"
         xmlns:Document_Profile_="http://www.magicdraw.com/schemas/manual/Document_Profile.xmi">
  <uml:Model xmi:id="_m" name="M">
    <packagedElement xmi:type="uml:Class" xmi:id="_vp" name="VP">
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_act" name="Method"/>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Machine"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_doc" name="Doc">
      <generalization xmi:type="uml:Generalization" xmi:id="_gen" general="_vp"/>
    </packagedElement>
  </uml:Model>
  <Document_Profile_:Document xmi:id="_st_doc" base_Class="_doc"/>
  <sysml:Viewpoint xmi:id="_st_vp" base_Class="_vp" %s/>
  <sysml:Conform xmi:id="_st_conform" base_Generalization="_gen"/>
</xmi:XMI>`
	for _, tc := range []struct{ tag, method, why string }{
		{``, "", ""},
		{`method="_act"`, "_act", ""},
		{`method="_gone"`, "", `method "_gone" names no element`},
		{`method="_sm"`, "", `method "_sm" names a StateMachine, not an Activity`},
		{`method="_gone _act"`, "_act", ""},
	} {
		m, err := Parse([]byte(fmt.Sprintf(model, tc.tag)))
		if err != nil {
			t.Fatal(err)
		}
		if len(m.Documents) != 1 {
			t.Fatalf("%s: %d documents, want 1", tc.tag, len(m.Documents))
		}
		v := m.Documents[0].Root
		if v.Viewpoint == nil || v.Viewpoint.ID != "_vp" {
			t.Fatalf("%s: viewpoint %v, want VP", tc.tag, v.Viewpoint)
		}
		var method string
		if v.Method != nil {
			method = v.Method.ID
		}
		if method != tc.method {
			t.Errorf("%s: method %q, want %q", tc.tag, method, tc.method)
		}
		if v.MethodMalformed != tc.why {
			t.Errorf("%s: malformed %q, want %q", tc.tag, v.MethodMalformed, tc.why)
		}
	}
}
