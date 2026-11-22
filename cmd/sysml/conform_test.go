package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

func TestConformTracePreservesTypedEvidence(t *testing.T) {
	data := `{"observe":{"shown":false,"season":"2026","id":9007199254740993}}
{"signal":"Example::Open","args":{"attempt":-9223372036854775808},"observe":{"id":9007199254740993,"season":"2026","shown":true}}
`
	rows, err := readConformTrace(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := rows[0].Observe["id"].Const.Int; got != 9007199254740993 {
		t.Fatalf("integer lost precision: %d", got)
	}
	if got := rows[1].Args["attempt"].Const.Int; got != -9223372036854775808 {
		t.Fatalf("integer argument lost precision: %d", got)
	}
	if v := rows[0].Observe["season"]; v.Kind != runtime.ValString || v.Str() != "2026" {
		t.Fatalf("string was coerced: %v", v)
	}
	if v := rows[1].Observe["shown"]; v.Kind != runtime.ValConst || !v.Const.Bool {
		t.Fatalf("Boolean was coerced: %v", v)
	}
}

func TestConformTracePreservesUnicodeEvidence(t *testing.T) {
	const initial = `{"observe":{"\ud83d\ude00":"�","text":"\\ud800"}}` + "\n"
	const observed = `{"signal":"Example::\ud83d\ude00","args":{"raw":"\\udfff","symbol":"\uD83D\uDE00"},"observe":{"😀":"\ufffd","text":"\"\\\ud83d\ude00"}}`
	rows, err := readConformTrace(strings.NewReader(initial + observed))
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Observe["😀"].Str() != "�" || rows[1].Observe["😀"].Str() != "�" {
		t.Fatalf("replacement character changed: %+v", rows)
	}
	if rows[0].Observe["text"].Str() != `\ud800` || rows[1].Args["raw"].Str() != `\udfff` {
		t.Fatalf("literal escapes changed: %+v", rows)
	}
	if rows[1].Signal != "Example::😀" || rows[1].Args["symbol"].Str() != "😀" || rows[1].Observe["text"].Str() != `"\😀` {
		t.Fatalf("paired surrogate or escaped quote changed: %+v", rows[1])
	}
}

func TestConformTraceRejectsMalformedEvidence(t *testing.T) {
	const initial = `{"observe":{"shown":false}}` + "\n"
	for _, tc := range []struct {
		name, data, message string
		row                 int
	}{
		{"empty", "", "initial observation", 0},
		{"initial signal", `{"signal":"","observe":{"shown":false}}`, "initial observation must omit signal", 1},
		{"initial args", `{"args":{},"observe":{"shown":false}}`, "initial observation must omit args", 1},
		{"duplicate member", `{"observe":{"shown":false},"observe":{"shown":true}}`, "duplicate", 1},
		{"duplicate observable", `{"observe":{"shown":false,"shown":true}}`, "duplicate", 1},
		{"escaped duplicate", `{"observe":{"shown":false,"sh\u006fwn":true}}`, "duplicate", 1},
		{"unknown field", `{"extra":true,"observe":{"shown":false}}`, "unknown field", 1},
		{"exact field names", `{"Observe":{"shown":false}}`, "unknown field", 1},
		{"empty field name", `{"observe":{"":false}}`, "nonempty field name", 1},
		{"null row", "null", "not null", 1},
		{"null observation", `{"observe":null}`, "not null", 1},
		{"null initial args", `{"args":null,"observe":{"shown":false}}`, "initial observation must omit args", 1},
		{"null args", initial + `{"signal":"Open","args":null,"observe":{"shown":true}}`, "not null", 2},
		{"null signal", initial + `{"signal":null,"observe":{"shown":true}}`, "nonempty string", 2},
		{"exponent", `{"observe":{"shown":3e0}}`, "signed 64-bit integer", 1},
		{"missing signal", initial + `{"observe":{"shown":true}}`, "signal is required", 2},
		{"missing observation", initial + `{"signal":"Example::Open"}`, "at least one feature", 2},
		{"changed projection", initial + `{"signal":"Example::Open","observe":{"other":false}}`, "same feature names", 2},
		{"null", `{"observe":{"shown":null}}`, "signed 64-bit integer", 1},
		{"float", `{"observe":{"shown":3.0}}`, "signed 64-bit integer", 1},
		{"overflow", `{"observe":{"shown":9223372036854775808}}`, "signed 64-bit integer", 1},
		{"lone high in observation", `{"observe":{"shown":"\ud800"}}`, "surrogate pair", 1},
		{"lone low in argument", initial + `{"signal":"Open","args":{"text":"\udfff"},"observe":{"shown":true}}`, "surrogate pair", 2},
		{"unpaired signal", initial + `{"signal":"\ud83d","observe":{"shown":true}}`, "surrogate pair", 2},
		{"unpaired field name", `{"observe":{"\ude00":false}}`, "surrogate pair", 1},
		{"lows before highs", `{"observe":{"shown":"\udc00\udbff"}}`, "surrogate pair", 1},
		{"nonadjacent pair", `{"observe":{"shown":"\udbffx\udc00"}}`, "surrogate pair", 1},
		{"escaped low after high", `{"observe":{"shown":"\ud800\\udc00"}}`, "surrogate pair", 1},
		{"high followed by nonsurrogate", `{"observe":{"shown":"\ud800\ufffd"}}`, "surrogate pair", 1},
		{"high followed by high", `{"observe":{"shown":"\ud800\ud801"}}`, "surrogate pair", 1},
		{"extra JSON", initial + `{"signal":"Example::Open","observe":{"shown":true}}{}`, "expected one JSON object", 2},
		{"blank line", initial + "\n", "blank lines", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readConformTrace(strings.NewReader(tc.data))
			if err == nil {
				t.Fatal("invalid trace accepted")
			}
			report := invalidTrace(err)
			if report.Row != tc.row || !strings.Contains(report.Message, tc.message) {
				t.Fatalf("expected row %d and %q, got %+v", tc.row, tc.message, report)
			}
		})
	}
}

func TestConformTracePreservesArgumentPresence(t *testing.T) {
	const data = `{"observe":{"x":0}}
{"signal":"Next","observe":{"x":1}}
{"signal":"Next","args":{},"observe":{"x":2}}`
	rows, err := readConformTrace(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if rows[1].Args != nil || rows[2].Args == nil || len(rows[2].Args) != 0 {
		t.Fatalf("want absent arguments to stay nil and an explicit empty object to stay present")
	}
}

func TestConformTraceRejectsOversizedEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		size          int
		row           int
	}{
		{"row", "token too long", 1 << 20, 1},
		{"file", "trace exceeds", (8 << 20) + 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readConformTrace(strings.NewReader(strings.Repeat(" ", tc.size)))
			if err == nil {
				t.Fatal("oversized trace accepted")
			}
			report := invalidTrace(err)
			if report.Row != tc.row || !strings.Contains(report.Message, tc.message) {
				t.Fatalf("expected size error at row %d, got %+v", tc.row, report)
			}
		})
	}
}

func TestConformTraceJSONAndExitStatus(t *testing.T) {
	for _, tc := range []struct {
		status string
		exit   int
	}{
		{"matched", exitHolds},
		{"no_match", exitFailed},
		{"inconclusive", exitUnevaluable},
		{"invalid", exitUnevaluable},
	} {
		t.Run(tc.status, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			input := conformResult{Status: tc.status, Row: 3, Moves: 7, Message: "reason"}
			if got := input.write(&stdout, &stderr, true); got != tc.exit {
				t.Fatalf("exit %d, want %d", got, tc.exit)
			}
			if stderr.Len() > 0 {
				t.Fatalf("unexpected stderr: %s", stderr.String())
			}
			var got conformResult
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || got.Status != tc.status || got.Row != 3 || got.Moves != 7 || got.Message != "reason" {
				t.Fatalf("unexpected JSON: %s (%v)", stdout.String(), err)
			}
		})
	}
}
