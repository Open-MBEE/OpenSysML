package grpc

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/ast/astcodec"
	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// msgModelNotFound formats the not-found status for an unknown model hash.
const msgModelNotFound = "model not found: %s"

// CapabilityTypeFacts names the capability of populating SymbolInfo.type_info,
// .multiplicity and .specializations, which typed code generation requires.
const CapabilityTypeFacts = "type_facts"

// CapabilityConvert names the capability of the Convert RPC, which writes a
// model back out as SysML notation or RDF Turtle. The RDF direction is
// experimental, which the response says per conversion.
const CapabilityConvert = "convert"

// CapabilityVerification names the capability of the verification RPCs, which
// answer the questions the REPL's %constraint, %requirement, %satisfy and %calc
// answer.
const CapabilityVerification = "verification"

// CapabilityQuery names the capability of the Query RPC, which evaluates a
// SysML v2 API & Services Query over a parsed model.
const CapabilityQuery = "query"

// CapabilityDocumentQuery names the capability of the RunDocumentQuery RPC,
// which runs a named document query and answers with typed rows.
const CapabilityDocumentQuery = "document_query"

// CapabilityRenderDocument names the capability of the RenderDocument RPC,
// which renders a named document to Markdown.
const CapabilityRenderDocument = "render_document"

// CapabilityOSLCQuery names the capability of evaluating OSLC Query text.
const CapabilityOSLCQuery = "oslc_query"

// CapabilityEnumValues names the capability of carrying an enumeration literal
// as Value.enum_literal, rather than reporting it as an unsupported null.
const CapabilityEnumValues = "enum_values"

// CapabilityEvaluateSubject names the capability of evaluating an expression
// against an instantiated subject.
const CapabilityEvaluateSubject = "evaluate_subject"

// CapabilitySymbolAttributes names the capability of populating
// SymbolInfo.attributes, rather than always reporting none.
const CapabilitySymbolAttributes = "symbol_attributes"

// CapabilityUnsetValue names the capability of reporting a valueless feature of
// a value type as Value.unset, rather than as the empty object it materializes.
const CapabilityUnsetValue = "unset_value"

// CapabilityApplyEdits names the capability of the ApplyEdits RPC, which edits
// a parsed model's own source, preserving everything the edit did not touch.
const CapabilityApplyEdits = "apply_edits"

// CapabilityAuthoring names add-member and delete source authoring operations.
const CapabilityAuthoring = "authoring"

// CapabilityInlineLanguage names explicit language selection for inline content.
const CapabilityInlineLanguage = "inline_language"

// CapabilityStrictConformance names ParseFileRequest.strict_conformance, which
// asks whether the source is conforming SysML v2.
const CapabilityStrictConformance = "strict_conformance"

// CapabilityFeatureValues names the capability of populating
// Instance.feature_values, which replaced the pre-0.1.0 Instance.slots.
const CapabilityFeatureValues = "feature_values"

// CapabilityParseSources names the ParseSources RPC, which parses several
// documents as one model so a name one declares resolves in another.
const CapabilityParseSources = "parse_sources"

// CapabilityComplexValues names the capability of carrying a complex number as
// Value.complex, rather than reporting it as an unsupported null.
const CapabilityComplexValues = "complex_values"

// CapabilityStructuredValues names the capability of carrying an array, a
// vector and a vector quantity as Value.array, Value.vector and
// Value.vector_quantity, rather than reporting them as unsupported nulls.
const CapabilityStructuredValues = "structured_values"

// CapabilityMeasurementRefs names the capability of carrying a bare measurement
// reference as Value.measurement_ref, rather than reporting it as an
// unsupported null. Distinct from structured_values, which predates the arm.
const CapabilityMeasurementRefs = "measurement_refs"

// CapabilityFunctionValues names the capability of carrying a calc held as a
// value as Value.function, named by its declaration, rather than reporting it
// as an unsupported null.
const CapabilityFunctionValues = "function_values"

// CapabilitySetValues names the capability of carrying a unique, unordered
// collection as Value.set, rather than reporting it as an unsupported null.
const CapabilitySetValues = "set_values"

// CapabilityTensorValues names the capability of carrying a tensor quantity of
// any rank as Value.tensor_quantity, rather than as an unsupported null.
const CapabilityTensorValues = "tensor_values"

// CapabilityVerificationVerdicts names the capability of reporting what the body
// of a verification case answered as VerificationVerdict, and of running a
// verification case through RunAnalysis.
const CapabilityVerificationVerdicts = "verification_verdicts"

// CapabilityInfinityValue names the capability of carrying the unbounded value
// `*` as Value.infinity, rather than reporting it as an unsupported null.
const CapabilityInfinityValue = "infinity_value"

// CapabilityDiagnosticCodes names the capability of populating Diagnostic.code,
// so an empty code is a finding none was assigned rather than an older service.
const CapabilityDiagnosticCodes = "diagnostic_codes"

// CapabilitySchedule names the `schedule` field of ExecuteActionRequest,
// ExecuteStateRequest and RunAnalysisRequest, which selects the scheduling
// policy a run resolves its choice points under. A service without it runs
// every request under the default, so a client must not send the field to one.
const CapabilitySchedule = "schedule"

// CapabilityCaseEvaluations names the capability of reporting each application
// an analysis run made of one of the case's calcs as a function value — a trade
// study's evaluation of each alternative — and of keeping what a failed run
// computed beside its error.
const CapabilityCaseEvaluations = "case_evaluations"

// CapabilityScheduleExplore names the capability of answering a request under
// `explore[:runs=<n>,depth=<d>]` with `outcomes` and an `exploration` status.
const CapabilityScheduleExplore = "schedule_explore"

// CapabilityFinalTime names the capability of populating final_time on
// ExecuteActionResponse and ExecuteStateResponse, the run's simulation clock
// when it ended. Without it the field is 0 whatever the run waited on.
const CapabilityFinalTime = "final_time"

// CapabilityMetaobjectValues names the capability of carrying an element
// reflected on as an instance of its metaclass (`x meta T`, the last element of
// `x.metadata`) as Value.metaobject, rather than as an unsupported null.
const CapabilityMetaobjectValues = "metaobject_values"

// capabilities is what this build supports, in report order. A capability is
// only ever added: renaming or dropping one breaks clients that require it.
var capabilities = []string{
	CapabilityTypeFacts, CapabilityConvert, CapabilityVerification, CapabilityQuery,
	CapabilityOSLCQuery, CapabilityEnumValues, CapabilityEvaluateSubject,
	CapabilitySymbolAttributes, CapabilityUnsetValue, CapabilityFeatureValues,
	CapabilityApplyEdits, CapabilityAuthoring, CapabilityInlineLanguage,
	CapabilityStrictConformance, CapabilityDocumentQuery, CapabilityRenderDocument,
	CapabilityParseSources, CapabilityComplexValues, CapabilityStructuredValues,
	CapabilityMeasurementRefs, CapabilityFunctionValues, CapabilitySetValues,
	CapabilityTensorValues, CapabilityVerificationVerdicts, CapabilityInfinityValue,
	CapabilityDiagnosticCodes, CapabilitySchedule, CapabilityCaseEvaluations,
	CapabilityScheduleExplore, CapabilityFinalTime, CapabilityEngines,
	CapabilityMetaobjectValues,
}

type capabilityAvailability struct {
	available map[string]struct{}
}

func newCapabilityAvailability(withheld []string) (capabilityAvailability, error) {
	available := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		available[capability] = struct{}{}
	}
	for _, capability := range withheld {
		if _, ok := available[capability]; !ok {
			return capabilityAvailability{}, fmt.Errorf("unknown capability %q", capability)
		}
		delete(available, capability)
	}
	return capabilityAvailability{available: available}, nil
}

func (a capabilityAvailability) has(capability string) bool {
	_, ok := a.available[capability]
	return ok
}

func (a capabilityAvailability) names() []string {
	names := make([]string, 0, len(a.available))
	for _, capability := range capabilities {
		if a.has(capability) {
			names = append(names, capability)
		}
	}
	return names
}

// Capabilities returns the capability names this build of the service reports.
func Capabilities() []string {
	return append([]string(nil), capabilities...)
}

// Service implements SysMLServiceServer
type Service struct {
	pb.UnimplementedSysMLServiceServer
	cache *Cache
	// libIndexes hands each model an overlay over the one standard library index
	// the service holds: the library does not depend on the model, so no model
	// pays for loading it, and no model keeps a copy of it.
	libIndexes *libraryBase
	// prewarm is whether Prewarm builds the library ahead of the first request.
	prewarm bool
	// budgets bounds every runtime context the service creates, read once from
	// the environment at construction.
	budgets runtime.Budgets
	// jobs is how many runs of one plan go concurrently, OPENSYSML_JOBS read once at
	// construction; no request sets it.
	jobs int
	// engines answers every analysis question the runtime RPCs put, under auto.
	engines *analysis.Registry
	// version is the build version GetServerInfo reports, informational only.
	version string
	// capabilities decides both what this service reports and what it supplies.
	capabilities capabilityAvailability
}

// NewService creates a gRPC service with specified cache size, reporting
// version as its build version. It returns an error if cacheSize is not
// positive, if a budget variable or OPENSYSML_JOBS holds anything but a positive
// integer, or if the prewarm setting is not a non-negative integer. It does not load the
// standard library: call Prewarm to have that happen in the background, ahead of
// the requests that need it.
func NewService(cacheSize int, version string) (*Service, error) {
	return newService(cacheSize, version, nil)
}

// NewServiceWithUnavailableCapabilitiesForTesting creates a service that
// deliberately lacks named capabilities for conformance testing.
func NewServiceWithUnavailableCapabilitiesForTesting(cacheSize int, version string, unavailable []string) (*Service, error) {
	return newService(cacheSize, version, unavailable)
}

func newService(cacheSize int, version string, unavailable []string) (*Service, error) {
	cache, err := NewCache(cacheSize)
	if err != nil {
		return nil, err
	}
	availability, err := newCapabilityAvailability(unavailable)
	if err != nil {
		return nil, err
	}
	budgets, err := runtime.BudgetsFromEnv()
	if err != nil {
		return nil, err
	}
	jobs, err := analysis.JobsFromEnv()
	if err != nil {
		return nil, err
	}
	prewarm, err := indexPrewarmFromEnv()
	if err != nil {
		return nil, err
	}
	engines, err := analysis.DefaultFromEnv()
	if err != nil {
		return nil, err
	}
	return &Service{
		cache:        cache,
		libIndexes:   newLibraryBase(buildLibraryIndex),
		prewarm:      prewarm > 0,
		budgets:      budgets,
		jobs:         jobs,
		engines:      engines,
		version:      version,
		capabilities: availability,
	}, nil
}

// Prewarm starts building the service's standard library index in the
// background, so the first model to arrive is not the one that pays for the
// library. It returns immediately and is safe to call more than once.
func (s *Service) Prewarm() {
	if !s.prewarm {
		return
	}
	s.libIndexes.prewarm()
}

// Close waits for a background library build in flight and releases the
// service's reference to the shared index. The service still answers
// afterwards, building the library on the request that needs it.
func (s *Service) Close() {
	s.libIndexes.close()
}

// GetServerInfo reports the service's build version and capabilities. A service
// too old to have this RPC fails the call with UNIMPLEMENTED, which is itself
// the answer that it predates every capability.
func (s *Service) GetServerInfo(ctx context.Context, req *pb.ServerInfoRequest) (*pb.ServerInfoResponse, error) {
	return &pb.ServerInfoResponse{
		Version:      s.version,
		Capabilities: s.capabilities.names(),
	}, nil
}

func (s *Service) requireCapability(capability string) error {
	if s.capabilities.has(capability) {
		return nil
	}
	return statusErrorf(connect.CodeUnimplemented, "capability %q is unavailable", capability)
}

// requireValueCapabilities refuses a supplied value of a kind whose capability
// is unavailable, rather than reading it as something else.
func (s *Service) requireValueCapabilities(pv *pb.Value) error {
	if ValueCarriesComplex(pv) {
		if err := s.requireCapability(CapabilityComplexValues); err != nil {
			return err
		}
	}
	if ValueCarriesStructured(pv) {
		if err := s.requireCapability(CapabilityStructuredValues); err != nil {
			return err
		}
	}
	if ValueCarriesMeasurementRef(pv) {
		if err := s.requireCapability(CapabilityMeasurementRefs); err != nil {
			return err
		}
	}
	if ValueCarriesFunction(pv) {
		if err := s.requireCapability(CapabilityFunctionValues); err != nil {
			return err
		}
	}
	if ValueCarriesInfinity(pv) {
		if err := s.requireCapability(CapabilityInfinityValue); err != nil {
			return err
		}
	}
	if ValueCarriesSet(pv) {
		if err := s.requireCapability(CapabilitySetValues); err != nil {
			return err
		}
	}
	if ValueCarriesTensor(pv) {
		if err := s.requireCapability(CapabilityTensorValues); err != nil {
			return err
		}
	}
	if ValueCarriesMetaobject(pv) {
		return s.requireCapability(CapabilityMetaobjectValues)
	}
	return nil
}

// newRuntime returns a runtime context under the service's budgets on a worker of the
// request's own, so requests on one model run beside each other and share nothing mutable.
func (s *Service) newRuntime(cached *CachedModel) *runtime.Context {
	return s.newRuntimeOver(cached.worker())
}

// newRuntimeOver builds a runtime context under the service's budgets on a worker;
// every explored run gets one of its own.
func (s *Service) newRuntimeOver(w *analysis.Worker) *runtime.Context {
	ctx := runtime.NewContext(w.Model, s.budgets.MaxSteps)
	if err := ctx.SetBudgets(s.budgets); err != nil {
		// Unreachable: NewService validated these budgets.
		panic(fmt.Sprintf("grpc: invalid service budgets: %v", err))
	}
	return ctx
}

// model is the cached model as the engines reach it: a worker per plan over the shared
// index, a context per run under the service's budgets.
func (s *Service) model(cached *CachedModel) *analysis.Model {
	return &analysis.Model{
		Semantics: cached.Semantics,
		Fresh:     func(w *analysis.Worker) (*runtime.Context, error) { return s.newRuntimeOver(w), nil },
	}
}

// schedulePolicy reads a request's schedule field. Empty is the default policy;
// anything else needs the schedule capability and must spell a policy, and an
// exploring one needs the schedule_explore capability too.
func (s *Service) schedulePolicy(spelling string) (runtime.SchedulePolicy, error) {
	if spelling == "" {
		return runtime.DefaultSchedulePolicy, nil
	}
	if err := s.requireCapability(CapabilitySchedule); err != nil {
		return runtime.SchedulePolicy{}, err
	}
	if runtime.ReplaySpelling(spelling) {
		return runtime.SchedulePolicy{}, statusError(connect.CodeInvalidArgument,
			fmt.Sprintf("invalid scheduling policy %q: a replay follows a witness file of the caller's, which a request does not carry", spelling))
	}
	policy, err := runtime.ParseSchedulePolicy(spelling)
	if err != nil {
		return runtime.SchedulePolicy{}, statusError(connect.CodeInvalidArgument, err.Error())
	}
	if _, explores := policy.Exploration(); explores {
		if err := s.requireCapability(CapabilityScheduleExplore); err != nil {
			return runtime.SchedulePolicy{}, err
		}
	}
	return policy, nil
}

// sourceInput is one document a parse request named, read and ready to parse.
type sourceInput struct {
	name     string
	language string
	content  string
	kind     source.Kind
}

// ParseFile parses a SysML file and caches the result
func (s *Service) ParseFile(ctx context.Context, req *pb.ParseFileRequest) (*pb.ParseFileResponse, error) {
	if req.StrictConformance {
		if err := s.requireCapability(CapabilityStrictConformance); err != nil {
			return nil, err
		}
	}
	if _, inline := req.Source.(*pb.ParseFileRequest_Content); inline && req.Language != "" {
		if err := s.requireCapability(CapabilityInlineLanguage); err != nil {
			return nil, err
		}
	}

	var input sourceInput
	var err error
	switch src := req.Source.(type) {
	case *pb.ParseFileRequest_Content:
		input, err = inlineInput("<content>", req.Language, src.Content)
	case *pb.ParseFileRequest_FilePath:
		input, err = fileInput(src.FilePath)
	default:
		return nil, statusError(connect.CodeInvalidArgument, "source must be file_path or content")
	}
	if err != nil {
		return nil, err
	}

	mode := conformance.ModeOf(req.StrictConformance)
	modelHash, model := s.parseModel([]sourceInput{input}, mode)
	return s.buildParseResponse(modelHash, model), nil
}

// ParseSources parses several documents as one model, so a name one document
// declares resolves in another.
func (s *Service) ParseSources(ctx context.Context, req *pb.ParseSourcesRequest) (*pb.ParseSourcesResponse, error) {
	if err := s.requireCapability(CapabilityParseSources); err != nil {
		return nil, err
	}
	if req.StrictConformance {
		if err := s.requireCapability(CapabilityStrictConformance); err != nil {
			return nil, err
		}
	}
	if len(req.Documents) == 0 {
		return nil, statusError(connect.CodeInvalidArgument, "documents must name at least one document")
	}

	inputs := make([]sourceInput, 0, len(req.Documents))
	named := make(map[string]int, len(req.Documents))
	for position, doc := range req.Documents {
		input, err := s.documentInput(doc, position)
		if err != nil {
			return nil, err
		}
		if first, taken := named[input.name]; taken {
			return nil, statusErrorf(connect.CodeInvalidArgument,
				"documents %d and %d are both named %q: each document of a model needs its own name",
				first, position, input.name)
		}
		named[input.name] = position
		inputs = append(inputs, input)
	}

	modelHash, model := s.parseModel(inputs, conformance.ModeOf(req.StrictConformance))
	roots := make([]*pb.SymbolInfo, 0, len(model.Documents))
	for _, doc := range model.Documents {
		roots = append(roots, s.rootSymbol(model, doc))
	}
	return &pb.ParseSourcesResponse{
		ModelHash:   modelHash,
		Roots:       roots,
		Diagnostics: s.modelDiagnostics(model),
	}, nil
}

// documentInput reads one document of a ParseSources request. position names an
// inline document the request left unnamed, so two of them stay distinct.
func (s *Service) documentInput(doc *pb.SourceDocument, position int) (sourceInput, error) {
	switch src := doc.GetSource().(type) {
	case *pb.SourceDocument_Content:
		if doc.Language != "" {
			if err := s.requireCapability(CapabilityInlineLanguage); err != nil {
				return sourceInput{}, err
			}
		}
		name := doc.Name
		if name == "" {
			name = fmt.Sprintf("<content-%d>", position)
		}
		return inlineInput(name, doc.Language, src.Content)
	case *pb.SourceDocument_FilePath:
		return fileInput(src.FilePath)
	default:
		return sourceInput{}, statusErrorf(connect.CodeInvalidArgument,
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
		return sourceInput{}, statusErrorf(connect.CodeInvalidArgument,
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
		return sourceInput{}, statusErrorf(connect.CodeNotFound, "file not found: %v", err)
	}
	return sourceInput{name: path, content: string(data), kind: source.KindOf(path)}, nil
}

// parseModel parses the documents into one model and caches it, or returns the
// cached model when these very documents were parsed before. Every document goes
// into one index, so a name one declares resolves in the others.
//
// Semantic analysis runs only when every document parsed clean, per the tier
// gating in AGENTS.md §4: a document that failed to parse contributes no symbols,
// so analyzing its siblings would report names as unresolved that the model
// declares.
func (s *Service) parseModel(inputs []sourceInput, mode conformance.Mode) (string, *CachedModel) {
	// Keyed by what was read, not by the hash a request carried: a hash
	// disagreeing with its content would serve another model. Each document's
	// name is part of the key, since its diagnostics name the document they came
	// from. The conformance mode is part of the key: the same source asks a
	// different question in each mode, so one mode's diagnostics may not serve
	// the other. Every field is length-delimited, so no document set can spell
	// the key of another whose names or contents carry the delimiter.
	var key strings.Builder
	fmt.Fprintf(&key, "%s\x00%d", mode.String(), len(inputs))
	for _, input := range inputs {
		for _, field := range []string{input.name, input.language, input.content} {
			fmt.Fprintf(&key, "\x00%d\x00%s", len(field), field)
		}
	}
	modelHash := computeHash(key.String())
	if cached, ok := s.cache.Get(modelHash); ok {
		return modelHash, cached
	}

	// Take an index carrying the standard library, which type resolution needs:
	// an overlay over the one library index the service holds. The model's own
	// documents go into the overlay alone, so what they resolve against does not
	// depend on prewarming or on any other model.
	idx, library := s.libIndexes.get()

	documents := make([]*CachedDocument, 0, len(inputs))
	parsedClean := true
	for _, input := range inputs {
		srcFile := source.NewWithKind(input.name, []byte(input.content), input.kind)
		p := parser.New(srcFile)
		root := p.ParseFile()
		idx.AddDocumentWithKind(input.name, root, input.kind)
		if len(p.Diagnostics) > 0 {
			parsedClean = false
		}
		documents = append(documents, &CachedDocument{
			Root:       root,
			Source:     srcFile,
			ParseDiags: p.Diagnostics,
		})
	}

	// Registers what a wildcard import re-exports, so a qualified name reaches a
	// symbol another document imported. Only sound once every document is in.
	idx.ExpandWildcardImports()

	if parsedClean {
		for i, doc := range documents {
			doc.PassesDiags = passes.AnalyzeWithOptions(inputs[i].name, inputs[i].kind, doc.Root,
				make([]passes.Diagnostic, 0), idx, passes.Options{Conformance: mode})
		}
	}

	model := &CachedModel{Documents: documents, Index: idx, Library: library}
	s.cache.Put(modelHash, model)
	return modelHash, model
}

// GetSymbol retrieves symbol information by FQN
func (s *Service) GetSymbol(ctx context.Context, req *pb.GetSymbolRequest) (*pb.SymbolResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, req.ModelHash)
	}

	// Lookup symbol by FQN
	syms := lookupNamed(cached.Index, req.SymbolId)
	if len(syms) == 0 {
		return &pb.SymbolResponse{
			Error: fmt.Sprintf("symbol not found: %s", req.SymbolId),
		}, nil
	}

	// Convert first match to proto
	return &pb.SymbolResponse{
		Symbol: s.symbolToProto(syms[0], cached.SymbolContext()),
	}, nil
}

// GetDiagnostics retrieves all diagnostics for a parsed model (parser + semantic passes)
func (s *Service) GetDiagnostics(ctx context.Context, req *pb.DiagnosticsRequest) (*pb.DiagnosticsResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, req.ModelHash)
	}

	return &pb.DiagnosticsResponse{
		Diagnostics: s.modelDiagnostics(cached),
	}, nil
}

// modelDiagnostics are every document's diagnostics, document by document, each
// located in the source it came from.
func (s *Service) modelDiagnostics(model *CachedModel) []*pb.Diagnostic {
	var pbDiags []*pb.Diagnostic
	for _, doc := range model.Documents {
		for _, diag := range doc.ParseDiags {
			pbDiags = append(pbDiags, ParserDiagnosticToProto(diag, doc.Source))
		}
		for _, diag := range doc.PassesDiags {
			pbDiags = append(pbDiags, DiagnosticToProto(diag, doc.Source))
		}
	}
	return s.filterDiagnosticCapabilities(pbDiags)
}

// Evaluate evaluates a SysML expression in the context of a parsed model
func (s *Service) Evaluate(ctx context.Context, req *pb.EvaluateRequest) (*pb.EvaluateResponse, error) {
	if req.SubjectSymbolId != "" {
		if err := s.requireCapability(CapabilityEvaluateSubject); err != nil {
			return nil, err
		}
	}

	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, req.ModelHash)
	}

	// Parse expression
	exprSource := source.New("<expression>", []byte(req.Expression))
	p := parser.New(exprSource)
	exprNode := p.ParseExpression()

	// Check for parse errors
	if len(p.Diagnostics) > 0 {
		var pbDiags []*pb.Diagnostic
		for _, diag := range p.Diagnostics {
			pbDiags = append(pbDiags, ParserDiagnosticToProto(diag, exprSource))
		}
		return &pb.EvaluateResponse{
			Diagnostics: s.filterDiagnosticCapabilities(pbDiags),
			Error:       "expression parse failed",
		}, nil
	}

	// The request states one expression, so text the parser leaves unread is
	// reported rather than evaluating the prefix it did read (`1 = 1` answering 1).
	if p.Offset() < len(strings.TrimRight(req.Expression, " \t\r\n")) {
		return &pb.EvaluateResponse{
			Error: fmt.Sprintf("expression parse failed: unexpected %q after the expression",
				strings.TrimSpace(req.Expression[p.Offset():])),
		}, nil
	}

	// A subject is both what the expression is evaluated against and, unless a
	// context is named, the namespace its features are named in — the way the
	// prompt evaluates in the context it pinned.
	var subject *symbols.Symbol
	if req.SubjectSymbolId != "" {
		syms := lookupNamed(cached.Index, req.SubjectSymbolId)
		if len(syms) == 0 {
			return &pb.EvaluateResponse{
				Error: fmt.Sprintf("subject not found: %s", req.SubjectSymbolId),
			}, nil
		}
		subject = syms[0]
	}

	// Determine scope
	var scope *symbols.Scope
	if req.ContextSymbolId != "" {
		// Lookup context symbol
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
		scope = cached.PrimaryRoot()
	}

	runtimeCtx := s.newRuntime(cached)

	var self *runtime.Instance
	if subject != nil {
		inst, err := runtimeCtx.Instantiate(subject)
		if err != nil {
			return &pb.EvaluateResponse{
				Error: fmt.Sprintf("instantiation of subject %s failed: %v", req.SubjectSymbolId, err),
			}, nil
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
		return &pb.EvaluateResponse{
			Error: fmt.Sprintf("evaluation failed: %v", err),
		}, nil
	}

	return &pb.EvaluateResponse{
		Result: s.valueToProto(runtimeCtx, result, cached.Index),
	}, nil
}

// evalScope is the namespace a subject's features are named in: its own scope,
// so a member is named unqualified, else the scope it was declared in. Mirrors
// the REPL's contextScope.
func evalScope(sym *symbols.Symbol, cached *CachedModel) *symbols.Scope {
	switch {
	case sym == nil:
		return nil
	case sym.Scope != nil:
		return sym.Scope
	case sym.OwnerScope != nil:
		return sym.OwnerScope
	default:
		return cached.PrimaryRoot()
	}
}

// Instantiate creates a runtime instance of a part/usage
func (s *Service) Instantiate(ctx context.Context, req *pb.InstantiateRequest) (*pb.InstantiateResponse, error) {
	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, req.ModelHash)
	}

	// Lookup symbol
	syms := lookupNamed(cached.Index, req.SymbolId)
	if len(syms) == 0 {
		return &pb.InstantiateResponse{
			Error: fmt.Sprintf("symbol not found: %s", req.SymbolId),
		}, nil
	}
	sym := syms[0]

	runtimeCtx := s.newRuntime(cached)

	// Instantiate
	inst, err := runtimeCtx.Instantiate(sym)
	if err != nil {
		return &pb.InstantiateResponse{
			Error: fmt.Sprintf("instantiation failed: %v", err),
		}, nil
	}

	root, all := s.instanceGraphToProto(runtimeCtx, inst, cached.Index)
	return &pb.InstantiateResponse{
		Instance:  root,
		Instances: all,
	}, nil
}

// ExecuteAction executes an action definition
func (s *Service) ExecuteAction(ctx context.Context, req *pb.ExecuteActionRequest) (*pb.ExecuteActionResponse, error) {
	schedule, err := s.schedulePolicy(req.Schedule)
	if err != nil {
		return nil, err
	}

	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, req.ModelHash)
	}

	// Lookup action symbol
	syms := lookupNamed(cached.Index, req.ActionSymbolId)
	if len(syms) == 0 {
		return &pb.ExecuteActionResponse{
			Error: fmt.Sprintf("action not found: %s", req.ActionSymbolId),
		}, nil
	}
	action := syms[0]

	runtimeCtx := s.newRuntime(cached)

	// Converted against the model's index, so a quantity input keeps the base
	// units it is commensurable with instead of binding an unusable value.
	readInputs := func(ctx *runtime.Context) (map[string]runtime.Value, *pb.ExecuteActionResponse, error) {
		if len(req.Inputs) == 0 {
			return nil, nil, nil
		}
		inputs := make(map[string]runtime.Value, len(req.Inputs))
		for name, pv := range req.Inputs {
			if err := s.requireValueCapabilities(pv); err != nil {
				return nil, nil, err
			}
			val, cerr := ProtoToRuntimeValue(ctx, pv, cached.Index, ctx.Semantics())
			if cerr != nil {
				return nil, &pb.ExecuteActionResponse{
					Error: fmt.Sprintf("input %q could not be read: %v", name, cerr),
				}, nil
			}
			inputs[name] = val
		}
		return inputs, nil, nil
	}
	inputs, resp, err := readInputs(runtimeCtx)
	if resp != nil || err != nil {
		return resp, err
	}

	if _, explores := schedule.Exploration(); explores {
		// Inputs are read again on each run's own context, so an object among them
		// belongs to the run that binds it.
		x, err := s.explore(ctx, req.ActionSymbolId, schedule, analysis.Auto(), cached, func(rt *runtime.Context) (runtime.Outcome, error) {
			inputs, resp, err := readInputs(rt)
			if err != nil {
				return runtime.Outcome{}, err
			}
			if resp != nil {
				return runtime.Outcome{}, errors.New(resp.Error)
			}
			outputs, err := rt.ExecuteActionWithInputs(action, inputs)
			if err != nil {
				return runtime.Outcome{}, fmt.Errorf("action execution failed: %w", err)
			}
			return rt.ActionOutcome(outputs), nil
		})
		if err != nil {
			return nil, err
		}
		return &pb.ExecuteActionResponse{Outcomes: x.outcomes, Exploration: x.status}, nil
	}
	if err := runtimeCtx.SetSchedule(schedule); err != nil {
		return nil, statusError(connect.CodeInvalidArgument, err.Error())
	}

	// Execute action with the supplied inputs
	outputs, _, err := performOn(ctx, s, runtimeCtx, analysis.Auto(), req.ActionSymbolId, func(rt *runtime.Context) (map[string]runtime.Value, error) {
		return rt.ExecuteActionWithInputs(action, inputs)
	}, heldAnswer)
	if gone := callerGone(ctx, err); gone != nil {
		return nil, gone
	}
	// The choices the run made are reported with its outcome, failed or not: a
	// failure may hang on the order taken.
	diags := s.filterDiagnosticCapabilities(RunNoteDiagnosticsToProto(runtimeCtx.Notes(), cached))
	if err != nil {
		return &pb.ExecuteActionResponse{
			Error:       fmt.Sprintf("action execution failed: %v", err),
			Diagnostics: diags,
			FinalTime:   s.finalTime(runtimeCtx),
		}, nil
	}

	// Convert outputs to protobuf
	pbOutputs := make(map[string]*pb.Value)
	for name, val := range outputs {
		pbOutputs[name] = s.valueToProto(runtimeCtx, val, cached.Index)
	}

	return &pb.ExecuteActionResponse{
		Outputs:     pbOutputs,
		Diagnostics: diags,
		FinalTime:   s.finalTime(runtimeCtx),
	}, nil
}

// finalTime is the run's clock when it ended, under the final_time capability.
func (s *Service) finalTime(runtimeCtx *runtime.Context) float64 {
	if !s.capabilities.has(CapabilityFinalTime) {
		return 0
	}
	return runtimeCtx.Clock().Now()
}

// ExecuteState executes a state machine
func (s *Service) ExecuteState(ctx context.Context, req *pb.ExecuteStateRequest) (*pb.ExecuteStateResponse, error) {
	schedule, err := s.schedulePolicy(req.Schedule)
	if err != nil {
		return nil, err
	}

	// Lookup cached model
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, req.ModelHash)
	}

	// Lookup state machine symbol
	syms := lookupNamed(cached.Index, req.StateMachineSymbolId)
	if len(syms) == 0 {
		return &pb.ExecuteStateResponse{
			Error: fmt.Sprintf("state machine not found: %s", req.StateMachineSymbolId),
		}, nil
	}
	stateMachine := syms[0]

	if _, explores := schedule.Exploration(); explores {
		x, err := s.explore(ctx, req.StateMachineSymbolId, schedule, analysis.Auto(), cached, func(rt *runtime.Context) (runtime.Outcome, error) {
			return rt.StateOutcomeWithEvents(stateMachine, req.Events)
		})
		if err != nil {
			return nil, err
		}
		return &pb.ExecuteStateResponse{Outcomes: x.outcomes, Exploration: x.status}, nil
	}

	runtimeCtx := s.newRuntime(cached)
	if err := runtimeCtx.SetSchedule(schedule); err != nil {
		return nil, statusError(connect.CodeInvalidArgument, err.Error())
	}

	// Execute state machine, injecting the requested events and capturing the
	// real ordered state-visit trace.
	ran, _, err := performOn(ctx, s, runtimeCtx, analysis.Auto(), req.StateMachineSymbolId, func(rt *runtime.Context) (stateRun, error) {
		final, visited, err := rt.ExecuteStateWithEvents(stateMachine, req.Events)
		return stateRun{final: final, visited: visited}, err
	}, func(ran stateRun, err error) analysis.Answer { return heldAnswer(ran.final, err) })
	if gone := callerGone(ctx, err); gone != nil {
		return nil, gone
	}
	finalContext, statesVisited := ran.final, ran.visited
	diags := s.filterDiagnosticCapabilities(RunNoteDiagnosticsToProto(runtimeCtx.Notes(), cached))
	if err != nil {
		return &pb.ExecuteStateResponse{
			Error:       fmt.Sprintf("state machine execution failed: %v", err),
			Diagnostics: diags,
			FinalTime:   s.finalTime(runtimeCtx),
		}, nil
	}

	// Convert final context to protobuf
	pbContext := make(map[string]*pb.Value)
	for name, val := range finalContext {
		pbContext[name] = s.valueToProto(runtimeCtx, val, cached.Index)
	}

	return &pb.ExecuteStateResponse{
		StatesVisited: statesVisited,
		FinalContext:  pbContext,
		Diagnostics:   diags,
		FinalTime:     s.finalTime(runtimeCtx),
	}, nil
}

// buildParseResponse constructs ParseFileResponse from cached model
func (s *Service) buildParseResponse(modelHash string, model *CachedModel) *pb.ParseFileResponse {
	return &pb.ParseFileResponse{
		ModelHash:   modelHash,
		Root:        s.rootSymbol(model, model.Primary()),
		Diagnostics: s.modelDiagnostics(model),
	}
}

// rootSymbol is one document's root namespace: the index's own root symbol for
// it, else the document's root scope described as one, which is what a document
// declaring no root symbol has.
func (s *Service) rootSymbol(model *CachedModel, doc *CachedDocument) *pb.SymbolInfo {
	if doc.Root == nil {
		return nil
	}
	rootScope := model.Index.DocumentRoot(doc.Source.Name())
	for _, sym := range model.Index.LookupQualified("") { // Root has empty name
		if sym.Scope == rootScope {
			return s.symbolToProto(sym, model.SymbolContext())
		}
	}
	if rootScope == nil {
		return nil
	}
	return &pb.SymbolInfo{
		Id:       "",
		Name:     "",
		Kind:     "RootNamespace",
		Metadata: make(map[string]string),
		ChildIds: collectChildIDs(rootScope, model.Index),
	}
}

// collectChildIDs extracts child symbol FQNs from a scope
func collectChildIDs(scope *symbols.Scope, idx *symbols.Index) []string {
	var ids []string
	for _, sym := range scope.AllMembers() {
		ids = append(ids, idx.GetFQN(sym))
	}
	return ids
}

// computeHash generates SHA-256 hash of content
func computeHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// lookupNamed resolves a symbol ID written in either spelling: the quoted,
// notation-legal form a model author writes ('My Pkg'::Car), or the unquoted
// spelling the index records (My Pkg::Car), which keeps working as it did.
// Model elements come before library homonyms: an ID naming both denotes the
// model's own element, which is what the client asked about.
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
