package wasm

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
	fsyntax "github.com/Open-MBEE/OpenSysML/internal/frontend/syntax"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// syntaxGzipBudget bounds the gzipped js build of sysml-syntax: staying small is
// the point of serving no resolution and no stdlib, so exceeding the budget
// means the command pulled in a dependency it must not have.
const syntaxGzipBudget = 1600000

// syntaxCalls are the calls the stdio session and the host surface both make,
// one of each method over its fixture.
func syntaxCalls(t *testing.T) []engineCall {
	return []engineCall{
		{method: "Parse", params: syntaxParams(t, "malformed.sysml", nil)},
		{method: "Format", params: syntaxParams(t, "unformatted.sysml", nil)},
		{method: "Tokens", params: syntaxParams(t, "tokens.sysml", nil)},
	}
}

// syntaxParams is the request body for one syntax call: the fixture's content
// inline, so a WebAssembly build needs no filesystem to answer it.
func syntaxParams(t *testing.T, fixtureName string, extra map[string]any) string {
	t.Helper()
	data, err := os.ReadFile(fixture(t, fixtureName))
	if err != nil {
		t.Fatalf("reading the fixture %s: %v", fixtureName, err)
	}
	req := map[string]any{"content": string(data)}
	for k, v := range extra {
		req[k] = v
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("rendering the request: %v", err)
	}
	return string(body)
}

// nativeCall answers what the same call answers on a native build, byte for byte.
func nativeCall(t *testing.T, method, params string) string {
	t.Helper()
	body, err := fsyntax.Call(context.Background(), method, []byte(params))
	if err != nil {
		t.Fatalf("native %s refused: %v", method, err)
	}
	return string(body)
}

// frames reads every Content-Length-delimited answer body out of a session's
// whole output.
func frames(t *testing.T, output string) []string {
	t.Helper()
	r := bufio.NewReader(strings.NewReader(output))
	var bodies []string
	for {
		body, err := frame(r)
		if err == io.EOF {
			return bodies
		}
		if err != nil {
			t.Fatalf("reading a session answer: %v\noutput:\n%s", err, output)
		}
		bodies = append(bodies, string(body))
	}
}

// syntaxSubtests runs the sysml-syntax half of the run gate on bins: a stdio
// session doing real work on both targets, plus the globalThis host surface and
// the size budget that surface exists to keep, on js.
func syntaxSubtests(t *testing.T, target wasmTarget, r runner, bins map[string]string) {
	t.Run("answers a syntax session over stdio", func(t *testing.T) {
		calls := syntaxCalls(t)
		session := engineSession(t, calls...)
		var args []string
		if target.goos == "js" {
			// The js build installs globalThis.sysmlSyntax unless asked for the pipe.
			args = append(args, "-stdio")
		}
		got := r.runWithInput(t, bins["sysml-syntax"], session, args...)
		if got.code != 0 {
			t.Fatalf("sysml-syntax -stdio exited %d:\n%s", got.code, got.output)
		}
		bodies := frames(t, got.output)
		if len(bodies) != len(calls) {
			t.Fatalf("the session answered %d frames, want %d:\n%s", len(bodies), len(calls), got.output)
		}
		for i, call := range calls {
			var answer struct {
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Code    uint32 `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(bodies[i]), &answer); err != nil {
				t.Fatalf("%s: the session answer is not JSON: %v\n%s", call.method, err, bodies[i])
			}
			if answer.Error != nil {
				t.Fatalf("%s: the session refused: %s", call.method, answer.Error.Message)
			}
			if want := nativeCall(t, call.method, call.params); string(answer.Result) != want {
				t.Errorf("%s answered\n%s\nwant the native answer\n%s", call.method, answer.Result, want)
			}
		}
	})

	if target.goos != "js" {
		return
	}

	t.Run("answers through globalThis.sysmlSyntax", func(t *testing.T) {
		// The script runs under plain node, not through the wasm target runner:
		// it loads wasm_exec.js and the module itself.
		got := nodeRun(t, fixture(t, "syntax.mjs"),
			wasmExecJS(t), bins["sysml-syntax"],
			fixture(t, "malformed.sysml"), fixture(t, "unformatted.sysml"), fixture(t, "tokens.sysml"))
		if !strings.Contains(got.output, "version "+versionStamp) {
			t.Errorf("syntax.mjs output is missing the stamped version:\n%s", got.output)
		}
		answers := map[string]string{}
		for _, line := range strings.Split(got.output, "\n") {
			method, rest, _ := strings.Cut(line, " ")
			answers[method] = rest
		}
		for _, call := range syntaxCalls(t) {
			want := jsonrpc.Envelope([]byte(nativeCall(t, call.method, call.params)), nil)
			if answers[call.method] != want {
				t.Errorf("%s answered\n%s\nwant\n%s", call.method, answers[call.method], want)
			}
		}
		if !strings.Contains(answers["Evaluate"], `"code":12`) {
			t.Errorf("an unserved method answered %s, want code 12", answers["Evaluate"])
		}
	})

	t.Run("fits the syntax size budget", func(t *testing.T) {
		data, err := os.ReadFile(bins["sysml-syntax"])
		if err != nil {
			t.Fatalf("reading %s: %v", bins["sysml-syntax"], err)
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
		if size := compressed.Len(); size > syntaxGzipBudget {
			t.Errorf("gzipped sysml-syntax.wasm is %d bytes, over the %d-byte budget: "+
				"the command pulled in a dependency it must not have (protobuf, the gRPC "+
				"service, name resolution, the standard library)", size, syntaxGzipBudget)
		} else {
			t.Logf("gzipped sysml-syntax.wasm is %d bytes, under the %d-byte budget", size, syntaxGzipBudget)
		}
	})
}

// TestSyntaxCLIParity proves the command answers what the CLI answers: Parse's
// diagnostics are the messages `sysml -convert sysml` reports for broken input,
// and Format's content is what the converter writes.
func TestSyntaxCLIParity(t *testing.T) {
	ctx := context.Background()

	malformed, err := os.ReadFile(fixture(t, "malformed.sysml"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	body, err := fsyntax.Call(ctx, "Parse", []byte(syntaxParams(t, "malformed.sysml", nil)))
	if err != nil {
		t.Fatalf("Parse refused: %v", err)
	}
	var parsed struct {
		Diagnostics []struct {
			Message  string `json:"message"`
			StartCol int32  `json:"-"`
			Span     struct {
				StartLine int32 `json:"startLine"`
				StartCol  int32 `json:"startCol"`
			} `json:"span"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("decoding the Parse answer: %v\n%s", err, body)
	}
	var got []string
	for _, d := range parsed.Diagnostics {
		got = append(got, fmt.Sprintf("%d:%d: %s", d.Span.StartLine, d.Span.StartCol, d.Message))
	}
	_, cerr := convert.Convert("<content>", malformed, convert.FormatSysML, convert.FormatSysML)
	var syntaxErr *convert.SyntaxError
	if cerr == nil || !errors.As(cerr, &syntaxErr) {
		t.Fatalf("Convert error = %v, want a *convert.SyntaxError", cerr)
	}
	if len(got) < 2 {
		t.Fatalf("Parse reported %d diagnostics, want at least 2", len(got))
	}
	if fmt.Sprint(got) != fmt.Sprint(syntaxErr.Messages) {
		t.Errorf("Parse diagnostics = %v, want the converter's %v", got, syntaxErr.Messages)
	}

	// Format answers what the converter writes, and changes a badly indented
	// document rather than echoing it.
	unformatted, err := os.ReadFile(fixture(t, "unformatted.sysml"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	fbody, err := fsyntax.Call(ctx, "Format", []byte(syntaxParams(t, "unformatted.sysml", nil)))
	if err != nil {
		t.Fatalf("Format refused: %v", err)
	}
	var formatted struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(fbody, &formatted); err != nil {
		t.Fatalf("decoding the Format answer: %v\n%s", err, fbody)
	}
	want, cerr := convert.Convert("unformatted.sysml", unformatted, convert.FormatSysML, convert.FormatSysML)
	if cerr != nil {
		t.Fatalf("Convert refused the fixture: %v", cerr)
	}
	if formatted.Content != string(want) {
		t.Errorf("Format content =\n%s\nwant the converter's\n%s", formatted.Content, want)
	}
	if formatted.Content == string(unformatted) {
		t.Errorf("Format echoed the badly indented input unchanged")
	}

	// A document with syntax errors is refused in the response's error, with
	// the message the converter reports — unless the request tolerates them.
	ebody, err := fsyntax.Call(ctx, "Format", []byte(syntaxParams(t, "malformed.sysml", nil)))
	if err != nil {
		t.Fatalf("Format refused: %v", err)
	}
	var refused struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(ebody, &refused); err != nil {
		t.Fatalf("decoding the Format refusal: %v\n%s", err, ebody)
	}
	if _, cerr := convert.Convert("<content>", malformed, convert.FormatSysML, convert.FormatSysML); cerr == nil || refused.Error != cerr.Error() {
		t.Errorf("Format error = %q, want the converter's %v", refused.Error, cerr)
	}
}

// TestSyntaxDependencies proves the command links none of what a syntactic
// surface must not pull in: no protobuf or gRPC machinery, no semantic or
// workspace layer, and no networking — which is what keeps the js build small.
func TestSyntaxDependencies(t *testing.T) {
	out, err := goFor(t, wasmTarget{name: "js", goos: "js"}, "list", "-deps", "./cmd/sysml-syntax")
	if err != nil {
		t.Fatalf("go list -deps ./cmd/sysml-syntax: %v\n%s", err, out)
	}
	forbidden := []string{
		"google.golang.org/protobuf",
		"connectrpc.com",
		"github.com/Open-MBEE/OpenSysML/api/proto",
		"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc",
		"github.com/Open-MBEE/OpenSysML/internal/workspace/",
		"github.com/Open-MBEE/OpenSysML/internal/semantic/",
		"github.com/Open-MBEE/OpenSysML/internal/exec/",
		"github.com/Open-MBEE/OpenSysML/internal/check/",
		"github.com/Open-MBEE/OpenSysML/internal/ir/",
		"github.com/Open-MBEE/OpenSysML/internal/translate/",
		"net",
		"net/",
	}
	for _, pkg := range strings.Split(string(out), "\n") {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" {
			continue
		}
		for _, bad := range forbidden {
			if pkg == strings.TrimSuffix(bad, "/") || strings.HasPrefix(pkg, bad) {
				t.Errorf("%s is a dependency of cmd/sysml-syntax", pkg)
			}
		}
	}
}
