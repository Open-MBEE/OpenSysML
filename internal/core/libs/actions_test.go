package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestActionsFile(t *testing.T) {
	src := &embedSource{}
	data, _ := src.Read("Systems Library/Actions.sysml")
	p := parser.New(source.New("Actions.sysml", data))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Logf("Actions.sysml has %d errors:", len(p.Diagnostics))
		for i, d := range p.Diagnostics {
			if i >= 10 {
				break
			}
			t.Logf("  offset=%d: %s", d.Span.Offset, d.Message)
		}
		t.Fail()
	} else {
		t.Log("Actions.sysml PASS")
	}
}
