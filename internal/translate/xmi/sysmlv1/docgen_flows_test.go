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
