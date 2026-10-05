package wasm

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// engineGzipBudget bounds growth of the gzipped js build of sysml-engine;
// TestEngineDependencies, not this budget, catches a forbidden dependency.
const engineGzipBudget = 7700000

func TestEngineDependencies(t *testing.T) {
	const module = "github.com/Open-MBEE/OpenSysML"
	required := []string{
		module + "/internal/frontend/engine",
		module + "/internal/frontend/jsonrpc",
		module + "/internal/exec/runtime",
		module + "/internal/check/passes",
		module + "/internal/workspace/libs",
	}
	checkWasmDependencies(t, "./cmd/sysml-engine", required, func(dependency string) bool {
		return forbiddenEngineDependency(module, dependency)
	})
}

func forbiddenEngineDependency(module, dependency string) bool {
	// The engine serves JSON, not protobuf wire formats.
	for _, prefix := range []string{
		module + "/api",
		"google.golang.org/protobuf",
		"google.golang.org/grpc",
		"connectrpc.com",
	} {
		if hasDependencyPrefix(dependency, prefix) {
			return true
		}
	}
	// The engine does not host these transports or servers.
	for _, prefix := range []string{
		module + "/internal/frontend/grpc",
		module + "/internal/frontend/protoconv",
		module + "/internal/frontend/stdiorpc",
		module + "/internal/frontend/combined",
		module + "/internal/frontend/lsp",
		module + "/internal/frontend/repl",
	} {
		if hasDependencyPrefix(dependency, prefix) {
			return true
		}
	}
	// The engine does not include analysis frameworks or engines.
	for _, prefix := range []string{
		module + "/internal/exec/analysis",
		module + "/internal/exec/engines",
		module + "/internal/exec/smt",
		module + "/internal/exec/solve",
		module + "/internal/exec/fmi",
	} {
		if hasDependencyPrefix(dependency, prefix) {
			return true
		}
	}
	// ExecuteState's trace events come from queryexec; the document IR and backends stay out.
	if hasDependencyPrefix(dependency, module+"/internal/doc") && dependency != module+"/internal/doc/queryexec" {
		return true
	}
	// The workspace pipeline belongs to the LSP and REPL.
	for _, prefix := range []string{
		module + "/internal/workspace/model",
		module + "/internal/workspace/modeldoc",
		module + "/internal/workspace/modelrt",
	} {
		if hasDependencyPrefix(dependency, prefix) {
			return true
		}
	}
	// RDF is the only converter shared with the engine.
	translate := module + "/internal/translate"
	return hasDependencyPrefix(dependency, translate) && dependency != translate+"/rdf"
}

func hasDependencyPrefix(dependency, prefix string) bool {
	prefix = strings.TrimSuffix(prefix, "/")
	return dependency == prefix || strings.HasPrefix(dependency, prefix+"/")
}

// engineSubtests runs the sysml-engine half of the run gate on bins, linked
// into bins["sysml-engine"] with the other commands so it is not built twice:
// a stdio session doing real work on both targets, plus the globalThis host
// surface and the size budget that surface exists to keep, on js.
func engineSubtests(t *testing.T, target wasmTarget, r runner, bins map[string]string) {
	t.Run("answers an engine session over stdio", func(t *testing.T) {
		session := engineSession(t, engineCalls(t)...)
		var args []string
		if target.goos == "js" {
			// The js build installs globalThis.sysmlEngine unless asked for the pipe.
			args = append(args, "-stdio")
		}
		got := r.runWithInput(t, bins["sysml-engine"], session, args...)
		if got.code != 0 {
			t.Fatalf("sysml-engine -stdio exited %d:\n%s", got.code, got.output)
		}
		for _, want := range []string{`"modelHash"`, `"realValue":6`, `"intValue":"12"`, `"on"`} {
			if !strings.Contains(got.output, want) {
				t.Errorf("the session output is missing %s:\n%s", want, got.output)
			}
		}
	})

	if target.goos != "js" {
		return
	}

	t.Run("answers through globalThis.sysmlEngine", func(t *testing.T) {
		// The script runs under plain node, not through the wasm target runner:
		// it loads wasm_exec.js and the module itself.
		got := nodeRun(t, fixture(t, "engine.mjs"),
			wasmExecJS(t), bins["sysml-engine"], fixture(t, "model.sysml"))
		for _, want := range []string{"modelHash ", `"realValue":6`} {
			if !strings.Contains(got.output, want) {
				t.Errorf("engine.mjs output is missing %s:\n%s", want, got.output)
			}
		}
	})

	t.Run("fits the engine size budget", func(t *testing.T) {
		data, err := os.ReadFile(bins["sysml-engine"])
		if err != nil {
			t.Fatalf("reading %s: %v", bins["sysml-engine"], err)
		}
		var compressed bytes.Buffer
		w, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
		if err != nil {
			t.Fatalf("gzip writer: %v", err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("compressing: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("compressing: %v", err)
		}
		if size := compressed.Len(); size > engineGzipBudget {
			t.Errorf("gzipped sysml-engine.wasm is %d bytes, over the %d-byte budget: "+
				"the engine grew past its size bound (forbidden dependencies are caught by TestEngineDependencies)",
				size, engineGzipBudget)
		} else {
			t.Logf("gzipped sysml-engine.wasm is %d bytes, under the %d-byte budget", size, engineGzipBudget)
		}
	})
}

// engineCalls are the calls a stdio session makes over the two fixtures:
// parse both models, evaluate an attribute, run the action with an input, run
// the machine.
func engineCalls(t *testing.T) []engineCall {
	hash := engineModelHash(t)
	return []engineCall{
		{method: "ParseSources", params: engineParseParams(t)},
		{method: "Evaluate", params: fmt.Sprintf(`{"modelHash":%q,"expression":"gatedemo::total"}`, hash)},
		{method: "ExecuteAction", params: fmt.Sprintf(`{"modelHash":%q,"actionSymbolId":"enginedemo::Double","inputs":{"x":{"intValue":"6"}}}`, hash)},
		{method: "ExecuteState", params: fmt.Sprintf(`{"modelHash":%q,"stateMachineSymbolId":"enginedemo::Switch"}`, hash)},
	}
}

// engineModelHash is the hash a model parsed from engineParseParams carries: a
// request cannot know the hash ahead of the parse, but the hash is
// deterministic, so the test derives it from a throwaway engine.
func engineModelHash(t *testing.T) string {
	t.Helper()
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	resp, err := eng.Call(context.Background(), "ParseSources", []byte(engineParseParams(t)))
	if err != nil {
		t.Fatalf("deriving the model hash: %v", err)
	}
	var parsed struct {
		ModelHash string `json:"modelHash"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		t.Fatalf("decoding the engine's ParseSources response: %v\n%s", err, resp)
	}
	return parsed.ModelHash
}

// engineParseParams is the ParseSources request for both fixtures as inline
// content, so a WebAssembly build needs no filesystem to answer it.
func engineParseParams(t *testing.T) string {
	t.Helper()
	docs := []map[string]string{}
	for _, name := range []string{"model.sysml", "engine.sysml"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("reading the fixture %s: %v", name, err)
		}
		docs = append(docs, map[string]string{"name": name, "content": string(data)})
	}
	body, err := json.Marshal(map[string]any{"documents": docs})
	if err != nil {
		t.Fatalf("rendering the ParseSources request: %v", err)
	}
	return string(body)
}

// engineCall is one call of the session: its method and its params body.
type engineCall struct {
	method string
	params string
}

// engineSession renders the calls as one JSON-RPC stdio session's input, each
// request a frame.
func engineSession(t *testing.T, calls ...engineCall) string {
	t.Helper()
	var session strings.Builder
	for i, call := range calls {
		req, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": i + 1, "method": call.method,
			"params": json.RawMessage(call.params),
		})
		if err != nil {
			t.Fatalf("rendering the request: %v", err)
		}
		fmt.Fprintf(&session, "Content-Length: %d\r\n\r\n%s", len(req), req)
	}
	return session.String()
}

// wasmExecJS is the toolchain's wasm_exec.js, which the Makefile copies beside
// the build for a page to load; the test loads it the same way.
func wasmExecJS(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatalf("go env GOROOT: %v", err)
	}
	path := filepath.Join(strings.TrimSpace(string(out)), "lib", "wasm", "wasm_exec.js")
	if _, err := os.Stat(path); err != nil {
		skipOrFail(t, path+" is not readable", "the Go toolchain ships wasm_exec.js at $GOROOT/lib/wasm")
	}
	return path
}

// nodeRun runs a .mjs script under node directly — the host surface case, which
// is not a wasm binary and so cannot go through the target runner.
func nodeRun(t *testing.T, script string, args ...string) result {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		skipOrFail(t, "node is not on PATH", "install Node 24 or later")
		return result{}
	}
	// --stack-size is what go_js_wasm_exec passes: model traversal recurses deeper
	// than Node's default stack.
	full := append([]string{"--no-warnings", "--stack-size=8192", script}, args...)
	cmd := exec.Command(node, full...)
	cmd.Env = childEnv(t)
	out, err := cmd.CombinedOutput()
	res := result{output: string(out)}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			res.code = exit.ExitCode()
		} else {
			t.Fatalf("running %s: %v\n%s", script, err, res.output)
		}
	}
	if res.code != 0 {
		t.Fatalf("%s exited %d:\n%s", script, res.code, res.output)
	}
	return res
}

// TestEngineWireParity proves the engine answers what sysml-grpc answers: the
// same requests put to a default-capabilities grpc.NewService and marshalled by
// protojson, and to engine.Call, decode to the same JSON. ParseSources compares
// modelHash alone: the engine deliberately reports the syntax diagnostics only
// and no roots, where the service reports the analyzed diagnostics and the root
// symbols.
func TestEngineWireParity(t *testing.T) {
	ctx := context.Background()
	svc, err := sysmlgrpc.NewService(4, "parity")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}

	parseParams := engineParseParams(t)
	grpcResp, err := svc.ParseSources(ctx, mustUnmarshal[pb.ParseSourcesRequest](t, parseParams))
	if err != nil {
		t.Fatalf("grpc ParseSources: %v", err)
	}
	var engineResp struct {
		ModelHash string `json:"modelHash"`
	}
	engineBody := mustCall(t, eng, "ParseSources", parseParams)
	if err := json.Unmarshal(engineBody, &engineResp); err != nil {
		t.Fatalf("decoding the engine's ParseSources response: %v\n%s", err, engineBody)
	}
	if grpcResp.GetModelHash() != engineResp.ModelHash {
		t.Fatalf("modelHash: grpc %s, engine %s", grpcResp.GetModelHash(), engineResp.ModelHash)
	}
	hash := grpcResp.GetModelHash()

	// The equal cases: everything else the engine serves is byte-compatible in
	// meaning, so both answers decode to the same JSON value.
	equal := func(method string, params string, grpcOut []byte) {
		t.Helper()
		engineOut := mustCall(t, eng, method, params)
		var grpcJSON, engineJSON any
		if err := json.Unmarshal(grpcOut, &grpcJSON); err != nil {
			t.Fatalf("%s: the grpc answer is not JSON: %v\n%s", method, err, grpcOut)
		}
		if err := json.Unmarshal(engineOut, &engineJSON); err != nil {
			t.Fatalf("%s: the engine answer is not JSON: %v\n%s", method, err, engineOut)
		}
		if !reflect.DeepEqual(grpcJSON, engineJSON) {
			t.Errorf("%s diverges:\ngrpc:   %s\nengine: %s", method, grpcOut, engineOut)
		}
	}

	for _, expression := range []string{
		"7", "1.5", "true", `"enginedemo"`, "(1, 2, 3)",
		"10.0 [SI::m] / 2.0 [SI::s]", "enginedemo::Color::red",
		"gatedemo::total", "enginedemo::speed",
		"9223372036854775807 + 1", "-9223372036854775808 - 1", "2 ** 70",
		"2 ** 70 - 2 ** 70 + 1", "2 ** 70 [SI::m]",
	} {
		params := fmt.Sprintf(`{"modelHash":%q,"expression":%s}`,
			hash, mustJSON(t, expression))
		res, err := svc.Evaluate(ctx, mustUnmarshal[pb.EvaluateRequest](t, params))
		if err != nil {
			t.Fatalf("grpc Evaluate %q: %v", expression, err)
		}
		equal("Evaluate", params, mustMarshal(t, res))
		if expression == "(1, 2, 3)" {
			// Equal answers would also match on a shared parse error: the case
			// must exercise a real sequence, so check what the engine returned.
			var seq struct {
				Result struct {
					Sequence struct {
						Elements []json.RawMessage `json:"elements"`
					} `json:"sequence"`
				} `json:"result"`
			}
			if err := json.Unmarshal(mustCall(t, eng, "Evaluate", params), &seq); err != nil {
				t.Fatalf("decoding the sequence answer: %v", err)
			}
			if len(seq.Result.Sequence.Elements) != 3 {
				t.Errorf("(1, 2, 3) answered %d sequence elements, want 3", len(seq.Result.Sequence.Elements))
			}
		}
	}

	instParams := fmt.Sprintf(`{"modelHash":%q,"symbolId":"enginedemo::Switch"}`, hash)
	instRes, err := svc.Instantiate(ctx, mustUnmarshal[pb.InstantiateRequest](t, instParams))
	if err != nil {
		t.Fatalf("grpc Instantiate: %v", err)
	}
	equal("Instantiate", instParams, mustMarshal(t, instRes))

	actionParams := fmt.Sprintf(`{"modelHash":%q,"actionSymbolId":"enginedemo::Double","inputs":{"x":{"intValue":"6"}}}`, hash)
	actionRes, err := svc.ExecuteAction(ctx, mustUnmarshal[pb.ExecuteActionRequest](t, actionParams))
	if err != nil {
		t.Fatalf("grpc ExecuteAction: %v", err)
	}
	equal("ExecuteAction", actionParams, mustMarshal(t, actionRes))

	// An Integer beyond int64 crosses as bigIntValue both ways, as it does
	// through the service.
	wideParams := fmt.Sprintf(`{"modelHash":%q,"actionSymbolId":"enginedemo::Double","inputs":{"x":{"bigIntValue":"590295810358705651712"}}}`, hash)
	wideRes, err := svc.ExecuteAction(ctx, mustUnmarshal[pb.ExecuteActionRequest](t, wideParams))
	if err != nil {
		t.Fatalf("grpc ExecuteAction with a wide input: %v", err)
	}
	wideOut := mustMarshal(t, wideRes)
	if !strings.Contains(string(wideOut), `"bigIntValue":"1180591620717411303424"`) {
		t.Errorf("the service doubled 2**69 to %s, want bigIntValue 1180591620717411303424", wideOut)
	}
	equal("ExecuteAction", wideParams, wideOut)

	stateParams := fmt.Sprintf(`{"modelHash":%q,"stateMachineSymbolId":"enginedemo::Switch"}`, hash)
	stateRes, err := svc.ExecuteState(ctx, mustUnmarshal[pb.ExecuteStateRequest](t, stateParams))
	if err != nil {
		t.Fatalf("grpc ExecuteState: %v", err)
	}
	equal("ExecuteState", stateParams, mustMarshal(t, stateRes))

	// Malformed requests answer InvalidArgument rather than panicking the
	// session: protojson rejects null elements and a oneof written twice.
	for name, malformed := range map[string]struct{ method, params string }{
		"a null document":           {"ParseSources", `{"documents":[null]}`},
		"a dual-arm value":          {"ExecuteAction", fmt.Sprintf(`{"modelHash":%q,"actionSymbolId":"enginedemo::Double","inputs":{"x":{"intValue":"6","realValue":7}}}`, hash)},
		"a dual-magnitude quantity": {"ExecuteAction", fmt.Sprintf(`{"modelHash":%q,"actionSymbolId":"enginedemo::Double","inputs":{"x":{"quantity":{"intMagnitude":"1","realMagnitude":2.0}}}}`, hash)},
		"a dual-source document":    {"ParseSources", `{"documents":[{"name":"a.sysml","filePath":"a.sysml","content":"package a {}"}]}`},
		"a dual-width Integer":      {"ExecuteAction", fmt.Sprintf(`{"modelHash":%q,"actionSymbolId":"enginedemo::Double","inputs":{"x":{"intValue":"6","bigIntValue":"9223372036854775808"}}}`, hash)},
	} {
		_, err := eng.Call(ctx, malformed.method, []byte(malformed.params))
		var callErr *jsonrpc.Error
		if !errors.As(err, &callErr) || callErr.Code != 3 {
			t.Errorf("%s: Call error = %v, want InvalidArgument (3)", name, err)
		}
	}
	// A null vector component is refused where the input is read — protojson
	// itself rejects the request before grpc's ExecuteAction ever sees it, so
	// there is no service answer to compare; the engine must simply not panic.
	vectorParams := fmt.Sprintf(`{"modelHash":%q,"actionSymbolId":"enginedemo::Double","inputs":{"x":{"vector":{"components":[null]}}}}`, hash)
	var vectorAnswer struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(mustCall(t, eng, "ExecuteAction", vectorParams), &vectorAnswer); err != nil {
		t.Fatalf("decoding the null-component answer: %v", err)
	}
	if vectorAnswer.Error == "" {
		t.Errorf("a null vector component answered no error")
	}

	// The engine still answers a normal request after the malformed ones.
	mustCall(t, eng, "Evaluate", fmt.Sprintf(`{"modelHash":%q,"expression":"gatedemo::total"}`, hash))

	// Documentation parity: the body of an element's doc comment, read
	// reflectively, is the same string on both services.
	docDoc := `package docdemo { part def Wheel { doc /* Turns. */ } }`
	docJSON, err := json.Marshal(docDoc)
	if err != nil {
		t.Fatalf("encoding the doc fixture: %v", err)
	}
	docParams := fmt.Sprintf(`{"documents":[{"name":"docdemo.sysml","content":%s}]}`, docJSON)
	docGrpc, err := svc.ParseSources(ctx, mustUnmarshal[pb.ParseSourcesRequest](t, docParams))
	if err != nil {
		t.Fatalf("grpc ParseSources of the doc fixture: %v", err)
	}
	var docEngine struct {
		ModelHash string `json:"modelHash"`
	}
	if err := json.Unmarshal(mustCall(t, eng, "ParseSources", docParams), &docEngine); err != nil {
		t.Fatalf("decoding the engine's doc ParseSources: %v", err)
	}
	if docGrpc.GetModelHash() != docEngine.ModelHash {
		t.Fatalf("doc modelHash: grpc %s, engine %s", docGrpc.GetModelHash(), docEngine.ModelHash)
	}
	bodyParams := fmt.Sprintf(`{"modelHash":%q,"expression":"(docdemo::Wheel meta KerML::Element).documentation.body"}`,
		docGrpc.GetModelHash())
	bodyRes, err := svc.Evaluate(ctx, mustUnmarshal[pb.EvaluateRequest](t, bodyParams))
	if err != nil {
		t.Fatalf("grpc Evaluate of the documentation body: %v", err)
	}
	equal("Evaluate", bodyParams, mustMarshal(t, bodyRes))
	var bodyAnswer struct {
		Result struct {
			Sequence struct {
				Elements []struct {
					StringValue string `json:"stringValue"`
				} `json:"elements"`
			} `json:"sequence"`
		} `json:"result"`
	}
	if err := json.Unmarshal(mustCall(t, eng, "Evaluate", bodyParams), &bodyAnswer); err != nil {
		t.Fatalf("decoding the documentation body answer: %v", err)
	}
	bodies := bodyAnswer.Result.Sequence.Elements
	if len(bodies) != 1 || !strings.Contains(bodies[0].StringValue, "Turns.") {
		t.Errorf("documentation.body answered %v, want one string carrying %q",
			bodies, "Turns.")
	}
}

// mustCall runs one engine call, failing on a refused one.
func mustCall(t *testing.T, eng *engine.Engine, method, params string) []byte {
	t.Helper()
	body, err := eng.Call(context.Background(), method, []byte(params))
	if err != nil {
		t.Fatalf("%s refused: %v", method, err)
	}
	return body
}

// mustUnmarshal decodes params into a new request message of type T as
// protojson would read it: a test states requests as the JSON a client sends.
func mustUnmarshal[T any, PT interface {
	*T
	proto.Message
}](t *testing.T, params string) PT {
	t.Helper()
	msg := PT(new(T))
	if err := protojson.Unmarshal([]byte(params), msg); err != nil {
		t.Fatalf("decoding the test request: %v\n%s", err, params)
	}
	return msg
}

// mustMarshal is protojson.Marshal with a test failure on error.
func mustMarshal(t *testing.T, msg proto.Message) []byte {
	t.Helper()
	body, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatalf("protojson.Marshal: %v", err)
	}
	return body
}

// mustJSON encodes one string as a JSON string literal.
func mustJSON(t *testing.T, s string) string {
	t.Helper()
	body, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("encoding %q: %v", s, err)
	}
	return string(body)
}
