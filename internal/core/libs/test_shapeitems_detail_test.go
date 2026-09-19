package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestShapeItemsDetail(t *testing.T) {
	src := &embedSource{}
	content, err := src.Read("Domain Libraries/Geometry/ShapeItems.sysml")
	if err != nil {
		t.Fatalf("Failed to load ShapeItems.sysml: %v", err)
	}

	p := parser.New(source.New("ShapeItems.sysml", content))
	_ = p.ParseFile()

	t.Logf("ShapeItems.sysml: %d diagnostics", len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		byteOffset := d.Span.Offset
		char := ""
		if byteOffset < len(content) {
			char = string(content[byteOffset])
		}

		// Show context around error
		start := byteOffset - 40
		if start < 0 {
			start = 0
		}
		end := byteOffset + 40
		if end > len(content) {
			end = len(content)
		}
		context := string(content[start:end])

		t.Logf("  - offset=%d (char=%q): %s", byteOffset, char, d.Message)
		t.Logf("    context: %q", context)
	}
}
