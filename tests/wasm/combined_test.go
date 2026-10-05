package wasm

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/combined"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
)

const combinedGzipBudget = 8300000

const combinedModel = `package Demo { part def Item; }`

func combinedSubtests(t *testing.T, target wasmTarget, r runner, bins map[string]string) {
	t.Run("answers a combined session over stdio", func(t *testing.T) {
		server, err := combined.New(versionStamp)
		if err != nil {
			t.Fatalf("combined.New: %v", err)
		}
		parseBody, err := server.Call(context.Background(), "ParseFile", []byte(fmt.Sprintf(
			`{"content":%s}`, mustJSON(t, combinedModel))))
		if err != nil {
			t.Fatalf("deriving the inline model hash: %v", err)
		}
		var parsed struct {
			ModelHash string `json:"modelHash"`
		}
		if err := json.Unmarshal(parseBody, &parsed); err != nil {
			t.Fatalf("decoding the inline model hash: %v\n%s", err, parseBody)
		}
		calls := []engineCall{
			{method: "ParseFile", params: fmt.Sprintf(`{"content":%s}`, mustJSON(t, combinedModel))},
			{method: "Evaluate", params: fmt.Sprintf(`{"modelHash":%q,"expression":"1 + 1"}`, parsed.ModelHash)},
			{method: "GetSymbol", params: fmt.Sprintf(`{"modelHash":%q,"symbolId":"Demo::Item"}`, parsed.ModelHash)},
			{method: "GetDiagnostics", params: fmt.Sprintf(`{"modelHash":%q}`, parsed.ModelHash)},
			{method: "Instantiate", params: fmt.Sprintf(`{"modelHash":%q,"symbolId":"Demo::Item"}`, parsed.ModelHash)},
			{method: "GetServerInfo", params: `{}`},
			{method: "Convert", params: `{}`},
			{method: "ParseFile", params: `"{"`},
			{method: "GetDiagnostics", params: fmt.Sprintf(`{"modelHash":%q}`, parsed.ModelHash)},
		}
		args := []string(nil)
		if target.goos == "js" {
			args = append(args, "-stdio")
		}
		got := r.runWithInput(t, bins["sysml-wasm"], engineSession(t, calls...), args...)
		if got.code != 0 {
			t.Fatalf("sysml-wasm stdio exited %d:\n%s", got.code, got.output)
		}
		bodies := frames(t, got.output)
		if len(bodies) != len(calls) {
			t.Fatalf("combined session answered %d frames, want %d:\n%s", len(bodies), len(calls), got.output)
		}
		for i, body := range bodies {
			var answer struct {
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Code    uint32 `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(body), &answer); err != nil {
				t.Fatalf("decoding combined frame %d: %v\n%s", i, err, body)
			}
			switch i {
			case 0, 1, 2, 3, 4, 5, 8:
				if answer.Error != nil {
					t.Fatalf("combined call %s failed: %+v", calls[i].method, answer.Error)
				}
			case 6:
				if answer.Error == nil || answer.Error.Code != jsonrpc.CodeUnimplemented {
					t.Errorf("unserved method error = %+v, want Unimplemented", answer.Error)
				}
			case 7:
				if answer.Error == nil || answer.Error.Code != jsonrpc.CodeInvalidArgument {
					t.Errorf("malformed params error = %+v, want InvalidArgument", answer.Error)
				}
			}
			if i == 1 && !strings.Contains(string(answer.Result), `"intValue":"2"`) {
				t.Errorf("Evaluate result = %s, want intValue 2", answer.Result)
			}
			if i == 2 && !strings.Contains(string(answer.Result), `"symbol"`) {
				t.Errorf("GetSymbol returned no symbol: %s", answer.Result)
			}
			if i == 4 && !strings.Contains(string(answer.Result), `"instance"`) {
				t.Errorf("Instantiate returned no instance: %s", answer.Result)
			}
			if i == 5 && !strings.Contains(string(answer.Result), `"capabilities"`) {
				t.Errorf("GetServerInfo returned no capabilities: %s", answer.Result)
			}
		}
	})

	if target.goos != "js" {
		return
	}

	t.Run("answers through globalThis.sysmlWasm", func(t *testing.T) {
		got := nodeRun(t, fixture(t, "combined.mjs"), wasmExecJS(t), bins["sysml-wasm"])
		answers := make(map[string]string)
		for _, line := range strings.Split(got.output, "\n") {
			method, rest, _ := strings.Cut(line, " ")
			answers[method] = rest
		}
		if answers["version"] != versionStamp {
			t.Errorf("sysmlWasm.version = %q, want %q", answers["version"], versionStamp)
		}
		for _, method := range []string{"ParseFile", "Evaluate", "GetSymbol", "GetDiagnostics", "afterMalformed"} {
			var answer struct {
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := json.Unmarshal([]byte(answers[method]), &answer); err != nil {
				t.Errorf("%s returned invalid JSON: %v\n%s", method, err, answers[method])
			} else if len(answer.Error) != 0 {
				t.Errorf("%s returned an error: %s", method, answer.Error)
			}
		}
		if !strings.Contains(answers["malformed"], `"code":3`) {
			t.Errorf("malformed params answered %s, want InvalidArgument (3)", answers["malformed"])
		}
	})

	t.Run("fits the combined size budget", func(t *testing.T) {
		data, err := os.ReadFile(bins["sysml-wasm"])
		if err != nil {
			t.Fatalf("reading %s: %v", bins["sysml-wasm"], err)
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
		if size := compressed.Len(); size > combinedGzipBudget {
			t.Errorf("gzipped sysml-wasm.wasm is %d bytes, over the %d-byte budget", size, combinedGzipBudget)
		} else {
			t.Logf("sysml-wasm.wasm is %d raw bytes and %d bytes gzipped -9", len(data), size)
		}
	})
}

func TestCombinedWireParity(t *testing.T) {
	ctx := context.Background()
	server, err := combined.New("combined-wire-test")
	if err != nil {
		t.Fatalf("combined.New: %v", err)
	}
	svc, err := sysmlgrpc.NewService(16, "combined-wire-test")
	if err != nil {
		t.Fatalf("grpc.NewService: %v", err)
	}

	call := func(method, params string) []byte {
		t.Helper()
		body, err := server.Call(ctx, method, []byte(params))
		if err != nil {
			t.Fatalf("combined %s refused: %v", method, err)
		}
		return body
	}
	equal := func(method, params string, grpcBody, combinedBody []byte) {
		t.Helper()
		var grpcJSON, combinedJSON any
		if err := json.Unmarshal(grpcBody, &grpcJSON); err != nil {
			t.Fatalf("decoding grpc %s: %v\n%s", method, err, grpcBody)
		}
		if err := json.Unmarshal(combinedBody, &combinedJSON); err != nil {
			t.Fatalf("decoding combined %s: %v\n%s", method, err, combinedBody)
		}
		if !reflect.DeepEqual(combinedJSON, grpcJSON) {
			t.Errorf("%s response mismatch for %s:\ncombined: %s\ngrpc: %s", method, params, combinedBody, grpcBody)
		}
	}

	parseParams := engineParseParams(t)
	combinedParse := call("ParseSources", parseParams)
	grpcParse, err := svc.ParseSources(ctx, mustUnmarshal[pb.ParseSourcesRequest](t, parseParams))
	if err != nil {
		t.Fatalf("grpc ParseSources: %v", err)
	}
	grpcParseBody := mustMarshal(t, grpcParse)
	equal("ParseSources", parseParams, grpcParseBody, combinedParse)
	var parsed struct {
		ModelHash string `json:"modelHash"`
	}
	if err := json.Unmarshal(combinedParse, &parsed); err != nil {
		t.Fatalf("decoding ParseSources response: %v\n%s", err, combinedParse)
	}
	hash := parsed.ModelHash

	diagnosticsParams := fmt.Sprintf(`{"modelHash":%q}`, hash)
	grpcDiagnostics, err := svc.GetDiagnostics(ctx, mustUnmarshal[pb.DiagnosticsRequest](t, diagnosticsParams))
	if err != nil {
		t.Fatalf("grpc GetDiagnostics: %v", err)
	}
	equal("GetDiagnostics", diagnosticsParams, mustMarshal(t, grpcDiagnostics), call("GetDiagnostics", diagnosticsParams))

	for _, expression := range []string{"7", "1.5", "true", "enginedemo::speed", "2 ** 70"} {
		params := fmt.Sprintf(`{"modelHash":%q,"expression":%s}`, hash, mustJSON(t, expression))
		response, err := svc.Evaluate(ctx, mustUnmarshal[pb.EvaluateRequest](t, params))
		if err != nil {
			t.Fatalf("grpc Evaluate %q: %v", expression, err)
		}
		equal("Evaluate", params, mustMarshal(t, response), call("Evaluate", params))
	}

	for _, test := range []struct {
		method string
		params string
		call   func() ([]byte, error)
	}{
		{
			method: "GetSymbol",
			params: fmt.Sprintf(`{"modelHash":%q,"symbolId":"enginedemo::Double"}`, hash),
			call: func() ([]byte, error) {
				response, err := svc.GetSymbol(ctx, mustUnmarshal[pb.GetSymbolRequest](t,
					fmt.Sprintf(`{"modelHash":%q,"symbolId":"enginedemo::Double"}`, hash)))
				if err != nil {
					return nil, err
				}
				return mustMarshal(t, response), nil
			},
		},
		{
			method: "Instantiate",
			params: fmt.Sprintf(`{"modelHash":%q,"symbolId":"enginedemo::Switch"}`, hash),
			call: func() ([]byte, error) {
				response, err := svc.Instantiate(ctx, mustUnmarshal[pb.InstantiateRequest](t,
					fmt.Sprintf(`{"modelHash":%q,"symbolId":"enginedemo::Switch"}`, hash)))
				if err != nil {
					return nil, err
				}
				return mustMarshal(t, response), nil
			},
		},
		{
			method: "ExecuteAction",
			params: fmt.Sprintf(`{"modelHash":%q,"actionSymbolId":"enginedemo::Double","inputs":{"x":{"intValue":"6"}}}`, hash),
			call: func() ([]byte, error) {
				request := fmt.Sprintf(`{"modelHash":%q,"actionSymbolId":"enginedemo::Double","inputs":{"x":{"intValue":"6"}}}`, hash)
				response, err := svc.ExecuteAction(ctx, mustUnmarshal[pb.ExecuteActionRequest](t, request))
				if err != nil {
					return nil, err
				}
				return mustMarshal(t, response), nil
			},
		},
		{
			method: "ExecuteState",
			params: fmt.Sprintf(`{"modelHash":%q,"stateMachineSymbolId":"enginedemo::Switch"}`, hash),
			call: func() ([]byte, error) {
				request := fmt.Sprintf(`{"modelHash":%q,"stateMachineSymbolId":"enginedemo::Switch"}`, hash)
				response, err := svc.ExecuteState(ctx, mustUnmarshal[pb.ExecuteStateRequest](t, request))
				if err != nil {
					return nil, err
				}
				return mustMarshal(t, response), nil
			},
		},
	} {
		grpcBody, err := test.call()
		if err != nil {
			t.Fatalf("grpc %s: %v", test.method, err)
		}
		equal(test.method, test.params, grpcBody, call(test.method, test.params))
	}

	inlineParams := fmt.Sprintf(`{"content":%s}`, mustJSON(t, readFile(t, "engine.sysml")))
	grpcFile, err := svc.ParseFile(ctx, mustUnmarshal[pb.ParseFileRequest](t, inlineParams))
	if err != nil {
		t.Fatalf("grpc ParseFile inline: %v", err)
	}
	inlineBody := call("ParseFile", inlineParams)
	equal("ParseFile inline", inlineParams, mustMarshal(t, grpcFile), inlineBody)
	var inlineParsed struct {
		ModelHash string `json:"modelHash"`
	}
	if err := json.Unmarshal(inlineBody, &inlineParsed); err != nil {
		t.Fatalf("decoding inline ParseFile: %v\n%s", err, inlineBody)
	}
	for method, params := range map[string]string{
		"Evaluate":  fmt.Sprintf(`{"modelHash":%q,"expression":"enginedemo::speed"}`, inlineParsed.ModelHash),
		"GetSymbol": fmt.Sprintf(`{"modelHash":%q,"symbolId":"enginedemo::Double"}`, inlineParsed.ModelHash),
	} {
		switch method {
		case "Evaluate":
			response, err := svc.Evaluate(ctx, mustUnmarshal[pb.EvaluateRequest](t, params))
			if err != nil {
				t.Fatalf("grpc Evaluate using ParseFile hash: %v", err)
			}
			equal(method+" after ParseFile", params, mustMarshal(t, response), call(method, params))
		case "GetSymbol":
			response, err := svc.GetSymbol(ctx, mustUnmarshal[pb.GetSymbolRequest](t, params))
			if err != nil {
				t.Fatalf("grpc GetSymbol using ParseFile hash: %v", err)
			}
			equal(method+" after ParseFile", params, mustMarshal(t, response), call(method, params))
		}
	}

	path, err := filepath.Abs(filepath.Join("testdata", "engine.sysml"))
	if err != nil {
		t.Fatalf("resolving engine fixture: %v", err)
	}
	fileParams := fmt.Sprintf(`{"filePath":%q}`, path)
	grpcPathFile, err := svc.ParseFile(ctx, mustUnmarshal[pb.ParseFileRequest](t, fileParams))
	if err != nil {
		t.Fatalf("grpc ParseFile filePath: %v", err)
	}
	equal("ParseFile filePath", fileParams, mustMarshal(t, grpcPathFile), call("ParseFile", fileParams))

	infoBody := call("GetServerInfo", `{}`)
	var info struct {
		Version      string   `json:"version"`
		Capabilities []string `json:"capabilities"`
	}
	if err := json.Unmarshal(infoBody, &info); err != nil {
		t.Fatalf("decoding combined GetServerInfo: %v\n%s", err, infoBody)
	}
	expectedCapabilities := []string{
		"type_facts", "enum_values", "evaluate_subject", "symbol_attributes",
		"unset_value", "feature_values", "inline_language", "strict_conformance",
		"parse_sources", "complex_values", "structured_values", "measurement_refs",
		"function_values", "set_values", "tensor_values", "infinity_value",
		"diagnostic_codes", "schedule", "final_time", "metaobject_values",
		"undetermined_value", "performer", "big_int_values",
	}
	if info.Version != "combined-wire-test" {
		t.Errorf("GetServerInfo version = %q", info.Version)
	}
	if !reflect.DeepEqual(info.Capabilities, expectedCapabilities) {
		t.Errorf("GetServerInfo capabilities = %v, want %v", info.Capabilities, expectedCapabilities)
	}
	reported := make(map[string]bool)
	for _, capability := range sysmlgrpc.Capabilities() {
		reported[capability] = true
	}
	for _, capability := range expectedCapabilities {
		if !reported[capability] {
			t.Errorf("capability %q is not reported by sysml-grpc", capability)
		}
	}
}

func TestCombinedDependencies(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolving repository root: %v", err)
	}
	const module = "github.com/Open-MBEE/OpenSysML"
	required := []string{
		module + "/internal/frontend/engine",
		module + "/internal/frontend/core",
	}
	for _, target := range []wasmTarget{{name: "js", goos: "js"}, {name: "wasip1", goos: "wasip1"}} {
		t.Run(target.name, func(t *testing.T) {
			cmd := exec.Command("go", "list", "-deps", "./cmd/sysml-wasm")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GOOS="+target.goos, "GOARCH=wasm")
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go list -deps for %s: %v\n%s", target.name, err, output)
			}
			deps := make(map[string]bool)
			for _, dependency := range strings.Fields(string(output)) {
				deps[dependency] = true
			}
			for _, dependency := range required {
				if !deps[dependency] {
					t.Errorf("dependency list does not contain required package %s", dependency)
				}
			}
			for dependency := range deps {
				if forbiddenCombinedDependency(module, dependency) {
					t.Errorf("forbidden dependency in %s build: %s", target.name, dependency)
				}
			}
		})
	}
}

func forbiddenCombinedDependency(module, dependency string) bool {
	for _, prefix := range []string{
		module + "/api/",
		"google.golang.org/protobuf",
		"google.golang.org/grpc",
		"connectrpc.com/",
		module + "/internal/frontend/grpc",
		module + "/internal/frontend/protoconv",
		module + "/internal/frontend/stdiorpc",
		module + "/internal/doc/",
	} {
		if strings.HasPrefix(dependency, prefix) {
			return true
		}
	}
	return false
}
