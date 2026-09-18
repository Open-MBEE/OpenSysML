package main

import (
	"slices"
	"testing"
)

func TestParseCapabilities(t *testing.T) {
	got, err := parseCapabilities("strict_conformance, oslc_query")
	if err != nil {
		t.Fatalf("parseCapabilities: %v", err)
	}
	want := []string{"strict_conformance", "oslc_query"}
	if !slices.Equal(got, want) {
		t.Errorf("capabilities = %v, want %v", got, want)
	}
	if _, err := parseCapabilities("query,query"); err == nil {
		t.Error("duplicate capability was accepted")
	}
}

func TestValidateWithheldCapabilities(t *testing.T) {
	defaults := []string{"oslc_query", "query", "strict_conformance"}
	actual := []string{"query"}
	if err := validateWithheldCapabilities(defaults, actual,
		[]string{"oslc_query", "strict_conformance"}); err != nil {
		t.Errorf("valid withheld set: %v", err)
	}
	if err := validateWithheldCapabilities(defaults, defaults,
		[]string{"strict_conformance"}); err == nil {
		t.Error("service that retained a withheld capability was accepted")
	}
	if err := validateWithheldCapabilities(defaults, defaults,
		[]string{"unknown"}); err == nil {
		t.Error("capability absent from the default service was accepted")
	}
}

func TestValidateFallbackExecution(t *testing.T) {
	summary := &Summary{Results: []*Result{
		{
			ID:                  "strict",
			Outcome:             "pass",
			WithoutCapability:   true,
			MissingCapabilities: []string{"strict_conformance"},
		},
		{
			ID:                  "oslc",
			Outcome:             "pass",
			WithoutCapability:   true,
			MissingCapabilities: []string{"oslc_query"},
		},
		{
			ID:                  "skipped",
			Outcome:             "skip",
			MissingCapabilities: []string{"oslc_query"},
		},
	}}
	unavailable := []string{"strict_conformance", "oslc_query"}
	if err := validateFallbackExecution(summary, unavailable, false); err != nil {
		t.Errorf("executed fallbacks: %v", err)
	}
	if err := validateFallbackExecution(summary, append(unavailable, "query"), false); err == nil {
		t.Error("unexecuted fallback was accepted")
	}

	summary.Results[2].MissingCapabilities = []string{"verification"}
	if err := validateFallbackExecution(summary, unavailable, false); err == nil {
		t.Error("unexpected missing capability was accepted")
	}
}
