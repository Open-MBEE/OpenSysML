package lsp

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

func relatedElements(t *testing.T, s *Server, docURI uri.URI, element string) (*RelatedElementsResult, error) {
	t.Helper()
	raw, err := call(t, s, MethodRelatedElements, &RelatedElementsParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		Element:      element,
	})
	if err != nil {
		return nil, err
	}
	var out RelatedElementsResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return &out, nil
}

// The request groups what relates to an element by relationship kind, each
// element with the qualified name a render request includes it by, located
// in its document; its own groups' kinds are the ones the capability lists.
func TestRelatedElementsGroupsByRelationshipKind(t *testing.T) {
	s, docURI := renderServer(t, "rel.sysml", includeModel)
	out, err := relatedElements(t, s, docURI, "Rel::Vehicle")
	if err != nil {
		t.Fatalf("relatedElements: %v", err)
	}
	if out.Element.FQN != "Rel::Vehicle" || out.Element.Kind != "part def" || out.Element.Name != "Vehicle" || !out.Element.DeclaredHere || out.Element.Origin == nil {
		t.Errorf("element = %+v, want Rel::Vehicle, a part def declared here and located", out.Element)
	}
	got := map[string][]string{}
	for _, group := range out.Groups {
		if group.Label == "" {
			t.Errorf("group %s has no label", group.Kind)
		}
		for _, elem := range group.Elements {
			got[group.Kind] = append(got[group.Kind], elem.FQN)
			if elem.Origin == nil || !elem.DeclaredHere {
				t.Errorf("%s %s: origin %v, declaredHere %v; want located and declared here", group.Kind, elem.FQN, elem.Origin, elem.DeclaredHere)
			}
		}
	}
	want := map[string][]string{
		string(view.RelatedSpecializations): {"Rel::Car", "Rel::Truck"},
		string(view.RelatedOwner):           {"Rel"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("groups = %v, want %v", got, want)
	}

	out, err = relatedElements(t, s, docURI, "Rel::car")
	if err != nil {
		t.Fatalf("relatedElements car: %v", err)
	}
	var kinds []string
	for _, group := range out.Groups {
		kinds = append(kinds, group.Kind)
	}
	if strings.Join(kinds, " ") != "types owner ownedMembers" {
		t.Errorf("car groups = %v, want types, owner, ownedMembers in listing order", kinds)
	}
}

// An element no held document declares, or none named, is refused.
func TestRelatedElementsRefusesAnUnknownElement(t *testing.T) {
	s, docURI := renderServer(t, "rel.sysml", includeModel)
	if _, err := relatedElements(t, s, docURI, "Rel::Nothing"); err == nil || !strings.Contains(err.Error(), "no element named Rel::Nothing") {
		t.Errorf("unknown element: err = %v, want it refused", err)
	}
	if _, err := relatedElements(t, s, docURI, ""); err == nil || !strings.Contains(err.Error(), "names no element") {
		t.Errorf("empty element: err = %v, want it refused", err)
	}
}

func TestInitializeAdvertisesTheRelatedElementsCapability(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	res, err := s.Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	experimental := experimentalCaps(t, res)
	kinds, ok := experimental[RelatedElementsCapability].([]string)
	if !ok || len(kinds) != len(view.RelatedKinds()) || kinds[0] != string(view.RelatedGeneralizations) {
		t.Errorf("%s = %#v, want the relationship kinds in listing order", RelatedElementsCapability, experimental[RelatedElementsCapability])
	}
}
