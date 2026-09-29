package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func TestSelfScopeReferenceRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "owning definition type",
			src:  `package P { part def A { part x : A; } }`,
			want: "part x : A;",
		},
		{
			name: "satisfy subject",
			src:  `package P { requirement def R; part s { satisfy requirement r : R by s; } }`,
			want: "satisfy requirement r : R by s;",
		},
		{
			name: "assert satisfy subject",
			src:  `package P { requirement def R; part s { assert satisfy requirement r : R by s; } }`,
			want: "assert satisfy requirement r : R by s;",
		},
		{
			name: "subsets enclosing usage",
			src:  `package P { part a { part b :> a; } }`,
			want: "part b subsets a;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkSysML(t, tt.name+".input.sysml", []byte(tt.src))
			back := mappingAloneRoundTrip(t, tt.name+".sysml", []byte(tt.src))
			checkSysML(t, tt.name+".decoded.sysml", []byte(back))
			if !strings.Contains(back, tt.want) {
				t.Errorf("structural round trip lost %q:\n%s", tt.want, back)
			}
		})
	}
}

func checkSysML(t *testing.T, name string, notation []byte) {
	t.Helper()
	file := source.New(name, notation)
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("%s does not parse: %v\n%s", name, p.Diagnostics, notation)
	}
	index := libs.NewModelIndex()
	index.AddDocument(file.Name(), root)
	if diagnostics := passes.Analyze(file.Name(), root, nil, index); len(diagnostics) != 0 {
		t.Fatalf("%s does not check cleanly: %v\n%s", name, diagnostics, notation)
	}
}
