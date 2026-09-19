package symbols

import "github.com/Open-MBEE/OpenSysML/internal/syntax/source"

// Members returns distinct symbols declared directly in this scope in
// declaration order, with duplicate keys interleaved as declared.
// A symbol registered under both its short and primary name appears once.
func (s *Scope) Members() []*Symbol {
	seen := map[*Symbol]bool{}
	var out []*Symbol
	for _, sym := range s.syms {
		if seen[sym] {
			continue
		}
		seen[sym] = true
		out = append(out, sym)
	}
	return out
}

// DeclaredAt returns the symbol whose declaration was parsed from exactly span,
// in this scope or one nested in it, or nil. An anonymous declaration is found
// as a named one is, so an edit can reach an element no qualified name does.
func (s *Scope) DeclaredAt(span source.Span) *Symbol {
	if span.Len <= 0 {
		return nil
	}
	return s.declared(span, func(decl source.Span) bool { return decl == span })
}

// DeclaredFrom returns the symbol whose declaration was parsed starting at
// offset, in this scope or one nested in it, or nil. It relocates a declaration
// whose extent an edit changed but whose start it left in place.
func (s *Scope) DeclaredFrom(offset int) *Symbol {
	return s.declared(source.Span{Offset: offset, Len: 1}, func(decl source.Span) bool { return decl.Offset == offset })
}

// declared finds the member of this scope tree whose DeclSpan satisfies match,
// descending only into children whose node covers span.
func (s *Scope) declared(span source.Span, match func(source.Span) bool) *Symbol {
	if s == nil {
		return nil
	}
	var found *Symbol
	s.ForEachMember(func(sym *Symbol) bool {
		if match(sym.DeclSpan) {
			found = sym
			return false
		}
		return true
	})
	if found != nil {
		return found
	}
	for _, child := range s.children {
		if node := child.node; node != nil {
			if sp := node.Span(); sp.Offset > span.Offset || sp.End() < span.End() {
				continue
			}
		}
		if sym := child.declared(span, match); sym != nil {
			return sym
		}
	}
	return nil
}

// DocNameOf is the document a scope belongs to, read from the name SetDocName
// stamped on the scope and its symbols. It identifies a scope tree the index does
// not hold — a document builds its own for the editor — by name rather than by
// identity, a document declaring nothing but an import included.
func DocNameOf(scope *Scope) string {
	for s := scope; s != nil; s = s.Parent() {
		if s.docName != "" {
			return s.docName
		}
		if owner := s.Owner(); owner != nil && owner.DocName != "" {
			return owner.DocName
		}
		for _, sym := range s.Members() {
			if sym.DocName != "" {
				return sym.DocName
			}
		}
	}
	return ""
}

// SetDocName stamps name onto every symbol in the scope tree (members and
// all descendant scopes), recording which document declares each symbol.
// Recursion follows the child links, so scopes no symbol owns — loop bodies and
// body-expression parameters — are stamped too.
func SetDocName(scope *Scope, name string) {
	if scope == nil {
		return
	}
	scope.docName = name
	// AllMembers, so an anonymous declaration — a `connect a to b;` with no name
	// of its own — is stamped as well and stays locatable.
	for _, sym := range scope.AllMembers() {
		sym.DocName = name
	}
	for _, child := range scope.Children() {
		SetDocName(child, name)
	}
}
