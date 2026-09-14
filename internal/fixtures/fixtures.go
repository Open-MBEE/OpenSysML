// Package fixtures enumerates conformance cases the way the gates run them, so
// the tests and the generated documentation figures share one definition.
package fixtures

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	expectedSuffix      = ".expected.json"
	checkExpectedSuffix = ".check.expected.json"
	knownFailuresFile   = "known_failures.txt"
	traceGoldenSuffix   = ".trace.golden"
)

// SweepPolicies are the non-default schedule policies the whole suite runs
// under, spelled as the runtime's ParseSchedulePolicy reads them.
var SweepPolicies = []string{"declared", "seed:1"}

// PolicyFileTag spells a policy as a file-name segment: `seed:1` becomes `seed-1`.
func PolicyFileTag(spelling string) string {
	return strings.ReplaceAll(spelling, ":", "-")
}

// IsCase tells a case's `.expected.json` from the `.check.expected.json` beside
// it, which states what a check of every schedule finds.
func IsCase(fileName string) bool {
	return strings.HasSuffix(fileName, expectedSuffix) && !strings.HasSuffix(fileName, checkExpectedSuffix)
}

// CaseName returns the case whose expectation fileName is, if it is one.
func CaseName(fileName string) (string, bool) {
	if !IsCase(fileName) {
		return "", false
	}
	return strings.TrimSuffix(fileName, expectedSuffix), true
}

// Cases lists the conformance case names under dir in name order.
func Cases(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var cases []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if name, ok := CaseName(entry.Name()); ok {
			cases = append(cases, name)
		}
	}
	sort.Strings(cases)
	return cases, nil
}

// KnownFailures reads dir's known_failures.txt — one case name per line, `#`
// starting a comment — and returns the set; an absent file is an empty set.
func KnownFailures(dir string) (map[string]bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, knownFailuresFile)) // #nosec G304 -- a fixture directory the caller names
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", knownFailuresFile, err)
	}
	failures := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		failures[line] = true
	}
	return failures, nil
}

// Trace is a golden trace the trace harness reads: a case's own under the
// default schedule (Policy "") or one pinning it under a sweep policy.
type Trace struct {
	Case   string
	Policy string
}

// traceExpectation is the part of a case's expectation that decides which
// traces the harness schedules: the opt-in and whether it admits several outcomes.
type traceExpectation struct {
	Trace    bool              `json:"trace"`
	Outcomes []json.RawMessage `json:"outcomes"`
}

// Traces lists the `.trace.golden` files under dir the trace harness reads, in
// name order. A known failure's are passed over, as the harness skips the case;
// one no case owns, one under no sweep policy, or a policy golden the harness
// would not schedule (the case admits one outcome or owns no default golden) is
// an error, since no gate would read it.
func Traces(dir string) ([]Trace, error) {
	cases, err := Cases(dir)
	if err != nil {
		return nil, err
	}
	knownFailures, err := KnownFailures(dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var traces []Trace
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), traceGoldenSuffix) {
			continue
		}
		trace, err := traceOf(strings.TrimSuffix(entry.Name(), traceGoldenSuffix), cases)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if knownFailures[trace.Case] {
			continue
		}
		if trace.Policy != "" {
			if err := scheduledUnderPolicies(dir, trace.Case); err != nil {
				return nil, fmt.Errorf("%s: %w", entry.Name(), err)
			}
		}
		traces = append(traces, trace)
	}
	return traces, nil
}

// traceOf resolves a golden's stem to the case and sweep policy it records.
func traceOf(stem string, cases []string) (Trace, error) {
	if containsCase(cases, stem) {
		return Trace{Case: stem}, nil
	}
	base, tag, ok := strings.Cut(stem, ".")
	if !ok || !containsCase(cases, base) {
		return Trace{}, fmt.Errorf("belongs to no conformance case")
	}
	for _, policy := range SweepPolicies {
		if PolicyFileTag(policy) == tag {
			return Trace{Case: base, Policy: policy}, nil
		}
	}
	return Trace{}, fmt.Errorf("pins %s under %q, which is no sweep policy", base, tag)
}

// scheduledUnderPolicies checks that the harness runs the case under the sweep
// policies: it must own a default golden and admit several outcomes.
func scheduledUnderPolicies(dir, name string) error {
	data, err := os.ReadFile(filepath.Join(dir, name+expectedSuffix)) // #nosec G304 -- a fixture directory the caller names
	if err != nil {
		return err
	}
	var expectation traceExpectation
	if err := json.Unmarshal(data, &expectation); err != nil {
		return fmt.Errorf("%s%s: %w", name, expectedSuffix, err)
	}
	if len(expectation.Outcomes) == 0 {
		return fmt.Errorf("%s admits one outcome, so the harness records it under no sweep policy", name)
	}
	if _, err := os.Stat(filepath.Join(dir, name+traceGoldenSuffix)); err != nil && !expectation.Trace {
		return fmt.Errorf("%s owns no default golden, so the harness records it under no sweep policy", name)
	}
	return nil
}

func containsCase(sorted []string, name string) bool {
	i := sort.SearchStrings(sorted, name)
	return i < len(sorted) && sorted[i] == name
}
