package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Strict mode must not reject the bundled library, including OpenSysML's
// extensions, which use only standard SysML or KerML notation.
func TestStdlibIsConformingUnderStrictMode(t *testing.T) {
	src := &embedSource{}
	paths := src.List()
	if len(paths) == 0 {
		t.Fatal("no embedded library files")
	}
	for _, path := range paths {
		data, err := src.Read(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		p := parser.New(source.New(path, data))
		root := p.ParseFile()
		ctx := passes.NewContextWithOptions(path, source.KindOf(path),
			symbols.NewIndexFromDoc(path, root), nil,
			passes.Options{Conformance: diag.ConformanceStrict})
		for _, d := range (passes.NonstandardNotationPass{}).Run(ctx, path, root) {
			t.Errorf("%s: %s: %s", path, d.Code, d.Message)
		}
	}
}
