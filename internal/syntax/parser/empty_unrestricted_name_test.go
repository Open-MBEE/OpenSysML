package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The repetition of UNRESTRICTED_NAME admits no character (KerML §8.2.2.3), so
// two single quotes declare the empty name, which is not the absence of a name.
func TestParseEmptyUnrestrictedNameIsADeclaredName(t *testing.T) {
	for _, tc := range []struct {
		src             string
		name, short     string
		named, shortned bool
	}{
		{"part '';", "", "", true, false},
		{"part <''>;", "", "", false, true},
		{"part <''> '';", "", "", true, true},
		{"part <''> x;", "x", "", true, true},
		{"part '   ';", "   ", "", true, false},
		{"part;", "", "", false, false},
		{"part : Pt;", "", "", false, false},
	} {
		root := parseClean(t, "package P { part def Pt; "+tc.src+" }")
		var id *ast.Identification
		ast.Inspect(root, func(n ast.Node) bool {
			if u, ok := n.(*ast.Usage); ok {
				id = &u.Ident
			}
			return true
		})
		if id == nil {
			t.Fatalf("%s: no usage parsed", tc.src)
		}
		if id.Name != tc.name || id.HasName() != tc.named || id.ShortName != tc.short || id.HasShortName() != tc.shortned {
			t.Errorf("%s: name %q (declared %t), short name %q (declared %t); want %q (%t), %q (%t)", tc.src,
				id.Name, id.HasName(), id.ShortName, id.HasShortName(), tc.name, tc.named, tc.short, tc.shortned)
		}
		if id.Declared() != (tc.named || tc.shortned) {
			t.Errorf("%s: Declared() = %t", tc.src, id.Declared())
		}
	}
}

// A quote that is not closed, or a third one, is malformed input: it is
// diagnosed, and never read as the empty name.
func TestNegativeEmptyUnrestrictedName(t *testing.T) {
	for _, src := range []string{
		"package P { part ' ; }",
		"package P { part '''; }",
		"package P { part <'> x; }",
		"package P { part x :> P::'; }",
		"package P { part ''x; }",
	} {
		p := New(source.New("neg.sysml", []byte(src)))
		if root := p.ParseFile(); root == nil {
			t.Errorf("%s: no tree", src)
		}
		if len(p.Diagnostics) == 0 {
			t.Errorf("%s: parsed without a diagnostic", src)
		}
	}
}
