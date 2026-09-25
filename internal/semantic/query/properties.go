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
	identity  func(*symbols.Symbol) string
}

// NewPropertyReader constructs a reader over one index and semantic model.
func NewPropertyReader(index *symbols.Index, resolver *resolve.Resolver, model *semantics.Model) *PropertyReader {
	return &PropertyReader{index: index, resolver: resolver, semantics: model}
}

// WithIdentity overrides how element identities are reported by this reader.
func (r *PropertyReader) WithIdentity(identity func(*symbols.Symbol) string) *PropertyReader {
	r.identity = identity
	return r
}

// Values returns the values of one queryable property.
func (r *PropertyReader) Values(sym *symbols.Symbol, property string) ([]string, bool) {
	if sym == nil || r == nil || r.index == nil {
		return nil, false
	}
	fqn := r.index.GetFQN(sym)
	switch property {
	case PropertyID:
		if r.identity != nil {
			return presentValues(r.identity(sym))
		}
		return presentValues(fqn)
	case PropertyQualifiedName:
		if r.identity != nil {
			if identity := r.identity(sym); identity != "" && identity != fqn {
				return nil, false
			}
		}
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
			owner := sym.OwnerScope.Owner()
			if r.identity != nil {
				if identity := r.identity(owner); identity != "" {
					return presentValues(identity)
				}
			}
			return presentValues(r.index.GetFQN(owner))
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
	case PropertySatisfiedRequirement, PropertySatisfyingFeature:
		return r.satisfyEnd(sym, property)
	}
	return nil, false
}

func (r *PropertyReader) satisfyEnd(sym *symbols.Symbol, property string) ([]string, bool) {
	if sym.Kind != symbols.SymbolSatisfyRequirementUsage {
		return nil, false
	}
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok || decl.Kind != ast.UsageSatisfy || decl.IsVerifiedRequirement() {
		return nil, false
	}
	if property == PropertySatisfiedRequirement {
		if decl.DeclaresRequirement {
			return presentValues(r.elementIdentity(sym))
		}
		rel := decl.ReferenceSubsetting()
		if rel == nil {
			return nil, false
		}
		return r.resolveTargetIdentity(sym, rel.Target)
	}
	for _, rel := range decl.Relationships {
		if rel != nil && rel.Kind == ast.RelSubject {
			return r.resolveTargetIdentity(sym, rel.Target)
		}
	}
	return nil, false
}

func (r *PropertyReader) resolveTargetIdentity(sym *symbols.Symbol, target ast.Node) ([]string, bool) {
	if r.resolver == nil {
		return nil, false
	}
	resolved, ok := r.resolver.ResolveTarget(sym.OwnerScope, target)
	if !ok || resolved == nil {
		return nil, false
	}
	if alias, ok := r.resolver.ResolveAliasTarget(resolved); ok {
		resolved = alias
	}
	return presentValues(r.elementIdentity(resolved))
}

func (r *PropertyReader) elementIdentity(sym *symbols.Symbol) string {
	if r.identity != nil {
		if identity := r.identity(sym); identity != "" {
			return identity
		}
	}
	return r.index.GetFQN(sym)
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
