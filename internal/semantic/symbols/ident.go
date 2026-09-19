package symbols

import "github.com/Open-MBEE/OpenSysML/internal/syntax/ast"

// DeclIdent returns the identification a declaration node carries: the long and
// short names it states, each with the span it was written at.
func DeclIdent(decl ast.Node) (ast.Identification, bool) {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Ident, true
	case *ast.Usage:
		return d.Ident, true
	case *ast.Package:
		return d.Ident, true
	case *ast.Namespace:
		return d.Ident, true
	case *ast.Alias:
		return d.Ident, true
	case *ast.MultiplicityDecl:
		return d.Ident, true
	case *ast.Dependency:
		return d.Ident, true
	case *ast.RelationshipMember:
		return d.Ident, true
	case *ast.Comment:
		return d.Ident, true
	case *ast.Documentation:
		return d.Ident, true
	case *ast.TextualRepresentation:
		return d.Ident, true
	case *ast.SubjectMember:
		return d.Ident, true
	case *ast.AssumeMember:
		return d.Ident, true
	case *ast.RequireMember:
		return d.Ident, true
	default:
		return ast.Identification{}, false
	}
}
