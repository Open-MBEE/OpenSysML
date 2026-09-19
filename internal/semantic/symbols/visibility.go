package symbols

import "github.com/Open-MBEE/OpenSysML/internal/syntax/ast"

// VisibleAs reports whether a membership of the given visibility is visible to a
// reference made inside its namespace (inside) or from one that specializes it
// (inheriting), per KerML 8.2.3.5: private only inside, protected also through
// specialization, public everywhere.
func VisibleAs(v ast.Visibility, inside, inheriting bool) bool {
	switch v {
	case ast.VisibilityPrivate:
		return inside
	case ast.VisibilityProtected:
		return inside || inheriting
	default:
		return true
	}
}

// VisibleOutside reports whether a membership may be named through its
// namespace, which only a public membership may.
func VisibleOutside(v ast.Visibility) bool {
	return VisibleAs(v, false, false)
}
