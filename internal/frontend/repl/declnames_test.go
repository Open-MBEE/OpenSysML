package repl

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func parseRoot(src string) *ast.RootNamespace {
	return parser.New(source.New("<t>", []byte(src))).ParseFile()
}

func TestDeclaredNames(t *testing.T) {
	cases := []struct {
		src  string
		want []string
	}{
		{"package P { }", []string{"P"}},
		{"namespace N;", []string{"N"}},
		{"alias A for P::Q;", []string{"A"}},
		{"import P::Q;", nil}, // imports declare nothing
		{"package P {} namespace N;", []string{"P", "N"}},
		{"", nil},
	}
	for _, c := range cases {
		got := declaredNames(parseRoot(c.src))
		if !equalStrings(got, c.want) {
			t.Errorf("declaredNames(%q) = %v, want %v", c.src, got, c.want)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
