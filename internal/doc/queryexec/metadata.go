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

// annotationSite identifies an annotation's facts across the direct and
// nested reads, so their values can be emitted in the annotation's place.
type annotationSite struct {
	typeFQN string
	span    source.Span
}

// annotationValues reports a row's bound values as
// `<metadata>::<feature>=<value>` strings, in annotation declaration order.
func (e *executor) annotationValues(sym *symbols.Symbol) ([]Value, bool, error) {
	if sym == nil {
		return nil, false, nil
	}
	origin := ElementValue(sym).Origin()
	var out []Value
	emit := func(annotation symbols.AnnotationFacts) {
		for _, bound := range annotation.Values {
			if !bound.Bound {
				continue
			}
			for _, value := range bound.Values {
				converted, ok := e.filterValue(value, sym)
				if !ok {
					// A bulk read reports the bindings it can read; one binding
					// the model cannot evaluate must not fail every row.
					continue
				}
				text, ok := e.context.PropertyText(converted)
				if !ok {
					continue
				}
				out = append(out, valueAt(StringValue(annotation.TypeFQN+"::"+bound.Feature+"="+text), origin))
			}
		}
	}
	// Pair nested facts with their site: an earlier annotation's nested binding
	// must precede a later one's direct binding, since a caller may read only the first value.
	nestedFacts := e.context.Model.AnnotationNestedFactsOf(sym)
	nested := make(map[annotationSite][]symbols.AnnotationFacts, len(nestedFacts))
	for _, facts := range nestedFacts {
		site := annotationSite{facts.TypeFQN, facts.Span}
		nested[site] = append(nested[site], facts)
	}
	used := make(map[annotationSite]int, len(nested))
	emitNested := func(site annotationSite) {
		for i := used[site]; i < len(nested[site]); i++ {
			emit(nested[site][i])
		}
		used[site] = len(nested[site])
	}
	for _, facts := range e.context.Model.AnnotationFactsOf(sym) {
		emit(facts)
		emitNested(annotationSite{facts.TypeFQN, facts.Span})
	}
	// A nested site no direct fact reported still belongs on the row.
	for _, facts := range nestedFacts {
		site := annotationSite{facts.TypeFQN, facts.Span}
		if used[site] < len(nested[site]) {
			emitNested(site)
		}
	}
	return out, len(out) > 0, nil
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
