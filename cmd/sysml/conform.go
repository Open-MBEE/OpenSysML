package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

const checkTraceFlag = "check-trace"

type conformResult struct {
	Status  string   `json:"status"`
	Row     int      `json:"row"`
	Moves   int      `json:"moves"`
	Message string   `json:"message,omitempty"`
	Witness []string `json:"witness,omitempty"`
}

func (r conformResult) write(out, errOut io.Writer, jsonOut bool) int {
	if jsonOut {
		if err := json.NewEncoder(out).Encode(r); err != nil {
			fmt.Fprintln(errOut, errPrefix, err)
			return exitUnevaluable
		}
	} else {
		message := r.text()
		if r.Message != "" {
			message += ": " + r.Message
		}
		if _, err := fmt.Fprintln(out, message); err != nil {
			fmt.Fprintln(errOut, errPrefix, err)
			return exitUnevaluable
		}
	}
	switch r.Status {
	case string(runtime.TraceMatched):
		return exitHolds
	case string(runtime.TraceNoMatch):
		return exitFailed
	default:
		return exitUnevaluable
	}
}

func (r conformResult) text() string {
	switch r.Status {
	case string(runtime.TraceMatched):
		return fmt.Sprintf("Trace matches through row %d", r.Row)
	case string(runtime.TraceNoMatch):
		return fmt.Sprintf("Trace does not match at row %d", r.Row)
	case string(runtime.TraceInconclusive):
		if r.Row > 0 {
			return fmt.Sprintf("Trace check inconclusive after matching through row %d", r.Row)
		}
		return "Trace check inconclusive"
	default:
		if r.Row > 0 {
			return fmt.Sprintf("Invalid trace input at row %d", r.Row)
		}
		return "Cannot check trace"
	}
}

func invalidTrace(err error) conformResult {
	result := conformResult{Status: "invalid", Message: err.Error()}
	var parsed *conformRowError
	var semantic *runtime.TraceInputError
	switch {
	case errors.As(err, &parsed):
		result.Row = parsed.row
	case errors.As(err, &semantic):
		result.Row = semantic.Row
	}
	return result
}

func conformCLI(fs *flag.FlagSet, c checks) int {
	format, misuse := reportFormat(c.format, c.jsonOut)
	if misuse != "" {
		return invalidTrace(errors.New(misuse)).write(os.Stdout, os.Stderr, c.jsonOut)
	}
	if format != formatText && format != formatJSON {
		err := fmt.Errorf("-%s supports -format text or json, not %s", checkTraceFlag, format)
		return invalidTrace(err).write(os.Stdout, os.Stderr, false)
	}
	jsonOut := format == formatJSON
	var incompatible string
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case checkTraceFlag, "state", "json", "format", checkDepthFlag, checkStatesFlag, "check-timeout",
			"quiet", "debug", "strict", "disable-lint", "enable-lint", "library-root", "no-record-cache", "jobs":
		default:
			if incompatible == "" {
				incompatible = f.Name
			}
		}
	})
	if incompatible != "" {
		result := invalidTrace(fmt.Errorf("-%s cannot be combined with -%s", checkTraceFlag, incompatible))
		return result.write(os.Stdout, os.Stderr, jsonOut)
	}
	if status := resolveRunBounds(); status != 0 {
		return status
	}
	return runConform(fs.Args(), c).write(os.Stdout, os.Stderr, jsonOut)
}

func runConform(files []string, c checks) conformResult {
	if len(c.states) != 1 {
		return invalidTrace(fmt.Errorf("-%s requires one -state; for example, -state \"Machine rig.controller\"", checkTraceFlag))
	}
	if c.checkTrace == "" {
		return invalidTrace(fmt.Errorf("-%s requires a JSON Lines file", checkTraceFlag))
	}
	if len(files) == 0 {
		return invalidTrace(fmt.Errorf("no model to check; name a model file containing the state machine"))
	}
	f, err := os.Open(c.checkTrace) // #nosec G304 -- the operator names the trace file on the command line
	if err != nil {
		return invalidTrace(err)
	}
	defer f.Close()
	rows, err := readConformTrace(f)
	if err != nil {
		return invalidTrace(fmt.Errorf("%s: %w", c.checkTrace, err))
	}
	sess := newSession()
	paths, err := sess.ExpandPaths(files)
	if err != nil {
		return invalidTrace(err)
	}
	if _, err := sess.LoadFilesSummary(paths); err != nil {
		return invalidTrace(err)
	}
	if sess.HasErrors() {
		writeLines(os.Stderr, sess.DiagnosticLines())
		return invalidTrace(fmt.Errorf("%s did not analyse cleanly; no trace check was made", namedModels(files)))
	}
	budget := runtime.CheckBudget{}
	if c.checker.depth.given {
		budget.Depth = c.checker.depth.value
	}
	if c.checker.states.given {
		budget.States = c.checker.states.value
	}
	timeout := 30 * time.Second
	if c.checker.timeout.given {
		timeout = c.checker.timeout.value
	}
	report, err := sess.ConformTrace(behaviors(c.states)[0], rows, budget, timeout)
	if err != nil {
		return invalidTrace(err)
	}
	result := conformResult{
		Status: string(report.Status), Row: report.Row, Moves: report.Moves, Message: report.Reason, Witness: report.Witness,
	}
	if report.Status == runtime.TraceNoMatch {
		result.Row++
		result.Message = "the model cannot reproduce the initial observation"
		if result.Row > 1 {
			result.Message = fmt.Sprintf("the model cannot reproduce the observation after %s", rows[result.Row-1].Signal)
		}
	}
	return result
}

type conformRowError struct {
	row int
	err error
}

func (e *conformRowError) Error() string { return fmt.Sprintf("row %d: %v", e.row, e.err) }

// readConformTrace preserves evidence as typed values and requires the same
// observation projection at every checkpoint. It doesn't interpret model names.
func readConformTrace(reader io.Reader) ([]runtime.TraceRow, error) {
	const limit = 8 << 20
	input, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if len(input) > limit {
		return nil, fmt.Errorf("trace exceeds %d bytes", limit)
	}
	if !utf8.Valid(input) {
		return nil, fmt.Errorf("trace is not UTF-8")
	}
	scanner := bufio.NewScanner(bytes.NewReader(input))
	scanner.Buffer(nil, 1<<20)
	var rows []runtime.TraceRow
	for scanner.Scan() {
		line := len(rows) + 1
		row, err := decodeTraceRow(scanner.Bytes(), line == 1)
		if err != nil {
			return nil, &conformRowError{row: line, err: err}
		}
		if line > 1 && (len(row.Observe) != len(rows[0].Observe) || !sameObservationKeys(row.Observe, rows[0].Observe)) {
			return nil, &conformRowError{row: line, err: errors.New("observe must contain the same feature names as row 1")}
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, &conformRowError{row: len(rows) + 1, err: err}
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("the initial observation is required")
	}
	return rows, nil
}

func sameObservationKeys(a, b map[string]runtime.Value) bool {
	for key := range a {
		if _, ok := b[key]; !ok {
			return false
		}
	}
	return true
}

func decodeTraceRow(input []byte, initial bool) (runtime.TraceRow, error) {
	if len(bytes.TrimSpace(input)) == 0 {
		return runtime.TraceRow{}, fmt.Errorf("blank lines are not observations")
	}
	if !pairedTraceSurrogates(input) {
		return runtime.TraceRow{}, fmt.Errorf("JSON string contains an unpaired UTF-16 surrogate")
	}
	dec := json.NewDecoder(bytes.NewReader(input))
	object, err := uniqueTraceObject(dec)
	if err != nil {
		return runtime.TraceRow{}, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return runtime.TraceRow{}, fmt.Errorf("expected one JSON object per line")
	}
	var row runtime.TraceRow
	for _, key := range sortedTraceKeys(object) {
		raw := object[key]
		var err error
		switch key {
		case "signal":
			if initial {
				return row, fmt.Errorf("initial observation must omit signal")
			}
			if err = json.Unmarshal(raw, &row.Signal); err != nil || row.Signal == "" {
				return row, fmt.Errorf("signal must be a nonempty string")
			}
		case "args":
			if initial {
				return row, fmt.Errorf("initial observation must omit args")
			}
			row.Args, err = traceValues(raw)
		case "observe":
			row.Observe, err = traceValues(raw)
		default:
			return row, fmt.Errorf("unknown field %q", key)
		}
		if err != nil {
			return row, fmt.Errorf("%s: %w", key, err)
		}
	}
	if !initial && row.Signal == "" {
		return row, fmt.Errorf("signal is required after the initial observation")
	}
	if len(row.Observe) == 0 {
		return row, fmt.Errorf("observe must name at least one feature")
	}
	return row, nil
}

const traceUnicodeEscapeLen = len(`\u0000`)

// pairedTraceSurrogates checks raw strings before encoding/json can replace
// a lone surrogate with U+FFFD. The decoder still checks JSON syntax.
func pairedTraceSurrogates(input []byte) bool {
	inString := false
	for i := 0; i < len(input); i++ {
		switch input[i] {
		case '"':
			inString = !inString
		case '\\':
			if !inString || i+1 == len(input) {
				continue
			}
			switch unit := traceUnicodeEscape(input[i:]); {
			case unit >= 0xDC00 && unit <= 0xDFFF:
				return false
			case unit >= 0xD800 && unit <= 0xDBFF:
				low := traceUnicodeEscape(input[i+traceUnicodeEscapeLen:])
				if low < 0xDC00 || low > 0xDFFF {
					return false
				}
				i += 2*traceUnicodeEscapeLen - 1
			default:
				i++ // Skip the escaped byte, including literal backslashes and quotes.
			}
		}
	}
	return true
}

func traceUnicodeEscape(input []byte) uint64 {
	if len(input) < traceUnicodeEscapeLen || input[0] != '\\' || input[1] != 'u' {
		return 0
	}
	unit, err := strconv.ParseUint(string(input[2:traceUnicodeEscapeLen]), 16, 16)
	if err != nil {
		return 0
	}
	return unit
}

func traceValues(raw json.RawMessage) (map[string]runtime.Value, error) {
	object, err := uniqueTraceObject(json.NewDecoder(bytes.NewReader(raw)))
	if err != nil {
		return nil, err
	}
	values := make(map[string]runtime.Value, len(object))
	for _, key := range sortedTraceKeys(object) {
		raw := object[key]
		switch raw[0] {
		case '"':
			var value string
			if err := json.Unmarshal(raw, &value); err != nil {
				return nil, err
			}
			values[key] = runtime.NewStringValue(value)
		case 't', 'f':
			values[key] = runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: raw[0] == 't'}}
		default:
			number, err := strconv.ParseInt(string(raw), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%q: want a string, Boolean or signed 64-bit integer", key)
			}
			values[key] = runtime.Value{Kind: runtime.ValConst, Const: semantics.IntValue(number)}
		}
	}
	return values, nil
}

func sortedTraceKeys(values map[string]json.RawMessage) []string {
	return slices.Sorted(maps.Keys(values))
}

// uniqueTraceObject rejects duplicate keys, which encoding/json would
// otherwise silently overwrite, potentially replacing contrary evidence.
func uniqueTraceObject(dec *json.Decoder) (map[string]json.RawMessage, error) {
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('{') {
		return nil, fmt.Errorf("expected a JSON object")
	}
	object := map[string]json.RawMessage{}
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok || key == "" {
			return nil, fmt.Errorf("expected a nonempty field name")
		}
		if _, exists := object[key]; exists {
			return nil, fmt.Errorf("duplicate field %q", key)
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		object[key] = value
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return object, nil
}
