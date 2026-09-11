// Command toolstandin stands in for an external tool in the tests of the tool protocol:
// it reads the one JSON request on standard input and answers as TOOL_STANDIN_MODE says.
// It is built by the tests that run it and lives under testdata, out of every build.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// ModeEnv selects how the stand-in answers; the empty mode answers the request.
const ModeEnv = "TOOL_STANDIN_MODE"

// RecordEnv names a file every request is appended to, one JSON object per line.
const RecordEnv = "TOOL_STANDIN_RECORD"

// CounterEnv names a file counting invocations, which the varying mode answers with.
const CounterEnv = "TOOL_STANDIN_COUNTER"

// value is one protocol value: a JSON value and, for a quantity, its unit.
type value struct {
	Value json.RawMessage `json:"value"`
	Unit  string          `json:"unit,omitempty"`
}

// request is the protocol's request object.
type request struct {
	ToolName string           `json:"toolName"`
	URI      string           `json:"uri"`
	Inputs   map[string]value `json:"inputs"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "toolstandin:", err)
		os.Exit(3)
	}
}

func run() error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	if record := os.Getenv(RecordEnv); record != "" {
		f, err := os.OpenFile(record, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- the test names the record file
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(f, "%s\n", data); err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	var req request
	if err := json.Unmarshal(data, &req); err != nil {
		return fmt.Errorf("request is not the protocol's object: %w", err)
	}
	switch mode := os.Getenv(ModeEnv); mode {
	case "", "answer":
		return reply(answer(req))
	case "missing-output":
		out := answer(req)
		delete(out, "x")
		return reply(out)
	case "unknown-output":
		out := answer(req)
		out["y"] = out["x"]
		return reply(out)
	case "string-unit":
		out := answer(req)
		out["a"] = value{Value: json.RawMessage(`"fast"`), Unit: "m/s**2"}
		return reply(out)
	case "wrong-unit":
		out := answer(req)
		out["a"] = value{Value: out["a"].Value, Unit: "kg"}
		return reply(out)
	case "wrong-type":
		out := answer(req)
		out["a"] = value{Value: json.RawMessage(`true`)}
		return reply(out)
	case "duplicate-output":
		_, err := fmt.Fprint(os.Stdout, `{"outputs":{"a":{"value":1,"unit":"m/s**2"},"a":{"value":2,"unit":"m/s**2"},"v":{"value":1,"unit":"m/s"},"x":{"value":1,"unit":"m"}}}`)
		return err
	case "malformed":
		_, err := fmt.Fprint(os.Stdout, "a = 3 m/s^2")
		return err
	case "flood":
		// A reply that never ends: the interpreter must stop reading, not the tool writing.
		line := []byte(`{"outputs":{"a":{"value":1,"unit":"m/s**2"},"padding":"` + strings.Repeat("x", 1<<16))
		for {
			if _, err := os.Stdout.Write(line); err != nil {
				return err
			}
		}
	case "error":
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"error": "equation did not converge"})
	case "exit":
		fmt.Fprintln(os.Stderr, "license server unreachable")
		os.Exit(2)
	case "hang":
		time.Sleep(time.Minute)
		return reply(answer(req))
	case "varying":
		n, err := count()
		if err != nil {
			return err
		}
		out := answer(req)
		out["a"] = value{Value: raw(float64(n)), Unit: "m/s**2"}
		return reply(out)
	default:
		return fmt.Errorf("%s=%q is not a mode", ModeEnv, mode)
	}
	return nil
}

// answer computes the pilot fixture's outputs from its inputs: a from C_D, v from v0 in
// v0's own unit, x from x0 in centimetres.
func answer(req request) map[string]value {
	cd := number(req.Inputs["C_D"])
	v0 := req.Inputs["v0"]
	x0 := number(req.Inputs["x0"])
	return map[string]value{
		"a": {Value: raw(cd * 10), Unit: "m/s**2"},
		"v": {Value: raw(number(v0) + 7.2), Unit: v0.Unit},
		"x": {Value: raw(x0*100 + 1000), Unit: "cm"},
	}
}

func number(v value) float64 {
	f, _ := strconv.ParseFloat(string(v.Value), 64)
	return f
}

// raw spells a real as JSON, keeping a decimal point so it stays a Real on the wire.
func raw(f float64) json.RawMessage {
	text := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.ContainsAny(text, ".eE") {
		text += ".0"
	}
	return json.RawMessage(text)
}

func reply(outputs map[string]value) error {
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"outputs": outputs})
}

// count increments the invocation counter and returns its new value.
func count() (int, error) {
	path := os.Getenv(CounterEnv)
	if path == "" {
		return 0, fmt.Errorf("%s is not set", CounterEnv)
	}
	n := 0
	// #nosec G304 -- the test names the counter file
	if data, err := os.ReadFile(path); err == nil {
		n, _ = strconv.Atoi(string(data))
	}
	n++
	return n, os.WriteFile(path, []byte(strconv.Itoa(n)), 0o600)
}
