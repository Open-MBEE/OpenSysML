package libs

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestStdlibParserCoverage analyzes which stdlib files parse cleanly
// and identifies missing parser features
func TestStdlibParserCoverage(t *testing.T) {
	src := &embedSource{}
	files := src.List()

	parsed := 0
	failed := 0
	failures := make(map[string][]string) // file -> diagnostic messages

	for _, name := range files {
		data, err := src.Read(name)
		if err != nil {
			t.Fatalf("Read(%q): %v", name, err)
		}

		p := parser.New(source.New(name, data))
		_ = p.ParseFile()

		if len(p.Diagnostics) > 0 {
			failed++
			var msgs []string
			for _, d := range p.Diagnostics {
				msgs = append(msgs, d.Message)
			}
			failures[name] = msgs
		} else {
			parsed++
		}
	}

	t.Logf("\n=== Stdlib Parser Coverage ===")
	t.Logf("Total files: %d", len(files))
	t.Logf("Parsed cleanly: %d (%.1f%%)", parsed, float64(parsed)*100/float64(len(files)))
	t.Logf("Failed: %d (%.1f%%)", failed, float64(failed)*100/float64(len(files)))

	if failed > 0 {
		t.Logf("\n=== Parse Failures ===")

		// Group by error patterns
		errorPatterns := make(map[string]int)
		for _, msgs := range failures {
			for _, msg := range msgs {
				// Extract pattern (first 60 chars)
				pattern := msg
				if len(pattern) > 60 {
					pattern = pattern[:60] + "..."
				}
				errorPatterns[pattern]++
			}
		}

		t.Logf("\nTop error patterns:")
		type kv struct {
			pattern string
			count   int
		}
		var sorted []kv
		for k, v := range errorPatterns {
			sorted = append(sorted, kv{k, v})
		}
		// Simple bubble sort by count
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j].count > sorted[i].count {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}

		for i, kv := range sorted {
			if i >= 10 {
				break
			}
			t.Logf("  %3d: %s", kv.count, kv.pattern)
		}

		// Show sample failures
		t.Logf("\nSample failures (first 5):")
		count := 0
		for name, msgs := range failures {
			if count >= 5 {
				break
			}
			t.Logf("\n%s (%d diagnostics):", name, len(msgs))
			for i, msg := range msgs {
				if i >= 3 {
					t.Logf("  ... and %d more", len(msgs)-3)
					break
				}
				// Truncate long messages
				if len(msg) > 100 {
					msg = msg[:100] + "..."
				}
				t.Logf("  - %s", msg)
			}
			count++
		}
	}
}

// TestStdlibFileCategories groups files by parsing status for planning
func TestStdlibFileCategories(t *testing.T) {
	src := &embedSource{}
	files := src.List()

	// Categorize by directory
	categories := make(map[string][]string) // category -> files
	parseStatus := make(map[string]bool)    // file -> cleanly parsed

	for _, name := range files {
		data, err := src.Read(name)
		if err != nil {
			t.Fatalf("Read(%q): %v", name, err)
		}

		p := parser.New(source.New(name, data))
		_ = p.ParseFile()

		cleanParse := len(p.Diagnostics) == 0
		parseStatus[name] = cleanParse

		// Extract category from path
		category := "Root"
		if strings.Contains(name, "/") {
			parts := strings.Split(name, "/")
			category = parts[0]
		}

		categories[category] = append(categories[category], name)
	}

	t.Logf("\n=== Stdlib by Category ===")
	for cat, fileList := range categories {
		parsed := 0
		for _, f := range fileList {
			if parseStatus[f] {
				parsed++
			}
		}
		t.Logf("\n%s: %d files (%d parsed, %d failed)",
			cat, len(fileList), parsed, len(fileList)-parsed)

		if parsed > 0 {
			t.Logf("  Parsed:")
			for _, f := range fileList {
				if parseStatus[f] {
					t.Logf("    ✓ %s", f)
				}
			}
		}

		failed := len(fileList) - parsed
		if failed > 0 && failed <= 10 {
			t.Logf("  Failed:")
			for _, f := range fileList {
				if !parseStatus[f] {
					t.Logf("    ✗ %s", f)
				}
			}
		} else if failed > 10 {
			t.Logf("  Failed: %d files (too many to list)", failed)
		}
	}
}
