package queryexec

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// metadataType is the metadata definition the qualified name names; nil when
// it names none, or something other than a metadata def.
func (e *executor) metadataType(name string) *symbols.Symbol {
	for _, sym := range e.context.Index.LookupQualified(name) {
		if sym.Kind == symbols.SymbolMetadataDef {
			return sym
		}
	}
	return nil
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

// metadataPathValues reads a property spelled <metadata def>::<feature>, the
// qualified name of a feature of a metadata def: what the annotations of that
// def on the row bind the feature to. Not such a spelling: present is false.
func (e *executor) metadataPathValues(sym *symbols.Symbol, property string) ([]Value, bool, error) {
	i := strings.LastIndex(property, "::")
	if i < 0 {
		return nil, false, nil
	}
	metadataSegments, ok := source.QualifiedNameSegments(property[:i])
	if !ok || len(metadataSegments) == 0 {
		return nil, false, nil
	}
	raw := property[i+2:]
	if raw == "" {
		return nil, false, nil
	}
	feature := raw
	if featureSegments, ok := source.MemberPathSegments(raw); ok && len(featureSegments) == 1 {
		feature = featureSegments[0]
	}
	declaring := strings.Join(metadataSegments, "::")
	metadata := e.metadataType(declaring)
	if metadata == nil {
		return nil, false, nil
	}
	member, ok := e.context.Model.LookupMember(metadata, feature)
	if !ok || member == nil || !member.IsFeature() {
		return nil, false, nil
	}
	return e.annotationFeatureValues(sym, declaring, feature)
}
