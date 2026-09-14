// Package fixtures enumerates conformance cases the way the gates run them, so
// the tests and the generated documentation figures share one definition.
package fixtures

import (
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
)

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
