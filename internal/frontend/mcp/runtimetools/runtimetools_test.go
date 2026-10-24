package runtimetools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// demoModel exercises every runtime tool: values to evaluate, a part to
// instantiate and validate, an action, a state machine, constraints,
// requirements and satisfactions that hold and fail, calcs, an analysis case
// to run and sweep, and a view to render.
const demoModel = `package Demo {
	private import ScalarValues::*;
	private import Views::*;

	part def Vehicle {
		attribute mass : Real default = 1500.0;
		constraint massPositive { mass > 0.0 }
		constraint massLight { mass < 100.0 }
		requirement lightEnough { require constraint { mass < 2000.0 } }
	}

	requirement def MassLimit {
		subject vehicle : Vehicle;
		attribute maxMass : Real;
		require constraint { vehicle.mass <= maxMass }
	}
	requirement massLimit : MassLimit { attribute :>> maxMass = 2000.0; }
	requirement massTiny : MassLimit { attribute :>> maxMass = 10.0; }

	part sedan : Vehicle { attribute :>> mass = 1200.0; }

	part checks {
		assert satisfy massLimit by sedan;
		assert satisfy massTiny by sedan;
	}

	calc def Twice { in n : Integer[1]; return : Integer = n * 2; }
	calc add { in x : Real = 1.0; in y : Real = 2.0; return : Real = x + y; }

	action def Count {
		in lo : Integer;
		out hi : Integer;
		attribute n : Integer = 0;
		first start;
		then action bump assign n := lo + 1;
		then action emit assign hi := n;
		then done;
	}

	action def Spin {
		attribute n : Integer = 0;
		first start;
		then action a assign n := n + 1;
		then action b assign n := n + 1;
		then action c assign n := n + 1;
		then done;
	}

	state def Lamp {
		attribute level : Integer = 0;
		entry; then off;
		state off;
		state on;
		transition off_on first off accept Switch then on do assign level := level + 1;
		transition on_off first on accept Switch then off;
	}
	attribute def Switch;

	analysis def CostAnalysis {
		subject s : Vehicle;
		in limit : Real = 2000.0;
		out total : Real = s.mass * 2.0;
		objective affordable { require constraint { total <= limit * 2.0 } }
	}
	analysis sedanCost : CostAnalysis { subject s = sedan; }

	view overview {
		expose Vehicle;
		render asInterconnectionDiagram;
	}
}
`

// telescopeFixture is the document pipeline's own fixture, which declares
// document queries and a document to render.
const telescopeFixture = "../../../doc/docrender/testdata/telescope_report.sysml"

// migrateFixture is a small SysML v1 XMI document.
const migrateFixture = "../../../../tests/migrate/testdata/xmi/association_qualifiers.xmi"

// harness is a runtime tool group connected to a client in memory.
type harness struct {
	t      *testing.T
	root   string
	client *mcp.ClientSession
	// progress holds every progress notification the client received.
	mu       sync.Mutex
	progress []*mcp.ProgressNotificationParams
}

func newService(t *testing.T, unavailable ...string) *sysmlgrpc.Service {
	t.Helper()
	svc, err := sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(16, "test", unavailable)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Close)
	return svc
}

// attach registers the group over svc, with the demo model written to a fresh
// workspace root as demo.sysml, and returns the connected client.
func attach(t *testing.T, svc *sysmlgrpc.Service) *harness {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "demo.sysml"), []byte(demoModel), 0o600); err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, root: root}
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "0"}, nil)
	if err := Register(server, svc, root); err != nil {
		t.Fatal(err)
	}
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	go func() { _ = server.Run(ctx, serverT) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, req *mcp.ProgressNotificationClientRequest) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.progress = append(h.progress, req.Params)
		},
	})
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	h.client = cs
	return h
}

// call invokes a tool and fails the test when the call itself fails; the
// result's IsError is the caller's to judge.
func (h *harness) call(name string, args map[string]any) *mcp.CallToolResult {
	h.t.Helper()
	res, err := h.client.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		h.t.Fatalf("%s: %v", name, err)
	}
	return res
}

// ok calls a tool that must succeed and decodes its structured content into
// resp through protojson, which proves the result reads back as the response.
func (h *harness) ok(name string, args map[string]any, resp proto.Message) (*mcp.CallToolResult, string) {
	h.t.Helper()
	res := h.call(name, args)
	if res.IsError {
		h.t.Fatalf("%s failed: %s", name, text(res))
	}
	roundTrip(h.t, res, resp)
	return res, text(res)
}

// fails calls a tool that must fail and returns the error text.
func (h *harness) fails(name string, args map[string]any) string {
	h.t.Helper()
	res := h.call(name, args)
	if !res.IsError {
		h.t.Fatalf("%s succeeded: %s", name, text(res))
	}
	return text(res)
}

// text is the text content of a result, joined.
func text(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// structured is the structured content as a map.
func structured(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("structured content %s: %v", raw, err)
	}
	return out
}

// roundTrip reads the structured content back as resp: the tool's output is
// the response message as protojson writes it, plus modelHash when the tool
// parsed the model itself.
func roundTrip(t *testing.T, res *mcp.CallToolResult, resp proto.Message) {
	t.Helper()
	out := structured(t, res)
	if resp.ProtoReflect().Descriptor().Fields().ByJSONName("modelHash") == nil {
		delete(out, "modelHash")
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := protojson.Unmarshal(raw, resp); err != nil {
		t.Fatalf("structured content does not read as a %T: %v\n%s", resp, err, raw)
	}
}

func (h *harness) parse() string {
	h.t.Helper()
	var resp pb.ParseSourcesResponse
	_, out := h.ok("parse", map[string]any{"paths": []string{"demo.sysml"}}, &resp)
	if resp.GetModelHash() == "" {
		h.t.Fatalf("parse answered no model hash: %s", out)
	}
	for _, d := range resp.GetDiagnostics() {
		if d.GetSeverity() == "error" {
			h.t.Fatalf("the demo model has errors: %s", out)
		}
	}
	return resp.GetModelHash()
}

func value(kind string, v any) map[string]any { return map[string]any{kind: v} }

func TestEveryToolTakingAModelTakesPathsAndSources(t *testing.T) {
	h := attach(t, newService(t))
	res, err := h.client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	modelTools := 0
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		switch {
		case tool.Annotations == nil:
			t.Errorf("%s declares no annotations", tool.Name)
		case tool.Name == "instantiate":
			if tool.Annotations.ReadOnlyHint || tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
				t.Errorf("instantiate adds an object the server keeps: annotations = %+v", tool.Annotations)
			}
		case !tool.Annotations.ReadOnlyHint:
			t.Errorf("%s is not annotated read-only", tool.Name)
		}
		if tool.Description == "" {
			t.Errorf("%s has no description", tool.Name)
		}
		raw, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Properties map[string]any `json:"properties"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("%s: input schema %s: %v", tool.Name, raw, err)
		}
		if _, takesModel := schema.Properties["modelHash"]; !takesModel {
			continue
		}
		modelTools++
		for _, key := range []string{"paths", "sources", "strictConformance"} {
			if _, ok := schema.Properties[key]; !ok {
				t.Errorf("%s takes modelHash but not %s", tool.Name, key)
			}
		}
	}
	if modelTools < 15 {
		t.Errorf("%d tools take a model, want the model tools: %v", modelTools, names)
	}
	slices.Sort(names)
	want := []string{"apply_edits", "check_behavior", "compare_results", "convert", "diagnostics", "evaluate", "evaluate_calc", "execute_action", "execute_state", "explain_diagnostic", "export_graphs", "import_data", "instantiate", "list_engines", "migrate", "parse", "query", "record_run", "render_model_document", "render_model_view", "run_analysis", "run_document_query", "run_runs", "run_sweep", "self_check", "server_info", "symbol", "validate_instance", "verify_constraint", "verify_requirement", "verify_satisfaction"}
	if !slices.Equal(names, want) {
		t.Errorf("tools = %v, want %v", names, want)
	}
}

func TestParseReportsRootsDiagnosticsAndAHash(t *testing.T) {
	h := attach(t, newService(t))
	var resp pb.ParseSourcesResponse
	_, out := h.ok("parse", map[string]any{
		"paths":   []string{"demo.sysml"},
		"sources": []map[string]any{{"path": "extra.sysml", "text": "package Extra { part def Q; part broken : Missing; }"}},
	}, &resp)
	if len(resp.GetRoots()) != 2 {
		t.Fatalf("roots = %d, want 2: %s", len(resp.GetRoots()), out)
	}
	if !strings.Contains(out, "model "+resp.GetModelHash()) || !strings.Contains(out, "document 1 declares: Demo") {
		t.Errorf("text lacks the hash or the roots:\n%s", out)
	}
	var errors int
	for _, d := range resp.GetDiagnostics() {
		if d.GetSeverity() == "error" && strings.Contains(d.GetSpan().GetFile(), "extra.sysml") {
			errors++
		}
	}
	if errors == 0 {
		t.Errorf("the unresolved name in extra.sysml produced no error diagnostic:\n%s", out)
	}
	if !strings.Contains(out, "extra.sysml") {
		t.Errorf("text does not name the document of the diagnostic:\n%s", out)
	}

	// The same documents parse to the same hash; a different set to another.
	hash := h.parse()
	if again := h.parse(); again != hash {
		t.Errorf("parsing the same file twice gave %s then %s", hash, again)
	}
	if hash == resp.GetModelHash() {
		t.Error("two document sets share one hash")
	}

	// A KerML source is read as KerML by its extension.
	var kerml pb.ParseSourcesResponse
	h.ok("parse", map[string]any{"sources": []map[string]any{{"path": "k.kerml", "text": "package K { class C; }"}}}, &kerml)
	for _, d := range kerml.GetDiagnostics() {
		if d.GetSeverity() == "error" {
			t.Errorf("KerML source read as SysML: %s", d.GetMessage())
		}
	}

	// An empty inline document is a document, as the service has it.
	var blank pb.ParseSourcesResponse
	h.ok("parse", map[string]any{"sources": []map[string]any{{"path": "blank.sysml", "text": ""}}}, &blank)
	if blank.GetModelHash() == "" || len(blank.GetRoots()) != 1 {
		t.Errorf("an empty document: %v", &blank)
	}

	if msg := h.fails("parse", map[string]any{}); !strings.Contains(msg, "INVALID_ARGUMENT") {
		t.Errorf("parse of nothing: %s", msg)
	}
	if msg := h.fails("parse", map[string]any{"paths": []string{"missing.sysml"}}); !strings.Contains(msg, "missing.sysml") {
		t.Errorf("parse of a missing file: %s", msg)
	}
}

func TestSymbolDescribesAnElement(t *testing.T) {
	h := attach(t, newService(t))
	var resp pb.SymbolResponse
	res, out := h.ok("symbol", map[string]any{"paths": []string{"demo.sysml"}, "symbolId": "Demo::Vehicle"}, &resp)
	if resp.GetSymbol().GetKind() != "partDef" || !strings.Contains(out, "partDef Demo::Vehicle") {
		t.Errorf("symbol = %v\n%s", resp.GetSymbol(), out)
	}
	if resp.GetSymbol().GetMetadata()["visibility"] != "default" || !strings.Contains(out, "metadata: visibility=default") {
		t.Errorf("symbol metadata = %v\n%s", resp.GetSymbol().GetMetadata(), out)
	}
	if structured(t, res)["modelHash"] == "" {
		t.Error("an implicit parse did not answer the model hash")
	}
	hash := h.parse()
	res, out = h.ok("symbol", map[string]any{"modelHash": hash, "symbolId": "Demo::Nowhere"}, &resp)
	if resp.GetError() == "" || !strings.HasPrefix(out, "error: ") {
		t.Errorf("unknown symbol: %v\n%s", resp.GetError(), out)
	}
	if _, has := structured(t, res)["modelHash"]; has {
		t.Error("a call naming a hash repeated it in the result")
	}
}

func TestEvaluateSpellsTheValueAndTheError(t *testing.T) {
	h := attach(t, newService(t))
	hash := h.parse()
	var resp pb.EvaluateResponse
	_, out := h.ok("evaluate", map[string]any{"modelHash": hash, "expression": "1 + 2 * 3"}, &resp)
	if resp.GetResult().GetIntValue() != 7 || !strings.HasPrefix(out, "7") {
		t.Errorf("1 + 2 * 3 = %v\n%s", resp.GetResult(), out)
	}
	_, out = h.ok("evaluate", map[string]any{"modelHash": hash, "expression": "mass", "subjectSymbolId": "Demo::sedan"}, &resp)
	if resp.GetResult().GetRealValue() != 1200.0 {
		t.Errorf("sedan.mass = %v\n%s", resp.GetResult(), out)
	}
	_, out = h.ok("evaluate", map[string]any{"modelHash": hash, "expression": "nothing + 1"}, &resp)
	if resp.GetError() == "" || !strings.HasPrefix(out, "error: ") {
		t.Errorf("an unresolved name evaluated: %v\n%s", resp.GetResult(), out)
	}
	if msg := h.fails("evaluate", map[string]any{"expression": "1"}); !strings.Contains(msg, "INVALID_ARGUMENT") {
		t.Errorf("evaluate without a model: %s", msg)
	}
	if msg := h.fails("evaluate", map[string]any{"modelHash": hash, "paths": []string{"demo.sysml"}, "expression": "1"}); !strings.Contains(msg, "not both") {
		t.Errorf("evaluate with a hash and paths: %s", msg)
	}
}

func TestInstantiateReportsTheObject(t *testing.T) {
	h := attach(t, newService(t))
	var resp pb.InstantiateResponse
	_, out := h.ok("instantiate", map[string]any{"paths": []string{"demo.sysml"}, "symbolId": "Demo::sedan"}, &resp)
	if resp.GetInstance().GetTypeSymbolId() != "Demo::sedan" {
		t.Errorf("instance = %v", resp.GetInstance())
	}
	if !strings.Contains(out, "mass = 1200") {
		t.Errorf("text lacks the mass:\n%s", out)
	}
}

func TestExecuteActionBindsInputsAndReportsOutputs(t *testing.T) {
	h := attach(t, newService(t))
	hash := h.parse()
	var resp pb.ExecuteActionResponse
	_, out := h.ok("execute_action", map[string]any{
		"modelHash": hash, "actionSymbolId": "Demo::Count", "inputs": map[string]any{"lo": value("intValue", "41")},
	}, &resp)
	// The bare `out hi` is [0..*], so its one value is reported as a one-element sequence.
	hi := resp.GetOutputs()["hi"].GetSequence().GetElements()
	if resp.GetError() != "" || len(hi) != 1 || hi[0].GetIntValue() != 42 {
		t.Errorf("Count(41) = %v, error %q\n%s", resp.GetOutputs(), resp.GetError(), out)
	}
	if !strings.Contains(out, "hi = (42)") {
		t.Errorf("text lacks the output:\n%s", out)
	}

	_, out = h.ok("execute_action", map[string]any{"modelHash": hash, "actionSymbolId": "Demo::Spin", "schedule": "explore"}, &resp)
	if resp.GetExploration() == nil || !resp.GetExploration().GetComplete() || len(resp.GetOutcomes()) == 0 {
		t.Errorf("explore: %v\n%s", resp.GetExploration(), out)
	}
	if !strings.Contains(out, "exploration: ") {
		t.Errorf("text lacks the exploration:\n%s", out)
	}

	if msg := h.fails("execute_action", map[string]any{"modelHash": hash, "actionSymbolId": "Demo::Spin", "schedule": "sideways"}); !strings.Contains(msg, "INVALID_ARGUMENT") {
		t.Errorf("bad schedule: %s", msg)
	}
	if msg := h.fails("execute_action", map[string]any{
		"modelHash": hash, "actionSymbolId": "Demo::Count", "inputs": map[string]any{"lo": value("intvalue", "1")},
	}); !strings.Contains(msg, "INVALID_ARGUMENT") || !strings.Contains(msg, "ExecuteActionRequest") {
		t.Errorf("a misspelled Value arm: %s", msg)
	}
}

func TestExecuteStateRunsEventsAndTraces(t *testing.T) {
	h := attach(t, newService(t))
	var resp pb.ExecuteStateResponse
	_, out := h.ok("execute_state", map[string]any{
		"paths": []string{"demo.sysml"}, "stateMachineSymbolId": "Demo::Lamp", "events": []string{"Switch", "Switch", "Switch"}, "trace": true,
	}, &resp)
	if resp.GetError() != "" {
		t.Fatalf("Lamp: %s\n%s", resp.GetError(), out)
	}
	if got := resp.GetStatesVisited(); !slices.Equal(got, []string{"off", "on", "off", "on"}) {
		t.Errorf("states visited = %v", got)
	}
	if resp.GetFinalContext()["level"].GetIntValue() != 2 {
		t.Errorf("final context = %v", resp.GetFinalContext())
	}
	if len(resp.GetTrace()) == 0 || !strings.Contains(out, "trace:") {
		t.Errorf("no trace:\n%s", out)
	}
	if !strings.Contains(out, "states: off -> on -> off -> on") {
		t.Errorf("text lacks the states:\n%s", out)
	}
}

func TestVerificationToolsAnswerVerdicts(t *testing.T) {
	h := attach(t, newService(t))
	hash := h.parse()

	var constraint pb.VerifyConstraintResponse
	_, out := h.ok("verify_constraint", map[string]any{"modelHash": hash, "symbolId": "Demo::Vehicle::massLight", "subjectSymbolId": "Demo::sedan"}, &constraint)
	if constraint.GetVerdict().GetHolds() || constraint.GetVerdict().GetError() != "" {
		t.Errorf("massLight on sedan: %v", constraint.GetVerdict())
	}
	if !strings.Contains(out, "constraint ") || !strings.Contains(out, "violated") {
		t.Errorf("text:\n%s", out)
	}
	h.ok("verify_constraint", map[string]any{"modelHash": hash, "symbolId": "Demo::Vehicle::massPositive"}, &constraint)
	if !constraint.GetVerdict().GetHolds() {
		t.Errorf("massPositive on defaults: %v", constraint.GetVerdict())
	}

	var requirement pb.VerifyRequirementResponse
	_, out = h.ok("verify_requirement", map[string]any{"modelHash": hash, "symbolId": "Demo::massLimit", "subjectSymbolId": "Demo::sedan"}, &requirement)
	if !requirement.GetVerdict().GetHolds() || !strings.Contains(out, "holds") {
		t.Errorf("massLimit by sedan: %v\n%s", requirement.GetVerdict(), out)
	}

	var satisfaction pb.VerifySatisfactionResponse
	_, out = h.ok("verify_satisfaction", map[string]any{"modelHash": hash}, &satisfaction)
	if len(satisfaction.GetVerdicts()) != 2 || !satisfaction.GetVerdicts()[0].GetHolds() || satisfaction.GetVerdicts()[1].GetHolds() {
		t.Errorf("satisfactions: %v\n%s", satisfaction.GetVerdicts(), out)
	}
	if strings.Count(out, "satisfy ") != 2 {
		t.Errorf("text:\n%s", out)
	}

	var validation pb.ValidateInstanceResponse
	_, out = h.ok("validate_instance", map[string]any{"modelHash": hash, "symbolId": "Demo::sedan"}, &validation)
	if validation.GetSummary() == nil || !strings.Contains(out, "summary: object") {
		t.Errorf("validate sedan: %v\n%s", validation.GetSummary(), out)
	}
	if len(validation.GetVerdicts()) == 0 {
		t.Errorf("validate sedan answered no verdicts:\n%s", out)
	}

	var calc pb.EvaluateCalcResponse
	_, out = h.ok("evaluate_calc", map[string]any{"modelHash": hash, "symbolId": "Demo::Twice", "arguments": []any{value("intValue", 21)}}, &calc)
	if held := calc.GetResult().GetSequence().GetElements(); len(held) != 1 || held[0].GetIntValue() != 42 || !strings.HasPrefix(out, "(42)") {
		t.Errorf("Twice(21) = %v\n%s", calc.GetResult(), out)
	}
	_, out = h.ok("evaluate_calc", map[string]any{"modelHash": hash, "symbolId": "Demo::add"}, &calc)
	if held := calc.GetResult().GetSequence().GetElements(); len(held) != 1 || held[0].GetRealValue() != 3 || !strings.HasPrefix(out, "(3") {
		t.Errorf("add from its members: %v\n%s", calc.GetResult(), out)
	}

	var analysis pb.RunAnalysisResponse
	_, out = h.ok("run_analysis", map[string]any{"modelHash": hash, "symbolId": "Demo::sedanCost"}, &analysis)
	if analysis.GetError() != "" || len(analysis.GetOutputs()) == 0 || len(analysis.GetVerdicts()) == 0 {
		t.Errorf("sedanCost: %v %v %q\n%s", analysis.GetOutputs(), analysis.GetVerdicts(), analysis.GetError(), out)
	}
	if !strings.Contains(out, "total = (2400)") || !strings.Contains(out, "objective") {
		t.Errorf("text:\n%s", out)
	}

	var sweep pb.RunSweepResponse
	_, out = h.ok("run_sweep", map[string]any{
		"modelHash": hash, "symbolId": "Demo::Twice",
		"ranges": []any{map[string]any{"parameter": "n", "start": value("intValue", 1), "end": value("intValue", 3)}},
	}, &sweep)
	if sweep.GetError() != "" || len(sweep.GetRows()) != 3 {
		t.Errorf("sweep: %d rows, error %q\n%s", len(sweep.GetRows()), sweep.GetError(), out)
	}
	if !strings.Contains(out, "3 rows over n") || !strings.Contains(out, "n=3 -> result=(6)") {
		t.Errorf("text:\n%s", out)
	}
}

func TestQueriesAndRenderings(t *testing.T) {
	h := attach(t, newService(t))
	hash := h.parse()

	var query pb.QueryResponse
	_, out := h.ok("query", map[string]any{"modelHash": hash, "query": map[string]any{
		"scope":  []string{"Demo"},
		"select": []string{"name"},
		"where":  map[string]any{"primitive": map[string]any{"property": "@type", "operator": "PRIMITIVE_OPERATOR_EQUAL", "value": []string{"PartUsage"}}},
	}}, &query)
	var ids []string
	for _, e := range query.GetElements() {
		ids = append(ids, e.GetId())
	}
	if !slices.Contains(ids, "Demo::sedan") || !slices.Contains(ids, "Demo::checks") {
		t.Errorf("part usages = %v\n%s", ids, out)
	}
	for _, e := range query.GetElements() {
		if len(e.GetProperties()) != 1 || e.GetProperties()["name"] == "" {
			t.Errorf("%s selected %v, want name alone", e.GetId(), e.GetProperties())
		}
	}
	h.ok("query", map[string]any{"modelHash": hash, "oslcQuery": "oslc.where=sysml:name=\"sedan\""}, &query)
	if msg := h.fails("query", map[string]any{"modelHash": hash, "query": map[string]any{"where": map[string]any{"composite": map[string]any{"operator": "COMPOSITE_OPERATOR_AND"}}}}); !strings.Contains(msg, "INVALID_ARGUMENT") {
		t.Errorf("an empty composite: %s", msg)
	}

	telescope := h.copyIn(telescopeFixture, "telescope.sysml")
	var rows pb.RunDocumentQueryResponse
	_, out = h.ok("run_document_query", map[string]any{
		"paths": []string{telescope}, "queryId": "Observatory::SubsystemTable",
		"bindings": []any{map[string]any{"parameter": "root", "values": []any{map[string]any{"elementId": "Observatory::telescope"}}}},
	}, &rows)
	if len(rows.GetColumns()) != 2 || len(rows.GetRows()) != 4 {
		t.Errorf("SubsystemTable: %d columns, %d rows\n%s", len(rows.GetColumns()), len(rows.GetRows()), out)
	}
	if !strings.Contains(out, "columns: element | name | mass") || !strings.Contains(out, "Observatory::telescope::mount") {
		t.Errorf("text:\n%s", out)
	}

	var doc pb.RenderDocumentResponse
	documents := h.documentNames(telescope)
	if len(documents) == 0 {
		t.Fatal("the telescope fixture declares no document")
	}
	_, out = h.ok("render_model_document", map[string]any{"paths": []string{telescope}, "documentId": documents[0]}, &doc)
	if doc.GetMarkdown() == "" || !strings.Contains(out, strings.TrimRight(doc.GetMarkdown(), "\n")) {
		t.Errorf("markdown of %s:\n%s", documents[0], out)
	}
	h.ok("render_model_document", map[string]any{"paths": []string{telescope}, "documentId": documents[0], "form": "html"}, &doc)
	if doc.GetHtml() == "" || doc.GetMarkdown() != "" {
		t.Errorf("html form: markdown %d bytes, html %d bytes", len(doc.GetMarkdown()), len(doc.GetHtml()))
	}
	_, out = h.ok("render_model_document", map[string]any{"paths": []string{telescope}, "documentId": documents[0], "form": "asciidoc"}, &doc)
	if doc.GetAsciidoc() == "" || doc.GetMarkdown() != "" || doc.GetHtml() != "" || !strings.Contains(out, strings.TrimRight(doc.GetAsciidoc(), "\n")) {
		t.Errorf("asciidoc form: markdown %d bytes, html %d bytes, asciidoc %d bytes:\n%s", len(doc.GetMarkdown()), len(doc.GetHtml()), len(doc.GetAsciidoc()), out)
	}
	if msg := h.fails("render_model_document", map[string]any{"paths": []string{telescope}, "documentId": documents[0], "form": "pdf"}); !strings.Contains(msg, "INVALID_ARGUMENT") {
		t.Errorf("pdf form: %s", msg)
	}

	var view pb.RenderViewResponse
	_, out = h.ok("render_model_view", map[string]any{"modelHash": hash, "view": "Demo::overview"}, &view)
	if len(view.GetNodes()) == 0 || !strings.Contains(out, "node ") {
		t.Errorf("overview: %v\n%s", view.GetNodes(), out)
	}
	h.ok("render_model_view", map[string]any{"modelHash": hash, "view": "#tree:Demo"}, &view)
	if view.GetKind() != "tree" {
		t.Errorf("pseudo-view kind = %q", view.GetKind())
	}
	h.ok("render_model_view", map[string]any{"modelHash": hash, "view": "Demo::overview", "show": []string{"edge-labels"}}, &view)
	if !slices.Contains(view.GetShown(), "edge-labels") {
		t.Errorf("show edge-labels: shown = %v", view.GetShown())
	}

	var graphs pb.ExportGraphsResponse
	_, out = h.ok("export_graphs", map[string]any{"modelHash": hash, "subject": "Demo::Lamp"}, &graphs)
	if graphs.GetVersion() == 0 || !strings.HasPrefix(out, "graphs:") || !strings.Contains(out, strings.TrimRight(graphs.GetContent(), "\n")) {
		t.Errorf("export Lamp: version %d\n%s", graphs.GetVersion(), out)
	}
	var exported struct {
		States []json.RawMessage `json:"states"`
	}
	if err := json.Unmarshal([]byte(graphs.GetContent()), &exported); err != nil || len(exported.States) != 1 {
		t.Errorf("exported content: %v, %d states", err, len(exported.States))
	}
}

// documentNames are the DocumentQueries::Document specializations of a model.
// copyIn copies a fixture into the workspace root under name and answers the
// path a tool argument names it by: the tools read only under the root.
func (h *harness) copyIn(fixture, name string) string {
	h.t.Helper()
	data, err := os.ReadFile(fixture)
	if err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.root, name), data, 0o600); err != nil {
		h.t.Fatal(err)
	}
	return name
}

// Paths stay under the workspace root: a path that leaves it, lexically or
// through a symlink, is refused by every tool that reads a file, and a path
// under a link that stays inside the root is accepted.
func TestPathsStayUnderTheRoot(t *testing.T) {
	h := attach(t, newService(t))
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.sysml"), []byte(demoModel), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string]map[string]any{
		"parse ..":            {"paths": []string{"../" + filepath.Base(outside) + "/secret.sysml"}},
		"parse absolute":      {"paths": []string{filepath.Join(outside, "secret.sysml")}},
		"convert filePath":    {"filePath": filepath.Join(outside, "secret.sysml"), "toFormat": "ttl"},
		"migrate filePath":    {"filePath": filepath.Join(outside, "secret.sysml"), "toFormat": "sysml"},
		"migrate layoutPath":  {"filePath": "v1.xmi", "layoutPath": filepath.Join(outside, "layout.json"), "toFormat": "sysml"},
		"run_document_query ": {"paths": []string{filepath.Join(outside, "secret.sysml")}, "queryId": "Q"},
	} {
		tool := strings.Fields(name)[0]
		if msg := h.fails(tool, args); !strings.Contains(msg, "outside the workspace root") {
			t.Errorf("%s: %s", name, msg)
		}
	}
	if err := os.Symlink(filepath.Join(outside, "secret.sysml"), filepath.Join(h.root, "link.sysml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if msg := h.fails("parse", map[string]any{"paths": []string{"link.sysml"}}); !strings.Contains(msg, "outside the workspace root") {
		t.Errorf("a link out of the root: %s", msg)
	}
	if err := os.Symlink(filepath.Join(h.root, "demo.sysml"), filepath.Join(h.root, "inside.sysml")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.root, "broken.sysml"), []byte("package Broken {"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(h.root, "broken.sysml"), filepath.Join(h.root, "brokenlink.sysml")); err != nil {
		t.Fatal(err)
	}
	var resp pb.ParseSourcesResponse
	_, out := h.ok("parse", map[string]any{"paths": []string{"inside.sysml", "brokenlink.sysml"}}, &resp)
	resolved, err := filepath.EvalSymlinks(filepath.Join(h.root, "broken.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, resolved) || strings.Contains(out, "brokenlink.sysml") {
		t.Errorf("the service must be handed the resolved file, not the link:\n%s", out)
	}
	if err := os.Symlink(filepath.Join(h.root, "demo.sysml"), filepath.Join(h.root, "alias.kerml")); err != nil {
		t.Fatal(err)
	}
	if msg := h.fails("parse", map[string]any{"paths": []string{"alias.kerml"}}); !strings.Contains(msg, "whose extension names another format") {
		t.Errorf("a link whose extension differs from its target's: %s", msg)
	}
	if err := os.WriteFile(filepath.Join(h.root, "graph.turtle"), []byte("@prefix ex: <http://example.org/> .\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(h.root, "graph.turtle"), filepath.Join(h.root, "graph.ttl")); err != nil {
		t.Fatal(err)
	}
	if res := h.call("convert", map[string]any{"filePath": "graph.ttl", "toFormat": "sysml"}); strings.Contains(text(res), "another format") {
		t.Errorf(".ttl and .turtle name the same format: %s", text(res))
	}
	if res := h.call("parse", map[string]any{"paths": []string{"alias.kerml"}}); !strings.Contains(text(res), "another format") {
		t.Errorf("a link refused for its format must say so: %s", text(res))
	}
	if res := h.call("convert", map[string]any{"filePath": "alias.kerml", "fromFormat": "sysml", "toFormat": "ttl"}); !strings.Contains(text(res), "another format") {
		t.Errorf("an explicit fromFormat does not choose the language, so the .kerml link is still refused: %s", text(res))
	}
	if err := os.Symlink(filepath.Join(h.root, "demo.sysml"), filepath.Join(h.root, "input.data")); err != nil {
		t.Fatal(err)
	}
	if res := h.call("convert", map[string]any{"filePath": "input.data", "fromFormat": "sysml", "toFormat": "ttl"}); !strings.Contains(text(res), "another format") {
		t.Errorf("the parser reads a name with neither model extension as its own kind, so the link is refused even with fromFormat: %s", text(res))
	}
	if err := os.Symlink(filepath.Join(h.root, "demo.sysml"), filepath.Join(h.root, "copy.sysml")); err != nil {
		t.Fatal(err)
	}
	if res := h.call("convert", map[string]any{"filePath": "copy.sysml", "fromFormat": "sysml", "toFormat": "ttl"}); res.IsError {
		t.Errorf("a link of the target's kind converts: %s", text(res))
	}
	if msg := h.fails("parse", map[string]any{"paths": []string{"missing/dir/new.sysml"}}); strings.Contains(msg, "outside") {
		t.Errorf("a path under the root that does not exist yet is not outside it: %s", msg)
	}
}

func (h *harness) documentNames(path string) []string {
	h.t.Helper()
	var query pb.QueryResponse
	h.ok("query", map[string]any{"paths": []string{path}, "query": map[string]any{
		"select": []string{"name"},
		"where":  map[string]any{"primitive": map[string]any{"property": "@type", "operator": "PRIMITIVE_OPERATOR_EQUAL", "value": []string{"PartDefinition"}}},
	}}, &query)
	var names []string
	for _, e := range query.GetElements() {
		var sym pb.SymbolResponse
		h.ok("symbol", map[string]any{"paths": []string{path}, "symbolId": e.GetId()}, &sym)
		for _, sp := range sym.GetSymbol().GetSpecializations() {
			if sp.GetTargetId() == "DocumentQueries::Document" {
				names = append(names, e.GetId())
			}
		}
	}
	return names
}

func TestConvertMigrateAndApplyEditsAnswerText(t *testing.T) {
	h := attach(t, newService(t))
	hash := h.parse()

	var convert pb.ConvertResponse
	_, out := h.ok("convert", map[string]any{"modelHash": hash, "toFormat": "ttl"}, &convert)
	if convert.GetError() != "" || !strings.Contains(convert.GetContent(), "@prefix") || !convert.GetExperimental() {
		t.Errorf("to ttl: error %q, experimental %v\n%s", convert.GetError(), convert.GetExperimental(), out)
	}
	if !strings.Contains(out, "notice: ") || !strings.Contains(out, strings.TrimRight(convert.GetContent(), "\n")) {
		t.Errorf("text:\n%s", out)
	}
	h.ok("convert", map[string]any{"filePath": "demo.sysml", "toFormat": "sysml"}, &convert)
	if convert.GetFromFormat() != "sysml" || !strings.Contains(convert.GetContent(), "part def Vehicle") {
		t.Errorf("file to notation: from %q\n%s", convert.GetFromFormat(), convert.GetContent())
	}
	h.ok("convert", map[string]any{"content": "package P { part def Q; }", "fromFormat": "sysml", "toFormat": "api-json"}, &convert)
	if !strings.Contains(convert.GetContent(), "PartDefinition") {
		t.Errorf("content to api-json:\n%s", convert.GetContent())
	}
	h.ok("convert", map[string]any{"content": "", "fromFormat": "sysml", "toFormat": "sysml"}, &convert)
	if convert.GetFromFormat() != "sysml" {
		t.Errorf("an empty inline document was not converted: %v", &convert)
	}
	if msg := h.fails("convert", map[string]any{"toFormat": "ttl"}); !strings.Contains(msg, "exactly one source") {
		t.Errorf("convert of nothing: %s", msg)
	}
	if msg := h.fails("convert", map[string]any{"modelHash": hash, "filePath": "demo.sysml", "toFormat": "ttl"}); !strings.Contains(msg, "exactly one source") {
		t.Errorf("convert of two sources: %s", msg)
	}
	if msg := h.fails("convert", map[string]any{"modelHash": hash}); !strings.Contains(msg, "toFormat") {
		t.Errorf("convert without a target format: %s", msg)
	}

	xmi, err := os.ReadFile(migrateFixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.root, "v1.xmi"), xmi, 0o600); err != nil {
		t.Fatal(err)
	}
	var migrate pb.MigrateResponse
	_, out = h.ok("migrate", map[string]any{"filePath": "v1.xmi", "toFormat": "sysml", "report": true}, &migrate)
	if migrate.GetError() != "" || !strings.Contains(migrate.GetContent(), "occurrence def A") || migrate.GetReport().GetMapped() == 0 {
		t.Errorf("migrate: error %q, mapped %d\n%s", migrate.GetError(), migrate.GetReport().GetMapped(), migrate.GetContent())
	}
	if !strings.Contains(out, migrate.GetReport().GetSummary()) || !strings.Contains(out, strings.TrimRight(migrate.GetContent(), "\n")) {
		t.Errorf("text:\n%s", out)
	}
	h.ok("migrate", map[string]any{"content": base64.StdEncoding.EncodeToString(xmi), "fromFormat": "xmi", "toFormat": "sysml"}, &migrate)
	if migrate.GetError() != "" || !strings.Contains(migrate.GetContent(), "occurrence def B") {
		t.Errorf("migrate inline: error %q\n%s", migrate.GetError(), migrate.GetContent())
	}
	if msg := h.fails("migrate", map[string]any{"toFormat": "sysml"}); !strings.Contains(msg, "exactly one source") {
		t.Errorf("migrate of nothing: %s", msg)
	}

	var edits pb.ApplyEditsResponse
	_, out = h.ok("apply_edits", map[string]any{
		"modelHash":  hash,
		"operations": []any{map[string]any{"setValue": map[string]any{"target": "Demo::sedan::mass", "value": "1300.0"}}},
	}, &edits)
	if edits.GetError() != "" || !strings.Contains(edits.GetContent(), "mass = 1300.0") || len(edits.GetApplied()) != 1 {
		t.Errorf("set value: error %q, %d applied\n%s", edits.GetError(), len(edits.GetApplied()), edits.GetContent())
	}
	if !strings.Contains(out, `"1200.0" -> "1300.0"`) || !strings.Contains(out, strings.TrimRight(edits.GetContent(), "\n")) {
		t.Errorf("text:\n%s", out)
	}
	if data, err := os.ReadFile(filepath.Join(h.root, "demo.sysml")); err != nil || string(data) != demoModel {
		t.Error("apply_edits wrote to the workspace")
	}
	_, out = h.ok("apply_edits", map[string]any{
		"modelHash":  hash,
		"operations": []any{map[string]any{"setValue": map[string]any{"target": "Demo::nowhere", "value": "1"}}},
	}, &edits)
	if edits.GetError() == "" || edits.GetFailure() != pb.EditFailure_EDIT_FAILURE_UNKNOWN_TARGET || !strings.Contains(out, "EDIT_FAILURE_UNKNOWN_TARGET") {
		t.Errorf("unknown target: %q %v\n%s", edits.GetError(), edits.GetFailure(), out)
	}
	if msg := h.fails("apply_edits", map[string]any{"modelHash": hash, "operations": []any{map[string]any{"setvalue": map[string]any{}}}}); !strings.Contains(msg, "ApplyEditsRequest") {
		t.Errorf("a misspelled operation arm: %s", msg)
	}
}

func TestListEnginesAndServerInfo(t *testing.T) {
	h := attach(t, newService(t))
	var engines pb.ListEnginesResponse
	_, out := h.ok("list_engines", map[string]any{}, &engines)
	if len(engines.GetEngines()) == 0 || !strings.Contains(out, engines.GetEngines()[0].GetName()) {
		t.Errorf("engines: %v\n%s", engines.GetEngines(), out)
	}
	res := h.call("server_info", map[string]any{})
	if res.IsError {
		t.Fatalf("server_info: %s", text(res))
	}
	var info struct {
		Version      string   `json:"version"`
		Capabilities []string `json:"capabilities"`
	}
	raw, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &info); err != nil {
		t.Fatal(err)
	}
	if info.Version != "test" || !slices.Contains(info.Capabilities, sysmlgrpc.CapabilityVerification) {
		t.Errorf("server info = %+v", info)
	}
}

func TestAnUnknownModelHashIsNotFound(t *testing.T) {
	h := attach(t, newService(t))
	for name, args := range map[string]map[string]any{
		"symbol":                {"symbolId": "Demo"},
		"evaluate":              {"expression": "1"},
		"instantiate":           {"symbolId": "Demo::sedan"},
		"execute_action":        {"actionSymbolId": "Demo::Spin"},
		"execute_state":         {"stateMachineSymbolId": "Demo::Lamp"},
		"verify_constraint":     {"symbolId": "Demo::Vehicle::massPositive"},
		"verify_requirement":    {"symbolId": "Demo::massLimit"},
		"verify_satisfaction":   {},
		"validate_instance":     {"symbolId": "Demo::sedan"},
		"evaluate_calc":         {"symbolId": "Demo::add"},
		"run_analysis":          {"symbolId": "Demo::sedanCost"},
		"run_sweep":             {"symbolId": "Demo::Twice", "ranges": []any{}},
		"run_runs":              {"actionSymbolId": "Demo::Spin", "runs": 1, "draws": "min"},
		"self_check":            {},
		"record_run":            {"analysisSymbolId": "Demo::sedanCost"},
		"query":                 {},
		"run_document_query":    {"queryId": "Q"},
		"render_model_document": {"documentId": "D"},
		"render_model_view":     {"view": "Demo::overview"},
		"export_graphs":         {"subject": "Demo::Lamp"},
		"convert":               {"toFormat": "ttl"},
		"apply_edits":           {"operations": []any{map[string]any{"setValue": map[string]any{"target": "x", "value": "1"}}}},
		"import_data":           {"imports": []any{map[string]any{"name": "d.csv", "content": "element,x\n"}}},
		"compare_results":       {"results": `{"source":"t","configurations":[]}`},
	} {
		args["modelHash"] = "0000000000000000000000000000000000000000000000000000000000000000"
		msg := h.fails(name, args)
		if !strings.HasPrefix(msg, "NOT_FOUND: ") {
			t.Errorf("%s with an unknown hash: %s", name, msg)
		}
	}
}

func TestAWithheldCapabilityIsUnimplemented(t *testing.T) {
	h := attach(t, newService(t, sysmlgrpc.CapabilityVerification, sysmlgrpc.CapabilityConvert, sysmlgrpc.CapabilityStateTrace))
	hash := h.parse()
	for name, args := range map[string]map[string]any{
		"verify_constraint": {"symbolId": "Demo::Vehicle::massPositive"},
		"run_analysis":      {"symbolId": "Demo::sedanCost"},
		"convert":           {"toFormat": "ttl"},
		"execute_state":     {"stateMachineSymbolId": "Demo::Lamp", "trace": true},
	} {
		args["modelHash"] = hash
		msg := h.fails(name, args)
		if !strings.HasPrefix(msg, "UNIMPLEMENTED: ") || !strings.Contains(msg, "capability") {
			t.Errorf("%s without its capability: %s", name, msg)
		}
	}
	var resp pb.ExecuteStateResponse
	h.ok("execute_state", map[string]any{"modelHash": hash, "stateMachineSymbolId": "Demo::Lamp", "events": []string{"Switch"}}, &resp)
	if resp.GetError() != "" || len(resp.GetTrace()) != 0 {
		t.Errorf("a run without a trace: %q, %d records", resp.GetError(), len(resp.GetTrace()))
	}
}

func TestAnExhaustedBudgetIsReportedInTheAnswer(t *testing.T) {
	t.Setenv(runtime.MaxActionStepsEnvVar, "1")
	h := attach(t, newService(t))
	var resp pb.ExecuteActionResponse
	_, out := h.ok("execute_action", map[string]any{"paths": []string{"demo.sysml"}, "actionSymbolId": "Demo::Spin"}, &resp)
	if resp.GetError() == "" || !strings.Contains(strings.ToLower(resp.GetError()), "step") {
		t.Fatalf("Spin under one action step: error %q\n%s", resp.GetError(), out)
	}
	if !strings.HasPrefix(strings.SplitN(out, "\n", 3)[1], "error: ") && !strings.Contains(out, "\nerror: ") {
		t.Errorf("text does not lead with the error:\n%s", out)
	}
}

func TestLongRunningToolsReportProgressWhenAsked(t *testing.T) {
	h := attach(t, newService(t))
	hash := h.parse()
	params := &mcp.CallToolParams{Name: "execute_action", Arguments: map[string]any{"modelHash": hash, "actionSymbolId": "Demo::Spin"}}
	params.SetProgressToken("spin-1")
	res, err := h.client.CallTool(context.Background(), params)
	if err != nil || res.IsError {
		t.Fatalf("execute_action: %v %s", err, text(res))
	}
	// The client dispatches notifications on its own goroutine, so the finish
	// may land after the result does.
	progress := h.progressUntil(t, "finished")
	if len(progress) < 2 {
		t.Fatalf("progress notifications = %v, want the start and the finish", progress)
	}
	for _, p := range progress {
		if p.ProgressToken != "spin-1" {
			t.Errorf("progress token = %v", p.ProgressToken)
		}
	}
	if first, last := progress[0], progress[len(progress)-1]; first.Progress >= last.Progress || !strings.Contains(last.Message, "finished") {
		t.Errorf("progress ran %v to %v", first, last)
	}
}

// progressUntil returns the notifications received once one carries message,
// or whatever arrived within the deadline.
func (h *harness) progressUntil(t *testing.T, message string) []*mcp.ProgressNotificationParams {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		h.mu.Lock()
		got := append([]*mcp.ProgressNotificationParams(nil), h.progress...)
		h.mu.Unlock()
		if n := len(got); n > 0 && strings.Contains(got[n-1].Message, message) || time.Now().After(deadline) {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestATokenlessCallSendsNoProgress(t *testing.T) {
	h := attach(t, newService(t))
	h.ok("execute_action", map[string]any{"paths": []string{"demo.sysml"}, "actionSymbolId": "Demo::Spin"}, &pb.ExecuteActionResponse{})
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.progress) != 0 {
		t.Errorf("progress notifications without a token: %v", h.progress)
	}
}

// Every capability the service can report is served by a registered tool, so
// server_info never advertises what no tool acts on; a capability the service
// withholds is refused by that tool (TestAWithheldCapabilityIsUnimplemented).
func TestEveryCapabilityIsServedByATool(t *testing.T) {
	h := attach(t, newService(t))
	res, err := h.client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	registered := map[string]bool{}
	for _, tool := range res.Tools {
		registered[tool.Name] = true
	}
	for _, capability := range sysmlgrpc.Capabilities() {
		tool, mapped := CapabilityTools[capability]
		if !mapped {
			t.Errorf("capability %q is served by no tool", capability)
			continue
		}
		if !registered[tool] {
			t.Errorf("capability %q names tool %q, which is not registered", capability, tool)
		}
	}
	for capability := range CapabilityTools {
		if !slices.Contains(sysmlgrpc.Capabilities(), capability) {
			t.Errorf("CapabilityTools names %q, which the service does not report", capability)
		}
	}
	var info pb.ServerInfoResponse
	resp := h.call("server_info", map[string]any{})
	raw, _ := json.Marshal(resp.StructuredContent)
	if err := protojson.Unmarshal(raw, &info); err != nil {
		t.Fatal(err)
	}
	for _, capability := range info.GetCapabilities() {
		if !registered[CapabilityTools[capability]] {
			t.Errorf("server_info advertises %q without a tool behind it", capability)
		}
	}
}

func TestDiagnosticsReportsEveryDiagnostic(t *testing.T) {
	h := attach(t, newService(t))
	var resp pb.DiagnosticsResponse
	res, out := h.ok("diagnostics", map[string]any{"paths": []string{"demo.sysml"}}, &resp)
	if len(resp.GetDiagnostics()) != 0 || !strings.HasSuffix(out, "no diagnostics") {
		t.Errorf("a clean model: %v\n%s", resp.GetDiagnostics(), out)
	}
	if structured(t, res)["modelHash"] == "" {
		t.Error("an implicit parse did not answer the model hash")
	}
	bad := "package Broken { part def A { attribute m : Real = nowhere; } }"
	res, out = h.ok("diagnostics", map[string]any{"sources": []map[string]any{{"path": "broken.sysml", "text": bad}}}, &resp)
	if len(resp.GetDiagnostics()) == 0 {
		t.Fatalf("a model naming an unknown symbol reports no diagnostic\n%s", out)
	}
	if d := resp.GetDiagnostics()[0]; strings.Count(out, d.GetMessage()) != 1 || !strings.Contains(out, "broken.sysml") {
		t.Errorf("the text does not spell the diagnostic %v exactly once:\n%s", d, out)
	}
	if n := len(resp.GetDiagnostics()); strings.Count(out, "\n") != n {
		t.Errorf("the text has %d lines after the model hash for %d diagnostics:\n%s", strings.Count(out, "\n"), n, out)
	}
	if _, has := structured(t, res)["diagnostics"]; !has {
		t.Error("the structured result is not the DiagnosticsResponse")
	}
	if msg := h.fails("diagnostics", map[string]any{"modelHash": "nope"}); !strings.Contains(msg, "NOT_FOUND") {
		t.Errorf("an unknown hash: %s", msg)
	}
}

func TestADanglingLinkIsJudgedByWhereItPoints(t *testing.T) {
	h := attach(t, newService(t))
	if err := os.Symlink(filepath.Join(filepath.Dir(h.root), "not-yet.sysml"), filepath.Join(h.root, "dangle.sysml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for name, args := range map[string]map[string]any{
		"parse":   {"paths": []string{"dangle.sysml"}},
		"convert": {"filePath": "dangle.sysml", "toFormat": "ttl"},
	} {
		if msg := h.fails(name, args); !strings.Contains(msg, "INVALID_ARGUMENT") || !strings.Contains(msg, "leads through a symlink to") {
			t.Errorf("%s through a dangling link out of the root: %s", name, msg)
		}
	}
	if err := os.Symlink("later.sysml", filepath.Join(h.root, "soon.sysml")); err != nil {
		t.Fatal(err)
	}
	if msg := h.fails("parse", map[string]any{"paths": []string{"soon.sysml"}}); strings.Contains(msg, "outside") {
		t.Errorf("a dangling link staying inside the root is a file not yet written, not outside: %s", msg)
	}
}
