package migrate

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

func TestQueryNameMetadataPathSegment(t *testing.T) {
	for _, tc := range []struct {
		names      []string
		definition []string
		feature    string
	}{
		{[]string{"Frank's Braindump", "Composition Example", "review's grade"}, []string{`Frank\'s Braindump`, "Composition Example"}, `review\'s grade`},
		{[]string{"Review Profile", "Reviewed", "Safety::Scope"}, []string{"Review Profile", "Reviewed"}, "Safety::Scope"},
		{[]string{"Profile::Review", "part", "level"}, []string{"Profile::Review", "part"}, "level"},
	} {
		segments := make([]string, len(tc.names))
		for i, name := range tc.names {
			segments[i] = pathSegment(queryName(name))
		}
		path := strings.Join(segments, "::")
		definition, feature, ok := source.MetadataPathSegments(path)
		if !ok || !reflect.DeepEqual(definition, tc.definition) || feature != tc.feature {
			t.Errorf("MetadataPathSegments(%q) = %v, %q, %v", path, definition, feature, ok)
		}
	}
}

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
