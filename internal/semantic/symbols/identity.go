package symbols

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ElementKey identifies the declaration a symbol was built from, since a document
// and the global index build their own symbol for one declaration. A symbol with no
// declaring document — restored from cache — is identified by pointer instead.
type ElementKey struct {
	doc     string
	span    source.Span
	sym     *Symbol
	ordinal int // which written relationship of the owner, 0 for the element itself
}

// KeyOf is the identity of the element sym declares. A relationship written
// as notation is identified by its owner and its position among the owner's
// relationships, so two relationships of one owner stay distinct across Models.
func KeyOf(sym *Symbol) ElementKey {
	switch {
	case sym == nil:
		return ElementKey{}
	case sym.Implicit != nil:
		key := KeyOf(sym.Implicit.Owner)
		key.ordinal = sym.Implicit.Ordinal + 1
		return key
	case sym.DocName == "":
		return ElementKey{sym: sym}
	}
	return ElementKey{doc: sym.DocName, span: sym.DeclSpan}
}

// String spells the key, distinct for distinct elements, for use in a name.
func (k ElementKey) String() string {
	var base string
	if k.doc == "" {
		base = fmt.Sprintf("%p", k.sym)
	} else {
		base = fmt.Sprintf("%s\x00%d\x00%d", k.doc, k.span.Offset, k.span.Len)
	}
	if k.ordinal != 0 {
		return fmt.Sprintf("%s\x00%d", base, k.ordinal)
	}
	return base
}

// SameElement reports whether a and b denote one element, whichever scope tree
// each was reached through. A nil symbol denotes no element, itself included.
func SameElement(a, b *Symbol) bool {
	if a == nil || b == nil {
		return false
	}
	return KeyOf(a) == KeyOf(b)
}
