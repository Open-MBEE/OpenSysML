// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// Package core serves parsing, validation diagnostics and symbol facts without
// protobuf, Connect or execution dependencies.
package core

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	fsyntax "github.com/Open-MBEE/OpenSysML/internal/frontend/syntax"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const (
	maxCachedModels  = 16
	msgModelNotFound = "model not found: %s"
)

type sourceInput struct {
	name     string
	language string
	content  string
	kind     source.Kind
}

type cachedDocument struct {
	root        *ast.RootNamespace
	source      *source.SourceFile
	diagnostics []diag.Diagnostic
}

type cachedModel struct {
	documents []*cachedDocument
	index     *symbols.Index
	library   libs.Source
	mode      diag.ConformanceMode
	symbols   *symbolfacts.Context
}

type cacheEntry struct {
	hash  string
	model *cachedModel
}

// Core serves the validation RPCs sequentially and is not safe for concurrent use.
type Core struct {
	libraryIndex *symbols.Index
	library      libs.Source
	models       *list.List
	byHash       map[string]*list.Element
}

// New builds a core server over the frozen standard-library snapshot.
func New() (*Core, error) {
	index, library := libs.FrozenLibrary()
	return &Core{
		libraryIndex: index,
		library:      library,
		models:       list.New(),
		byHash:       make(map[string]*list.Element),
	}, nil
}

// Call runs a supported SysMLService method against params, the protojson-shaped
// request body, and returns the protojson-shaped response body.
func (c *Core) Call(ctx context.Context, method string, params []byte) (result []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = &jsonrpc.Error{
				Code:    jsonrpc.CodeInternal,
				Message: fmt.Sprintf("internal error in %s: %v", method, r),
			}
		}
	}()

	switch method {
	case "ParseSources":
		var req JParseSourcesRequest
		if err := jsonrpc.Decode(params, &req); err != nil {
			return nil, err
		}
		return c.parseSources(&req)
	case "ParseFile":
		var req JParseFileRequest
		if err := jsonrpc.Decode(params, &req); err != nil {
			return nil, err
		}
		return c.parseFile(&req)
	case "GetDiagnostics":
		var req JDiagnosticsRequest
		if err := jsonrpc.Decode(params, &req); err != nil {
			return nil, err
		}
		return c.getDiagnostics(&req)
	case "GetSymbol":
		var req JGetSymbolRequest
		if err := jsonrpc.Decode(params, &req); err != nil {
			return nil, err
		}
		return c.getSymbol(&req)
	default:
		return nil, jsonrpc.Errorf(jsonrpc.CodeUnimplemented,
			"%s is not served by sysml-core: execution is served by sysml-engine, and every other method by sysml-grpc", method)
	}
}

// Serve answers Content-Length-delimited JSON-RPC frames until r reaches its end.
func (c *Core) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	return jsonrpc.Serve(ctx, r, w, c.Call)
}

func (c *Core) parseSources(req *JParseSourcesRequest) ([]byte, error) {
	if len(req.Documents) == 0 {
		return nil, jsonrpc.Errorf(jsonrpc.CodeInvalidArgument, "documents must name at least one document")
	}

	inputs := make([]sourceInput, 0, len(req.Documents))
	named := make(map[string]int, len(req.Documents))
	for position, document := range req.Documents {
		input, err := documentInput(document, position)
		if err != nil {
			return nil, err
		}
		if first, taken := named[input.name]; taken {
			return nil, jsonrpc.Errorf(jsonrpc.CodeInvalidArgument,
				"documents %d and %d are both named %q: each document of a model needs its own name",
				first, position, input.name)
		}
		named[input.name] = position
		inputs = append(inputs, input)
	}

	modelHash, model := c.parseModel(inputs, diag.ConformanceModeOf(req.StrictConformance))
	roots := make([]*JSymbolInfo, 0, len(model.documents))
	for _, document := range model.documents {
		roots = append(roots, rootSymbol(model, document))
	}
	return json.Marshal(&JParseSourcesResponse{
		ModelHash:   modelHash,
		Roots:       roots,
		Diagnostics: modelDiagnostics(model),
	})
}

func (c *Core) parseFile(req *JParseFileRequest) ([]byte, error) {
	var input sourceInput
	var err error
	switch {
	case req.Content != nil:
		input, err = inlineInput("<content>", req.Language, *req.Content)
	case req.FilePath != nil:
		input, err = fileInput(*req.FilePath)
	default:
		return nil, jsonrpc.Errorf(jsonrpc.CodeInvalidArgument, "source must be file_path or content")
	}
	if err != nil {
		return nil, err
	}

	modelHash, model := c.parseModel([]sourceInput{input}, diag.ConformanceModeOf(req.StrictConformance))
	return json.Marshal(&JParseFileResponse{
		ModelHash:   modelHash,
		Root:        rootSymbol(model, model.documents[0]),
		Diagnostics: modelDiagnostics(model),
	})
}

func documentInput(document *JSourceDocument, position int) (sourceInput, error) {
	if document == nil {
		return sourceInput{}, jsonrpc.Errorf(jsonrpc.CodeInvalidArgument,
			"document %d must carry file_path or content", position)
	}
	switch {
	case document.Content != nil:
		name := document.Name
		if name == "" {
			name = fmt.Sprintf("<content-%d>", position)
		}
		return inlineInput(name, document.Language, *document.Content)
	case document.FilePath != nil:
		return fileInput(*document.FilePath)
	default:
		return sourceInput{}, jsonrpc.Errorf(jsonrpc.CodeInvalidArgument,
			"document %d must carry file_path or content", position)
	}
}

func inlineInput(name, language, content string) (sourceInput, error) {
	kind := source.KindSysML
	switch language {
	case "", "sysml":
	case "kerml":
		kind = source.KindKerML
	default:
		return sourceInput{}, jsonrpc.Errorf(jsonrpc.CodeInvalidArgument,
			"language must be sysml or kerml, got %q", language)
	}
	return sourceInput{name: name, language: language, content: content, kind: kind}, nil
}

func fileInput(path string) (sourceInput, error) {
	// #nosec G304 -- the client names the model file it wants parsed; reading
	// arbitrary paths is the service's purpose, and it runs with the caller's
	// own privileges.
	data, err := os.ReadFile(path)
	if err != nil {
		return sourceInput{}, jsonrpc.Errorf(jsonrpc.CodeNotFound, "file not found: %v", err)
	}
	return sourceInput{name: path, content: string(data), kind: source.KindOf(path)}, nil
}

func (c *Core) parseModel(inputs []sourceInput, mode diag.ConformanceMode) (string, *cachedModel) {
	var key strings.Builder
	fmt.Fprintf(&key, "%s\x00%d", mode.String(), len(inputs))
	for _, input := range inputs {
		for _, field := range []string{input.name, input.language, input.content} {
			fmt.Fprintf(&key, "\x00%d\x00%s", len(field), field)
		}
	}
	modelHash := fmt.Sprintf("%x", sha256.Sum256([]byte(key.String())))
	if model, ok := c.get(modelHash); ok {
		return modelHash, model
	}

	index := symbols.NewOverlay(c.libraryIndex)
	documents := make([]*cachedDocument, 0, len(inputs))
	parsedClean := true
	for _, input := range inputs {
		src := source.NewWithKind(input.name, []byte(input.content), input.kind)
		p := parser.New(src)
		root := p.ParseFile()
		index.AddDocumentWithKind(input.name, root, input.kind)
		if len(p.Diagnostics) > 0 {
			parsedClean = false
		}
		documents = append(documents, &cachedDocument{
			root:        root,
			source:      src,
			diagnostics: parser.AsDiagnostics(p.Diagnostics, p.Warnings),
		})
	}
	index.ExpandWildcardImports()

	if parsedClean {
		names := make([]string, len(inputs))
		for i, input := range inputs {
			names[i] = input.name
		}
		batch := &passes.Batch{Documents: names, Gathers: passes.NewGathers()}
		passes.PrepareBatch(index, batch)
		for i, document := range documents {
			document.diagnostics, _ = passes.AnalyzeInBatch(inputs[i].name, inputs[i].kind,
				document.root, document.diagnostics, index, passes.Options{Conformance: mode}, batch)
		}
	}

	model := &cachedModel{
		documents: documents,
		index:     index,
		library:   c.library,
		mode:      mode,
	}
	return modelHash, c.add(modelHash, model)
}

func (c *Core) get(hash string) (*cachedModel, bool) {
	elem, ok := c.byHash[hash]
	if !ok {
		return nil, false
	}
	c.models.MoveToFront(elem)
	return elem.Value.(*cacheEntry).model, true
}

func (c *Core) add(hash string, model *cachedModel) *cachedModel {
	if elem, ok := c.byHash[hash]; ok {
		c.models.MoveToFront(elem)
		return elem.Value.(*cacheEntry).model
	}
	if c.models.Len() >= maxCachedModels {
		if oldest := c.models.Back(); oldest != nil {
			c.models.Remove(oldest)
			delete(c.byHash, oldest.Value.(*cacheEntry).hash)
		}
	}
	c.byHash[hash] = c.models.PushFront(&cacheEntry{hash: hash, model: model})
	return model
}

func (c *Core) getDiagnostics(req *JDiagnosticsRequest) ([]byte, error) {
	model, ok := c.get(req.ModelHash)
	if !ok {
		return nil, jsonrpc.Errorf(jsonrpc.CodeNotFound, msgModelNotFound, req.ModelHash)
	}
	return json.Marshal(&JDiagnosticsResponse{Diagnostics: modelDiagnostics(model)})
}

func (c *Core) getSymbol(req *JGetSymbolRequest) ([]byte, error) {
	model, ok := c.get(req.ModelHash)
	if !ok {
		return nil, jsonrpc.Errorf(jsonrpc.CodeNotFound, msgModelNotFound, req.ModelHash)
	}
	matches := symbolfacts.LookupNamed(model.index, req.SymbolId)
	if len(matches) == 0 {
		return json.Marshal(&JSymbolResponse{Error: fmt.Sprintf("symbol not found: %s", req.SymbolId)})
	}
	info := symbolfacts.Of(matches[0], model.symbolContext())
	return json.Marshal(&JSymbolResponse{Symbol: symbolInfo(info)})
}

func rootSymbol(model *cachedModel, document *cachedDocument) *JSymbolInfo {
	if document.root == nil {
		return nil
	}
	rootScope := model.index.DocumentRoot(document.source.Name())
	return symbolInfo(symbolfacts.Root(model.symbolContext(), rootScope))
}

func (m *cachedModel) symbolContext() *symbolfacts.Context {
	if m.symbols != nil {
		return m.symbols
	}
	m.symbols = symbolfacts.NewContext(m.index)
	sources := make(map[string]*source.SourceFile, len(m.documents))
	for _, document := range m.documents {
		if document.source != nil {
			sources[document.source.Name()] = document.source
		}
	}
	m.symbols.Semantics.SetSourceText(source.TextOf(sources, libs.Text(m.library)))
	return m.symbols
}

func modelDiagnostics(model *cachedModel) []*JDiagnostic {
	var diagnostics []*JDiagnostic
	for _, document := range model.documents {
		for _, d := range document.diagnostics {
			diagnostics = append(diagnostics, fsyntax.FromDiag(d, document.source))
		}
	}
	return diagnostics
}

func symbolInfo(info *symbolfacts.Info) *JSymbolInfo {
	if info == nil {
		return nil
	}
	out := &JSymbolInfo{
		Id:                        info.Id,
		Name:                      info.Name,
		Kind:                      info.Kind,
		Metadata:                  info.Metadata,
		ChildIds:                  info.ChildIds,
		WithheldLibraryAttributes: info.WithheldLibraryAttributes,
	}
	if t := info.TypeInfo; t != nil {
		out.TypeInfo = &JTypeInfo{
			Declared:        t.Declared,
			ResolvedId:      t.ResolvedId,
			ResolvedKind:    t.ResolvedKind,
			Primitive:       t.Primitive,
			PrimitiveSource: t.PrimitiveSource,
			Quantity:        t.Quantity,
			Unit:            t.Unit,
		}
	}
	if m := info.Multiplicity; m != nil {
		out.Multiplicity = &JMultiplicityInfo{Lower: m.Lower, Upper: m.Upper}
	}
	for _, specialization := range info.Specializations {
		out.Specializations = append(out.Specializations, &JSpecialization{
			Kind:       specialization.Kind,
			Declared:   specialization.Declared,
			TargetId:   specialization.TargetId,
			TargetKind: specialization.TargetKind,
		})
	}
	for _, attribute := range info.Attributes {
		out.Attributes = append(out.Attributes, &JAttributeInfo{
			Name:  attribute.Name,
			Type:  attribute.Type,
			Unit:  attribute.Unit,
			Value: attributeValue(attribute.Value),
		})
	}
	return out
}

func attributeValue(value *symbolfacts.Value) *JValue {
	if value == nil {
		return nil
	}
	if value.String != nil {
		return &JValue{StringValue: value.String}
	}
	if value.Const == nil {
		return nil
	}
	switch value.Const.Kind {
	case semantics.ValInt:
		return &JValue{IntValue: ptr(I64(value.Const.Int))}
	case semantics.ValReal:
		return &JValue{RealValue: ptr(F64(value.Const.Real))}
	case semantics.ValBool:
		return &JValue{BoolValue: ptr(value.Const.Bool)}
	case semantics.ValInfinity:
		return &JValue{Infinity: ptr(true)}
	default:
		return &JValue{Null: ptr("unsupported const kind")}
	}
}

func ptr[T any](v T) *T { return &v }
