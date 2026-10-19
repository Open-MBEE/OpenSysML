package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// requestOf builds the request message of an RPC from its protojson form.
func requestOf(t *testing.T, method, body string) protoreflect.Message {
	t.Helper()
	descriptor, err := methodByName(method)
	if err != nil {
		t.Fatal(err)
	}
	request := dynamicpb.NewMessage(descriptor.Input())
	if err := protojson.Unmarshal([]byte(body), request); err != nil {
		t.Fatalf("%s request: %v", method, err)
	}
	return request
}

func TestEveryServiceRPCHasARuntimeTool(t *testing.T) {
	methods := serviceDescriptor(t).Methods()
	for i := 0; i < methods.Len(); i++ {
		name := string(methods.Get(i).Name())
		if _, err := runtimeTool(name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := runtimeTool("NoSuchRPC"); err == nil {
		t.Error("an RPC without a tool was not reported")
	}
}

func serviceDescriptor(t *testing.T) protoreflect.ServiceDescriptor {
	t.Helper()
	descriptor, err := methodByName("GetServerInfo")
	if err != nil {
		t.Fatal(err)
	}
	return descriptor.Parent().(protoreflect.ServiceDescriptor)
}

func TestToolArgumentsAreTheRequestUnderItsJSONNames(t *testing.T) {
	request := requestOf(t, "RunSweep", `{"modelHash":"h","symbolId":"Sw::Sum","samples":"3",
		"seed":"18446744073709551615","arguments":[{"intValue":"9007199254740993"}],
		"ranges":[{"parameter":"x","start":{"intValue":"1"},"end":{"intValue":"2"}}]}`)
	got, err := toolArguments("RunSweep", request, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got["modelHash"] != "h" || got["symbolId"] != "Sw::Sum" {
		t.Errorf("arguments = %v, want the request's modelHash and symbolId", got)
	}
	// The request's own 64-bit integers are numbers, which the tool's typed arguments take.
	if got["samples"] != json.Number("3") || got["seed"] != json.Number("18446744073709551615") {
		t.Errorf("samples = %#v, seed = %#v, want JSON numbers", got["samples"], got["seed"])
	}
	// A nested Value keeps protojson's spelling, which the tool hands to protojson.
	value := got["arguments"].([]any)[0].(map[string]any)
	if value["intValue"] != "9007199254740993" {
		t.Errorf("arguments[0].intValue = %#v, want protojson's string", value["intValue"])
	}
	start := got["ranges"].([]any)[0].(map[string]any)["start"].(map[string]any)
	if start["intValue"] != "1" {
		t.Errorf("ranges[0].start.intValue = %#v, want protojson's string", start["intValue"])
	}

	if got, err := toolArguments("GetServerInfo", requestOf(t, "GetServerInfo", `{}`), t.TempDir()); err != nil || len(got) != 0 {
		t.Errorf("GetServerInfo arguments = %v, %v; want none", got, err)
	}
}

func TestParseFileArguments(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "vehicle.sysml")
	tests := []struct {
		name    string
		request string
		want    map[string]any
	}{
		{"content", `{"content":"package P;","language":"kerml","strict_conformance":true}`,
			map[string]any{"strictConformance": true,
				"sources": []any{map[string]any{"path": "<content>", "text": "package P;", "language": "kerml"}}}},
		{"path under the root", `{"file_path":"` + inside + `"}`,
			map[string]any{"paths": []any{inside}}},
		{"relative path", `{"file_path":"vehicle.sysml"}`,
			map[string]any{"paths": []any{"vehicle.sysml"}}},
		{"no source", `{}`, map[string]any{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := toolArguments("ParseFile", requestOf(t, "ParseFile", test.request), root)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("arguments = %#v, want %#v", got, test.want)
			}
		})
	}
	uncovered := map[string]string{
		"path outside the root":     `{"file_path":"/nonexistent/path/to/file.sysml"}`,
		"path with a language":      `{"file_path":"` + inside + `","language":"kerml"}`,
		"deprecated content hash":   `{"content":"package P;","content_hash":"abc"}`,
		"documents interleaved":     `{"documents":[{"content":"package A;"},{"file_path":"` + inside + `"}]}`,
		"path with a name":          `{"documents":[{"file_path":"` + inside + `","name":"v"}]}`,
		"document without a source": `{"documents":[{"language":"sysml"}]}`,
		// The service reads an unlanguaged inline document as SysML whatever its
		// name; the tool reads a .kerml name as KerML.
		"kerml name without a language": `{"documents":[{"content":"package C;","name":"c.KerML"}]}`,
	}
	for name, request := range uncovered {
		t.Run(name, func(t *testing.T) {
			method := "ParseFile"
			if request[2:11] == "documents" {
				method = "ParseSources"
			}
			_, err := toolArguments(method, requestOf(t, method, request), root)
			var reason *uncoveredError
			if !errors.As(err, &reason) {
				t.Fatalf("err = %v, want an uncovered reason", err)
			}
		})
	}
}

func TestParseSourcesArguments(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "lib.kerml")
	request := requestOf(t, "ParseSources", `{"documents":[
		{"file_path":"`+inside+`"},
		{"content":"package A;","name":"a.sysml"},
		{"content":"package B;","language":"kerml"},
		{"content":"package D;","name":"d.kerml","language":"kerml"}],
		"strict_conformance":true,"base_model_hash":"h0"}`)
	got, err := toolArguments("ParseSources", request, root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"strictConformance": true,
		"baseModelHash":     "h0",
		"paths":             []any{inside},
		"sources": []any{
			map[string]any{"path": "a.sysml", "text": "package A;"},
			map[string]any{"path": "", "text": "package B;", "language": "kerml"},
			map[string]any{"path": "d.kerml", "text": "package D;", "language": "kerml"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("arguments = %#v, want %#v", got, want)
	}
}

func TestToolStatusReadsTheCodeTheToolSpelled(t *testing.T) {
	refused := func(text string) *mcp.CallToolResult {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: text}}}
	}
	tests := []struct {
		text    string
		code    codes.Code
		message string
	}{
		{"NOT_FOUND: no such file: x.sysml", codes.NotFound, "no such file: x.sysml"},
		{"INVALID_ARGUMENT: imports must name a file", codes.InvalidArgument, "imports must name a file"},
		{"UNKNOWN: it broke", codes.Unknown, "it broke"},
		{schemaRefusal + `validating root: required: missing properties: ["imports"]`,
			codes.InvalidArgument, `validating root: required: missing properties: ["imports"]`},
		{"something else entirely: with a colon", codes.Unknown, "something else entirely: with a colon"},
		{"no colon at all", codes.Unknown, "no colon at all"},
	}
	for _, test := range tests {
		err := toolStatus(refused(test.text))
		if status.Code(err) != test.code || status.Convert(err).Message() != test.message {
			t.Errorf("toolStatus(%q) = %s %q, want %s %q", test.text,
				status.Code(err), status.Convert(err).Message(), test.code, test.message)
		}
	}
}

func TestToolResponseReadsTheStructuredResult(t *testing.T) {
	output := func(method string) protoreflect.MessageDescriptor {
		descriptor, err := methodByName(method)
		if err != nil {
			t.Fatal(err)
		}
		return descriptor.Output()
	}
	root := map[string]any{"id": "P", "name": "P", "kind": "package"}
	response, err := toolResponse("ParseFile", output("ParseFile"), map[string]any{
		"modelHash": "h", "roots": []any{root}, "affected": []any{}, "diagnostics": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	fields := response.Descriptor().Fields()
	if got := response.Get(fields.ByName("model_hash")).String(); got != "h" {
		t.Errorf("model_hash = %q, want h", got)
	}
	if got := response.Get(fields.ByName("root")).Message().Get(fields.ByName("root").Message().Fields().ByName("id")).String(); got != "P" {
		t.Errorf("root.id = %q, want the one root", got)
	}
	if _, err := toolResponse("ParseFile", output("ParseFile"), map[string]any{"roots": []any{root, root}}); err == nil {
		t.Error("two roots for one file were accepted")
	}
	if _, err := toolResponse("GetServerInfo", output("GetServerInfo"), "not an object"); err == nil {
		t.Error("a non-object result was accepted")
	}
	if _, err := toolResponse("GetServerInfo", output("GetServerInfo"), map[string]any{"noSuchField": 1}); err == nil {
		t.Error("a field the response lacks was accepted")
	}
	info, err := toolResponse("GetServerInfo", output("GetServerInfo"), map[string]any{"version": "v", "capabilities": []any{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Get(info.Descriptor().Fields().ByName("capabilities")).List(); got.Len() != 2 {
		t.Errorf("capabilities = %v, want two", got)
	}
}

func TestMCPProtocolRunsUnderEitherTransport(t *testing.T) {
	for _, transport := range []string{transportConnect, transportGRPC} {
		if err := validateProtocols([]string{"grpc", "mcp"}, transport); err != nil {
			t.Errorf("-transport %s refused mcp: %v", transport, err)
		}
	}
	if got, err := parseProtocols("mcp"); err != nil || !slices.Equal(got, []string{"mcp"}) {
		t.Errorf("parseProtocols(mcp) = %v, %v", got, err)
	}
}

// mcpUncoveredScenarios are the scenarios the runtime tools cannot reach: their
// path is outside the server's root, which the tools refuse before the service
// could answer NOT_FOUND for it. They are the only skips mcp has that grpc lacks.
var mcpUncoveredScenarios = []string{
	"parse/an_absent_file_is_not_found",
	"parse_sources/an_absent_file_is_not_found",
}

// TestSuitePassesThroughMCP runs the committed suite through sysml-mcp beside
// sysml-grpc, the way `make conformance-mcp` does, and requires the two to
// agree scenario by scenario, the uncovered ones excepted.
func TestSuitePassesThroughMCP(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and starts sysml-grpc and sysml-mcp")
	}
	binaries := map[string]string{}
	for _, name := range []string{"sysml-grpc", "sysml-mcp"} {
		binary := filepath.Join(t.TempDir(), name)
		build := exec.Command("go", gobuild.Args(binary)...)
		build.Dir = filepath.Join("..", "..", "..", "cmd", name)
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("building %s: %v\n%s", name, err, out)
		}
		binaries[name] = binary
	}

	report := filepath.Join(t.TempDir(), "report.json")
	err := runSuite(options{
		dir:       suiteDir,
		binary:    binaries["sysml-grpc"],
		mcpBinary: binaries["sysml-mcp"],
		report:    report,
		allowSkip: true,
		protocols: "grpc,mcp",
		transport: transportConnect,
		withhold:  "strict_conformance,oslc_query",
	})
	if err != nil {
		t.Fatalf("runSuite: %v", err)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var written Report
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("the report is not JSON: %v", err)
	}
	if written.Failed != 0 || written.Errored != 0 || written.Passed == 0 {
		t.Errorf("report: %d passed, %d failed, %d errored", written.Passed, written.Failed, written.Errored)
	}
	if written.MCPServer != binaries["sysml-mcp"] {
		t.Errorf("report names %q as the MCP server, want %q", written.MCPServer, binaries["sysml-mcp"])
	}
	if written.Service != binaries["sysml-grpc"] {
		t.Errorf("report names %q as the service, want %q", written.Service, binaries["sysml-grpc"])
	}
	for _, config := range written.Configurations {
		outcomes := map[string]map[string]string{}
		for _, protocol := range config.Protocols {
			outcomes[protocol.Protocol] = map[string]string{}
			for _, result := range protocol.Results {
				outcomes[protocol.Protocol][result.ID] = result.Outcome
			}
		}
		for id, outcome := range outcomes["grpc"] {
			want := outcome
			if slices.Contains(mcpUncoveredScenarios, id) {
				want = "skip"
			}
			if got := outcomes["mcp"][id]; got != want {
				t.Errorf("%s: %s is %q over mcp, %q over grpc", config.Name, id, got, outcome)
			}
		}
	}
}

// TestMCPAloneNeedsNoService runs the suite through sysml-mcp only, with no
// sysml-grpc to start: the mcp protocol owns its server.
func TestMCPAloneNeedsNoService(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and starts sysml-mcp")
	}
	binary := filepath.Join(t.TempDir(), "sysml-mcp")
	build := exec.Command("go", gobuild.Args(binary)...)
	build.Dir = filepath.Join("..", "..", "..", "cmd", "sysml-mcp")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building sysml-mcp: %v\n%s", err, out)
	}
	report := filepath.Join(t.TempDir(), "report.json")
	err := runSuite(options{
		dir:       suiteDir,
		binary:    filepath.Join(t.TempDir(), "no-such-sysml-grpc"),
		mcpBinary: binary,
		report:    report,
		run:       "^server_info/",
		protocols: "mcp",
		transport: transportConnect,
	})
	if err != nil {
		t.Fatalf("runSuite: %v", err)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var written Report
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("the report is not JSON: %v", err)
	}
	if written.Passed == 0 || written.Failed != 0 || written.Errored != 0 || written.Service != "" || written.MCPServer != binary {
		t.Errorf("report: %d passed, %d failed, %d errored, service %q, mcp server %q",
			written.Passed, written.Failed, written.Errored, written.Service, written.MCPServer)
	}
}
