package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const membersModel = `
package Demo {
    part def Base { attribute label; private attribute secret; }
    part def Car :> Base { attribute mass; }
    part car : Car;
}
`

// TestGetSymbolMembersWithholdLibraryUnlessAsked pins that the core's GetSymbol
// honours includeLibraryMembers as the gRPC service does: without it the
// members inherited from standard-library content are withheld and counted,
// with it they are listed and marked.
func TestGetSymbolMembersWithholdLibraryUnlessAsked(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	content := membersModel
	params, _ := json.Marshal(&JParseSourcesRequest{Documents: []*JSourceDocument{{Content: &content}}})
	body, err := c.Call(ctx, "ParseSources", params)
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	var parsed JParseSourcesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("decoding ParseSources: %v\n%s", err, body)
	}
	if parsed.Error != "" || parsed.ModelHash == "" {
		t.Fatalf("ParseSources answered %s", body)
	}

	get := func(includeLibrary bool) *JSymbolInfo {
		t.Helper()
		params := fmt.Sprintf(`{"modelHash":%q,"symbolId":"Demo::car","includeLibraryMembers":%t}`, parsed.ModelHash, includeLibrary)
		body, err := c.Call(ctx, "GetSymbol", []byte(params))
		if err != nil {
			t.Fatalf("GetSymbol: %v", err)
		}
		var response JSymbolResponse
		if err := json.Unmarshal(body, &response); err != nil {
			t.Fatalf("decoding GetSymbol: %v\n%s", err, body)
		}
		if response.Symbol == nil {
			t.Fatalf("GetSymbol answered %s", body)
		}
		return response.Symbol
	}

	withheld := get(false)
	var names []string
	for _, m := range withheld.Members {
		if m.Library {
			t.Errorf("library member %s listed without includeLibraryMembers", m.Id)
		}
		if !m.Inherited {
			t.Errorf("car declares nothing, yet %s is reported as its own", m.Id)
		}
		names = append(names, m.Name)
	}
	if want := []string{"mass", "label"}; fmt.Sprint(names) != fmt.Sprint(want) {
		t.Errorf("withheld members = %v, want %v (secret is private, library ones withheld)", names, want)
	}
	if withheld.WithheldLibraryMembers == 0 {
		t.Error("no withheld library member counted, yet every part inherits Parts::Part's")
	}

	included := get(true)
	if included.WithheldLibraryMembers != 0 {
		t.Errorf("includeLibraryMembers still withholds %d members", included.WithheldLibraryMembers)
	}
	var library int32
	unnamed := 0
	for _, m := range included.Members {
		if m.Library {
			library++
			if m.Name == "" && strings.HasPrefix(m.Id, m.InheritedFrom+"::@") {
				unnamed++
			}
			if !m.Inherited || m.InheritedFrom == "" {
				t.Errorf("library member %s lacks its inherited provenance", m.Id)
			}
		}
	}
	if unnamed == 0 {
		t.Error("no unnamed library member listed under a positional id, yet Occurrences::Occurrence declares several")
	}
	if library != withheld.WithheldLibraryMembers {
		t.Errorf("includeLibraryMembers lists %d library members, %d were withheld", library, withheld.WithheldLibraryMembers)
	}
	if len(included.Members) != len(withheld.Members)+int(library) {
		t.Errorf("included members = %d, want the %d withheld ones plus %d library", len(included.Members), len(withheld.Members), library)
	}
}
