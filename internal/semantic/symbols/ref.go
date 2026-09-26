package symbols

// ElementRef names an element for a record fact: the fully-qualified name of
// the nearest enclosing element that name alone declares (the element itself
// when it does), one member ordinal per step down from it to an element the
// name does not reach, and the declaring document when other documents declare
// the same name. A zero ElementRef names nothing.
type ElementRef struct {
	FQN  string
	Path []int32
	Doc  string
}

// IsZero reports whether the reference names nothing.
func (r ElementRef) IsZero() bool { return r.FQN == "" && len(r.Path) == 0 }

// MemberAt is the i-th registration of the scope in declaration order, nil when
// there is none.
func (s *Scope) MemberAt(i int32) *Symbol {
	if s == nil || i < 0 || int(i) >= len(s.members) {
		return nil
	}
	return s.members[i]
}

// declaringAll returns every symbol fqn declares, in index order.
func (idx *Index) declaringAll(fqn string) []*Symbol {
	var out []*Symbol
	for _, sym := range idx.LookupQualifiedFrom(fqn, fqn) {
		if sym != nil && HasFQN(sym, fqn) {
			out = append(out, sym)
		}
	}
	return out
}

// RefTo names sym for a record fact; ok is false for an element no reference
// reaches, such as one declared twice in its own document.
func (idx *Index) RefTo(sym *Symbol) (ref ElementRef, ok bool) {
	var path []int32
	for cur := sym; cur != nil; {
		if cur.Name != "" {
			if ref, ok := idx.namedRef(cur); ok {
				ref.Path = path
				return ref, true
			}
		}
		if cur.OwnerScope == nil {
			return ElementRef{}, false
		}
		at := int32(-1)
		for i, m := range cur.OwnerScope.members {
			if m == cur {
				at = memberIndexOf(i)
				break
			}
		}
		if at < 0 {
			return ElementRef{}, false
		}
		path = append([]int32{at}, path...)
		cur = cur.OwnerScope.owner
	}
	return ElementRef{}, false
}

// namedRef names sym by its fully-qualified name when that name declares it
// alone, in the index or in its document.
func (idx *Index) namedRef(sym *Symbol) (ElementRef, bool) {
	fqn := FQNOf(sym)
	if fqn == "" {
		return ElementRef{}, false
	}
	decls := idx.declaringAll(fqn)
	if len(decls) == 1 && decls[0] == sym {
		return ElementRef{FQN: fqn}, true
	}
	var inDoc []*Symbol
	for _, d := range decls {
		if d.DocName == sym.DocName {
			inDoc = append(inDoc, d)
		}
	}
	if len(inDoc) == 1 && inDoc[0] == sym {
		return ElementRef{FQN: fqn, Doc: sym.DocName}, true
	}
	return ElementRef{}, false
}

// Element restores the element a reference names, nil when nothing does.
func (idx *Index) Element(ref ElementRef) *Symbol {
	if ref.IsZero() {
		return nil
	}
	var sym *Symbol
	for _, d := range idx.declaringAll(ref.FQN) {
		if ref.Doc == "" || d.DocName == ref.Doc {
			sym = d
			break
		}
	}
	for _, at := range ref.Path {
		if sym == nil {
			return nil
		}
		sym = sym.Scope.MemberAt(at)
	}
	return sym
}
