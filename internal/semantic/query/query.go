// Package query names the queryable properties of a model's elements and
// reads them from shared model semantics. Query surfaces — OSLC text, the API
// Query and document queries — are translated to query plans elsewhere and
// evaluated by internal/doc/queryexec over these readers.
package query

import "sort"

const (
	// PropertyID is the element identity property.
	PropertyID = "@id"
	// PropertyType is the metamodel type property.
	PropertyType = "@type"
	// PropertyName is the effective local name property.
	PropertyName = "name"
	// PropertyDeclaredName is the explicitly declared name property.
	PropertyDeclaredName = "declaredName"
	// PropertyShortName is the effective short name property.
	PropertyShortName = "shortName"
	// PropertyDeclaredShortName is the explicitly declared short name property.
	PropertyDeclaredShortName = "declaredShortName"
	// PropertyDocumentation is the body text of each `doc` comment, in order.
	PropertyDocumentation = "documentation"
	// PropertyTextualRepresentation is the body text of each `rep` the element
	// owns, in order (Element::textualRepresentation).
	PropertyTextualRepresentation = "textualRepresentation"
	// PropertyBody is the exact body of a `comment`, `doc` or `rep` element
	// itself (Comment::body, TextualRepresentation::body).
	PropertyBody = "body"
	// PropertyLocale is Comment::locale of a `comment` or `doc` declaring one.
	PropertyLocale = "locale"
	// PropertyLanguage is TextualRepresentation::language of a `rep`.
	PropertyLanguage = "language"
	// PropertyQualifiedName is the fully qualified name property.
	PropertyQualifiedName = "qualifiedName"
	// PropertyOwner is the owning element's qualified name property.
	PropertyOwner = "owner"
	// PropertyElementType is the resolved declared element type property.
	PropertyElementType = "type"
	// PropertyGeneral is the types a definition specializes or a usage is typed by,
	// in declaration order.
	PropertyGeneral = "general"
	// PropertyIsAbstract is the abstractness property.
	PropertyIsAbstract = "isAbstract"
	// PropertyIsIndividual is the `individual` modifier of a definition or usage.
	PropertyIsIndividual = "isIndividual"
	// PropertyMultiplicityLower is the lower multiplicity bound property.
	PropertyMultiplicityLower = "multiplicityLower"
	// PropertyMultiplicityUpper is the upper multiplicity bound property.
	PropertyMultiplicityUpper = "multiplicityUpper"
	// PropertySatisfiedRequirement is the requirement a satisfy usage satisfies.
	PropertySatisfiedRequirement = "satisfiedRequirement"
	// PropertySatisfyingFeature is the feature a satisfy usage is by.
	PropertySatisfyingFeature = "satisfyingFeature"
	// PropertyPayloadArgument, PropertySenderArgument and PropertyReceiverArgument
	// are the arguments of a send action as written: what it sends, the `via`
	// sender and the `to` receiver (SysML v2 §8.3.17.15 SendActionUsage).
	PropertyPayloadArgument  = "payloadArgument"
	PropertySenderArgument   = "senderArgument"
	PropertyReceiverArgument = "receiverArgument"
	// PropertyElementID is the elementId a conversion of the model writes for
	// the element, reported only where the reader is given one (WithElementID).
	PropertyElementID = "elementId"
)

// propertyNames is the closed set of properties supported by element queries.
var propertyNames = []string{
	PropertyID, PropertyType, PropertyName, PropertyDeclaredName,
	PropertyShortName, PropertyDeclaredShortName, PropertyDocumentation,
	PropertyTextualRepresentation, PropertyBody, PropertyLocale, PropertyLanguage,
	PropertyQualifiedName, PropertyOwner, PropertyIsAbstract, PropertyIsIndividual, PropertyElementType,
	PropertyGeneral,
	PropertyMultiplicityLower, PropertyMultiplicityUpper,
	PropertySatisfiedRequirement, PropertySatisfyingFeature,
	PropertyPayloadArgument, PropertySenderArgument, PropertyReceiverArgument,
	PropertyElementID,
}

// PropertyNames returns the supported property names in stable order.
func PropertyNames() []string {
	out := append([]string(nil), propertyNames...)
	sort.Strings(out)
	return out
}

// IsProperty reports whether name belongs to the closed queryable-property set.
func IsProperty(name string) bool {
	for _, candidate := range propertyNames {
		if candidate == name {
			return true
		}
	}
	return false
}
