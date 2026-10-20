// Package runtimetools is the runtime group of sysml-mcp tools: the one-shot
// operations of the sysml-grpc service (parse, evaluate, instantiate, execute,
// verify, query, render, convert), each tool one RPC with the RPC's contract.
//
// A tool's arguments are the RPC's request message under its proto JSON field
// names, and its structured result is the response message as protojson writes
// it, so what an agent reads here is what every other client of the service
// reads. Models are content-addressed: every tool taking a modelHash also takes
// paths or inline sources and parses them first, so one call suffices.
package runtimetools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// group holds what every runtime tool shares: the service and the workspace
// root that relative paths are resolved against.
type group struct {
	svc  *sysmlgrpc.Service
	root toolkit.Root
}

// Register adds the runtime tools to s over svc; root is the workspace folder a
// relative path in a tool argument is resolved against, which must exist.
func Register(s *mcp.Server, svc *sysmlgrpc.Service, root string) error {
	r, err := toolkit.NewRoot(root)
	if err != nil {
		return err
	}
	g := &group{svc: svc, root: r}
	g.registerInfo(s)
	g.registerExplain(s)
	g.registerParse(s)
	g.registerDiagnostics(s)
	g.registerRuntime(s)
	g.registerVerification(s)
	g.registerRuns(s)
	g.registerSelfCheck(s)
	g.registerRecord(s)
	g.registerCheck(s)
	g.registerQueries(s)
	g.registerConversion(s)
	g.registerImportData(s)
	g.registerCompareResults(s)
	return nil
}

// Source is one document given inline.
type Source struct {
	Path     string `json:"path" jsonschema:"name the document is reported by in diagnostics and indexed under; a .kerml extension makes it KerML unless language says otherwise"`
	Text     string `json:"text" jsonschema:"the document's source text; empty is an empty document"`
	Language string `json:"language,omitempty" jsonschema:"sysml or kerml; empty is inferred from path's extension, sysml when it has none"`
}

// ModelInput names the model a tool works on: a modelHash an earlier call
// returned, or the documents to parse first. Its field names are the ones the
// parse tool takes, so an agent passes the same arguments either way.
type ModelInput struct {
	ModelHash         string   `json:"modelHash,omitempty" jsonschema:"the model_hash a parse answered; give this, or paths and/or sources to parse first"`
	Paths             []string `json:"paths,omitempty" jsonschema:"model files to parse as one model, absolute or relative to the workspace root"`
	Sources           []Source `json:"sources,omitempty" jsonschema:"documents given inline, parsed with paths as one model"`
	StrictConformance bool     `json:"strictConformance,omitempty" jsonschema:"judge the documents as conforming SysML v2: notation no pinned production admits is an error rather than a warning"`
}

// modelInputKeys are ModelInput's own fields, which no request message has.
var modelInputKeys = []string{"paths", "sources", "strictConformance"}

// model is a resolved model: its hash, and the parse that produced it when the
// call parsed one (nil when the call named a hash).
type model struct {
	hash   string
	parsed *pb.ParseSourcesResponse
}

// resolve finds the model in names, parsing paths and sources when given.
func (g *group) resolve(ctx context.Context, in ModelInput) (model, error) {
	given := len(in.Paths)+len(in.Sources) > 0
	switch {
	case in.ModelHash != "" && given:
		return model{}, argumentError("give modelHash, or paths and sources to parse, not both")
	case in.ModelHash != "":
		return model{hash: in.ModelHash}, nil
	case !given:
		return model{}, argumentError("a model is needed: modelHash, or paths and/or sources to parse")
	}
	resp, err := g.parse(ctx, in, "")
	if err != nil {
		return model{}, err
	}
	return model{hash: resp.GetModelHash(), parsed: resp}, nil
}

// parse parses in's documents as one model, relative to base when given.
func (g *group) parse(ctx context.Context, in ModelInput, base string) (*pb.ParseSourcesResponse, error) {
	req := &pb.ParseSourcesRequest{StrictConformance: in.StrictConformance, BaseModelHash: base}
	for _, path := range in.Paths {
		if strings.TrimSpace(path) == "" {
			return nil, argumentError("paths must not hold an empty path")
		}
		resolved, err := g.path(path, toolkit.NameFormat)
		if err != nil {
			return nil, err
		}
		req.Documents = append(req.Documents, &pb.SourceDocument{
			Source: &pb.SourceDocument_FilePath{FilePath: resolved},
		})
	}
	for _, src := range in.Sources {
		language := src.Language
		if language == "" && strings.EqualFold(filepath.Ext(src.Path), ".kerml") {
			language = "kerml"
		}
		req.Documents = append(req.Documents, &pb.SourceDocument{
			Source:   &pb.SourceDocument_Content{Content: src.Text},
			Language: language,
			Name:     src.Path,
		})
	}
	if single, ok := singleFileRequest(req); ok && !g.svc.Supports(sysmlgrpc.CapabilityParseSources) {
		resp, err := g.svc.ParseFile(ctx, single)
		if err != nil {
			return nil, toolError(err)
		}
		return parseFileAsSources(resp), nil
	}
	resp, err := g.svc.ParseSources(ctx, req)
	if err != nil {
		return nil, toolError(err)
	}
	return resp, nil
}

// singleFileRequest spells a parse of one document as the ParseFile RPC, which
// the tool stands for as well: a service without parse_sources answers that
// much. ParseFile carries no base hash and names inline content <content>, so
// a request with a base hash or another name (an unnamed source would be
// <content-0>) is not one.
func singleFileRequest(req *pb.ParseSourcesRequest) (*pb.ParseFileRequest, bool) {
	if len(req.GetDocuments()) != 1 || req.GetBaseModelHash() != "" {
		return nil, false
	}
	doc := req.GetDocuments()[0]
	single := &pb.ParseFileRequest{Language: doc.GetLanguage(), StrictConformance: req.GetStrictConformance()}
	switch src := doc.GetSource().(type) {
	case *pb.SourceDocument_FilePath:
		single.Source = &pb.ParseFileRequest_FilePath{FilePath: src.FilePath}
	case *pb.SourceDocument_Content:
		if doc.GetName() != "<content>" {
			return nil, false
		}
		single.Source = &pb.ParseFileRequest_Content{Content: src.Content}
	default:
		return nil, false
	}
	return single, true
}

// parseFileAsSources reads a ParseFile answer as the ParseSources answer for the
// same one document.
func parseFileAsSources(resp *pb.ParseFileResponse) *pb.ParseSourcesResponse {
	out := &pb.ParseSourcesResponse{ModelHash: resp.GetModelHash(), Diagnostics: resp.GetDiagnostics(), Error: resp.GetError()}
	if root := resp.GetRoot(); root != nil {
		out.Roots = []*pb.SymbolInfo{root}
	}
	return out
}

// path resolves a path argument against the workspace root and refuses one
// that leaves it, by its lexical form or through a symlink, so a tool reads
// only under -root. The path handed on has its symlinks resolved, so the
// service opens the file the check saw, not what a link is later pointed at;
// where the service reads something from the name (role), a link whose name
// means another format or language than its target's is refused rather than
// read as the wrong one.
func (g *group) path(p string, role toolkit.NameRole) (string, error) {
	res, err := g.root.Resolve(p, toolkit.NameMeans(role))
	if err != nil {
		return "", argumentError("%s", err)
	}
	return res.Real, nil
}

// argumentError is a refused argument, reported as the service reports one.
func argumentError(format string, args ...any) error {
	return toolError(connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(format, args...)))
}

// toolError turns a refused RPC into a tool error that keeps the status code's
// name and the message, so an agent reads NOT_FOUND here as a client reads it
// on the wire.
func toolError(err error) error {
	var cerr *connect.Error
	if errors.As(err, &cerr) {
		return fmt.Errorf("%s: %s", strings.ToUpper(cerr.Code().String()), cerr.Message())
	}
	return err
}

// unmarshal reads a proto message from the JSON name/value fields of a tool's
// arguments: the arguments are the request under its proto JSON names, less
// ModelInput's own fields, with the resolved hash in modelHash.
func unmarshal(in any, hash string, req proto.Message) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for _, key := range modelInputKeys {
		delete(fields, key)
	}
	if hash != "" {
		fields["modelHash"] = json.RawMessage(fmt.Sprintf("%q", hash))
	}
	raw, err = json.Marshal(fields)
	if err != nil {
		return err
	}
	if err := protojson.Unmarshal(raw, req); err != nil {
		return argumentError("the arguments do not read as a %s: %v", string(req.ProtoReflect().Descriptor().Name()), err)
	}
	return nil
}

// marshalOptions writes a response as protojson does, with default-valued
// scalars (`holds: false`, `ready: false`) spelled out rather than left for the
// reader to infer, and unset messages left out rather than written null.
var marshalOptions = protojson.MarshalOptions{EmitDefaultValues: true}

// Result is a tool's structured result: the response message as protojson
// writes it, plus modelHash when the tool parsed the model itself.
type Result map[string]any

// result makes the structured result of resp over m.
func result(resp proto.Message, m model) (Result, error) {
	raw, err := marshalOptions.Marshal(resp)
	if err != nil {
		return nil, err
	}
	out := Result{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if m.parsed != nil {
		out["modelHash"] = m.hash
	}
	return out, nil
}

// answer builds a tool's result: the structured response and a text the agent
// reads, prefixed by the hash and the diagnostics of a model parsed on the way.
// The text is kept byte for byte, so a rendered document or converted source
// ending it is what the service answered.
func answer(resp proto.Message, m model, text string) (*mcp.CallToolResult, Result, error) {
	out, err := result(resp, m)
	if err != nil {
		return nil, nil, err
	}
	var b strings.Builder
	if m.parsed != nil {
		fmt.Fprintf(&b, "model %s\n", m.hash)
		if diags := m.parsed.GetDiagnostics(); len(diags) > 0 {
			b.WriteString(diagnosticsText(diags))
		}
	}
	b.WriteString(text)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: b.String()}}}, out, nil
}

// progressInterval is how often a running call reports that it still runs.
const progressInterval = 2 * time.Second

// withProgress runs run, reporting progress to the client while it does when
// the call carries a progress token: the service's calls are one answer each, so
// the report is a heartbeat, not a count.
func withProgress(ctx context.Context, req *mcp.CallToolRequest, what string, run func() error) error {
	token := req.Params.GetProgressToken()
	if token == nil {
		return run()
	}
	notify := func(progress float64, message string) {
		params := &mcp.ProgressNotificationParams{ProgressToken: token, Progress: progress, Message: message}
		if err := req.Session.NotifyProgress(ctx, params); err != nil {
			slog.Debug("progress notification not delivered", "tool", req.Params.Name, "error", err)
		}
	}
	notify(0, what+" started")
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(progressInterval)
		defer ticker.Stop()
		for beat := 1.0; ; beat++ {
			select {
			case <-done:
				return
			case <-ticker.C:
				notify(beat, what+" running")
			}
		}
	}()
	err := run()
	close(done)
	<-finished
	if err == nil {
		notify(1e9, what+" finished")
	}
	return err
}

// diagnosticsText spells diagnostics as the CLI prints them, one per line.
func diagnosticsText(diags []*pb.Diagnostic) string {
	var b strings.Builder
	for _, d := range diags {
		b.WriteString(diagnosticText(d))
		b.WriteByte('\n')
	}
	return b.String()
}

func diagnosticText(d *pb.Diagnostic) string {
	var b strings.Builder
	if span := d.GetSpan(); span != nil && (span.GetFile() != "" || span.GetStartLine() > 0) {
		b.WriteString(span.GetFile())
		if span.GetStartLine() > 0 {
			fmt.Fprintf(&b, ":%d:%d", span.GetStartLine(), span.GetStartCol())
		}
		b.WriteString(": ")
	}
	fmt.Fprintf(&b, "%s: %s", d.GetSeverity(), d.GetMessage())
	if d.GetCode() != "" {
		fmt.Fprintf(&b, " [%s]", d.GetCode())
	}
	return b.String()
}

// valueText spells a Value as the REPL prints one.
func valueText(v *pb.Value) string {
	if v == nil {
		return "<no value>"
	}
	switch k := v.GetKind().(type) {
	case *pb.Value_IntValue:
		return fmt.Sprint(k.IntValue)
	case *pb.Value_RealValue:
		return fmt.Sprint(k.RealValue)
	case *pb.Value_BoolValue:
		return fmt.Sprint(k.BoolValue)
	case *pb.Value_StringValue:
		return fmt.Sprintf("%q", k.StringValue)
	case *pb.Value_InstanceId:
		return fmt.Sprintf("#%d", k.InstanceId)
	case *pb.Value_Sequence:
		return "(" + valuesText(k.Sequence.GetElements()) + ")"
	case *pb.Value_Set:
		return "{" + valuesText(k.Set.GetElements()) + "}"
	case *pb.Value_Null:
		if k.Null != "" {
			return "null (" + k.Null + ")"
		}
		return "null"
	case *pb.Value_Unset:
		return "<unset>"
	case *pb.Value_Undetermined:
		return "<undetermined>"
	case *pb.Value_Infinity:
		return "*"
	case *pb.Value_Quantity:
		return quantityText(k.Quantity)
	case *pb.Value_EnumLiteral:
		if k.EnumLiteral.GetLiteralId() != "" {
			return k.EnumLiteral.GetLiteralId()
		}
		return k.EnumLiteral.GetName()
	case *pb.Value_Complex:
		return fmt.Sprintf("%g + %gi", k.Complex.GetReal(), k.Complex.GetImaginary())
	case *pb.Value_BigIntValue:
		return k.BigIntValue
	case *pb.Value_RationalValue:
		return k.RationalValue.GetNumerator() + "/" + k.RationalValue.GetDenominator()
	case *pb.Value_MeasurementRef:
		return "[" + k.MeasurementRef.GetUnit() + "]"
	case *pb.Value_Function:
		return k.Function.GetCalcId()
	case *pb.Value_Metaobject:
		return k.Metaobject.GetElementId() + " meta " + k.Metaobject.GetMetaclassId()
	}
	raw, err := protojson.Marshal(v)
	if err != nil {
		return "<value>"
	}
	return string(raw)
}

func valuesText(vs []*pb.Value) string {
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = valueText(v)
	}
	return strings.Join(parts, ", ")
}

func quantityText(q *pb.Quantity) string {
	var magnitude string
	switch m := q.GetMagnitude().(type) {
	case *pb.Quantity_IntMagnitude:
		magnitude = fmt.Sprint(m.IntMagnitude)
	case *pb.Quantity_RealMagnitude:
		magnitude = fmt.Sprint(m.RealMagnitude)
	case *pb.Quantity_BigIntMagnitude:
		magnitude = m.BigIntMagnitude
	case *pb.Quantity_RationalMagnitude:
		magnitude = m.RationalMagnitude.GetNumerator() + "/" + m.RationalMagnitude.GetDenominator()
	}
	if q.GetUnit() == "" {
		return magnitude
	}
	return magnitude + " [" + q.GetUnit() + "]"
}

// namedValuesText spells a name → value map in name order, one per line.
func namedValuesText(values map[string]*pb.Value) string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "%s = %s\n", name, valueText(values[name]))
	}
	return b.String()
}

// markedValuesText is namedValuesText with the order-dependence marker on each
// value the run's unresolved choice points touch, as the REPL prints it.
func markedValuesText(values map[string]*pb.Value, orderDependent map[string]int32) string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "%s = %s", name, valueText(values[name]))
		if marker := runtime.OrderDependenceText(int(orderDependent[name])); marker != "" {
			b.WriteString(" " + marker)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// errorText is the first line of a text for an answer whose `error` is set.
func errorText(err string) string {
	if err == "" {
		return ""
	}
	return "error: " + err + "\n"
}

// sortStrings sorts names in place, for maps spelled in name order.
func sortStrings(names []string) { sort.Strings(names) }
