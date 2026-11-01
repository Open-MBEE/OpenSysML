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
	ordinal int  // which relationship of the owner, 0 for the element itself
	form    int  // the ImplicitForm of that relationship
	chain   bool // the feature that relationship's chain target denotes
	implied int  // the ImpliedForm of an implied element plus one, 0 for a written one
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
		key.form = int(sym.Implicit.Form)
		return key
	case sym.Chain != nil:
		key := KeyOf(sym.Chain.Relationship)
		key.chain = true
		return key
	case sym.DocName == "":
		return ElementKey{sym: sym}
	}
	// An implied element shares the span of the declaration it is implied
	// for (parsed or recorded), so its form tells the two apart.
	return ElementKey{doc: sym.DocName, span: sym.DeclSpan, implied: impliedFormKey(sym)}
}

func impliedFormKey(sym *Symbol) int {
	if decl, ok := ImpliedDeclOf(sym); ok {
		return int(decl.Form) + 1
	}
	if sym.Recorded() && sym.Facts.Node == NodeConjugatedPortDef {
		return int(ImpliedConjugatedPortDefinition) + 1
	}
	return 0
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
		base = fmt.Sprintf("%s\x00%d", base, k.ordinal)
		if k.form != 0 {
			base = fmt.Sprintf("%s\x00f%d", base, k.form)
		}
	}
	if k.implied != 0 {
		base = fmt.Sprintf("%s\x00i%d", base, k.implied)
	}
	if k.chain {
		return base + "\x00chain"
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
