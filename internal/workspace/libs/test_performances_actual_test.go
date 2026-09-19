package libs

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestPerformancesActualPattern(t *testing.T) {
	src := &embedSource{}
	content, err := src.Read("Kernel Libraries/Kernel Semantic Library/Performances.kerml")
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	p := parser.New(source.New("Performances.kerml", content))
	_ = p.ParseFile()

	t.Logf("Performances.kerml diagnostics: %d", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		// Get line number
		lineNum := 1
		for i := 0; i < d.Span.Offset && i < len(content); i++ {
			if content[i] == '\n' {
				lineNum++
			}
		}

		char := ""
		if d.Span.Offset < len(content) {
			char = string(content[d.Span.Offset])
		}

		// Get line content
		lines := strings.Split(string(content), "\n")
		lineContent := ""
		if lineNum > 0 && lineNum <= len(lines) {
			lineContent = lines[lineNum-1]
			if len(lineContent) > 80 {
				lineContent = lineContent[:80] + "..."
			}
		}

		t.Logf("Line %d, offset=%d (char=%q): %s", lineNum, d.Span.Offset, char, d.Message)
		t.Logf("  Content: %s", lineContent)
	}
}
