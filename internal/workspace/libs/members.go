package libs

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TopMember is what a document's top-level member is, as its notation states
// it, which is what a session lists of a file it loaded whether the file is
// held loaded or as its record. Kind is the keyword the member is written with
// ("package", "part def", "import", "comment"), empty for a member the notation
// has no keyword of its own for.
type TopMember struct {
	Kind      string
	Name      string
	ShortName string
	// Named and ShortNamed tell a declared empty name (`''`) from none.
	Named, ShortNamed bool
	// Target is what an import names, wildcards included.
	Target string
	Span   source.Span
}

// TopMembers reports root's top-level members in declaration order; nil for a
// nil root.
func TopMembers(root *ast.RootNamespace) []TopMember {
	if root == nil {
		return nil
	}
	out := make([]TopMember, 0, len(root.Members))
	for _, m := range root.Members {
		out = append(out, TopMemberOf(m))
	}
	return out
}

// TopMemberOf is TopMembers over one top-level member.
func TopMemberOf(m ast.Node) TopMember {
	node := m
	if mem, ok := m.(*ast.Membership); ok {
		node = mem.Member
	}
	tm := TopMember{Span: m.Span()}
	switch d := node.(type) {
	case *ast.Package:
		tm.identify("package", d.Ident)
	case *ast.Namespace:
		tm.identify("namespace", d.Ident)
	case *ast.Alias:
		tm.identify("alias", d.Ident)
	case *ast.Import:
		tm.Kind, tm.Target = "import", importTarget(d)
	case *ast.Dependency:
		tm.identify("dependency", d.Ident)
	case *ast.Comment:
		tm.Kind = "comment"
	case *ast.RelationshipMember:
		tm.identify(ast.Notation(d), d.Ident)
	case *ast.Definition:
		tm.identify(ast.Notation(d), d.Ident)
	case *ast.Usage:
		tm.identify(ast.Notation(d), d.Ident)
	}
	return tm
}

func (tm *TopMember) identify(kind string, id ast.Identification) {
	tm.Kind, tm.Name, tm.ShortName = kind, id.Name, id.ShortName
	tm.Named, tm.ShortNamed = id.HasName(), id.HasShortName()
}

// importTarget spells what an import names, wildcards included.
func importTarget(imp *ast.Import) string {
	name := "<?>"
	if imp.Imported != nil {
		parts := make([]string, len(imp.Imported.Parts))
		for i, p := range imp.Imported.Parts {
			parts[i] = p.Text
		}
		name = strings.Join(parts, "::")
	}
	switch {
	case imp.Kind == ast.ImportNamespace && imp.IsRecursive:
		return name + "::*::**"
	case imp.IsRecursive:
		return name + "::**"
	case imp.Kind == ast.ImportNamespace:
		return name + "::*"
	}
	return name
}
