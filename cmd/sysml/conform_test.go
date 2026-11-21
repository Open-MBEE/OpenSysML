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

func TestConformTraceRejectsMalformedEvidence(t *testing.T) {
	const initial = `{"observe":{"shown":false}}` + "\n"
	for _, tc := range []struct {
		name, data, message string
		row                 int
	}{
		{"empty", "", "initial observation", 0},
		{"initial signal", `{"signal":"","observe":{"shown":false}}`, "initial observation must omit signal", 1},
		{"initial args", `{"args":{},"observe":{"shown":false}}`, "initial observation must omit args", 1},
		{"duplicate member", `{"observe":{"shown":false},"observe":{"shown":true}}`, "duplicate field", 1},
		{"duplicate observable", `{"observe":{"shown":false,"shown":true}}`, "duplicate field", 1},
		{"escaped duplicate", `{"observe":{"shown":false,"sh\u006fwn":true}}`, "duplicate field", 1},
		{"unknown field", `{"extra":true,"observe":{"shown":false}}`, "unknown field", 1},
		{"missing signal", initial + `{"observe":{"shown":true}}`, "signal is required", 2},
		{"missing observation", initial + `{"signal":"Example::Open"}`, "at least one feature", 2},
		{"changed projection", initial + `{"signal":"Example::Open","observe":{"other":false}}`, "same feature names", 2},
		{"null", `{"observe":{"shown":null}}`, "signed 64-bit integer", 1},
		{"float", `{"observe":{"shown":3.0}}`, "signed 64-bit integer", 1},
		{"overflow", `{"observe":{"shown":9223372036854775808}}`, "signed 64-bit integer", 1},
		{"extra JSON", initial + `{"signal":"Example::Open","observe":{"shown":true}}{}`, "one JSON object per line", 2},
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
