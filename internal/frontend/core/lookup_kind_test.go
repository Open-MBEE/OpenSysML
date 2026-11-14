package core

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// The core reads symbolId as the kind lookup states, as the gRPC service does: the unnamed
// member's positional id and the name declared like it are two elements, one per kind.
func TestGetSymbolReadsTheStatedLookupKind(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	content := `package Col { part def W { part a; part b; part { part inner; } part '@2' { part named; } } }`
	params, _ := json.Marshal(&JParseSourcesRequest{Documents: []*JSourceDocument{{Content: &content}}})
	body, err := c.Call(ctx, "ParseSources", params)
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	var parsed JParseSourcesResponse
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.ModelHash == "" {
		t.Fatalf("ParseSources answered %s (%v)", body, err)
	}
	for _, tc := range []struct{ symbolID, lookup, wantID, wantError string }{
		{"Col::W::@2", "LOOKUP_KIND_ELEMENT_ID", "Col::W::@2", ""},
		{"Col::W::'@2'", "LOOKUP_KIND_ELEMENT_ID", "Col::W::'@2'", ""},
		{"Col::W::'@2'", "LOOKUP_KIND_QUALIFIED_NAME", "Col::W::'@2'", ""},
		{"Col::W::@2::inner", "LOOKUP_KIND_QUALIFIED_NAME", "", "symbol not found: Col::W::@2::inner"},
		{"Col::W::@2", "", "Col::W::'@2'", ""},
		{"Col::W::@2", "LOOKUP_KIND_UNSPECIFIED", "Col::W::'@2'", ""},
	} {
		request := fmt.Sprintf(`{"modelHash":%q,"symbolId":%q,"lookup":%q}`, parsed.ModelHash, tc.symbolID, tc.lookup)
		body, err := c.Call(ctx, "GetSymbol", []byte(request))
		if err != nil {
			t.Fatalf("GetSymbol(%s, %s): %v", tc.symbolID, tc.lookup, err)
		}
		var response JSymbolResponse
		if err := json.Unmarshal(body, &response); err != nil {
			t.Fatalf("decoding GetSymbol: %v\n%s", err, body)
		}
		var id string
		if response.Symbol != nil {
			id = response.Symbol.Id
		}
		if id != tc.wantID || response.Error != tc.wantError {
			t.Errorf("GetSymbol(%s, %q) = id %q, error %q; want id %q, error %q", tc.symbolID, tc.lookup, id, response.Error, tc.wantID, tc.wantError)
		}
	}
	request := fmt.Sprintf(`{"modelHash":%q,"symbolId":"Col","lookup":"LOOKUP_KIND_NAME"}`, parsed.ModelHash)
	if _, err := c.Call(ctx, "GetSymbol", []byte(request)); err == nil {
		t.Error("an unknown lookup kind was accepted")
	}
}
