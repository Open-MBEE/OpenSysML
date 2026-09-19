package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The OMG library is written in standard notation only, so it must draw no
// extension or KerML-in-SysML warning: one here would mean the classification in
// docs/reference/grammar/conformance-audit.md is wrong, not that the library
// needs an exception.
func TestStdlibHasNoNonstandardNotation(t *testing.T) {
	src := &embedSource{}
	for _, path := range src.List() {
		data, err := src.Read(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		p := parser.New(source.New(path, data))
		root := p.ParseFile()
		ctx := passes.NewContext(path, symbols.NewIndexFromDoc(path, root), nil)
		for _, d := range (passes.NonstandardNotationPass{}).Run(ctx, path, root) {
			t.Errorf("%s: %s: %s", path, d.Code, d.Message)
		}
	}
}
