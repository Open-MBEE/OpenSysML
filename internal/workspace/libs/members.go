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
		tm.Kind, tm.Name, tm.ShortName = "package", d.Ident.Name, d.Ident.ShortName
	case *ast.Namespace:
		tm.Kind, tm.Name, tm.ShortName = "namespace", d.Ident.Name, d.Ident.ShortName
	case *ast.Alias:
		tm.Kind, tm.Name, tm.ShortName = "alias", d.Ident.Name, d.Ident.ShortName
	case *ast.Import:
		tm.Kind, tm.Target = "import", importTarget(d)
	case *ast.Dependency:
		tm.Kind, tm.Name, tm.ShortName = "dependency", d.Ident.Name, d.Ident.ShortName
	case *ast.Comment:
		tm.Kind = "comment"
	case *ast.RelationshipMember:
		tm.Kind, tm.Name, tm.ShortName = ast.Notation(d), d.Ident.Name, d.Ident.ShortName
	case *ast.Definition:
		tm.Kind, tm.Name, tm.ShortName = ast.Notation(d), d.Ident.Name, d.Ident.ShortName
	case *ast.Usage:
		tm.Kind, tm.Name, tm.ShortName = ast.Notation(d), d.Ident.Name, d.Ident.ShortName
	}
	return tm
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
