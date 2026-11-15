package naming

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Document is one document of an index: its source, the tree the index was
// built from, and its root scope.
type Document struct {
	File  *source.SourceFile
	Root  *ast.RootNamespace
	Scope *symbols.Scope
}

// PositionalIdentities names the declared symbols of docs that no qualified
// identity names (identifies reports those), by their position: `Owner::@N`.
func PositionalIdentities(index *symbols.Index, docs []Document, identifies func(*symbols.Symbol) bool) (map[*symbols.Symbol]string, map[string]*symbols.Symbol) {
	positional := make(map[*symbols.Symbol]string)
	byPositional := make(map[string]*symbols.Symbol)
	if index == nil || identifies == nil {
		return positional, byPositional
	}
	res := resolve.New(index)
	model := semantics.NewModel(res)
	res.SetModel(model)
	candidates := make(map[string]map[*symbols.Symbol]bool)
	for _, doc := range docs {
		positionalCandidatesOf(doc, res, model, identifies, candidates)
	}
	for _, candidate := range acceptedPositional(candidates) {
		sym := candidate.sym
		owner := sym.Owner()
		if sym.Name != "" && !sym.EffectiveName() {
			if owner == nil || positional[owner] == "" {
				continue
			}
		} else if owner != nil && !identifies(owner) && positional[owner] == "" {
			continue
		}
		positional[sym] = candidate.name
		byPositional[candidate.name] = sym
	}
	return positional, byPositional
}

type positionalCandidate struct {
	name string
	sym  *symbols.Symbol
}

// positionalCandidatesOf records, under each name the document derives, the
// symbols claiming it.
func positionalCandidatesOf(doc Document, res *resolve.Resolver, model *semantics.Model, identifies func(*symbols.Symbol) bool, candidates map[string]map[*symbols.Symbol]bool) {
	if doc.File == nil || doc.Root == nil || doc.Scope == nil {
		return
	}
	res.ResolveDocument(doc.File.Name(), doc.Root)
	names, err := Build(doc.File, doc.Root, model, identity.Build(model, res, doc.Scope))
	if err != nil {
		return
	}
	index := res.Index()
	seen := make(map[*symbols.Scope]bool)
	var walk func(*symbols.Scope)
	walk = func(current *symbols.Scope) {
		if current == nil || seen[current] {
			return
		}
		seen[current] = true
		for _, sym := range current.AllMembers() {
			if !positionalSymbol(sym, doc.File.Name(), identifies) {
				continue
			}
			// A name derived from the feature the symbol implicitly redefines (`Check::obj`) is one
			// the index resolves to nothing; an `@N` address is apart from a declared `'@N'`.
			name, ok := names.FQN[sym.Decl]
			if !ok || !IsPositionalIdentity(name) && len(index.LookupQualified(name)) != 0 {
				continue
			}
			if candidates[name] == nil {
				candidates[name] = make(map[*symbols.Symbol]bool)
			}
			candidates[name][sym] = true
		}
		for _, child := range current.Children() {
			walk(child)
		}
	}
	walk(doc.Scope)
}

// positionalSymbol reports a declared symbol of the document that no
// qualified identity names.
func positionalSymbol(sym *symbols.Symbol, docName string, identifies func(*symbols.Symbol) bool) bool {
	if sym == nil || sym.DocName != docName || sym.Implied() || identifies(sym) || sym.Decl == nil {
		return false
	}
	return sym.Name == "" || sym.EffectiveName() || sym.Owner() != nil
}

// acceptedPositional is every name exactly one symbol claims, shallowest
// first and then by name.
func acceptedPositional(candidates map[string]map[*symbols.Symbol]bool) []positionalCandidate {
	accepted := make([]positionalCandidate, 0, len(candidates))
	for name, claims := range candidates {
		if len(claims) != 1 {
			continue
		}
		for sym := range claims {
			accepted = append(accepted, positionalCandidate{name: name, sym: sym})
		}
	}
	slices.SortFunc(accepted, func(a, b positionalCandidate) int {
		aDepth, bDepth := len(Segments(a.name)), len(Segments(b.name))
		if aDepth < bDepth {
			return -1
		}
		if aDepth > bDepth {
			return 1
		}
		return strings.Compare(a.name, b.name)
	})
	return accepted
}
