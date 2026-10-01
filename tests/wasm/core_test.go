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

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/core"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
	"google.golang.org/protobuf/proto"
)

const coreGzipBudget = 6300000

const cleanCoreModel = `
package Demo {
    private import ScalarValues::*;
    private import SI::*;

    part def VehicleBase;
    part def 'Vehicle Type' :> VehicleBase {
        attribute integerValue : Integer = 4;
        attribute realValue : Real = 2.5;
        attribute stringValue : String = "car";
        attribute booleanValue : Boolean = true;
        attribute distance = 1500.0 [kg];
        part engine : Engine[0..*];
    }
    part def Engine;
}
`

// coreCalls are the validation RPCs a stdio session makes against one model.
func coreCalls(t *testing.T) []engineCall {
	t.Helper()
	params := coreValidationParams(t)
	hash := coreModelHash(t, params)
	return []engineCall{
		{method: "ParseSources", params: params},
		{method: "GetDiagnostics", params: fmt.Sprintf(`{"modelHash":%q}`, hash)},
		{method: "GetSymbol", params: fmt.Sprintf(`{"modelHash":%q,"symbolId":"CoreValidation::two"}`, hash)},
		{method: "ParseSources", params: `[]`},
		{method: "GetDiagnostics", params: fmt.Sprintf(`{"modelHash":%q}`, hash)},
		{method: "Evaluate", params: `{}`},
	}
}

func coreValidationParams(t *testing.T) string {
	t.Helper()
	content := readFile(t, "core-validation.sysml")
	return parseSourcesParams(t, "core-validation.sysml", content)
}

func coreModelHash(t *testing.T, params string) string {
	t.Helper()
	c, err := core.New()
	if err != nil {
		t.Fatalf("core.New: %v", err)
	}
	body, err := c.Call(context.Background(), "ParseSources", []byte(params))
	if err != nil {
		t.Fatalf("deriving the core model hash: %v", err)
	}
	var parsed struct {
		ModelHash string `json:"modelHash"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("decoding the core ParseSources response: %v\n%s", err, body)
	}
	return parsed.ModelHash
}

func coreSubtests(t *testing.T, target wasmTarget, r runner, bins map[string]string) {
	t.Run("answers a core session over stdio", func(t *testing.T) {
		calls := coreCalls(t)
		session := engineSession(t, calls...)
		var args []string
		if target.goos == "js" {
			args = append(args, "-stdio")
		}
		got := r.runWithInput(t, bins["sysml-core"], session, args...)
		if got.code != 0 {
			t.Fatalf("sysml-core -stdio exited %d:\n%s", got.code, got.output)
		}
		bodies := frames(t, got.output)
		if len(bodies) != len(calls) {
			t.Fatalf("the core session answered %d frames, want %d:\n%s", len(bodies), len(calls), got.output)
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
				t.Fatalf("decoding core frame %d: %v\n%s", i, err, body)
			}
			switch i {
			case 0, 1, 2, 4:
				if answer.Error != nil {
					t.Fatalf("core call %s failed: %+v", calls[i].method, answer.Error)
				}
			case 3:
				if answer.Error == nil || answer.Error.Code != jsonrpc.CodeInvalidArgument {
					t.Errorf("malformed request error = %+v, want InvalidArgument (3)", answer.Error)
				}
			case 5:
				if answer.Error == nil || answer.Error.Code != jsonrpc.CodeUnimplemented {
					t.Errorf("unknown method error = %+v, want Unimplemented (12)", answer.Error)
				}
			}
			if i == 0 && !strings.Contains(string(answer.Result), `"one-type"`) {
				t.Errorf("ParseSources did not report the validation diagnostic:\n%s", body)
			}
			if i == 1 && !strings.Contains(string(answer.Result), `"one-type"`) {
				t.Errorf("GetDiagnostics did not report the validation diagnostic:\n%s", body)
			}
			if i == 2 && !strings.Contains(string(answer.Result), `"symbol"`) {
				t.Errorf("GetSymbol returned no symbol:\n%s", body)
			}
		}
	})

	if target.goos != "js" {
		return
	}

	t.Run("answers through globalThis.sysmlCore", func(t *testing.T) {
		got := nodeRun(t, fixture(t, "core.mjs"),
			wasmExecJS(t), bins["sysml-core"], fixture(t, "core-validation.sysml"))
		answers := make(map[string]string)
		for _, line := range strings.Split(got.output, "\n") {
			method, rest, _ := strings.Cut(line, " ")
			answers[method] = rest
		}
		if answers["version"] != versionStamp {
			t.Errorf("sysmlCore.version = %q, want %q", answers["version"], versionStamp)
		}
		for _, method := range []string{"ParseSources", "ParseFile", "GetDiagnostics", "GetSymbol"} {
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
		if !strings.Contains(answers["afterMalformed"], `"result"`) {
			t.Errorf("the call after malformed params answered %s, want success", answers["afterMalformed"])
		}
		if !strings.Contains(answers["Evaluate"], `"code":12`) {
			t.Errorf("an unserved method answered %s, want code 12", answers["Evaluate"])
		}
	})

	t.Run("fits the core size budget", func(t *testing.T) {
		data, err := os.ReadFile(bins["sysml-core"])
		if err != nil {
			t.Fatalf("reading %s: %v", bins["sysml-core"], err)
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
		if size := compressed.Len(); size > coreGzipBudget {
			t.Errorf("gzipped sysml-core.wasm is %d bytes, over the %d-byte budget", size, coreGzipBudget)
		} else {
			t.Logf("gzipped sysml-core.wasm is %d bytes, under the %d-byte budget", size, coreGzipBudget)
		}
	})
}

func TestCoreWireParity(t *testing.T) {
	ctx := context.Background()
	c, err := core.New()
	if err != nil {
		t.Fatalf("core.New: %v", err)
	}
	svc, err := sysmlgrpc.NewService(16, "core-wire-test")
	if err != nil {
		t.Fatalf("grpc.NewService: %v", err)
	}

	callCore := func(method, params string) []byte {
		t.Helper()
		body, err := c.Call(ctx, method, []byte(params))
		if err != nil {
			t.Fatalf("core %s refused: %v", method, err)
		}
		return body
	}
	equal := func(method, params string, grpcResponse proto.Message, coreBody []byte) {
		t.Helper()
		want := mustMarshal(t, grpcResponse)
		var gotJSON, wantJSON any
		if err := json.Unmarshal(coreBody, &gotJSON); err != nil {
			t.Fatalf("decoding core %s response: %v\n%s", method, err, coreBody)
		}
		if err := json.Unmarshal(want, &wantJSON); err != nil {
			t.Fatalf("decoding grpc %s response: %v\n%s", method, err, want)
		}
		if !reflect.DeepEqual(gotJSON, wantJSON) {
			t.Errorf("%s response mismatch for %s:\ncore: %s\ngrpc: %s", method, params, coreBody, want)
		}
	}
	parseSources := func(name, content string) (string, []byte) {
		t.Helper()
		params := parseSourcesParams(t, name, content)
		body := callCore("ParseSources", params)
		grpcResponse, err := svc.ParseSources(ctx, mustUnmarshal[pb.ParseSourcesRequest](t, params))
		if err != nil {
			t.Fatalf("grpc ParseSources: %v", err)
		}
		equal("ParseSources", params, grpcResponse, body)
		var parsed struct {
			ModelHash string `json:"modelHash"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("decoding core ParseSources response: %v\n%s", err, body)
		}
		return parsed.ModelHash, body
	}

	hash, cleanBody := parseSources("core-clean.sysml", cleanCoreModel)
	var clean struct {
		Roots []json.RawMessage `json:"roots"`
	}
	if err := json.Unmarshal(cleanBody, &clean); err != nil {
		t.Fatalf("decoding clean ParseSources response: %v\n%s", err, cleanBody)
	}
	if len(clean.Roots) != 1 {
		t.Fatalf("clean ParseSources returned %d roots, want one", len(clean.Roots))
	}
	diagnosticsParams := fmt.Sprintf(`{"modelHash":%q}`, hash)
	diagnosticsBody := callCore("GetDiagnostics", diagnosticsParams)
	diagnosticsResponse, err := svc.GetDiagnostics(ctx, mustUnmarshal[pb.DiagnosticsRequest](t, diagnosticsParams))
	if err != nil {
		t.Fatalf("grpc GetDiagnostics: %v", err)
	}
	equal("GetDiagnostics", diagnosticsParams, diagnosticsResponse, diagnosticsBody)

	parseFileParams := fmt.Sprintf(`{"content":%s}`, mustJSON(t, cleanCoreModel))
	parseFileBody := callCore("ParseFile", parseFileParams)
	parseFileResponse, err := svc.ParseFile(ctx, mustUnmarshal[pb.ParseFileRequest](t, parseFileParams))
	if err != nil {
		t.Fatalf("grpc ParseFile: %v", err)
	}
	equal("ParseFile", parseFileParams, parseFileResponse, parseFileBody)

	for _, symbolID := range []string{
		"Demo::VehicleBase",
		"Demo::'Vehicle Type'::engine",
		"Demo::'Vehicle Type'", // a quoted segment resolves through the shared name parser
		"Demo::Missing",
	} {
		params := fmt.Sprintf(`{"modelHash":%q,"symbolId":%q}`, hash, symbolID)
		body := callCore("GetSymbol", params)
		response, err := svc.GetSymbol(ctx, mustUnmarshal[pb.GetSymbolRequest](t, params))
		if err != nil {
			t.Fatalf("grpc GetSymbol(%s): %v", symbolID, err)
		}
		equal("GetSymbol", params, response, body)
		var got struct {
			Symbol *struct {
				Attributes []struct {
					Name  string          `json:"name"`
					Unit  string          `json:"unit"`
					Value json.RawMessage `json:"value"`
				} `json:"attributes"`
				Multiplicity *struct {
					Lower string `json:"lower"`
					Upper string `json:"upper"`
				} `json:"multiplicity"`
				Specializations []json.RawMessage `json:"specializations"`
			} `json:"symbol"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("decoding GetSymbol(%s): %v\n%s", symbolID, err, body)
		}
		if symbolID == "Demo::Missing" {
			if got.Error != "symbol not found: Demo::Missing" {
				t.Errorf("missing symbol response error = %q", got.Error)
			}
			continue
		}
		if got.Symbol == nil {
			t.Errorf("GetSymbol(%s) returned no symbol: %s", symbolID, body)
			continue
		}
		switch symbolID {
		case "Demo::'Vehicle Type'":
			if len(got.Symbol.Specializations) == 0 {
				t.Error("Vehicle Type has no reported specialization")
			}
			attributes := make(map[string]struct {
				unit  string
				value json.RawMessage
			})
			for _, attribute := range got.Symbol.Attributes {
				attributes[attribute.Name] = struct {
					unit  string
					value json.RawMessage
				}{unit: attribute.Unit, value: attribute.Value}
			}
			for _, name := range []string{"integerValue", "realValue", "stringValue", "booleanValue", "distance"} {
				attribute, ok := attributes[name]
				if !ok {
					t.Errorf("Vehicle Type has no %s attribute", name)
				} else if len(attribute.value) == 0 || string(attribute.value) == "null" {
					t.Errorf("Vehicle Type attribute %s has no constant value", name)
				}
			}
			if attributes["distance"].unit == "" {
				t.Error("quantity attribute has no reported unit")
			}
		case "Demo::'Vehicle Type'::engine":
			if got.Symbol.Multiplicity == nil ||
				got.Symbol.Multiplicity.Lower != "0" ||
				got.Symbol.Multiplicity.Upper != "*" {
				t.Errorf("usage multiplicity = %+v, want [0..*]", got.Symbol.Multiplicity)
			}
		}
	}

	_, _ = parseSources("core-unresolved.sysml",
		`package Demo { part def Vehicle { part engine : MissingType; } }`)

	validationHash, validationBody := parseSources("core-validation.sysml", readFile(t, "core-validation.sysml"))
	if !diagnosticsHaveCode(t, validationBody, "one-type") {
		t.Fatal("core/grpc validation response is missing the one-type diagnostic")
	}
	validationParams := fmt.Sprintf(`{"modelHash":%q}`, validationHash)
	validationDiagnostics := callCore("GetDiagnostics", validationParams)
	validationGrpc, err := svc.GetDiagnostics(ctx, mustUnmarshal[pb.DiagnosticsRequest](t, validationParams))
	if err != nil {
		t.Fatalf("grpc GetDiagnostics for validation model: %v", err)
	}
	equal("GetDiagnostics", validationParams, validationGrpc, validationDiagnostics)

	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	engineBody, err := eng.Call(ctx, "ParseSources", []byte(coreValidationParams(t)))
	if err != nil {
		t.Fatalf("engine ParseSources: %v", err)
	}
	if diagnosticsHaveCode(t, engineBody, "one-type") {
		t.Fatal("sysml-engine reported one-type; the case must prove the validation pass ran only in core")
	}

	_, _ = parseSources("core-syntax-error.sysml", `package Demo { part def Broken {`)

	const missingHash = "missing-core-model"
	_, coreErr := c.Call(ctx, "GetDiagnostics", []byte(fmt.Sprintf(`{"modelHash":%q}`, missingHash)))
	var callErr *jsonrpc.Error
	if !errors.As(coreErr, &callErr) || callErr.Code != jsonrpc.CodeNotFound {
		t.Fatalf("core unknown-model error = %v, want NotFound", coreErr)
	}
	_, grpcErr := svc.GetDiagnostics(ctx, &pb.DiagnosticsRequest{ModelHash: missingHash})
	if grpcErr == nil || connect.CodeOf(grpcErr) != connect.CodeNotFound {
		t.Fatalf("grpc unknown-model error = %v, want NotFound", grpcErr)
	}
	const wantMissing = "model not found: " + missingHash
	if callErr.Message != wantMissing || !strings.Contains(grpcErr.Error(), wantMissing) {
		t.Errorf("unknown-model messages: core %q, grpc %q; want %q", callErr.Message, grpcErr, wantMissing)
	}

	_, err = c.Call(ctx, "Evaluate", []byte(`{}`))
	if !errors.As(err, &callErr) || callErr.Code != jsonrpc.CodeUnimplemented ||
		callErr.Message != "Evaluate is not served by sysml-core: execution is served by sysml-engine, and every other method by sysml-grpc" {
		t.Errorf("unserved method error = %v, want the sysml-core Unimplemented message", err)
	}
}

func diagnosticsHaveCode(t *testing.T, body []byte, code string) bool {
	t.Helper()
	var response struct {
		Diagnostics []struct {
			Code string `json:"code"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decoding diagnostics: %v\n%s", err, body)
	}
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func parseSourcesParams(t *testing.T, name, content string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{"documents": []map[string]string{{"name": name, "content": content}}})
	if err != nil {
		t.Fatalf("encoding ParseSources params: %v", err)
	}
	return string(body)
}

func TestCoreDependencies(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolving repository root: %v", err)
	}
	const module = "github.com/Open-MBEE/OpenSysML"
	required := []string{
		module + "/internal/check/passes",
		module + "/internal/workspace/libs",
		module + "/internal/frontend/jsonrpc",
		module + "/internal/frontend/syntax",
	}
	for _, target := range []wasmTarget{{name: "js", goos: "js"}, {name: "wasip1", goos: "wasip1"}} {
		t.Run(target.name, func(t *testing.T) {
			cmd := exec.Command("go", "list", "-deps", "./cmd/sysml-core")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GOOS="+target.goos, "GOARCH=wasm")
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go list -deps for %s: %v\n%s", target.name, err, output)
			}
			if target.goos == "js" {
				if err := os.WriteFile("/tmp/sysml-core-deps.txt", output, 0o600); err != nil {
					t.Fatalf("saving JS dependency list: %v", err)
				}
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
				if forbiddenCoreDependency(module, dependency) {
					t.Errorf("forbidden dependency in %s build: %s", target.name, dependency)
				}
			}
		})
	}
}

func forbiddenCoreDependency(module, dependency string) bool {
	for _, prefix := range []string{
		module + "/api/",
		"google.golang.org/protobuf/",
		"connectrpc.com/",
		module + "/internal/frontend/grpc",
		module + "/internal/frontend/protoconv",
		module + "/internal/frontend/engine",
		module + "/internal/frontend/stdiorpc",
		module + "/internal/exec/",
		module + "/internal/doc/",
	} {
		if strings.HasPrefix(dependency, prefix) {
			return true
		}
	}
	translate := module + "/internal/translate/"
	return strings.HasPrefix(dependency, translate) && dependency != module+"/internal/translate/rdf"
}
