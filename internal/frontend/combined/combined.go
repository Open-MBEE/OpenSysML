// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// Package combined serves the shared parsing, validation and execution surface.
package combined

import (
	"context"
	"encoding/json"
	"io"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/core"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

var capabilities = []string{
	"type_facts",
	"enum_values",
	"evaluate_subject",
	"symbol_attributes",
	"unset_value",
	"feature_values",
	"inline_language",
	"strict_conformance",
	"parse_sources",
	"complex_values",
	"structured_values",
	"measurement_refs",
	"function_values",
	"set_values",
	"tensor_values",
	"infinity_value",
	"diagnostic_codes",
	"schedule",
	"final_time",
	"metaobject_values",
	"undetermined_value",
	"performer",
	"big_int_values",
}

// Server combines the parsing/validation and execution JSON surfaces.
type Server struct {
	engine  *engine.Engine
	core    *core.Core
	version string
}

// New builds both frontends over one frozen standard-library snapshot.
func New(version string) (*Server, error) {
	index, src := libs.FrozenLibrary()
	execution, err := engine.NewWithLibrary(index, src)
	if err != nil {
		return nil, err
	}
	validation, err := core.NewWithLibrary(index, src)
	if err != nil {
		return nil, err
	}
	return &Server{engine: execution, core: validation, version: version}, nil
}

// Call runs one combined method with protojson-shaped request parameters.
func (s *Server) Call(ctx context.Context, method string, params []byte) ([]byte, error) {
	switch method {
	case "ParseSources":
		body, err := s.core.Call(ctx, method, params)
		if err != nil {
			return nil, err
		}
		engineBody, err := s.engine.Call(ctx, method, params)
		if err != nil {
			return nil, err
		}
		if err := sameModelHash(body, engineBody); err != nil {
			return nil, err
		}
		return body, nil
	case "ParseFile":
		body, err := s.core.Call(ctx, method, params)
		if err != nil {
			return nil, err
		}
		var req core.JParseFileRequest
		if err := jsonrpc.Decode(params, &req); err != nil {
			return nil, err
		}
		document := &engine.JSourceDocument{}
		if req.FilePath != nil {
			document.FilePath = req.FilePath
		} else if req.Content != nil {
			document.Name = "<content>"
			document.Content = req.Content
			document.Language = req.Language
		}
		engineParams, err := json.Marshal(&engine.JParseSourcesRequest{
			Documents:         []*engine.JSourceDocument{document},
			StrictConformance: req.StrictConformance,
		})
		if err != nil {
			return nil, jsonrpc.Errorf(jsonrpc.CodeInternal, "encoding engine ParseSources request: %v", err)
		}
		engineBody, err := s.engine.Call(ctx, "ParseSources", engineParams)
		if err != nil {
			return nil, err
		}
		if err := sameModelHash(body, engineBody); err != nil {
			return nil, err
		}
		return body, nil
	case "GetDiagnostics", "GetSymbol":
		return s.core.Call(ctx, method, params)
	case "Evaluate", "Instantiate", "ExecuteAction", "ExecuteState":
		return s.engine.Call(ctx, method, params)
	case "GetServerInfo":
		return json.Marshal(struct {
			Version      string   `json:"version,omitempty"`
			Capabilities []string `json:"capabilities,omitempty"`
		}{Version: s.version, Capabilities: capabilities})
	default:
		return nil, jsonrpc.Errorf(jsonrpc.CodeUnimplemented,
			"%s is not served by sysml-wasm: it is served by sysml-grpc", method)
	}
}

func sameModelHash(coreBody, engineBody []byte) error {
	var coreResponse, engineResponse struct {
		ModelHash string `json:"modelHash"`
	}
	if err := json.Unmarshal(coreBody, &coreResponse); err != nil {
		return jsonrpc.Errorf(jsonrpc.CodeInternal, "decoding core model hash: %v", err)
	}
	if err := json.Unmarshal(engineBody, &engineResponse); err != nil {
		return jsonrpc.Errorf(jsonrpc.CodeInternal, "decoding engine model hash: %v", err)
	}
	if coreResponse.ModelHash != engineResponse.ModelHash {
		return jsonrpc.Errorf(jsonrpc.CodeInternal,
			"sysml-wasm model hash mismatch: core %q, engine %q", coreResponse.ModelHash, engineResponse.ModelHash)
	}
	return nil
}

// Serve answers Content-Length-delimited JSON-RPC frames until r reaches its end.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	return jsonrpc.Serve(ctx, r, w, s.Call)
}
