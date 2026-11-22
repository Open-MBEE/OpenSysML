package hygiene

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// requireComparedToValue matches a require variable read and compared with one spelling.
var requireComparedToValue = regexp.MustCompile(`os\.Getenv\([^)]*(?i:requir)[^)]*\)\s*[!=]=\s*"[^"]`)

// TestRequireVariablesAreSetOrUnset keeps one rule for every require variable: any
// non-empty value requires. A gate that compares with "1" skips under =true.
func TestRequireVariablesAreSetOrUnset(t *testing.T) {
	for _, path := range trackedFiles(t) {
		if !strings.HasSuffix(path, ".go") || path == "tests/hygiene/require_variables_test.go" {
			continue
		}
		source, err := os.ReadFile(filepath.Join(repoRoot, path))
		if err != nil {
			t.Fatal(err)
		}
		for n, line := range strings.Split(string(source), "\n") {
			if requireComparedToValue.MatchString(line) {
				t.Errorf("%s:%d compares a require variable with one value; test it against \"\" so any setting requires:\n\t%s",
					path, n+1, strings.TrimSpace(line))
			}
		}
	}
}

func TestRequireComparedToValuePattern(t *testing.T) {
	for line, want := range map[string]bool{
		`if os.Getenv(requireEnv) == "1" {`:                        true,
		`if os.Getenv("OPENSYSML_REQUIRE_SMT") != "1" {`:           true,
		`if os.Getenv("OPENSYSML_REQUIRE_SMT")=="true" {`:          true,
		`if os.Getenv("OPENSYSML_REQUIRE_SMT") != "" {`:            false,
		`if os.Getenv(root.requireEnv) == "" {`:                    false,
		`if os.Getenv("OPENSYSML_CORPUS_SUBSET") == "construct" {`: false,
	} {
		if got := requireComparedToValue.MatchString(line); got != want {
			t.Errorf("match(%s) = %v, want %v", line, got, want)
		}
	}
}
