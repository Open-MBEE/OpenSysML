// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package combined

import (
	"context"
	"encoding/json"
	"errors"
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
