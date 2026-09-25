package query

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// PropertyReader reads queryable properties from shared model semantics.
type PropertyReader struct {
	index     *symbols.Index
	resolver  *resolve.Resolver
	semantics *semantics.Model
}

// NewPropertyReader constructs a reader over one index and semantic model.
func NewPropertyReader(index *symbols.Index, resolver *resolve.Resolver, model *semantics.Model) *PropertyReader {
	return &PropertyReader{index: index, resolver: resolver, semantics: model}
}

// Values returns the values of one queryable property.
func (r *PropertyReader) Values(sym *symbols.Symbol, property string) ([]string, bool) {
	if sym == nil || r == nil || r.index == nil {
		return nil, false
	}
	fqn := r.index.GetFQN(sym)
	switch property {
	case PropertyID, PropertyQualifiedName:
		return presentValues(fqn)
	case PropertyName:
		if r.semantics != nil {
			return presentValues(r.semantics.EffectiveNameOf(sym))
		}
		return presentValues(sym.Name)
	case PropertyDeclaredName:
		if sym.EffectiveName() {
			return nil, false
		}
		return presentValues(sym.Name)
	case PropertyShortName:
		if r.semantics != nil {
			return presentValues(r.semantics.EffectiveShortNameOf(sym))
		}
		return presentValues(sym.ShortName)
	case PropertyDeclaredShortName:
		return presentValues(sym.ShortName)
	case PropertyDocumentation:
		if r.semantics == nil {
			return nil, false
		}
		bodies := r.semantics.DocumentationOf(sym)
		return bodies, len(bodies) > 0
	case PropertyOwner:
		if sym.OwnerScope != nil && sym.OwnerScope.Owner() != nil {
			return presentValues(r.index.GetFQN(sym.OwnerScope.Owner()))
		}
		return presentValues(ownerName(fqn))
	case PropertyType:
		return presentValues(MetamodelTypeNameOf(sym))
	case PropertyElementType:
		return r.elementType(sym)
	case PropertyIsAbstract:
		switch decl := sym.Decl.(type) {
		case *ast.Usage:
			return []string{strconv.FormatBool(decl.IsAbstract)}, true
		case *ast.Definition:
			return []string{strconv.FormatBool(decl.IsAbstract)}, true
		}
	case PropertyIsIndividual:
		switch decl := sym.Decl.(type) {
		case *ast.Usage:
			return []string{strconv.FormatBool(decl.IsIndividual)}, true
		case *ast.Definition:
			return []string{strconv.FormatBool(decl.IsIndividual)}, true
		}
	case PropertyMultiplicityLower, PropertyMultiplicityUpper:
		if r.semantics == nil {
			return nil, false
		}
		rng, ok := r.semantics.MultiplicityOf(sym)
		if !ok {
			return nil, false
		}
		if property == PropertyMultiplicityLower {
			return boundValues(rng.Lower)
		}
		return boundValues(rng.Upper)
	}
	return nil, false
}

func (r *PropertyReader) elementType(sym *symbols.Symbol) ([]string, bool) {
	if r.resolver == nil {
		return nil, false
	}
	for _, relationship := range semantics.RelationshipsOf(sym) {
		if relationship.Kind != ast.RelTyping {
			continue
		}
		target := relationship.Target
		if reference, ok := target.(*ast.FeatureReference); ok {
			target = reference.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		resolved, ok := r.resolver.ResolveQualified(sym.OwnerScope, qn)
		if !ok || resolved == nil {
			continue
		}
		if alias, ok := r.resolver.ResolveAliasTarget(resolved); ok {
			resolved = alias
		}
		return presentValues(r.index.GetFQN(resolved))
	}
	return nil, false
}

func presentValues(value string) ([]string, bool) {
	if value == "" {
		return nil, false
	}
	return []string{value}, true
}

func ownerName(fqn string) string {
	if i := strings.LastIndex(fqn, "::"); i >= 0 {
		return fqn[:i]
	}
	return ""
}

func boundValues(bound semantics.Bound) ([]string, bool) {
	if !bound.Known {
		return nil, false
	}
	if bound.Infinite {
		return []string{"*"}, true
	}
	return []string{strconv.FormatInt(bound.Value, 10)}, true
}
