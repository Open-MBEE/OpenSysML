package symbols

// LookupBodyLocal finds the first declaration named name in the supplied body
// scope or one of its enclosing body-local scopes. It stops at the first
// non-body-local scope so declarations do not escape their expression body.
func LookupBodyLocal(scope *Scope, name string) (*Symbol, bool) {
	for current := scope; current != nil && current.BodyLocal(); current = current.Parent() {
		if sym, ok := current.LookupLocal(name); ok {
			return sym, true
		}
	}
	return nil, false
}
