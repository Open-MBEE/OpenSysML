package metamodel

// ReasonCategory identifies why a property derivation is unsupported.
type ReasonCategory string

const (
	ReasonNoRule                              ReasonCategory = "no-rule"
	ReasonUnresolvedReference                 ReasonCategory = "unresolved-reference"
	ReasonLibraryTargetWithoutCatalogIdentity ReasonCategory = "library-target-without-catalog-identity"
	ReasonDependsOnUnsupported                ReasonCategory = "depends-on-unsupported"
	ReasonUnrepresentedEndpoint               ReasonCategory = "unrepresented-endpoint"
	ReasonUnrepresentedElement                ReasonCategory = "unrepresented-element"
	ReasonUnresolvedOperatorFunction          ReasonCategory = "unresolved-operator-function"
	ReasonMissingStructure                    ReasonCategory = "missing-structure"
	ReasonMissingSemanticModel                ReasonCategory = "missing-semantic-model"
	ReasonNotApplicable                       ReasonCategory = "not-applicable"
	ReasonIncompleteStructure                 ReasonCategory = "incomplete-structure"
	ReasonImpliedRelationshipNotMaterialized  ReasonCategory = "implied-relationship-not-materialized"
	ReasonValueStatedUnderRedefinedProperty   ReasonCategory = "value-stated-under-redefined-property"
)

// Reason records the decision that made a property derivation unsupported.
type Reason struct {
	Category   ReasonCategory
	Detail     string
	Dependency string
}

// String formats the category and its identifying detail.
func (r Reason) String() string {
	detail := r.Detail
	if r.Category == ReasonDependsOnUnsupported {
		detail = r.Dependency
	}
	if detail == "" {
		return string(r.Category)
	}
	return string(r.Category) + " " + detail
}
