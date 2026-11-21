package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// An unrestricted name is the characters its escape sequences denote (KerML
// §8.2.2.3), in a declaration and in every reference to it alike; `\r`, the
// pilot's escape beyond Table 4, decodes too and is warned of at each spelling.
func TestParseUnrestrictedNameDecodesItsEscapes(t *testing.T) {
	for _, tc := range []struct{ written, want string }{
		{`'it\'s'`, `it's`},
		{`'a\\b'`, `a\b`},
		{`'q\"d'`, `q"d`},
		{`'bs\b'`, "bs\b"},
		{`'ht\t'`, "ht\t"},
		{`'lf\n'`, "lf\n"},
		{`'ff\f'`, "ff\f"},
		{`'cr\r'`, "cr\r"},
		{`'\\\''`, `\'`},
		{`'plain name'`, "plain name"},
		{`''`, ""},
	} {
		src := "package P { part def " + tc.written + "; part <" + tc.written + "> " + tc.written + " : " + tc.written +
			" { attribute " + tc.written + " = " + tc.written + "." + tc.written + "; } alias " + tc.written + " for P::" + tc.written + "; }"
		file := source.New("test.sysml", []byte(src))
		p := New(file)
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Errorf("%s\nerrors = %v, want none", src, p.Diagnostics)
		}
		if want := strings.Count(src, `\r`); len(p.Warnings) != want {
			t.Errorf("%s\nwarnings = %v, want %d", src, p.Warnings, want)
		}
		for _, w := range p.Warnings {
			if w.Code != codeNonstandardNotation || file.Text(w.Span) != `\r` {
				t.Errorf("%s\nwarning = %v, want nonstandard-notation on `\\r`", src, w)
			}
		}
		var names []string
		ast.Inspect(root, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Definition:
				names = append(names, n.Ident.Name)
			case *ast.Usage:
				if n.Ident.ShortName != "" {
					names = append(names, n.Ident.ShortName)
				}
			case *ast.Alias:
				names = append(names, n.Ident.Name)
			case *ast.QualifiedName:
				names = append(names, n.Parts[len(n.Parts)-1].Text)
			}
			return true
		})
		if len(names) < 6 {
			t.Errorf("%s: read %d names %q, want the definition, short name, type, chain member, alias and target", src, len(names), names)
		}
		for _, name := range names {
			if name != tc.want {
				t.Errorf("%s: a name reads %q, want %q", src, name, tc.want)
			}
		}
	}
}
