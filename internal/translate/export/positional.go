package export

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// PositionalDocument pairs a source file with the scope containing its symbols.
type PositionalDocument struct {
	File *source.SourceFile
	Root *symbols.Scope
}

// DeclarationNames maps declarations to their RDF qualified names by span.
// Unnamed declarations use the export's positional `@N` identity.
func DeclarationNames(file *source.SourceFile, root *ast.RootNamespace) (map[source.Span]string, error) {
	e, err := newEncoder(file, root, "", IDQualifiedName)
	if err != nil {
		return nil, err
	}
	names := make(map[source.Span]string, len(e.fqn))
	ambiguous := make(map[source.Span]bool)
	for node, name := range e.fqn {
		span := node.Span()
		if previous, exists := names[span]; exists && previous != name {
			delete(names, span)
			ambiguous[span] = true
			continue
		}
		if !ambiguous[span] {
			names[span] = name
		}
	}
	return names, nil
}

// PositionalIdentities maps symbols without qualified identities to export names.
func PositionalIdentities(index *symbols.Index, docs []PositionalDocument, identifies func(*symbols.Symbol) bool) (map[*symbols.Symbol]string, map[string]*symbols.Symbol) {
	positional := make(map[*symbols.Symbol]string)
	byPositional := make(map[string]*symbols.Symbol)
	if index == nil || identifies == nil {
		return positional, byPositional
	}

	candidates := make(map[string]map[*symbols.Symbol]bool)
	type positionalCandidate struct {
		name string
		sym  *symbols.Symbol
	}
	for _, doc := range docs {
		if doc.File == nil || doc.Root == nil {
			continue
		}
		file := source.NewWithKind(doc.File.Name(), doc.File.Bytes(), doc.File.Kind())
		p := parser.New(file)
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			continue
		}
		names, err := DeclarationNames(file, root)
		if err != nil {
			continue
		}
		seen := make(map[*symbols.Scope]bool)
		var walk func(*symbols.Scope)
		walk = func(current *symbols.Scope) {
			if current == nil || seen[current] {
				return
			}
			seen[current] = true
			for _, sym := range current.AllMembers() {
				if sym == nil || sym.DocName != doc.File.Name() ||
					identifies(sym) || sym.Decl == nil {
					continue
				}
				if sym.Name != "" && !sym.EffectiveName() && sym.Owner() == nil {
					continue
				}
				name, ok := names[sym.Decl.Span()]
				if !ok || !IsPositionalIdentity(name) || len(index.LookupQualified(name)) != 0 {
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
		walk(doc.Root)
	}

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
		aDepth, bDepth := strings.Count(a.name, "::"), strings.Count(b.name, "::")
		if aDepth < bDepth {
			return -1
		}
		if aDepth > bDepth {
			return 1
		}
		return strings.Compare(a.name, b.name)
	})

	for _, candidate := range accepted {
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

// IsPositionalIdentity reports whether a name contains a canonical `@N` segment.
func IsPositionalIdentity(name string) bool {
	for _, segment := range strings.Split(name, "::") {
		if len(segment) < 2 || segment[0] != '@' {
			continue
		}
		value, err := strconv.Atoi(segment[1:])
		if err == nil && value >= 0 && strconv.Itoa(value) == segment[1:] {
			return true
		}
	}
	return false
}
