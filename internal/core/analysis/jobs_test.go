package analysis

import (
	"errors"
	"testing"
)

// OPENSYSML_JOBS unset or blank leaves the jobs at one per CPU; a positive
// integer sets them; anything else is a JobsError naming the variable.
func TestJobsFromEnv(t *testing.T) {
	for _, raw := range []string{"", "  "} {
		got, err := jobsFromLookup(func(string) string { return raw })
		if err != nil || got != DefaultJobs() {
			t.Errorf("OPENSYSML_JOBS=%q: jobs %d, %v; want %d", raw, got, err, DefaultJobs())
		}
	}
	got, err := jobsFromLookup(func(name string) string {
		if name != JobsEnvVar {
			t.Fatalf("looked up %s", name)
		}
		return " 8 "
	})
	if err != nil || got != 8 {
		t.Errorf("OPENSYSML_JOBS=8: jobs %d, %v; want 8", got, err)
	}
	for _, raw := range []string{"0", "-2", "x", "1.5", "3 4"} {
		_, err := jobsFromLookup(func(string) string { return raw })
		var typed *JobsError
		if !errors.As(err, &typed) || !errors.Is(err, ErrJobs) {
			t.Errorf("OPENSYSML_JOBS=%q: %v, want a JobsError", raw, err)
			continue
		}
		if typed.Source != JobsEnvVar || typed.Value != raw {
			t.Errorf("OPENSYSML_JOBS=%q reported as %s=%q", raw, typed.Source, typed.Value)
		}
	}
}

// ParseJobs names its source in the error, so a flag and the variable are told apart.
func TestParseJobsNamesItsSource(t *testing.T) {
	if n, err := ParseJobs("-jobs", "4"); err != nil || n != 4 {
		t.Fatalf("ParseJobs(4) = %d, %v", n, err)
	}
	_, err := ParseJobs("-jobs", "none")
	var typed *JobsError
	if !errors.As(err, &typed) || typed.Source != "-jobs" {
		t.Fatalf("ParseJobs(none) = %v, want a JobsError from -jobs", err)
	}
	if want := `-jobs="none" is not a positive integer`; len(err.Error()) < len(want) || err.Error()[:len(want)] != want {
		t.Errorf("message %q does not open with %q", err.Error(), want)
	}
}
