package diff

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// diagnostic is one finding from either implementation, kept with its message so
// the human-readable report can explain a bucket. Only key() is ever compared.
type diagnostic struct {
	File     string
	Line     int
	Severity string
	Category Category
	Message  string
}

// key is the comparable tuple: message wording differs between the two
// implementations by design, so it is deliberately excluded.
type key struct {
	Line     int
	Severity string
	Category Category
}

func (d diagnostic) key() key {
	return key{Line: d.Line, Severity: d.Severity, Category: d.Category}
}

// openSysMLDiagnostics opens the whole batch in one workspace before reading any
// diagnostics — the corpus imports across files, so per-file runs would measure
// the traversal order instead of the implementation — and returns the
// diagnostics per file.
func openSysMLDiagnostics(repo, dir string, files []string) (map[string][]diagnostic, error) {
	ws := model.NewWorkspace()
	contents := make(map[string][]byte, len(files))
	for _, rel := range files {
		// #nosec G304 -- the corpus root to compare is named on the command line.
		content, err := os.ReadFile(filepath.Join(repo, dir, rel))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		contents[rel] = content
		ws.Open(rel, content, 1)
	}

	out := make(map[string][]diagnostic, len(files))
	for _, rel := range files {
		lines := source.New(rel, contents[rel]).Lines()
		for _, d := range ws.Diagnostics(rel) {
			out[rel] = append(out[rel], diagnostic{
				File:     rel,
				Line:     lines.PosAt(d.Span.Offset).Line,
				Severity: d.Severity.String(),
				Category: categorizeOpenSysML(d.Code, d.Source, d.Message),
				Message:  d.Message,
			})
		}
	}
	return out, nil
}
