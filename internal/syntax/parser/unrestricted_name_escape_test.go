package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// An unrestricted name is the characters its escape sequences denote (KerML
// §8.2.2.3), in a declaration and in every reference to it alike.
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
		root := parseClean(t, src)
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
