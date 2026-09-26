package queryexec

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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
				converted, ok := e.annotationValue(value, sym)
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

// annotationValue converts one bound value to a cell value: a reference to an
// element of the model becomes that element, so the cell prints its name.
func (e *executor) annotationValue(value symbols.FilterValue, sym *symbols.Symbol) (Value, bool) {
	if value.Kind == symbols.FilterValueRef {
		if targets := e.context.Index.LookupQualified(value.RefFQN); len(targets) > 0 {
			return valueAt(ElementValue(targets[0]), ElementValue(sym).Origin()), true
		}
	}
	return filterValue(value, sym)
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
	declaring, feature := property[:i], property[i+2:]
	if feature == "" || e.metadataType(declaring) == nil {
		return nil, false, nil
	}
	return e.annotationFeatureValues(sym, declaring, feature)
}
