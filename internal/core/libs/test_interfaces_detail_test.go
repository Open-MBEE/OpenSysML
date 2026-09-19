package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestSingleFile_Interfaces_Details(t *testing.T) {
	src := &embedSource{}
	data, err := src.Read("Systems Library/Interfaces.sysml")
	if err != nil {
		t.Fatal(err)
	}

	sf := source.New("Interfaces.sysml", data)
	p := parser.New(sf)
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Logf("Parse diagnostics (%d):", len(p.Diagnostics))
		for i, d := range p.Diagnostics {
			text := sf.Text(d.Span)
			if len(text) > 50 {
				text = text[:50] + "..."
			}
			t.Logf("  [%d] offset %d: %s [near: %q]", i+1, d.Span.Offset, d.Message, text)
		}
	} else {
		t.Log("Parsed cleanly!")
	}
}
