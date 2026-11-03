package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// metadataType is the metadata definition name names — its qualified name, or
// a name that is unique over the model, as WhereMetadata accepts; nil when it
// names none, several, or something other than a metadata def.
func (e *executor) metadataType(name string) *symbols.Symbol {
	if sym, ok := e.metadataTypes[name]; ok {
		return sym
	}
	var found *symbols.Symbol
	if sym := e.resolveClassification(name); sym != nil && sym.Kind == symbols.SymbolMetadataDef {
		found = sym
	}
	if e.metadataTypes != nil {
		e.metadataTypes[name] = found
	}
	return found
}

// annotationFeatureValues reads what the metadata annotating sym binds feature
// to, over every annotation whose type is the metadata def named declaring or
// specializes it. present reports whether sym carries such an annotation.
func (e *executor) annotationFeatureValues(sym *symbols.Symbol, declaring, feature string) ([]Value, bool, error) {
	var result []Value
	present := false
	for _, facts := range e.context.Model.AnnotationFactsOf(sym) {
		if !e.metadataConforms(facts.TypeFQN, declaring) {
			continue
		}
		present = true
		for _, bound := range facts.Values {
			if bound.Feature != feature {
				continue
			}
			for _, value := range bound.Values {
				converted, ok := e.filterValue(value, sym)
				if !ok {
					if value.Kind == symbols.FilterValueEmpty {
						continue
					}
					return nil, true, e.featureError(queryplan.Expression{}, feature, ElementValue(sym))
				}
				result = append(result, converted)
			}
		}
	}
	return result, present, nil
}

// metadataConforms reports whether the metadata type named actual is the one
// named declaring, or specializes it.
func (e *executor) metadataConforms(actual, declaring string) bool {
	if actual == declaring {
		return true
	}
	for _, sym := range e.context.Index.LookupQualified(actual) {
		if e.rowConformsTo(sym, declaring) {
			return true
		}
	}
	return false
}

// metadataPathValues reads a property spelled <metadata def>::<feature>, a
// feature of a metadata def named as metadataType resolves it: what the
// annotations of that def on the row bind the feature to. Not such a
// spelling: present is false.
func (e *executor) metadataPathValues(sym *symbols.Symbol, property string) ([]Value, bool, error) {
	metadataSegments, feature, ok := source.MetadataPathSegments(property)
	if !ok {
		return nil, false, nil
	}
	metadata := e.metadataType(source.QualifiedNameOf(metadataSegments))
	if metadata == nil {
		return nil, false, nil
	}
	member, ok := e.context.Model.LookupMember(metadata, feature)
	if !ok || member == nil || !member.IsFeature() {
		return nil, false, nil
	}
	return e.annotationFeatureValues(sym, symbols.FQNOf(metadata), feature)
}
