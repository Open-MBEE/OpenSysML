package migrate

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

func TestWrappedRegionPathsIncludeRootModel(t *testing.T) {
	model, err := sysmlv1.Parse([]byte(`<?xml version="1.0"?>
<xmi:XMI xmi:version="2.5.1"
         xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101">
  <uml:Model xmi:type="uml:Model" xmi:id="_model" name="Model">
    <packagedElement xmi:type="uml:StateMachine" xmi:id="_machine" name="Valve">
      <region xmi:type="uml:Region" xmi:id="_machine_region">
        <subvertex xmi:type="uml:State" xmi:id="_busy" name="Busy">
          <region xmi:type="uml:Region" xmi:id="_wrapped_region" name="region">
            <subvertex xmi:type="uml:State" xmi:id="_read" name="Read"/>
          </region>
          <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_leave" name="Leave" kind="exitPoint"/>
        </subvertex>
      </region>
    </packagedElement>
  </uml:Model>
</xmi:XMI>`))
	if err != nil {
		t.Fatal(err)
	}

	root, machine := model.Lookup("_model"), model.Lookup("_machine")
	busy, region := model.Lookup("_busy"), model.Lookup("_wrapped_region")
	read, leave := model.Lookup("_read"), model.Lookup("_leave")
	names := map[*sysmlv1.Element]string{region: "region"}
	asUsage := map[*sysmlv1.Element]bool{
		root: true, machine: true, busy: true, region: true, read: true, leave: true,
	}
	m := &migration{
		model:        model,
		names:        names,
		asUsage:      asUsage,
		regionStates: map[*sysmlv1.Element]string{region: ""},
	}

	for _, tc := range []struct {
		elem *sysmlv1.Element
		want string
	}{
		{read, "Model::Valve::Busy::region::Read"},
		{leave, "Model::Valve::Busy::region::Leave"},
	} {
		if got := strings.Join(m.segments(tc.elem), "::"); got != tc.want {
			t.Errorf("segments(%s) = %q, want %q", tc.elem.Name, got, tc.want)
		}
	}
}
