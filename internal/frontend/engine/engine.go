// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package engine

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"os"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/objref"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
	fsyntax "github.com/Open-MBEE/OpenSysML/internal/frontend/syntax"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast/astcodec"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// The canonical gRPC status codes an answer is reported under, numbered as
// every transport of the service numbers them.
const (
	codeCanceled           = jsonrpc.CodeCanceled
	codeUnknown            = jsonrpc.CodeUnknown
	codeInvalidArgument    = jsonrpc.CodeInvalidArgument
	codeDeadlineExceeded   = jsonrpc.CodeDeadlineExceeded
	codeNotFound           = jsonrpc.CodeNotFound
	codeResourceExhausted  = jsonrpc.CodeResourceExhausted
	codeFailedPrecondition = jsonrpc.CodeFailedPrecondition
	codeUnimplemented      = jsonrpc.CodeUnimplemented
	codeInternal           = jsonrpc.CodeInternal
)

// Error is a refused call: the canonical status code and its message.
type Error = jsonrpc.Error

func statusError(code uint32, message string) *Error {
	return &Error{Code: code, Message: message}
}

func statusErrorf(code uint32, format string, args ...any) *Error {
	return jsonrpc.Errorf(code, format, args...)
}

// contextError reports a caller-gone context under the codes connect.CodeOf
// maps it to on the gRPC path: Canceled as 1, DeadlineExceeded as 4.
func contextError(err error) *Error {
	if errors.Is(err, context.Canceled) {
		return statusError(codeCanceled, err.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return statusError(codeDeadlineExceeded, err.Error())
	}
	return statusError(codeUnknown, err.Error())
}

// msgModelNotFound formats the not-found status for an unknown model hash.
const msgModelNotFound = "model not found: %s"

// maxCachedModels bounds the parsed models the engine holds; the oldest is
// evicted when a new one arrives past the bound.
const maxCachedModels = 16

// cachedDocument is one parsed document of a model.
type cachedDocument struct {
	Root   *ast.RootNamespace
	Source *source.SourceFile
	// Errors are the parse's errors, warnings its warnings; the engine reports
	// only what the parser found, so both are kept apart.
	Errors   []parser.Diagnostic
	Warnings []parser.Diagnostic
}

// cachedModel holds one parsed model's documents, the index they share, and the
// runtime model built over them.
type cachedModel struct {
	Documents []*cachedDocument
	Index     *symbols.Index
	Library   libs.Source
	Mode      diag.ConformanceMode
	model     *runtime.Model
}

// semantics is the model-derived runtime part as a grpc CachedModel builds one:
// the checker-typed semantic model over one resolver, the inline expression
// parser installed, every document registered as a run's source.
func (m *cachedModel) semantics() *runtime.Model {
	if m.model != nil {
		return m.model
	}
	resolver := resolve.New(m.Index)
	sem := passes.NewTypedModel(resolver)
	sem.SetSourceText(m.sourceText())
	m.model = runtime.NewModel(sem, resolver)
	m.model.SetExpressionParser(parser.ParseOneExpression)
	for _, doc := range m.Documents {
		m.model.RegisterSource(doc.Source)
	}
	return m.model
}

// sourceText reads notation from whichever of the model's documents a span
// belongs to, and behind them from the library files its index holds, for the
// documentation bodies and rendering labels read verbatim, as grpc's
// CachedModel installs it. Nil when there are no sources and no library.
func (m *cachedModel) sourceText() view.SourceText {
	sources := make(map[string]*source.SourceFile, len(m.Documents))
	for _, doc := range m.Documents {
		if doc.Source != nil {
			sources[doc.Source.Name()] = doc.Source
		}
	}
	if len(sources) == 0 && m.Library == nil {
		return nil
	}
	return source.TextOf(sources, libs.Text(m.Library))
}

// primary is the document the model is named by.
func (m *cachedModel) primary() *cachedDocument { return m.Documents[0] }

// primaryRoot is the root scope of the document the model is named by.
func (m *cachedModel) primaryRoot() *symbols.Scope {
	return m.Index.DocumentRoot(m.primary().Source.Name())
}

// document is the source of the model's document named name, nil when none is.
func (m *cachedModel) document(name string) *source.SourceFile {
	if name == "" {
		return nil
	}
	for _, doc := range m.Documents {
		if doc.Source.Name() == name {
			return doc.Source
		}
	}
	return nil
}

// cacheEntry is one held model under its hash.
type cacheEntry struct {
	hash  string
	model *cachedModel
}

// Engine serves the execution RPCs of SysMLService over protojson-shaped JSON.
// Calls are handled sequentially in the caller's goroutine — deterministic and
// free of locking — so an Engine is not safe for concurrent use.
type Engine struct {
	// libIndex is the one frozen standard library index every model's index
	// overlays, built on the first parse.
	libIndex *symbols.Index
	libSrc   libs.Source
	budgets  runtime.Budgets
	// models is the bounded store of parsed models keyed by content hash,
	// oldest at the back.
	models *list.List
	byHash map[string]*list.Element
}

// New builds an engine over the frozen standard library snapshot, under the
// runtime budgets a default sysml-grpc NewService runs with.
func New() (*Engine, error) {
	budgets, err := runtime.BudgetsFromEnv()
	if err != nil {
		return nil, err
	}
	return &Engine{
		budgets: budgets,
		models:  list.New(),
		byHash:  make(map[string]*list.Element),
	}, nil
}

// get returns the model cached under hash, marking it the most recently used.
func (e *Engine) get(hash string) (*cachedModel, bool) {
	elem, ok := e.byHash[hash]
	if !ok {
		return nil, false
	}
	e.models.MoveToFront(elem)
	return elem.Value.(*cacheEntry).model, true
}

// add caches model under hash, evicting the oldest model at the bound.
func (e *Engine) add(hash string, model *cachedModel) {
	if elem, ok := e.byHash[hash]; ok {
		e.models.MoveToFront(elem)
		elem.Value.(*cacheEntry).model = model
		return
	}
	if e.models.Len() >= maxCachedModels {
		if oldest := e.models.Back(); oldest != nil {
			e.models.Remove(oldest)
			delete(e.byHash, oldest.Value.(*cacheEntry).hash)
		}
	}
	e.byHash[hash] = e.models.PushFront(&cacheEntry{hash: hash, model: model})
}

// lib returns the frozen library index for one model to overlay, building it
// once on the first parse.
func (e *Engine) lib() (*symbols.Index, libs.Source) {
	if e.libIndex == nil {
		e.libIndex, e.libSrc = libs.FrozenLibrary()
	}
	return e.libIndex, e.libSrc
}

// Call runs the method named — a method name of SysMLService — on params, the
// protojson request body, and returns the protojson response body. A name the
// engine does not serve is refused Unimplemented, and a request that does not
// decode InvalidArgument, matching what stdiorpc's DiscardUnknown decode means
// by each.
func (e *Engine) Call(ctx context.Context, method string, params []byte) (result []byte, err error) {
	defer func() {
		// A malformed request is InvalidArgument, but no bug of ours should
		// take down the session hosting the engine.
		if r := recover(); r != nil {
			result = nil
			err = &Error{Code: codeInternal, Message: fmt.Sprintf("internal error in %s: %v", method, r)}
		}
	}()
	switch method {
	case "ParseSources":
		var req JParseSourcesRequest
		if err := decode(params, &req); err != nil {
			return nil, err
		}
		return e.parseSources(ctx, &req)
	case "Evaluate":
		var req JEvaluateRequest
		if err := decode(params, &req); err != nil {
			return nil, err
		}
		return e.evaluate(ctx, &req)
	case "Instantiate":
		var req JInstantiateRequest
		if err := decode(params, &req); err != nil {
			return nil, err
		}
		return e.instantiate(ctx, &req)
	case "ExecuteAction":
		var req JExecuteActionRequest
		if err := decode(params, &req); err != nil {
			return nil, err
		}
		return e.executeAction(ctx, &req)
	case "ExecuteState":
		var req JExecuteStateRequest
		if err := decode(params, &req); err != nil {
			return nil, err
		}
		return e.executeState(ctx, &req)
	}
	return nil, statusErrorf(codeUnimplemented, "%s is not served by sysml-engine", method)
}

// decode reads the request body as protojson does over the lowerCamel field
// names: unknown fields are ignored, absent params are the empty request.
func decode(params []byte, into any) *Error {
	return jsonrpc.Decode(params, into)
}

// marshal writes the response; the engine's messages marshal only valid protojson.
func marshal(resp any) ([]byte, error) {
	body, err := json.Marshal(resp)
	if err != nil {
		return nil, statusError(codeInternal, err.Error())
	}
	return body, nil
}

// sourceInput is one document a parse request named, read and ready to parse.
type sourceInput struct {
	name     string
	language string
	content  string
	kind     source.Kind
}

// parseSources parses several documents as one model, so a name one document
// declares resolves in another, mirroring grpc's ParseSources except where the
// engine is deliberately narrower: only the parser's own diagnostics are
// reported (the validation passes do not run), and roots is not populated.
func (e *Engine) parseSources(_ context.Context, req *JParseSourcesRequest) ([]byte, error) {
	if len(req.Documents) == 0 {
		return nil, statusError(codeInvalidArgument, "documents must name at least one document")
	}

	inputs := make([]sourceInput, 0, len(req.Documents))
	named := make(map[string]int, len(req.Documents))
	for position, doc := range req.Documents {
		if doc == nil {
			return nil, statusErrorf(codeInvalidArgument, "document %d is null", position)
		}
		input, err := e.documentInput(doc, position)
		if err != nil {
			return nil, err
		}
		if first, taken := named[input.name]; taken {
			return nil, statusErrorf(codeInvalidArgument,
				"documents %d and %d are both named %q: each document of a model needs its own name",
				first, position, input.name)
		}
		named[input.name] = position
		inputs = append(inputs, input)
	}

	modelHash, model := e.parseModel(inputs, diag.ConformanceModeOf(req.StrictConformance))
	return marshal(&JParseSourcesResponse{
		ModelHash:   modelHash,
		Diagnostics: modelDiagnostics(model),
	})
}

// documentInput reads one document of a ParseSources request. position names an
// inline document the request left unnamed, so two of them stay distinct.
func (e *Engine) documentInput(doc *JSourceDocument, position int) (sourceInput, error) {
	switch {
	case doc.Content != nil:
		name := doc.Name
		if name == "" {
			name = fmt.Sprintf("<content-%d>", position)
		}
		return inlineInput(name, doc.Language, *doc.Content)
	case doc.FilePath != nil:
		return fileInput(*doc.FilePath)
	default:
		return sourceInput{}, statusErrorf(codeInvalidArgument,
			"document %d must carry file_path or content", position)
	}
}

// inlineInput is inline content as a document of the language named, SysML when
// the request named none.
func inlineInput(name, language, content string) (sourceInput, error) {
	kind := source.KindSysML
	switch language {
	case "", "sysml":
	case "kerml":
		kind = source.KindKerML
	default:
		return sourceInput{}, statusErrorf(codeInvalidArgument,
			"language must be sysml or kerml, got %q", language)
	}
	return sourceInput{name: name, language: language, content: content, kind: kind}, nil
}

// fileInput reads a document from the path the client named.
func fileInput(path string) (sourceInput, error) {
	// #nosec G304 -- the client names the model file it wants parsed; reading
	// arbitrary paths is the service's purpose, and it runs with the caller's
	// own privileges.
	data, err := os.ReadFile(path)
	if err != nil {
		return sourceInput{}, statusErrorf(codeNotFound, "file not found: %v", err)
	}
	return sourceInput{name: path, content: string(data), kind: source.KindOf(path)}, nil
}

// parseModel parses the documents into one model and caches it, or returns the
// cached model when these very documents were parsed before, keyed and hashed
// exactly as grpc's parseModel computes them so one request resolves the same
// model on either service. The engine runs no validation pass: the documents
// are parsed, indexed and expanded over the shared library index, and the
// model is ready to evaluate and execute.
func (e *Engine) parseModel(inputs []sourceInput, mode diag.ConformanceMode) (string, *cachedModel) {
	var key strings.Builder
	fmt.Fprintf(&key, "%s\x00%d", mode.String(), len(inputs))
	for _, input := range inputs {
		for _, field := range []string{input.name, input.language, input.content} {
			fmt.Fprintf(&key, "\x00%d\x00%s", len(field), field)
		}
	}
	modelHash := computeHash(key.String())
	if cached, ok := e.get(modelHash); ok {
		return modelHash, cached
	}

	base, library := e.lib()
	idx := symbols.NewOverlay(base)

	documents := make([]*cachedDocument, 0, len(inputs))
	for _, input := range inputs {
		srcFile := source.NewWithKind(input.name, []byte(input.content), input.kind)
		p := parser.New(srcFile)
		root := p.ParseFile()
		idx.AddDocumentWithKind(input.name, root, input.kind)
		documents = append(documents, &cachedDocument{
			Root:     root,
			Source:   srcFile,
			Errors:   p.Diagnostics,
			Warnings: p.Warnings,
		})
	}

	// Registers what a wildcard import re-exports, so a qualified name reaches a
	// symbol another document imported. Only sound once every document is in.
	idx.ExpandWildcardImports()

	model := &cachedModel{Documents: documents, Index: idx, Library: library, Mode: mode}
	e.add(modelHash, model)
	return modelHash, model
}

// newRuntime builds a runtime context over the model under the engine's
// budgets. The engine sets no tool runner: tool-computed behavior is served by
// sysml-grpc, not by a build that can start no process.
func (e *Engine) newRuntime(_ context.Context, model *cachedModel) *runtime.Context {
	rt := runtime.NewContext(model.semantics(), e.budgets.MaxSteps)
	if err := rt.SetBudgets(e.budgets); err != nil {
		// Unreachable: New validated these budgets.
		panic(fmt.Sprintf("engine: invalid budgets: %v", err))
	}
	return rt
}

// schedulePolicy reads a request's schedule field. Empty is the default policy;
// a replay is refused as the service refuses it (a request carries no witness
// file), and an exploring schedule is Unimplemented: exploration is served by
// sysml-grpc, a documented limitation of this product.
func (e *Engine) schedulePolicy(spelling string) (runtime.SchedulePolicy, error) {
	if spelling == "" {
		return runtime.DefaultSchedulePolicy, nil
	}
	if runtime.ReplaySpelling(spelling) {
		return runtime.SchedulePolicy{}, statusError(codeInvalidArgument,
			fmt.Sprintf("invalid scheduling policy %q: a replay follows a witness file of the caller's, which a request does not carry", spelling))
	}
	policy, err := runtime.ParseSchedulePolicy(spelling)
	if err != nil {
		return runtime.SchedulePolicy{}, statusError(codeInvalidArgument, err.Error())
	}
	if _, explores := policy.Exploration(); explores {
		return runtime.SchedulePolicy{}, statusError(codeUnimplemented,
			"exploration is not served by sysml-engine: exploring schedules are served by sysml-grpc")
	}
	return policy, nil
}

// evaluate evaluates a SysML expression in the context of a parsed model, as
// grpc's Evaluate does.
func (e *Engine) evaluate(ctx context.Context, req *JEvaluateRequest) ([]byte, error) {
	cached, ok := e.get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(codeNotFound, msgModelNotFound, req.ModelHash)
	}

	// Parse expression
	exprSource := source.New("<expression>", []byte(req.Expression))
	p := parser.New(exprSource)
	exprNode := p.ParseExpression()

	// Check for parse errors
	if len(p.Diagnostics) > 0 {
		var diags []*JDiagnostic
		for _, d := range p.Diagnostics {
			diags = append(diags, fsyntax.ParserDiagnostic(d, exprSource))
		}
		return marshal(&JEvaluateResponse{
			Diagnostics: diags,
			Error:       "expression parse failed",
		})
	}

	// The request states one expression, so text the parser leaves unread is
	// reported rather than evaluating the prefix it did read (`1 = 1` answering 1).
	if p.Offset() < len(strings.TrimRight(req.Expression, " \t\r\n")) {
		return marshal(&JEvaluateResponse{
			Error: fmt.Sprintf("expression parse failed: unexpected %q after the expression",
				strings.TrimSpace(req.Expression[p.Offset():])),
		})
	}

	// A subject is both what the expression is evaluated against and, unless a
	// context is named, the namespace its features are named in — the way the
	// prompt evaluates in the context it pinned.
	var subject *symbols.Symbol
	if req.SubjectSymbolId != "" {
		syms := lookupNamed(cached.Index, req.SubjectSymbolId)
		if len(syms) == 0 {
			return marshal(&JEvaluateResponse{
				Error: fmt.Sprintf("subject not found: %s", req.SubjectSymbolId),
			})
		}
		subject = syms[0]
	}

	// Determine scope
	var scope *symbols.Scope
	if req.ContextSymbolId != "" {
		syms := lookupNamed(cached.Index, req.ContextSymbolId)
		if len(syms) > 0 && syms[0].Scope != nil {
			scope = syms[0].Scope
		}
	}
	if scope == nil && subject != nil {
		scope = evalScope(subject, cached)
	}
	if scope == nil {
		// Use document root as default scope
		scope = cached.primaryRoot()
	}

	runtimeCtx := e.newRuntime(ctx, cached)

	var self *runtime.Instance
	if subject != nil {
		inst, err := runtimeCtx.Instantiate(subject)
		if err != nil {
			return marshal(&JEvaluateResponse{
				Error: fmt.Sprintf("instantiation of subject %s failed: %v", req.SubjectSymbolId, err),
			})
		}
		self = inst
	}

	// The expression is the request's, not the model's: the worker keeps what it
	// resolved of the model, not what it memoized about the expression's nodes.
	evalCtx := runtime.NewEvalContextIn(runtimeCtx, scope, self)
	var result runtime.Value
	var err error
	runtimeCtx.Resolver().Scratch(astcodec.Reachable(exprNode), func() { result, err = evalCtx.Eval(exprNode) })
	if err != nil {
		return marshal(&JEvaluateResponse{
			Error: fmt.Sprintf("evaluation failed: %v", err),
		})
	}

	return marshal(&JEvaluateResponse{
		Result: valueToProto(runtimeCtx, result, cached.Index),
	})
}

// evalScope is the namespace a subject's features are named in: its own scope,
// so a member is named unqualified, else the scope it was declared in.
func evalScope(sym *symbols.Symbol, cached *cachedModel) *symbols.Scope {
	switch {
	case sym == nil:
		return nil
	case sym.Scope != nil:
		return sym.Scope
	case sym.OwnerScope != nil:
		return sym.OwnerScope
	default:
		return cached.primaryRoot()
	}
}

// instantiate creates a runtime instance of a part/usage and serializes its
// graph, as grpc's Instantiate does — except the engine holds no objects: the
// graph the response carries is all that survives the call.
func (e *Engine) instantiate(ctx context.Context, req *JInstantiateRequest) ([]byte, error) {
	cached, ok := e.get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(codeNotFound, msgModelNotFound, req.ModelHash)
	}

	syms := lookupNamed(cached.Index, req.SymbolId)
	if len(syms) == 0 {
		return marshal(&JInstantiateResponse{
			Error: fmt.Sprintf("symbol not found: %s", req.SymbolId),
		})
	}
	sym := syms[0]

	runtimeCtx := e.newRuntime(ctx, cached)

	var graph instanceGraph
	_, err := runtimeCtx.InstantiateRead(sym, func(inst *runtime.Instance) error {
		graph = instanceGraphToProto(runtimeCtx, inst, cached.Index)
		for _, err := range graph.Errors {
			if errors.Is(err, runtime.ErrInstanceLimitExceeded) {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return marshal(&JInstantiateResponse{
			Error: fmt.Sprintf("instantiation failed: %v", err),
		})
	}
	return marshal(&JInstantiateResponse{
		Instance:  graph.Root,
		Instances: graph.All,
	})
}

// performer is the object a request's behavior is performed by, made on rt: the
// part/usage it named or the one a declaration-rooted path reaches; none when unnamed.
func (e *Engine) performer(cached *cachedModel, rt *runtime.Context, symbolID string) (*runtime.Instance, error) {
	return e.namedInstance(cached, rt, "performer", symbolID)
}

// namedInstance is the object symbolID names in the role given: an object of
// the declaration it names, or the one a declaration-rooted path reaches,
// spelled with `.` or `::` as the CLI reads it.
func (e *Engine) namedInstance(cached *cachedModel, rt *runtime.Context, role, symbolID string) (*runtime.Instance, error) {
	if symbolID == "" {
		return nil, nil
	}
	if ref, err := objref.Parse(symbolID); objref.LooksLikePath(symbolID) || err == nil && len(ref.Segments) > 1 {
		return e.objectAt(cached, rt, role, symbolID)
	}
	syms := lookupNamed(cached.Index, symbolID)
	if len(syms) == 0 {
		return nil, fmt.Errorf("symbol not found: %s", symbolID)
	}
	inst, err := rt.Instantiate(syms[0])
	if err != nil {
		return nil, fmt.Errorf("instantiation of %s %s failed: %w", role, symbolID, err)
	}
	return inst, nil
}

// objectAt is the object a declaration-rooted path names: the longest leading
// run of segments naming a declaration is instantiated, the rest walked from
// it; an id names none.
func (e *Engine) objectAt(cached *cachedModel, rt *runtime.Context, role, path string) (*runtime.Instance, error) {
	ref, err := objref.Parse(path)
	if err != nil {
		return nil, err
	}
	if ref.ID > 0 {
		return nil, fmt.Errorf("%s %s names an object by id, which a call creates none of: name a declaration, or a path from one such as mission.vehicle", role, path)
	}
	for i := objref.Head(ref.Segments); i > 0; i-- {
		name := objref.JoinTyped(ref.Segments[:i])
		syms := lookupNamed(cached.Index, name)
		if len(syms) == 0 {
			continue
		}
		if i > 1 && len(lookupNamed(cached.Index, objref.DeclaredRun(ref.Segments[:i]))) == 0 {
			continue
		}
		if objref.IsNamespace(syms[0]) {
			break
		}
		root, err := rt.Instantiate(syms[0])
		if err != nil {
			return nil, fmt.Errorf("instantiation of %s %s failed: %w", role, name, err)
		}
		walker := objref.Walker{Runtime: rt, Index: cached.Index}
		inst, _, err := walker.Walk(root, name, ref.Segments[i:])
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", role, path, err)
		}
		return inst, nil
	}
	return nil, fmt.Errorf("symbol not found: %s", objref.JoinTyped(ref.Segments[:max(objref.Head(ref.Segments), 1)]))
}

// executeAction executes an action definition, as grpc's non-exploring
// ExecuteAction does, run on the runtime directly rather than through the
// analysis framework this product does not carry.
func (e *Engine) executeAction(ctx context.Context, req *JExecuteActionRequest) ([]byte, error) {
	schedule, err := e.schedulePolicy(req.Schedule)
	if err != nil {
		return nil, err
	}

	cached, ok := e.get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(codeNotFound, msgModelNotFound, req.ModelHash)
	}

	syms := lookupNamed(cached.Index, req.ActionSymbolId)
	if len(syms) == 0 {
		return marshal(&JExecuteActionResponse{
			Error: fmt.Sprintf("action not found: %s", req.ActionSymbolId),
		})
	}
	action := syms[0]

	runtimeCtx := e.newRuntime(ctx, cached)

	// Converted against the model's index, so a quantity input keeps the base
	// units it is commensurable with instead of binding an unusable value.
	var inputs map[string]runtime.Value
	if len(req.Inputs) > 0 {
		inputs = make(map[string]runtime.Value, len(req.Inputs))
		for name, pv := range req.Inputs {
			val, cerr := protoToRuntimeValue(runtimeCtx, pv, cached.Index, runtimeCtx.Semantics())
			if cerr != nil {
				return marshal(&JExecuteActionResponse{
					Error: fmt.Sprintf("input %q could not be read: %v", name, cerr),
				})
			}
			inputs[name] = val
		}
	}

	if err := runtimeCtx.SetSchedule(schedule); err != nil {
		return nil, statusError(codeInvalidArgument, err.Error())
	}
	self, err := e.performer(cached, runtimeCtx, req.PerformerSymbolId)
	if err != nil {
		return marshal(&JExecuteActionResponse{Error: err.Error()})
	}

	// Execute action with the supplied inputs
	outputs, performer, err := runtimeCtx.ExecuteActionReportingPerformer(action, self, inputs)
	if err != nil {
		if gone := ctx.Err(); gone != nil {
			return nil, contextError(gone)
		}
	}
	// The choices the run made are reported with its outcome, failed or not: a
	// failure may hang on the order taken.
	diags := runNoteDiagnosticsToProto(runtimeCtx.Notes(), cached)
	if err != nil {
		return marshal(&JExecuteActionResponse{
			Error:       fmt.Sprintf("action execution failed: %v", err),
			Diagnostics: diags,
			FinalTime:   F64(runtimeCtx.Clock().Now()),
		})
	}

	return marshal(&JExecuteActionResponse{
		Outputs:             valuesToProto(runtimeCtx, outputs, cached.Index),
		Diagnostics:         diags,
		FinalTime:           F64(runtimeCtx.Clock().Now()),
		PerformerAttributes: valuesToProto(runtimeCtx, performer, cached.Index),
	})
}

// executeState executes a state machine, as grpc's non-exploring ExecuteState
// does.
func (e *Engine) executeState(ctx context.Context, req *JExecuteStateRequest) ([]byte, error) {
	schedule, err := e.schedulePolicy(req.Schedule)
	if err != nil {
		return nil, err
	}

	cached, ok := e.get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(codeNotFound, msgModelNotFound, req.ModelHash)
	}

	syms := lookupNamed(cached.Index, req.StateMachineSymbolId)
	if len(syms) == 0 {
		return marshal(&JExecuteStateResponse{
			Error: fmt.Sprintf("state machine not found: %s", req.StateMachineSymbolId),
		})
	}
	stateMachine := syms[0]

	runtimeCtx := e.newRuntime(ctx, cached)
	if err := runtimeCtx.SetSchedule(schedule); err != nil {
		return nil, statusError(codeInvalidArgument, err.Error())
	}
	self, err := e.performer(cached, runtimeCtx, req.PerformerSymbolId)
	if err != nil {
		return marshal(&JExecuteStateResponse{Error: err.Error()})
	}

	outcome, err := runtimeCtx.StateOutcomePerformedBy(stateMachine, self, req.Events)
	if err != nil {
		if gone := ctx.Err(); gone != nil {
			return nil, contextError(gone)
		}
	}
	finalContext, statesVisited := outcome.Outputs, outcome.StateVisits
	diags := runNoteDiagnosticsToProto(runtimeCtx.Notes(), cached)
	if err != nil {
		return marshal(&JExecuteStateResponse{
			Error:       fmt.Sprintf("state machine execution failed: %v", err),
			Diagnostics: diags,
			FinalTime:   F64(runtimeCtx.Clock().Now()),
		})
	}

	return marshal(&JExecuteStateResponse{
		StatesVisited: statesVisited,
		FinalContext:  valuesToProto(runtimeCtx, finalContext, cached.Index),
		Diagnostics:   diags,
		FinalTime:     F64(runtimeCtx.Clock().Now()),
	})
}

// valuesToProto converts each value of a named map.
func valuesToProto(rt *runtime.Context, values map[string]runtime.Value, idx *symbols.Index) map[string]*JValue {
	out := make(map[string]*JValue, len(values))
	for name, value := range values {
		out[name] = valueToProto(rt, value, idx)
	}
	return out
}

// modelDiagnostics is every document's parser diagnostics, document by
// document, each located in the source it came from — the engine reports no
// validation pass, so this is the syntax tier alone: errors as the passes
// would report them, warnings as warnings rather than as errors.
func modelDiagnostics(model *cachedModel) []*JDiagnostic {
	var diags []*JDiagnostic
	for _, doc := range model.Documents {
		for _, d := range doc.Errors {
			diags = append(diags, fsyntax.ParserDiagnostic(d, doc.Source))
		}
		for _, d := range parser.AsDiagnostics(nil, doc.Warnings) {
			diags = append(diags, fsyntax.FromDiag(d, doc.Source))
		}
	}
	return diags
}

// runNoteDiagnosticsToProto converts what a run noted about itself — its choice
// points and the guards it could not evaluate — to informational diagnostics,
// located in their model document; one outside the model carries no span.
func runNoteDiagnosticsToProto(notes []runtime.RunNote, model *cachedModel) []*JDiagnostic {
	if len(notes) == 0 {
		return nil
	}
	diags := make([]*JDiagnostic, 0, len(notes))
	for _, n := range notes {
		d := n.Diagnostic()
		file, _ := n.Location()
		if sf := model.document(file); sf != nil {
			diags = append(diags, fsyntax.FromDiag(d, sf))
			continue
		}
		diags = append(diags, &JDiagnostic{Severity: d.Severity.String(), Message: d.Message, Code: d.Code})
	}
	return diags
}

// computeHash generates SHA-256 hash of content
func computeHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// lookupNamed resolves a symbol ID written in either spelling: the quoted,
// notation-legal form a model author writes ('My Pkg'::Car), or the unquoted
// spelling the index records (My Pkg::Car), which keeps working as it did.
// Model elements come before library homonyms.
func lookupNamed(idx *symbols.Index, id string) []*symbols.Symbol {
	if syms := idx.LookupQualified(id); len(syms) > 0 {
		return modelFirst(idx, syms)
	}
	if plain, ok := unquotedName(id); ok && plain != id {
		return modelFirst(idx, idx.LookupQualified(plain))
	}
	return nil
}

// modelFirst reorders matches so the ones the model declares precede the ones
// standard-library content declares, each group keeping its index order.
func modelFirst(idx *symbols.Index, syms []*symbols.Symbol) []*symbols.Symbol {
	out := make([]*symbols.Symbol, 0, len(syms))
	var lib []*symbols.Symbol
	for _, sym := range syms {
		if idx.Library(sym) {
			lib = append(lib, sym)
			continue
		}
		out = append(out, sym)
	}
	return append(out, lib...)
}

// unquotedName is the name a notation-legal qualified name states, with the
// quoting of its unrestricted segments removed, or false for an ID the notation
// does not read as one whole name.
func unquotedName(id string) (string, bool) {
	if id == "" {
		return "", false
	}
	p := parser.New(source.New("<symbol-id>", []byte(id)))
	expr := p.ParseExpression()
	if len(p.Diagnostics) > 0 || p.Offset() != len(id) {
		return "", false
	}
	ref, ok := expr.(*ast.FeatureReference)
	if !ok || ref.Name == nil || ref.Name.Global || len(ref.Name.Parts) == 0 {
		return "", false
	}
	segments := make([]string, 0, len(ref.Name.Parts))
	for _, part := range ref.Name.Parts {
		if part.Text == "" {
			return "", false
		}
		segments = append(segments, part.Text)
	}
	return strings.Join(segments, "::"), true
}
