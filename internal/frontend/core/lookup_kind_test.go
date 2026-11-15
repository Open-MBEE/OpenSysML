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
	content := `package Col { part def W { part a; part b; part { part inner; } part '@2' { part named; } } }
package Col__V__c_pend0 { part marker; }
package Col { port def P; part def V { port q : P; port r : P; connection c connect q to r; } }
package Quoted { part '\'q\''; part q; }`
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
		{"Col__V__c_pend0", "LOOKUP_KIND_ELEMENT_ID", "Col__V__c_pend0", ""},
		{"Col__V__c_pend0", "LOOKUP_KIND_QUALIFIED_NAME", "Col_5f_5fV_5f_5fc_5fpend0", ""},
		{"Col__V__c_pend0", "", "Col_5f_5fV_5f_5fc_5fpend0", ""},
		{"Col_5f_5fV_5f_5fc_5fpend0", "LOOKUP_KIND_ELEMENT_ID", "Col_5f_5fV_5f_5fc_5fpend0", ""},
		{"Col_5f_5fV_5f_5fc_5fpend0", "LOOKUP_KIND_QUALIFIED_NAME", "", "symbol not found: Col_5f_5fV_5f_5fc_5fpend0"},
		{"Quoted::'q'", "LOOKUP_KIND_ELEMENT_ID", "Quoted::'q'", ""},
		{"Quoted::q", "LOOKUP_KIND_ELEMENT_ID", "Quoted::q", ""},
		{"Quoted::'q'", "LOOKUP_KIND_QUALIFIED_NAME", "Quoted::q", ""},
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

// coreSymbolReader parses content and answers GetSymbol's reported id, child ids and error.
func coreSymbolReader(t *testing.T, content string) func(symbolID, lookup string) (string, []string, string) {
	t.Helper()
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	params, _ := json.Marshal(&JParseSourcesRequest{Documents: []*JSourceDocument{{Content: &content}}})
	body, err := c.Call(ctx, "ParseSources", params)
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	var parsed JParseSourcesResponse
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.ModelHash == "" {
		t.Fatalf("ParseSources answered %s (%v)", body, err)
	}
	return func(symbolID, lookup string) (string, []string, string) {
		t.Helper()
		request := fmt.Sprintf(`{"modelHash":%q,"symbolId":%q,"lookup":%q}`, parsed.ModelHash, symbolID, lookup)
		body, err := c.Call(ctx, "GetSymbol", []byte(request))
		if err != nil {
			t.Fatalf("GetSymbol(%s, %s): %v", symbolID, lookup, err)
		}
		var response JSymbolResponse
		if err := json.Unmarshal(body, &response); err != nil {
			t.Fatalf("decoding GetSymbol: %v\n%s", err, body)
		}
		if response.Symbol == nil {
			return "", nil, response.Error
		}
		return response.Symbol.Id, response.Symbol.ChildIds, response.Error
	}
}

// A root connector the core reports under an escaped id still has its unnamed ends found
// by the ids it reports for them.
func TestEscapedRootConnectorEndsResolve(t *testing.T) {
	read := coreSymbolReader(t, `port def P; port q : P; port r : P;
connection A connect q to r; connection A_pend0 connect q to r;`)
	id, children, _ := read("A_pend0", "LOOKUP_KIND_QUALIFIED_NAME")
	if id != "A_5fpend0" || len(children) != 2 {
		t.Fatalf("A_pend0 by name = id %q, children %v; want the escaped connector and its two ends", id, children)
	}
	for _, child := range children {
		if got, _, errText := read(child, "LOOKUP_KIND_ELEMENT_ID"); got != child {
			t.Errorf("GetSymbol(%s, element id) = id %q, error %q", child, got, errText)
		}
	}
}

// A root declaration spelled like another's escaped id is escaped in turn, so the two keep
// ids of their own and each id finds its declaration.
func TestEscapedRootIdsStayDistinct(t *testing.T) {
	read := coreSymbolReader(t, `port def P; port q : P; port r : P; connection A connect q to r;
package A_pend0 { part m; } package A_5fpend0 { part n; }`)
	for _, tc := range []struct{ name, wantID, wantChild string }{
		{"A_pend0", "A_5fpend0", "A_pend0::m"},
		{"A_5fpend0", "A_5f5fpend0", "A_5fpend0::n"},
	} {
		id, _, errText := read(tc.name, "LOOKUP_KIND_QUALIFIED_NAME")
		if id != tc.wantID {
			t.Errorf("GetSymbol(%s, name) = id %q, error %q; want id %q", tc.name, id, errText, tc.wantID)
		}
		if got, children, errText := read(tc.wantID, "LOOKUP_KIND_ELEMENT_ID"); got != tc.wantID || len(children) != 1 || children[0] != tc.wantChild {
			t.Errorf("GetSymbol(%s, element id) = id %q, children %v, error %q; want the package holding %s", tc.wantID, got, children, errText, tc.wantChild)
		}
	}
}
