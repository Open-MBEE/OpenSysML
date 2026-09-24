package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestActionsErrorSummary(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Systems Library/Actions.sysml")
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	p := parser.New(source.New("Actions.sysml", data))
	_ = p.ParseFile()

	t.Logf("Actions.sysml: %d diagnostics\n", len(p.Diagnostics))

	code := string(data)

	// Group errors by unique offset
	offsetMap := make(map[int][]string)
	for _, d := range p.Diagnostics {
		offsetMap[d.Span.Offset] = append(offsetMap[d.Span.Offset], d.Message)
	}

	// Print unique error locations with context
	i := 1
	for offset, messages := range offsetMap {
		t.Logf("\n=== Error Group %d (offset %d) ===", i, offset)
		for _, msg := range messages {
			t.Logf("  - %s", msg)
		}

		// Show context (60 chars before and after)
		start := offset - 60
		if start < 0 {
			start = 0
		}
		end := offset + 60
		if end > len(code) {
			end = len(code)
		}

		before := code[start:offset]
		after := code[offset:end]
		if offset < len(code) {
			t.Logf("  Context: ...%s>>>%c<<<%s...", before, code[offset], after)
		}
		i++
	}
}
