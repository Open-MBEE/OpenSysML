// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// Package combined serves the shared parsing, validation and execution surface.
package combined

import (
	"container/list"
	"context"
	"encoding/json"
	"io"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/core"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const maxModels = 16

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
	models  *list.List
	byHash  map[string]*list.Element
}

// New builds both frontends over one frozen standard-library snapshot.
func New(version string) (*Server, error) {
	index, src := libs.FrozenLibrary()
	execution, err := engine.NewWithLibrary(index, src, 0)
	if err != nil {
		return nil, err
	}
	validation, err := core.NewWithLibrary(index, src, 0)
	if err != nil {
		return nil, err
	}
	return &Server{
		engine:  execution,
		core:    validation,
		version: version,
		models:  list.New(),
		byHash:  make(map[string]*list.Element),
	}, nil
}

// Call runs one combined method with protojson-shaped request parameters.
func (s *Server) Call(ctx context.Context, method string, params []byte) ([]byte, error) {
	switch method {
	case "ParseSources":
		body, err := s.core.Call(ctx, method, params)
		if err != nil {
			return nil, err
		}
		return s.finishParse(ctx, body, method, params)
	case "ParseFile":
		body, err := s.core.Call(ctx, method, params)
		if err != nil {
			return nil, err
		}
		var req core.JParseFileRequest
		if err := jsonrpc.Decode(params, &req); err != nil {
			s.discardUnretained(body)
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
			s.discardUnretained(body)
			return nil, jsonrpc.Errorf(jsonrpc.CodeInternal, "encoding engine ParseSources request: %v", err)
		}
		return s.finishParse(ctx, body, "ParseSources", engineParams)
	case "GetDiagnostics", "GetSymbol":
		body, err := s.core.Call(ctx, method, params)
		if err == nil {
			s.touchRequest(params)
		}
		return body, err
	case "Evaluate", "Instantiate", "ExecuteAction", "ExecuteState":
		body, err := s.engine.Call(ctx, method, params)
		if err == nil {
			s.touchRequest(params)
		}
		return body, err
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

func (s *Server) finishParse(ctx context.Context, coreBody []byte, method string, params []byte) ([]byte, error) {
	engineBody, err := s.engine.Call(ctx, method, params)
	if err != nil {
		s.discardUnretained(coreBody, engineBody)
		return nil, err
	}
	if err := sameModelHash(coreBody, engineBody); err != nil {
		s.discardUnretained(coreBody, engineBody)
		return nil, err
	}
	hash, err := responseModelHash(coreBody)
	if err != nil {
		s.discardUnretained(coreBody, engineBody)
		return nil, jsonrpc.Errorf(jsonrpc.CodeInternal, "decoding core model hash: %v", err)
	}
	s.retain(hash)
	return coreBody, nil
}

func (s *Server) retain(hash string) {
	if hash == "" {
		return
	}
	if elem, ok := s.byHash[hash]; ok {
		s.models.MoveToFront(elem)
		return
	}
	s.byHash[hash] = s.models.PushFront(hash)
	if s.models.Len() > maxModels {
		oldest := s.models.Back()
		evicted := oldest.Value.(string)
		s.models.Remove(oldest)
		delete(s.byHash, evicted)
		s.core.Evict(evicted)
		s.engine.Evict(evicted)
	}
}

func (s *Server) touchRequest(params []byte) {
	var req struct {
		ModelHash string `json:"modelHash"`
	}
	if json.Unmarshal(params, &req) != nil {
		return
	}
	if elem, ok := s.byHash[req.ModelHash]; ok {
		s.models.MoveToFront(elem)
	}
}

func (s *Server) discardUnretained(responses ...[]byte) {
	for _, body := range responses {
		hash, err := responseModelHash(body)
		if err != nil || hash == "" {
			continue
		}
		if _, retained := s.byHash[hash]; retained {
			continue
		}
		s.core.Evict(hash)
		s.engine.Evict(hash)
	}
}

func responseModelHash(body []byte) (string, error) {
	var response struct {
		ModelHash string `json:"modelHash"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	return response.ModelHash, nil
}

func sameModelHash(coreBody, engineBody []byte) error {
	coreHash, err := responseModelHash(coreBody)
	if err != nil {
		return jsonrpc.Errorf(jsonrpc.CodeInternal, "decoding core model hash: %v", err)
	}
	engineHash, err := responseModelHash(engineBody)
	if err != nil {
		return jsonrpc.Errorf(jsonrpc.CodeInternal, "decoding engine model hash: %v", err)
	}
	if coreHash != engineHash {
		return jsonrpc.Errorf(jsonrpc.CodeInternal,
			"sysml-wasm model hash mismatch: core %q, engine %q", coreHash, engineHash)
	}
	return nil
}

// Serve answers Content-Length-delimited JSON-RPC frames until r reaches its end.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	return jsonrpc.Serve(ctx, r, w, s.Call)
}
