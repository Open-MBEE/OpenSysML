package libs

import (
	"sort"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestStdlibErrorsByFile(t *testing.T) {
	src := &embedSource{}
	files := src.List()

	type fileError struct {
		name     string
		count    int
		messages []string
	}

	var failures []fileError

	for _, name := range files {
		data, err := src.Read(name)
		if err != nil {
			t.Fatalf("Read(%q): %v", name, err)
		}

		p := parser.New(source.New(name, data))
		_ = p.ParseFile()

		if len(p.Diagnostics) > 0 {
			var msgs []string
			for _, d := range p.Diagnostics {
				msgs = append(msgs, d.Message)
			}
			failures = append(failures, fileError{name: name, count: len(p.Diagnostics), messages: msgs})
		}
	}

	// Sort by error count descending
	sort.Slice(failures, func(i, j int) bool {
		return failures[i].count > failures[j].count
	})

	t.Logf("Files with errors (sorted by count):")
	for i, f := range failures {
		t.Logf("%2d. %s: %d errors", i+1, f.name, f.count)
		if i < 5 {
			// Show first 3 unique error messages for top 5 files
			seen := make(map[string]bool)
			count := 0
			for _, msg := range f.messages {
				if !seen[msg] && count < 3 {
					t.Logf("    - %s", msg)
					seen[msg] = true
					count++
				}
			}
		}
	}
}
