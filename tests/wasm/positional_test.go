package wasm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/combined"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/core"
)

// positionalCases are the gRPC conformance cases that pin the positional ids
// (`Owner::@N`) of unnamed members, in child_ids or as the id looked up.
var positionalCases = []string{
	"symbol_children_unnamed",
	"symbol_positional_id",
	"symbol_prefix_metadata",
	"symbol_prefix_metadata_parameter",
}

// TestJSONRPCPositionalChildIDs runs the positional conformance cases over the
// JSON-RPC core and combined servers, which answer the same ids the service does.
func TestJSONRPCPositionalChildIDs(t *testing.T) {
	ctx := context.Background()
	c, err := core.New()
	if err != nil {
		t.Fatalf("core.New: %v", err)
	}
	server, err := combined.New("positional-wire-test")
	if err != nil {
		t.Fatalf("combined.New: %v", err)
	}
	servers := map[string]func(method string, params []byte) ([]byte, error){
		"core": func(method string, params []byte) ([]byte, error) {
			return c.Call(ctx, method, params)
		},
		"combined": func(method string, params []byte) ([]byte, error) {
			return server.Call(ctx, method, params)
		},
	}
	dir := filepath.Join("..", "grpc", "testdata", "conformance")
	for _, name := range positionalCases {
		content, err := os.ReadFile(filepath.Join(dir, name+".sysml"))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		var tc struct {
			RPC              string   `json:"rpc"`
			SymbolID         string   `json:"symbol_id"`
			ExpectedKind     string   `json:"expected_kind"`
			ExpectedChildIDs []string `json:"expected_child_ids"`
		}
		expected, err := os.ReadFile(filepath.Join(dir, name+".expected.json"))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if err := json.Unmarshal(expected, &tc); err != nil {
			t.Fatalf("decoding %s.expected.json: %v", name, err)
		}
		if tc.RPC != "GetSymbol" {
			t.Fatalf("%s is not a GetSymbol case", name)
		}
		for serverName, call := range servers {
			t.Run(serverName+"/"+name, func(t *testing.T) {
				params, _ := json.Marshal(map[string]any{
					"documents": []map[string]string{{"name": name + ".sysml", "content": string(content)}},
				})
				body, err := call("ParseSources", params)
				if err != nil {
					t.Fatalf("ParseSources: %v", err)
				}
				var parsed struct {
					ModelHash string `json:"modelHash"`
				}
				if err := json.Unmarshal(body, &parsed); err != nil || parsed.ModelHash == "" {
					t.Fatalf("ParseSources answered %s (%v), want a modelHash", body, err)
				}
				params, _ = json.Marshal(map[string]string{"modelHash": parsed.ModelHash, "symbolId": tc.SymbolID})
				body, err = call("GetSymbol", params)
				if err != nil {
					t.Fatalf("GetSymbol %s: %v", tc.SymbolID, err)
				}
				var answer core.JSymbolResponse
				if err := json.Unmarshal(body, &answer); err != nil {
					t.Fatalf("decoding GetSymbol response: %v\n%s", err, body)
				}
				if answer.Error != "" || answer.Symbol == nil {
					t.Fatalf("GetSymbol %s answered %s, want the symbol", tc.SymbolID, body)
				}
				if tc.ExpectedKind != "" && answer.Symbol.Kind != tc.ExpectedKind {
					t.Errorf("kind = %q, want %q", answer.Symbol.Kind, tc.ExpectedKind)
				}
				if tc.ExpectedChildIDs != nil && !slices.Equal(answer.Symbol.ChildIds, tc.ExpectedChildIDs) {
					t.Errorf("childIds = %v, want %v", answer.Symbol.ChildIds, tc.ExpectedChildIDs)
				}
			})
		}
	}
}
