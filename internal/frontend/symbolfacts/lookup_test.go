package symbolfacts

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// shadowingModel declares names the standard library declares too: a root-level
// type named like the Items package, and a Parts::Part of its own.
const shadowingModel = `
part def Base {
	action inherited { first start; then done; }
}
part def Items :> Base;

package Parts {
	part def Core {
		action run { first start; then done; }
	}
	part def Part :> Core;
}
`

// modelWithLibrary indexes one document over the standard library, as every
// frontend does, and returns the index with a context over it.
func modelWithLibrary(t *testing.T, text string) (*symbols.Index, *Context) {
	t.Helper()
	base, _ := libs.SharedLibrary()
	idx := symbols.NewOverlay(base)
	p := parser.New(source.New("model.sysml", []byte(text)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("fixture does not parse: %v", p.Diagnostics)
	}
	idx.AddDocument("model.sysml", root)
	idx.ExpandWildcardImports()
	return idx, NewContext(idx)
}

func TestMemberChainPrefersTheModelOverALibraryHomonym(t *testing.T) {
	idx, sc := modelWithLibrary(t, shadowingModel)
	for _, tc := range []struct{ id, declaredUnder string }{
		{"Items::inherited", "Base::inherited"},
		{"Parts::Part::run", "Parts::Core::run"},
	} {
		sym, fqn := MemberChain(idx, sc.Semantics, tc.id)
		if sym == nil {
			t.Errorf("MemberChain(%q) found nothing; want %s", tc.id, tc.declaredUnder)
			continue
		}
		if fqn != tc.declaredUnder || idx.Library(sym) {
			t.Errorf("MemberChain(%q) = %s (library %v); want the model's %s", tc.id, fqn, idx.Library(sym), tc.declaredUnder)
		}
		if syms := LookupNamedMember(idx, sc.Semantics, tc.id); len(syms) != 1 || syms[0] != sym {
			t.Errorf("LookupNamedMember(%q) = %d symbols; want the one MemberChain found", tc.id, len(syms))
		}
	}
}

func TestMemberChainReachesTheLibraryWhenTheModelDoesNotDeclareTheName(t *testing.T) {
	idx, sc := modelWithLibrary(t, shadowingModel)
	// The model's Items is a part def, so Items::Item is the library's alone, and
	// the model's Parts declares no parts, so Parts::parts is too.
	for _, id := range []string{"Items::Item::timeSlices", "Parts::parts::timeSlices"} {
		sym, fqn := MemberChain(idx, sc.Semantics, id)
		if sym == nil || !idx.Library(sym) {
			t.Errorf("MemberChain(%q) = %v (%q); want a library member", id, sym != nil, fqn)
			continue
		}
		if fqn != "Occurrences::Occurrence::timeSlices" {
			t.Errorf("MemberChain(%q) declared under %q; want Occurrences::Occurrence::timeSlices", id, fqn)
		}
	}
}

func TestMemberChainRejectsANameTheModelDeclaresTwice(t *testing.T) {
	idx, sc := modelWithLibrary(t, `
part def Base { action inherited { first start; then done; } }
part def Twice :> Base;
part def Twice :> Base;
`)
	if sym, fqn := MemberChain(idx, sc.Semantics, "Twice::inherited"); sym != nil {
		t.Errorf("MemberChain resolved an ambiguous prefix to %q", fqn)
	}
}

func TestMemberChainDoesNotWalkAroundADuplicateThroughItsPackage(t *testing.T) {
	idx, sc := modelWithLibrary(t, `
package Demo {
	part def A { action run { first start; then done; } }
	part def B { action run { first start; then done; } }
	part def Part :> A;
	part def Part :> B;
}
`)
	// Demo::Part is declared twice; the walk from the one Demo must not pick
	// either Part for it.
	if sym, fqn := MemberChain(idx, sc.Semantics, "Demo::Part::run"); sym != nil {
		t.Errorf("MemberChain resolved Demo::Part::run through the package to %q", fqn)
	}
}
