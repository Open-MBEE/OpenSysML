package opensysml

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Client answers SysML v2 questions: parse, look up, evaluate, instantiate.
// New gives the default in-process implementation and Dial a remote one; both
// answer identically, which the conformance suite holds them to.
//
// A Client is safe for concurrent use: calls may be made from any number of
// goroutines, and each answer is the caller's own.
//
// The interface is sealed: implementations come from this package only, so a
// method can be added without breaking callers.
type Client interface {
	// ServerInfo reports the implementation's version and capabilities.
	ServerInfo(ctx context.Context) (*ServerInfo, error)

	// ListEngines lists the analysis engines the service answers with, in name
	// order, as `sysml -engines` does. Requires the engines capability.
	ListEngines(ctx context.Context) ([]EngineInfo, error)

	// ParseFile parses the model file at path. Diagnostics, including syntax
	// errors, arrive on the returned Model; only an unreadable path or an
	// invalid request fails the call.
	ParseFile(ctx context.Context, path string, opts ...ParseOption) (*Model, error)

	// ParseSource parses inline model source, SysML unless WithLanguage says
	// otherwise. Diagnostics arrive on the returned Model.
	ParseSource(ctx context.Context, content string, opts ...ParseOption) (*Model, error)

	// ParseFiles parses the model files at paths as one model, so a name one
	// file declares resolves in another and an import between them is
	// satisfied. Requires the parse_sources capability.
	ParseFiles(ctx context.Context, paths []string, opts ...ParseOption) (*Model, error)

	// ParseDocuments parses the documents named as one model, for files and
	// inline sources together. Requires the parse_sources capability.
	ParseDocuments(ctx context.Context, documents []Document, opts ...ParseOption) (*Model, error)

	// Diagnostics reports every diagnostic of the parsed model, parser and
	// semantic passes combined — the same list the Model already carries.
	Diagnostics(ctx context.Context, model *Model) ([]Diagnostic, error)

	// LookupSymbol looks a symbol up by fully qualified name. An unknown name
	// is a FailureError; an unknown model is CodeNotFound.
	LookupSymbol(ctx context.Context, model *Model, symbolID string) (*Symbol, error)

	// Evaluate evaluates a SysML expression against the parsed model. An
	// expression that does not parse or evaluate is a FailureError carrying
	// the diagnostics; an unknown model is CodeNotFound.
	Evaluate(ctx context.Context, model *Model, expression string, opts ...EvaluateOption) (Value, error)

	// Instantiate instantiates the named part or usage, answering the root
	// instance and everything reachable from it. An unknown symbol or a failed
	// instantiation is a FailureError; an unknown model is CodeNotFound.
	Instantiate(ctx context.Context, model *Model, symbolID string) (*Instantiation, error)

	// ExecuteAction executes the named action with the inputs given, bound by
	// parameter name, and reports the outputs it produced. A Complex input
	// requires the complex_values capability, an Array, Vector or
	// VectorQuantity input the structured_values one, a MeasurementRef input
	// the measurement_refs one, a Function input the function_values one, a Set
	// input the set_values one and a TensorQuantity input the tensor_values one,
	// checked before anything is sent. WithSchedule selects the scheduling
	// policy, which requires the schedule capability, checked the same way.
	ExecuteAction(ctx context.Context, model *Model, actionSymbolID string, inputs map[string]Value, opts ...ExecuteOption) (*ActionRun, error)

	// ExecuteState runs the named state machine, feeding it the events in
	// order, and reports the states visited and the context left behind.
	// WithSchedule selects the scheduling policy, which requires the schedule
	// capability, checked before anything is sent.
	ExecuteState(ctx context.Context, model *Model, stateMachineSymbolID string, events []string, opts ...ExecuteOption) (*StateRun, error)

	// ExploreAction runs the action once per linearization of its choice points,
	// within the budget WithSchedule spells; needs schedule and schedule_explore.
	ExploreAction(ctx context.Context, model *Model, actionSymbolID string, inputs map[string]Value, opts ...ExecuteOption) (*Exploration, error)

	// ExploreState runs the state machine on the events once per linearization
	// of its choice points; an outcome is its final state, visits and values.
	ExploreState(ctx context.Context, model *Model, stateMachineSymbolID string, events []string, opts ...ExecuteOption) (*Exploration, error)

	// ExploreAnalysis runs the case once per linearization of its actions' choice
	// points, under the budget Schedule spells; an outcome is outputs plus verdicts.
	ExploreAnalysis(ctx context.Context, model *Model, symbolID string, opts ...AnalysisOption) (*Exploration, error)

	// VerifyConstraint evaluates the named constraint, optionally Against a
	// part to instantiate and check, WithEngine naming the engine that answers.
	// Requires the verification capability, and engines for a named engine,
	// checked before anything is sent.
	VerifyConstraint(ctx context.Context, model *Model, symbolID string, opts ...VerifyOption) (*Verification, error)

	// VerifyRequirement evaluates the named requirement, optionally Against a
	// part to instantiate and check, WithEngine naming the engine that answers.
	// Requires the verification capability, and engines for a named engine,
	// checked before anything is sent.
	VerifyRequirement(ctx context.Context, model *Model, symbolID string, opts ...VerifyOption) (*Verification, error)

	// VerifySatisfaction evaluates the satisfaction assertions the model
	// states — every one, or those of the symbol named — WithEngine naming the
	// engine that answers; the assertions name their own subjects, so Against
	// is an invalid argument. Requires the verification capability, and engines
	// for a named engine, checked before anything is sent.
	VerifySatisfaction(ctx context.Context, model *Model, symbolID string, opts ...VerifyOption) (*Satisfaction, error)

	// EvaluateCalc invokes the named calculation with positional arguments, or,
	// given none, evaluates a calc usage from its own members. Requires the
	// verification capability, and the complex_values, structured_values,
	// measurement_refs, function_values, set_values or tensor_values capability
	// for a Complex, a structured, a MeasurementRef, a Function, a Set or a
	// TensorQuantity argument, checked before anything is sent.
	EvaluateCalc(ctx context.Context, model *Model, symbolID string, arguments ...Value) (*Calculation, error)

	// RunAnalysis runs the named analysis case — a definition or a usage — and
	// reports its outputs with the verdict of its objective and of each
	// assertion in its body. Positional arguments bind its inputs in
	// declaration order; Against names its subject and Binding a parameter by
	// name; Schedule selects the scheduling policy and Engine the engine that
	// answers. Requires the verification capability, the complex_values or
	// structured_values capability for a Complex or a structured argument, the
	// schedule capability for a policy and engines for a named engine, checked
	// before anything is sent.
	RunAnalysis(ctx context.Context, model *Model, symbolID string, opts ...AnalysisOption) (*Analysis, error)

	// Query selects the model's elements the query matches, in declaration
	// order. Requires the query capability.
	Query(ctx context.Context, model *Model, query Query) ([]QueryElement, error)

	// QueryOSLC selects elements with OSLC Query 3.0 parameter text. Requires
	// the oslc_query capability.
	QueryOSLC(ctx context.Context, model *Model, oslc string) ([]QueryElement, error)

	// RunDocumentQuery runs the named document query, binding its entry
	// parameters, and answers typed rows. Requires the document_query
	// capability.
	RunDocumentQuery(ctx context.Context, model *Model, queryID string, bindings ...Binding) (*Rows, error)

	// RenderDocument renders the named document to Markdown. Requires the
	// render_document capability.
	RenderDocument(ctx context.Context, model *Model, documentID string) (string, error)

	// Convert writes the model in another representation, from the source the
	// parse read, so WithFromFormat does not apply and is refused. Requires the
	// convert capability, and a model of one document. ConvertFile converts a
	// file the client never parsed.
	Convert(ctx context.Context, model *Model, to Format, opts ...ConvertOption) (*Conversion, error)

	// ConvertFile writes the model file at path in another representation,
	// inferring its notation from the extension unless WithFromFormat says
	// otherwise. Requires the convert capability.
	ConvertFile(ctx context.Context, path string, to Format, opts ...ConvertOption) (*Conversion, error)

	// ConvertSource writes inline content in another representation. Name the
	// notation it is written in with WithFromFormat: there is no file extension
	// to read it from. Requires the convert capability.
	ConvertSource(ctx context.Context, content string, to Format, opts ...ConvertOption) (*Conversion, error)

	// ApplyEdits answers the model's source with every edit applied, or refuses
	// them all with an EditError. Requires the apply_edits capability, and a
	// model of one document.
	ApplyEdits(ctx context.Context, model *Model, edits ...Edit) (*EditResult, error)

	// Close releases what the implementation holds. The Client answers no
	// further calls: each is refused with CodeUnavailable. Closing twice is
	// not an error.
	Close() error

	sealed()
}

// Document is one document of a multi-document parse: a file to read, or inline
// content under a name of the caller's choosing.
type Document struct {
	// Path is the file to read. Its extension says which notation it is, and
	// Content, Name and Language are ignored when it is set.
	Path string
	// Content is inline model source, parsed when Path is empty.
	Content string
	// Name is what diagnostics call inline content, and the name it is indexed
	// under. Two documents of one model may not share a name. Empty names it by
	// its position in the parse.
	Name string
	// Language is the notation of inline content, SysML when empty. Requires the
	// inline_language capability when set.
	Language Language
}

// File is the document read from path.
func File(path string) Document { return Document{Path: path} }

// Source is inline content as a document reported by name.
func Source(name, content string) Document {
	return Document{Name: name, Content: content}
}

// ParseOption configures the parse calls.
type ParseOption func(*parseOptions)

type parseOptions struct {
	language string
	strict   bool
}

// WithLanguage names the notation of inline content: LanguageSysML (the
// default) or LanguageKerML. File parsing infers the notation from the file
// extension instead.
func WithLanguage(language Language) ParseOption {
	return func(o *parseOptions) { o.language = string(language) }
}

// WithStrictConformance asks for the source to be judged as conforming SysML
// v2: notation no pinned production admits is an error rather than a warning.
// Requires the strict_conformance capability.
func WithStrictConformance() ParseOption {
	return func(o *parseOptions) { o.strict = true }
}

// EvaluateOption configures Evaluate.
type EvaluateOption func(*evaluateOptions)

type evaluateOptions struct {
	contextSymbolID string
	subjectSymbolID string
}

// WithContextSymbol names the symbol whose scope the expression's names are
// resolved in.
func WithContextSymbol(symbolID string) EvaluateOption {
	return func(o *evaluateOptions) { o.contextSymbolID = symbolID }
}

// WithSubject names a symbol to instantiate and evaluate against, so a feature
// reads that object's value rather than the declared default. Requires the
// evaluate_subject capability.
func WithSubject(symbolID string) EvaluateOption {
	return func(o *evaluateOptions) { o.subjectSymbolID = symbolID }
}

// caller is the transport behind a client: the in-process service or a remote
// one, speaking the same protobuf request and response types so both run the
// one service implementation.
type caller interface {
	serverInfo(ctx context.Context) (*pb.ServerInfoResponse, error)
	listEngines(ctx context.Context, req *pb.ListEnginesRequest) (*pb.ListEnginesResponse, error)
	parseFile(ctx context.Context, req *pb.ParseFileRequest) (*pb.ParseFileResponse, error)
	parseSources(ctx context.Context, req *pb.ParseSourcesRequest) (*pb.ParseSourcesResponse, error)
	getSymbol(ctx context.Context, req *pb.GetSymbolRequest) (*pb.SymbolResponse, error)
	getDiagnostics(ctx context.Context, req *pb.DiagnosticsRequest) (*pb.DiagnosticsResponse, error)
	evaluate(ctx context.Context, req *pb.EvaluateRequest) (*pb.EvaluateResponse, error)
	instantiate(ctx context.Context, req *pb.InstantiateRequest) (*pb.InstantiateResponse, error)
	executeAction(ctx context.Context, req *pb.ExecuteActionRequest) (*pb.ExecuteActionResponse, error)
	executeState(ctx context.Context, req *pb.ExecuteStateRequest) (*pb.ExecuteStateResponse, error)
	verifyConstraint(ctx context.Context, req *pb.VerifyConstraintRequest) (*pb.VerifyConstraintResponse, error)
	verifyRequirement(ctx context.Context, req *pb.VerifyRequirementRequest) (*pb.VerifyRequirementResponse, error)
	verifySatisfaction(ctx context.Context, req *pb.VerifySatisfactionRequest) (*pb.VerifySatisfactionResponse, error)
	evaluateCalc(ctx context.Context, req *pb.EvaluateCalcRequest) (*pb.EvaluateCalcResponse, error)
	runAnalysis(ctx context.Context, req *pb.RunAnalysisRequest) (*pb.RunAnalysisResponse, error)
	query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error)
	runDocumentQuery(ctx context.Context, req *pb.RunDocumentQueryRequest) (*pb.RunDocumentQueryResponse, error)
	renderDocument(ctx context.Context, req *pb.RenderDocumentRequest) (*pb.RenderDocumentResponse, error)
	convert(ctx context.Context, req *pb.ConvertRequest) (*pb.ConvertResponse, error)
	applyEdits(ctx context.Context, req *pb.ApplyEditsRequest) (*pb.ApplyEditsResponse, error)
	close() error
}

// client is the one Client implementation, over either caller.
type client struct {
	caller caller

	mu     sync.Mutex
	closed bool
	// info is the service's answer to ServerInfo, kept once a preflight asks
	// for it: a service does not change build while a client is open to it.
	info *ServerInfo
}

func (c *client) sealed() { /* marker: Client is closed to outside implementations */ }

// live refuses a call on a closed Client, the way a closed connection refuses
// one.
func (c *client) live() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return &StatusError{Code: CodeUnavailable, Message: "the client is closed"}
	}
	return nil
}

func (c *client) ServerInfo(ctx context.Context) (*ServerInfo, error) {
	if err := c.live(); err != nil {
		return nil, err
	}
	resp, err := c.caller.serverInfo(ctx)
	if err != nil {
		return nil, err
	}
	return &ServerInfo{
		Version:      resp.Version,
		Capabilities: append([]string(nil), resp.Capabilities...),
	}, nil
}

func (c *client) ParseFile(ctx context.Context, path string, opts ...ParseOption) (*Model, error) {
	if err := c.live(); err != nil {
		return nil, err
	}
	req := parseRequest(opts)
	req.Source = &pb.ParseFileRequest_FilePath{FilePath: path}
	return c.parse(ctx, req)
}

func (c *client) ParseSource(ctx context.Context, content string, opts ...ParseOption) (*Model, error) {
	if err := c.live(); err != nil {
		return nil, err
	}
	req := parseRequest(opts)
	req.Source = &pb.ParseFileRequest_Content{Content: content}
	return c.parse(ctx, req)
}

func (c *client) ParseFiles(ctx context.Context, paths []string, opts ...ParseOption) (*Model, error) {
	documents := make([]Document, 0, len(paths))
	for _, path := range paths {
		documents = append(documents, File(path))
	}
	return c.ParseDocuments(ctx, documents, opts...)
}

func (c *client) ParseDocuments(ctx context.Context, documents []Document, opts ...ParseOption) (*Model, error) {
	if err := c.live(); err != nil {
		return nil, err
	}
	if len(documents) == 0 {
		return nil, &StatusError{Code: CodeInvalidArgument, Message: "parse needs at least one document"}
	}
	var options parseOptions
	for _, opt := range opts {
		opt(&options)
	}
	req := &pb.ParseSourcesRequest{
		Documents:         make([]*pb.SourceDocument, 0, len(documents)),
		StrictConformance: options.strict,
	}
	for _, doc := range documents {
		pbDoc := &pb.SourceDocument{}
		if doc.Path != "" {
			pbDoc.Source = &pb.SourceDocument_FilePath{FilePath: doc.Path}
		} else {
			language := string(doc.Language)
			if language == "" {
				language = options.language
			}
			pbDoc.Source = &pb.SourceDocument_Content{Content: doc.Content}
			pbDoc.Name = doc.Name
			pbDoc.Language = language
		}
		req.Documents = append(req.Documents, pbDoc)
	}
	resp, err := c.caller.parseSources(ctx, req)
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &FailureError{Op: "ParseDocuments", Message: resp.Error, Diagnostics: diagnostics}
	}
	roots := make([]*Symbol, 0, len(resp.Roots))
	for _, root := range resp.Roots {
		roots = append(roots, symbolFromProto(root))
	}
	model := &Model{Hash: resp.ModelHash, Roots: roots, Diagnostics: diagnostics}
	if len(roots) > 0 {
		model.Root = roots[0]
	}
	return model, nil
}

func parseRequest(opts []ParseOption) *pb.ParseFileRequest {
	var options parseOptions
	for _, opt := range opts {
		opt(&options)
	}
	return &pb.ParseFileRequest{
		Language:          options.language,
		StrictConformance: options.strict,
	}
}

func (c *client) parse(ctx context.Context, req *pb.ParseFileRequest) (*Model, error) {
	resp, err := c.caller.parseFile(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &FailureError{Op: "ParseFile", Message: resp.Error, Diagnostics: diagnosticsFromProto(resp.Diagnostics)}
	}
	root := symbolFromProto(resp.Root)
	return &Model{
		Hash:        resp.ModelHash,
		Root:        root,
		Roots:       []*Symbol{root},
		Diagnostics: diagnosticsFromProto(resp.Diagnostics),
	}, nil
}

func (c *client) Diagnostics(ctx context.Context, model *Model) ([]Diagnostic, error) {
	if err := c.live(); err != nil {
		return nil, err
	}
	hash, err := modelHash(model)
	if err != nil {
		return nil, err
	}
	resp, err := c.caller.getDiagnostics(ctx, &pb.DiagnosticsRequest{ModelHash: hash})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &FailureError{Op: "Diagnostics", Message: resp.Error}
	}
	return diagnosticsFromProto(resp.Diagnostics), nil
}

func (c *client) LookupSymbol(ctx context.Context, model *Model, symbolID string) (*Symbol, error) {
	if err := c.live(); err != nil {
		return nil, err
	}
	hash, err := modelHash(model)
	if err != nil {
		return nil, err
	}
	resp, err := c.caller.getSymbol(ctx, &pb.GetSymbolRequest{ModelHash: hash, SymbolId: symbolID})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &FailureError{Op: "LookupSymbol", Message: resp.Error}
	}
	return symbolFromProto(resp.Symbol), nil
}

func (c *client) Evaluate(ctx context.Context, model *Model, expression string, opts ...EvaluateOption) (Value, error) {
	if err := c.live(); err != nil {
		return nil, err
	}
	hash, err := modelHash(model)
	if err != nil {
		return nil, err
	}
	var options evaluateOptions
	for _, opt := range opts {
		opt(&options)
	}
	resp, err := c.caller.evaluate(ctx, &pb.EvaluateRequest{
		ModelHash:       hash,
		Expression:      expression,
		ContextSymbolId: options.contextSymbolID,
		SubjectSymbolId: options.subjectSymbolID,
	})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &FailureError{Op: "Evaluate", Message: resp.Error, Diagnostics: diagnosticsFromProto(resp.Diagnostics)}
	}
	return valueFromProto(resp.Result), nil
}

func (c *client) Instantiate(ctx context.Context, model *Model, symbolID string) (*Instantiation, error) {
	if err := c.live(); err != nil {
		return nil, err
	}
	hash, err := modelHash(model)
	if err != nil {
		return nil, err
	}
	resp, err := c.caller.instantiate(ctx, &pb.InstantiateRequest{ModelHash: hash, SymbolId: symbolID})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &FailureError{Op: "Instantiate", Message: resp.Error, Diagnostics: diagnosticsFromProto(resp.Diagnostics)}
	}
	instances := make([]*Instance, 0, len(resp.Instances))
	for _, inst := range resp.Instances {
		instances = append(instances, instanceFromProto(inst))
	}
	return &Instantiation{
		Root:        instanceFromProto(resp.Instance),
		Instances:   instances,
		Diagnostics: diagnosticsFromProto(resp.Diagnostics),
	}, nil
}

// Close releases the implementation once: a second call is a no-op, so a
// deferred Close beside an explicit one is safe.
func (c *client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.caller.close()
}

// modelHash is the hash a call sends for a model, refusing a model that names
// none here rather than sending a hash the service cannot know.
func modelHash(model *Model) (string, error) {
	if model == nil {
		return "", &StatusError{Code: CodeInvalidArgument, Message: "model is nil"}
	}
	if model.Hash == "" {
		return "", &StatusError{Code: CodeInvalidArgument, Message: "model carries no hash: it did not come from a parse call"}
	}
	return model.Hash, nil
}

// call is the shared preamble of every model operation: a live client and the
// hash the model is named by.
func (c *client) call(model *Model) (string, error) {
	if err := c.live(); err != nil {
		return "", err
	}
	return modelHash(model)
}

// requireValueCapabilities refuses to send a value of a kind whose capability
// the service lacks — a Complex without complex_values, an Array, Vector or
// VectorQuantity without structured_values, a MeasurementRef without
// measurement_refs, a Function without function_values, a Set without
// set_values, a TensorQuantity without tensor_values — which would read it as
// null rather than refuse it.
func (c *client) requireValueCapabilities(ctx context.Context, values ...Value) error {
	var needed []string
	if slices.ContainsFunc(values, carriesComplex) {
		needed = append(needed, CapabilityComplexValues)
	}
	if slices.ContainsFunc(values, carriesStructured) {
		needed = append(needed, CapabilityStructuredValues)
	}
	if slices.ContainsFunc(values, carriesMeasurementRef) {
		needed = append(needed, CapabilityMeasurementRefs)
	}
	if slices.ContainsFunc(values, carriesFunction) {
		needed = append(needed, CapabilityFunctionValues)
	}
	if slices.ContainsFunc(values, carriesSet) {
		needed = append(needed, CapabilitySetValues)
	}
	if slices.ContainsFunc(values, carriesTensor) {
		needed = append(needed, CapabilityTensorValues)
	}
	if len(needed) == 0 {
		return nil
	}
	info, err := c.serverInfo(ctx)
	if err != nil {
		return err
	}
	for _, capability := range needed {
		if !info.Has(capability) {
			return &StatusError{
				Code:    CodeUnimplemented,
				Message: fmt.Sprintf("capability %q is unavailable", capability),
			}
		}
	}
	return nil
}

// serverInfo is ServerInfo asked at most once per client. A service that
// predates the RPC answers UNIMPLEMENTED, which is an empty capability list.
func (c *client) serverInfo(ctx context.Context) (*ServerInfo, error) {
	c.mu.Lock()
	info := c.info
	c.mu.Unlock()
	if info != nil {
		return info, nil
	}
	info, err := c.ServerInfo(ctx)
	var status *StatusError
	if errors.As(err, &status) && status.Code == CodeUnimplemented {
		info, err = &ServerInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.info = info
	c.mu.Unlock()
	return info, nil
}

// carriesComplex reports whether a value, or any value nested in it, is a
// Complex.
func carriesComplex(value Value) bool {
	if _, ok := value.(Complex); ok {
		return true
	}
	return slices.ContainsFunc(nestedValues(value), carriesComplex)
}

// carriesStructured reports whether a value, or any value nested in it, is an
// Array, a Vector or a VectorQuantity.
func carriesStructured(value Value) bool {
	switch value.(type) {
	case Array, Vector, VectorQuantity:
		return true
	}
	return slices.ContainsFunc(nestedValues(value), carriesStructured)
}

// carriesMeasurementRef reports whether a value, or any value nested in it, is
// a MeasurementRef.
func carriesMeasurementRef(value Value) bool {
	if _, ok := value.(MeasurementRef); ok {
		return true
	}
	return slices.ContainsFunc(nestedValues(value), carriesMeasurementRef)
}

// carriesFunction reports whether a value, or any value nested in it, is a
// Function.
func carriesFunction(value Value) bool {
	if _, ok := value.(Function); ok {
		return true
	}
	return slices.ContainsFunc(nestedValues(value), carriesFunction)
}

// carriesSet reports whether a value, or any value nested in it, is a Set.
func carriesSet(value Value) bool {
	if _, ok := value.(Set); ok {
		return true
	}
	return slices.ContainsFunc(nestedValues(value), carriesSet)
}

// carriesTensor reports whether a value, or any value nested in it, is a
// TensorQuantity.
func carriesTensor(value Value) bool {
	if _, ok := value.(TensorQuantity); ok {
		return true
	}
	return slices.ContainsFunc(nestedValues(value), carriesTensor)
}

// nestedValues are the values a value holds: a sequence's or a set's elements,
// an array's.
func nestedValues(value Value) []Value {
	switch v := value.(type) {
	case Sequence:
		return v
	case Set:
		return v
	case Array:
		return v.Elements
	}
	return nil
}
