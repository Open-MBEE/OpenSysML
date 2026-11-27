package view

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// PseudoViewPrefix marks a rendering of exposed elements rather than a declared
// view.
const PseudoViewPrefix = "#"

// PseudoViewKind reports the supported rendering kind named by name.
func PseudoViewKind(name string) (Kind, bool) {
	kind, ok := pseudoViewKinds()[name]
	return kind, ok
}

// ParsePseudoView parses #<kind> or #<kind>:<target>.
func ParsePseudoView(spec string) (Kind, string, bool) {
	if !strings.HasPrefix(spec, PseudoViewPrefix) {
		return "", "", false
	}
	name, target, _ := strings.Cut(strings.TrimPrefix(spec, PseudoViewPrefix), ":")
	kind, ok := PseudoViewKind(name)
	return kind, target, ok
}

// PseudoViewSpecs names the supported pseudo-views in sorted order.
func PseudoViewSpecs() []string {
	kinds := pseudoViewKinds()
	out := make([]string, 0, len(kinds))
	for name := range kinds {
		out = append(out, PseudoViewPrefix+name)
	}
	sort.Strings(out)
	return out
}

func pseudoViewKinds() map[string]Kind {
	out := map[string]Kind{}
	add := func(kinds map[string]Kind) {
		for _, kind := range kinds {
			if kind.Supported() {
				out[string(kind)] = kind
			}
		}
	}
	add(standardRenderings)
	add(standardViewDefinitions)
	out[string(KindMatrix)] = KindMatrix
	add(toolRenderings)
	add(toolViewDefinitions)
	return out
}

// PseudoViewExposed are the elements a pseudo-view of doc renders: the element target
// names there, or with no target the document's own top-level declarations. It is nil
// when target names nothing in the document.
func PseudoViewExposed(idx *symbols.Index, doc, target string) []*symbols.Symbol {
	if target == "" {
		return TopLevelDeclarations(idx.DocumentRoot(doc))
	}
	if sym := DeclaredIn(idx, doc, target); sym != nil {
		return []*symbols.Symbol{sym}
	}
	return nil
}

// DeclaredIn is the element fqn names in the document: by qualified name in the index,
// else by qualified or simple name among the document's own declarations.
func DeclaredIn(idx *symbols.Index, doc, fqn string) *symbols.Symbol {
	for _, sym := range idx.LookupQualified(fqn) {
		if sym.DocName == doc {
			return sym
		}
	}
	var found *symbols.Symbol
	idx.DocumentRoot(doc).WalkDeclarations(func(sym *symbols.Symbol) {
		if found != nil {
			return
		}
		if idx.GetFQN(sym) == fqn || sym.Name == fqn {
			found = sym
		}
	})
	return found
}

// TopLevelDeclarations returns root declarations, expanding packages one level.
func TopLevelDeclarations(root *symbols.Scope) []*symbols.Symbol {
	if root == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, sym := range root.Declarations() {
		if sym.Kind == symbols.SymbolPackage {
			out = append(out, sym.Scope.Declarations()...)
			continue
		}
		out = append(out, sym)
	}
	return out
}
