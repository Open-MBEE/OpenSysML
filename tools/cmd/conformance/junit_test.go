package main

import (
	"encoding/xml"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/junit"
)

func TestJUnitReport(t *testing.T) {
	report := &Report{
		Configurations: []*ConfigurationSummary{
			{
				Name: "default",
				Protocols: []*Summary{
					{
						Protocol: "grpc",
						Results: []*Result{
							{ID: "parse-01", Outcome: "pass", DurationMS: 12},
							{ID: "verify-02", Outcome: "fail", Reason: "response mismatch",
								Failures: []string{"status = INVALID_ARGUMENT, want OK"}},
							{ID: "query-03", Outcome: "skip", Reason: "missing capability query"},
							{ID: "bad-04", Outcome: "error", Reason: "unknown RPC"},
						},
					},
					{Protocol: "connect", Results: []*Result{{ID: "parse-01", Outcome: "pass"}}},
				},
			},
			{
				Name: "without-query",
				Protocols: []*Summary{
					{Protocol: "grpc", Results: []*Result{{ID: "parse-01", Outcome: "pass"}}},
				},
			},
		},
	}

	doc := junitReport(report)
	if got, want := len(doc.Suites), 3; got != want {
		t.Fatalf("suites = %d, want %d", got, want)
	}
	if doc.Tests != 6 || doc.Failures != 1 || doc.Errors != 1 || doc.Skipped != 1 {
		t.Fatalf("counts = %d/%d/%d/%d, want 6/1/1/1", doc.Tests, doc.Failures, doc.Errors, doc.Skipped)
	}
	if doc.Suites[0].Name != "default/grpc" || doc.Suites[2].Name != "without-query/grpc" {
		t.Fatalf("suite names = %q, %q", doc.Suites[0].Name, doc.Suites[2].Name)
	}

	cases := doc.Suites[0].Cases
	if cases[0].Time != 0.012 {
		t.Fatalf("pass case time = %v, want 0.012", cases[0].Time)
	}
	if cases[1].Failure == nil || cases[1].Failure.Text != "response mismatch" ||
		cases[1].Failure.Body != "status = INVALID_ARGUMENT, want OK" {
		t.Fatalf("fail case = %+v", cases[1])
	}
	if cases[2].Skipped == nil || cases[2].Skipped.Text != "missing capability query" {
		t.Fatalf("skip case = %+v", cases[2])
	}
	if cases[3].Error == nil || cases[3].Error.Text != "unknown RPC" {
		t.Fatalf("error case = %+v", cases[3])
	}

	data, err := junit.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var decoded junit.Testsuites
	if err := xml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output does not parse back: %v", err)
	}
}
