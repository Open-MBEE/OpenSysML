package core

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// TestGetSymbolReadsMemberOfAsAReportedID pins that memberOf is read the way symbolId is:
// an unnamed connector end's element id names the end, as the gRPC service answers it.
func TestGetSymbolReadsMemberOfAsAReportedID(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	content := `package Gen { part def V { part q; part r; connection c connect q to r; } }`
	params, _ := json.Marshal(&JParseSourcesRequest{Documents: []*JSourceDocument{{Content: &content}}})
	body, err := c.Call(ctx, "ParseSources", params)
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	var parsed JParseSourcesResponse
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.ModelHash == "" {
		t.Fatalf("ParseSources answered %s (%v)", body, err)
	}
	for _, owner := range []string{"Gen::V", "Gen__V__c_pend0"} {
		request := fmt.Sprintf(`{"modelHash":%q,"symbolId":"Gen::V::q","memberOf":%q}`, parsed.ModelHash, owner)
		body, err := c.Call(ctx, "GetSymbol", []byte(request))
		if err != nil {
			t.Fatalf("GetSymbol as a member of %s: %v", owner, err)
		}
		var response JSymbolResponse
		if err := json.Unmarshal(body, &response); err != nil {
			t.Fatalf("decoding GetSymbol: %v\n%s", err, body)
		}
		if response.Symbol == nil || response.Symbol.Id != "Gen::V::q" {
			t.Errorf("GetSymbol as a member of %s answered %s", owner, body)
		}
	}
}
