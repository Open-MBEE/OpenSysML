package lexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func tokenizeFile(t *testing.T, path string) (*source.SourceFile, []Token) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sf := source.New(filepath.Base(path), data)
	lx := New(sf)
	var toks []Token
	for {
		tk := lx.Next()
		toks = append(toks, tk)
		if tk.Kind == EOF {
			break
		}
	}
	return sf, toks
}

func TestFixturesNoErrors(t *testing.T) {
	for _, f := range []string{"../../../tests/testdata/lex/basic.sysml", "../../../tests/testdata/lex/basic.kerml"} {
		_, toks := tokenizeFile(t, f)
		for _, tk := range toks {
			if tk.Kind == Error {
				t.Errorf("%s: unexpected Error token at offset %d", f, tk.Span.Offset)
			}
		}
	}
}

func TestFixturesRoundTrip(t *testing.T) {
	for _, f := range []string{"../../../tests/testdata/lex/basic.sysml", "../../../tests/testdata/lex/basic.kerml"} {
		sf, toks := tokenizeFile(t, f)
		var rebuilt []byte
		for _, tk := range toks {
			if tk.Kind == EOF {
				continue
			}
			rebuilt = append(rebuilt, sf.Bytes()[tk.Span.Offset:tk.Span.End()]...)
		}
		if string(rebuilt) != string(sf.Bytes()) {
			t.Errorf("%s: round-trip mismatch", f)
		}
	}
}
