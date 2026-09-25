package libs

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestStdlibConformance is the gating conformance test per PARSER_ROBUSTNESS_PLAN.md Phase 1.
// It ensures:
// 1. Files NOT in the allowlist must parse with zero diagnostics (no regressions)
// 2. Files IN the allowlist that now parse clean trigger a failure (allowlist is stale)
func TestStdlibConformance(t *testing.T) {
	allowlistPath := filepath.Join("testdata", "stdlib_known_failures.txt")
	allowlist, err := loadAllowlist(allowlistPath)
	if err != nil {
		t.Fatalf("Failed to load allowlist %s: %v", allowlistPath, err)
	}

	src := &embedSource{}
	files := src.List()

	var regressions []string    // files not in allowlist that now fail
	var staleAllowlist []string // files in allowlist that now pass

	for _, path := range files {
		data, err := src.Read(path)
		if err != nil {
			t.Logf("SKIP %s: read error: %v", path, err)
			continue
		}

		sf := source.New(path, data)
		p := parser.New(sf)
		_ = p.ParseFile()

		hasDiagnostics := len(p.Diagnostics) > 0
		inAllowlist := allowlist[path]

		if hasDiagnostics && !inAllowlist {
			// Regression: file not in allowlist but has diagnostics
			regressions = append(regressions, path)
			for _, d := range p.Diagnostics {
				t.Logf("  REGRESSION %s: %s", path, d.Message)
			}
		} else if !hasDiagnostics && inAllowlist {
			// Stale allowlist: file in allowlist but now parses clean
			staleAllowlist = append(staleAllowlist, path)
		}
	}

	if len(regressions) > 0 {
		t.Errorf("REGRESSIONS: %d files not in allowlist have parse errors:", len(regressions))
		for _, f := range regressions {
			t.Errorf("  - %s", f)
		}
	}

	if len(staleAllowlist) > 0 {
		t.Errorf("STALE ALLOWLIST: %d files in allowlist now parse clean (remove from allowlist):", len(staleAllowlist))
		for _, f := range staleAllowlist {
			t.Errorf("  - %s", f)
		}
	}

	// Report stats
	failCount := len(allowlist)
	passCount := len(files) - failCount - len(regressions)
	t.Logf("\nStdlib conformance: %d/%d clean, %d in allowlist", passCount, len(files), failCount)
}

// loadAllowlist reads the allowlist file, returns map[path]true
// Lines starting with # are comments, blank lines ignored
func loadAllowlist(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		// Allowlist file not existing yet is acceptable (empty allowlist)
		if os.IsNotExist(err) {
			return make(map[string]bool), nil
		}
		return nil, err
	}
	defer f.Close()

	allowlist := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		allowlist[line] = true
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return allowlist, nil
}
