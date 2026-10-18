package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// FeatureSpecializationPart (KerML.xtext:574, SysML.xtext:366) is
// `FeatureSpecialization+ MultiplicityPart? FeatureSpecialization*`, and a
// declaration that opens with a specialization states no name
// (FeatureDeclaration), so `ordered`/`nonunique` after `:>> values` are the
// MultiplicityPart, recorded as they are when the part precedes the name.
func TestMultiplicityPartAfterPreNameSpecialization(t *testing.T) {
	tests := []struct {
		name      string
		member    string
		ordered   bool
		nonunique bool
		mult      bool
		rels      int
	}{
		{"redefines_ordered_typed", ":>> values ordered : Real;", true, false, false, 2},
		{"redefines_ordered", ":>> values ordered;", true, false, false, 1},
		{"redefines_ordered_nonunique", ":>> values ordered nonunique;", true, true, false, 1},
		{"redefines_nonunique_ordered", ":>> values nonunique ordered;", true, true, false, 1},
		{"redefines_mult_ordered_value", ":>> values [5] ordered = (1, 2, 3, 4, 5);", true, false, true, 1},
		{"keyword_redefines_mult_nonunique_ordered_subsets", "attribute :>> values [1..*] nonunique ordered :> more;", true, true, true, 2},
		{"keyword_redefines_mult_ordered_nonunique_typed", "attribute :>> values [0..*] ordered nonunique : Real;", true, true, true, 2},
		{"subsets_ordered", "attribute :> values ordered;", true, false, false, 1},
		{"references_nonunique", "attribute ::> values nonunique;", false, true, false, 1},
		{"typing_mult_nonunique_ordered_subsets", "attribute : Real [2] nonunique ordered :> values;", true, true, true, 2},
		{"word_redefines_ordered_nonunique_typed", "attribute redefines values ordered nonunique : Real;", true, true, false, 2},
		{"word_subsets_nonunique", "attribute subsets values nonunique;", false, true, false, 1},
	}
	for _, tt := range tests {
		for _, file := range []string{"test.sysml", "test.kerml"} {
			t.Run(tt.name+"/"+file, func(t *testing.T) {
				member, src := inKind(file, tt.member)
				p := New(source.New(file, []byte(src)))
				root := p.ParseFile()
				if len(p.Diagnostics) != 0 || len(p.Warnings) != 0 {
					t.Fatalf("%s\nerrors = %v, warnings = %v, want none", member, p.Diagnostics, p.Warnings)
				}
				u := specializingUsage(root)
				if u.Ident.Name != "" || u.Ident.ShortName != "" {
					t.Errorf("%s\nname = %q <%q>, want none", member, u.Ident.Name, u.Ident.ShortName)
				}
				if u.IsOrdered != tt.ordered || u.IsNonunique != tt.nonunique {
					t.Errorf("%s\nordered = %v nonunique = %v, want %v %v", member, u.IsOrdered, u.IsNonunique, tt.ordered, tt.nonunique)
				}
				if (u.Multiplicity != nil) != tt.mult {
					t.Errorf("%s\nmultiplicity present = %v, want %v", member, u.Multiplicity != nil, tt.mult)
				}
				if len(u.Relationships) != tt.rels {
					t.Errorf("%s\nrelationships = %d, want %d", member, len(u.Relationships), tt.rels)
				}
			})
		}
	}
}

// inKind spells the SysML member for the file's language and encloses it in a
// type: KerML declares the feature with `feature` inside a `classifier`.
func inKind(file, member string) (string, string) {
	if file == "test.kerml" {
		if member[0] != ':' {
			member = "feature" + member[len("attribute"):]
		}
		return member, "package P { classifier S { " + member + " } }"
	}
	return member, "package P { part def S { " + member + " } }"
}

// specializingUsage returns the first usage in root that states a specialization.
func specializingUsage(root ast.Node) *ast.Usage {
	var found *ast.Usage
	ast.Inspect(root, func(n ast.Node) bool {
		if u, ok := n.(*ast.Usage); ok && found == nil && len(u.Relationships) > 0 {
			found = u
			return false
		}
		return true
	})
	return found
}

// A name cannot follow a specialization part, and the part's `ordered` and
// `nonunique` are stated once each, after the bracketed multiplicity.
func TestMultiplicityPartAfterPreNameSpecializationMalformed(t *testing.T) {
	for _, member := range []string{
		":>> values ordered x;",
		":>> values <s> ordered;",
		":>> values ordered ordered;",
		":>> values nonunique nonunique;",
		":>> values ordered [2];",
	} {
		for _, file := range []string{"test.sysml", "test.kerml"} {
			_, src := inKind(file, member)
			p := New(source.New(file, []byte(src)))
			p.ParseFile()
			if len(p.Diagnostics) == 0 {
				t.Errorf("%s in %s: expected a diagnostic", member, file)
			}
		}
	}
}
