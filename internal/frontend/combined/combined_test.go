// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package combined

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
)

func TestUnservedMethod(t *testing.T) {
	server, err := New("combined-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = server.Call(context.Background(), "Convert", []byte(`{}`))
	var callErr *jsonrpc.Error
	if !errors.As(err, &callErr) || callErr.Code != jsonrpc.CodeUnimplemented {
		t.Fatalf("Call(Convert) error = %v, want Unimplemented", err)
	}
	if callErr.Message != "Convert is not served by sysml-wasm: it is served by sysml-grpc" {
		t.Errorf("Call(Convert) message = %q", callErr.Message)
	}
}

func TestGetServerInfo(t *testing.T) {
	server, err := New("combined-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	body, err := server.Call(context.Background(), "GetServerInfo", nil)
	if err != nil {
		t.Fatalf("GetServerInfo: %v", err)
	}
	var info struct {
		Version      string   `json:"version"`
		Capabilities []string `json:"capabilities"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		t.Fatalf("decoding GetServerInfo: %v\n%s", err, body)
	}
	if info.Version != "combined-test" {
		t.Errorf("version = %q, want combined-test", info.Version)
	}
	if !reflect.DeepEqual(info.Capabilities, capabilities) {
		t.Errorf("capabilities = %v, want %v", info.Capabilities, capabilities)
	}
}

func TestParseFileHashIsAcceptedByEvaluate(t *testing.T) {
	server, err := New("combined-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	parsed, err := server.Call(context.Background(), "ParseFile", []byte(`{"content":"package Demo {}"}`))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	var response struct {
		ModelHash string `json:"modelHash"`
	}
	if err := json.Unmarshal(parsed, &response); err != nil {
		t.Fatalf("decoding ParseFile response: %v\n%s", err, parsed)
	}
	evaluated, err := server.Call(context.Background(), "Evaluate", []byte(`{"modelHash":"`+response.ModelHash+`","expression":"1 + 1"}`))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	var answer struct {
		Result struct {
			IntValue string `json:"intValue"`
		} `json:"result"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(evaluated, &answer); err != nil {
		t.Fatalf("decoding Evaluate response: %v\n%s", err, evaluated)
	}
	if answer.Error != "" || answer.Result.IntValue != "2" {
		t.Errorf("Evaluate = %s, want intValue 2", evaluated)
	}
}

func TestCoreAccessRefreshesSharedRetention(t *testing.T) {
	server := newRetentionTestServer(t)
	hashes := parseRetentionModels(t, server, 16, 0)

	callRetentionMethod(t, server, "GetDiagnostics", hashes[0])
	parseRetentionModels(t, server, 1, 16)

	callRetentionMethod(t, server, "Evaluate", hashes[0])
	callRetentionMethod(t, server, "GetSymbol", hashes[0])
	assertRetentionNotFound(t, server, "Evaluate", hashes[1])
	assertRetentionNotFound(t, server, "GetSymbol", hashes[1])
}

func TestEngineAccessRefreshesSharedRetention(t *testing.T) {
	server := newRetentionTestServer(t)
	hashes := parseRetentionModels(t, server, 16, 0)

	callRetentionMethod(t, server, "Evaluate", hashes[0])
	parseRetentionModels(t, server, 1, 16)

	callRetentionMethod(t, server, "GetDiagnostics", hashes[0])
	callRetentionMethod(t, server, "GetSymbol", hashes[0])
	assertRetentionNotFound(t, server, "GetDiagnostics", hashes[1])
	assertRetentionNotFound(t, server, "Evaluate", hashes[1])
}

func TestReparsingRefreshesSharedRetention(t *testing.T) {
	server := newRetentionTestServer(t)
	hashes := parseRetentionModels(t, server, 16, 0)
	if got := parseRetentionModels(t, server, 1, 0)[0]; got != hashes[0] {
		t.Fatalf("reparsed hash = %q, want %q", got, hashes[0])
	}
	parseRetentionModels(t, server, 1, 16)

	callRetentionMethod(t, server, "GetDiagnostics", hashes[0])
	callRetentionMethod(t, server, "Evaluate", hashes[0])
	assertRetentionNotFound(t, server, "GetDiagnostics", hashes[1])
	assertRetentionNotFound(t, server, "Evaluate", hashes[1])
}

func newRetentionTestServer(t *testing.T) *Server {
	t.Helper()
	server, err := New("combined-retention-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return server
}

func parseRetentionModels(t *testing.T, server *Server, count, start int) []string {
	t.Helper()
	hashes := make([]string, 0, count)
	for i := start; i < start+count; i++ {
		params, err := json.Marshal(struct {
			Content string `json:"content"`
		}{Content: fmt.Sprintf("package M%d { attribute def T; }", i)})
		if err != nil {
			t.Fatalf("encoding ParseFile request: %v", err)
		}
		body, err := server.Call(context.Background(), "ParseFile", params)
		if err != nil {
			t.Fatalf("ParseFile(M%d): %v", i, err)
		}
		var response struct {
			ModelHash string `json:"modelHash"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			t.Fatalf("decoding ParseFile(M%d): %v\n%s", i, err, body)
		}
		hashes = append(hashes, response.ModelHash)
	}
	return hashes
}

func callRetentionMethod(t *testing.T, server *Server, method, hash string) {
	t.Helper()
	var params any
	switch method {
	case "Evaluate":
		params = struct {
			ModelHash  string `json:"modelHash"`
			Expression string `json:"expression"`
		}{ModelHash: hash, Expression: "1 + 1"}
	case "GetSymbol":
		params = struct {
			ModelHash string `json:"modelHash"`
			SymbolId  string `json:"symbolId"`
		}{ModelHash: hash, SymbolId: "M0::T"}
	default:
		params = struct {
			ModelHash string `json:"modelHash"`
		}{ModelHash: hash}
	}
	body, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("encoding %s request: %v", method, err)
	}
	responseBody, err := server.Call(context.Background(), method, body)
	if err != nil {
		t.Fatalf("%s(%s): %v", method, hash, err)
	}
	switch method {
	case "Evaluate":
		var response struct {
			Result struct {
				IntValue string `json:"intValue"`
			} `json:"result"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(responseBody, &response); err != nil {
			t.Fatalf("decoding Evaluate response: %v\n%s", err, responseBody)
		}
		if response.Error != "" || response.Result.IntValue != "2" {
			t.Fatalf("Evaluate response = %s, want intValue 2", responseBody)
		}
	case "GetSymbol":
		var response struct {
			Symbol json.RawMessage `json:"symbol"`
			Error  string          `json:"error"`
		}
		if err := json.Unmarshal(responseBody, &response); err != nil {
			t.Fatalf("decoding GetSymbol response: %v\n%s", err, responseBody)
		}
		if response.Error != "" || len(response.Symbol) == 0 {
			t.Fatalf("GetSymbol response = %s, want a symbol", responseBody)
		}
	}
}

func assertRetentionNotFound(t *testing.T, server *Server, method, hash string) {
	t.Helper()
	var params any
	switch method {
	case "Evaluate":
		params = struct {
			ModelHash  string `json:"modelHash"`
			Expression string `json:"expression"`
		}{ModelHash: hash, Expression: "1 + 1"}
	case "GetSymbol":
		params = struct {
			ModelHash string `json:"modelHash"`
			SymbolId  string `json:"symbolId"`
		}{ModelHash: hash, SymbolId: "M0::T"}
	default:
		params = struct {
			ModelHash string `json:"modelHash"`
		}{ModelHash: hash}
	}
	body, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("encoding %s request: %v", method, err)
	}
	_, err = server.Call(context.Background(), method, body)
	var callErr *jsonrpc.Error
	if !errors.As(err, &callErr) || callErr.Code != jsonrpc.CodeNotFound {
		t.Errorf("%s(%s) error = %v, want NotFound", method, hash, err)
	}
}
