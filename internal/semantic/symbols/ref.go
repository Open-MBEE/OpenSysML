package symbols

import "slices"

// ElementRef names an element for a record fact: the fully-qualified name of
// the nearest enclosing element that name alone declares in its document (the
// element itself when it does), that document, and one member ordinal per step
// down from it to an element the name does not reach. Naming the document
// keeps the reference on its target when another document later declares the
// same name. A zero ElementRef names nothing; one without a document names
// whichever declaration the index lists first; one whose document the index no
// longer holds names the library's declaration, if any: a copy of a library
// file standing in for the bundled one takes over the references the other
// library files make into it.
type ElementRef struct {
	FQN  string
	Path []int32
	Doc  string
}

// IsZero reports whether the reference names nothing.
func (r ElementRef) IsZero() bool { return r.FQN == "" && len(r.Path) == 0 }

// Clone returns a copy sharing no slice with r.
func (r ElementRef) Clone() ElementRef {
	r.Path = slices.Clone(r.Path)
	return r
}

// cloneRefs returns a copy of refs sharing no slice with it; nil for nil.
func clonePaths(paths [][]ElementRef) [][]ElementRef {
	if paths == nil {
		return nil
	}
	out := make([][]ElementRef, len(paths))
	for i, p := range paths {
		out[i] = cloneRefs(p)
	}
	return out
}

func cloneRefs(refs []ElementRef) []ElementRef {
	if refs == nil {
		return nil
	}
	out := make([]ElementRef, len(refs))
	for i, r := range refs {
		out[i] = r.Clone()
	}
	return out
}

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
	if sym == nil || sym.OwnerScope == nil || !sym.OwnerScope.inRecord() {
		return ElementRef{}, false
	}
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

// namedRef names sym by its fully-qualified name and document when that name
// declares it alone in its document.
func (idx *Index) namedRef(sym *Symbol) (ElementRef, bool) {
	fqn := FQNOf(sym)
	if fqn == "" {
		return ElementRef{}, false
	}
	var inDoc []*Symbol
	for _, d := range idx.declaringAll(fqn) {
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
	decls := idx.declaringAll(ref.FQN)
	for _, d := range decls {
		if ref.Doc == "" || d.DocName == ref.Doc {
			sym = d
			break
		}
	}
	if sym == nil && !idx.knows(ref.Doc) {
		for _, d := range decls {
			if idx.IsLibraryDocument(d.DocName) {
				sym = d
				break
			}
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
