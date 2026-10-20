package codes

// The sources the identity and diagram-layout codes cite.
const (
	specElementIdentity = "SysML v2 API and Services (element identity)"
	specDiagramLayout   = "OpenSysML diagram layout library"
)

// constraintCodes are the codes of the constraint tier: the well-formedness
// rules of the KerML and SysML v2 semantics, checked once names and types hold.
var constraintCodes = []Entry{
	{
		Code: "annotation-annotated-element-ownership", Tier: TierConstraint, Severity: "error",
		Summary: "an annotating element whose `about` names its own owner",
		Meaning: "An annotation owned by its annotated element must own its annotating element: a comment, documentation or metadata usage whose `about` names the element its body sits in annotates that element twice over, through ownership and through the clause. " +
			"Drop the `about` clause, or move the annotating element out of the body of what it annotates.",
		Spec: "KerML 1.0 validateAnnotationAnnotatedElementOwnership",
	},
	{
		Code: "association-end-types", Tier: TierConstraint, Severity: "error",
		Summary: "an association end with no type or several",
		Meaning: "Each end of an association is typed by exactly one type, which is what the end relates. " +
			"Give the end one type.",
		Spec: "KerML 1.0 validateAssociationEndTypes",
	},
	{
		Code: "binding-binary", Tier: TierConstraint, Severity: "error",
		Summary: "a binding connector with more or fewer than two ends",
		Meaning: "A binding connector asserts two features hold the same values, so it has exactly two ends. " +
			"Bind two features; chain bindings for more.",
		Spec: "KerML 1.0 validateBindingConnectorIsBinary",
	},
	{
		Code: "connector-ends", Tier: TierConstraint, Severity: "error",
		Summary: "a connector whose ends do not fit the link it specializes",
		Meaning: "A connector that specializes a binary link cannot declare more than two ends, and each of its ends must redefine an end of the link; this one declares too many ends or an end that redefines none. " +
			"Drop the extra ends or specialize an n-ary link, and redefine the link's ends.",
		Spec: "KerML 1.0 validateConnectorBinarySpecialization and validateAssociationBinarySpecialization",
	},
	{
		Code: "connector-end-referent", Tier: TierConstraint, Severity: "error",
		Summary: "a connector end naming no feature",
		Meaning: "Each end of a connector references a feature accessible where the connector is written; this end resolves to a definition, a package or another non-feature. " +
			"Name a usage, nested with dot notation where it sits inside another.",
		Spec: "KerML 1.0 §8.3.4.4 (connector ends reference features)",
	},
	{
		Code: "connector-related-features", Tier: TierConstraint, Severity: "error",
		Summary: "a connector or flow with fewer than two related features",
		Meaning: "A connector relates at least two features; this one names fewer, or names the same feature twice. " +
			"Name two features to connect.",
		Spec: "KerML 1.0 validateConnectorRelatedFeatures",
	},
	{
		Code: "connector-type-featuring", Tier: TierConstraint, Severity: "error",
		Summary: "a connector end that is not accessible from the connector's featuring type",
		Meaning: "Every related feature of a connector must be accessible from the type that features the connector: a feature of a sibling, reached without dot notation, is not. " +
			"Write the end as a chain from a feature of the connector's owner (`a.x`), or move the connector to where both ends are visible.",
		Spec: "KerML 1.0 validateConnectorTypeFeaturing",
	},
	{
		Code: "classifier-default-supertype", Tier: TierConstraint, Severity: "error",
		Summary: "a classifier that does not specialize its kind's base classifier",
		Meaning: "Every classifier specializes, directly or indirectly, the base classifier of its kind (`Base::Anything`, `Occurrences::Occurrence`, `Objects::Object` …); this one declares specializations that leave the base out. " +
			"Specialize a classifier of the right kind, or drop the specialization that leads elsewhere.",
		Spec: "KerML 1.0 validateClassifierDefaultSupertype",
	},
	{
		Code: "feature-has-type", Tier: TierConstraint, Severity: "error",
		Summary: "a feature with no type",
		Meaning: "Every feature has at least one type, declared or implied through what it subsets or redefines; this one has none. " +
			"Type the feature, or subset a typed one.",
		Spec: "KerML 1.0 validateFeatureHasType",
	},
	{
		Code: "chaining-feature-not-one", Tier: TierConstraint, Severity: "error",
		Summary: "a feature chained through one feature only",
		Meaning: "A feature chain (`chains a.b`) names at least two chaining features; a chain of one is the feature itself. " +
			"Chain two or more features, or subset the one feature.",
		Spec: "KerML 1.0 validateFeatureChainingFeatureNotOne",
	},
	{
		Code: "chaining-features-not-self", Tier: TierConstraint, Severity: "error",
		Summary: "a feature chained through itself",
		Meaning: "A feature cannot appear in its own feature chain. " +
			"Chain through other features.",
		Spec: "KerML 1.0 validateFeatureChainingFeaturesNotSelf",
	},
	{
		Code: "feature-chain-conformance", Tier: TierConstraint, Severity: "error",
		Summary: "a chained feature that is no feature of what precedes it",
		Meaning: "Each feature of a chain after the first is a feature of the type of the one before it; this segment names something the preceding feature's type does not feature. " +
			"Chain through members of the preceding feature's type.",
		Spec: "KerML 1.0 validateFeatureChainingFeatureConformance",
	},
	{
		Code: "feature-reference-referent", Tier: TierConstraint, Severity: "error",
		Summary: "a feature reference expression naming no feature",
		Meaning: "A name used as a value in an expression references a feature; this one resolves to a definition, a package or another non-feature. " +
			"Name a usage, or instantiate the type (`new T()`) where an instance is meant.",
		Spec: "KerML 1.0 validateFeatureReferenceExpressionReferentIsFeature",
	},
	{
		Code: "feature-reference-featuring-types", Tier: TierConstraint, Severity: "error",
		Summary: "a feature referenced where it is not accessible",
		Meaning: "A feature named in an expression must be accessible from the type the expression is featured by: a feature of a sibling part, or of the outer scope of a nested definition, is not reachable by its bare name. " +
			"Write a feature chain from a feature of the owner (`sibling.x`), or declare the feature where the expression is.",
		Spec: "KerML 1.0 §8.3.4.8 (Must be an accessible feature)",
	},
	{
		Code: "subsetting-featuring-types", Tier: TierConstraint, Severity: "error",
		Summary: "a subsetted feature that is not accessible from the subsetting one",
		Meaning: "A feature subsets or redefines a feature accessible from its featuring type: a feature of a sibling, named without dot notation, is not. " +
			"Subset through a feature chain (`:> sibling.x`), or move the declaration.",
		Spec: "KerML 1.0 validateSubsettingFeaturingTypes",
	},
	{
		Code: "subsetting-multiplicity", Tier: TierConstraint, Severity: "warning",
		Summary: "a subsetting or redefining feature with a larger upper bound than the subsetted one",
		Meaning: "The values of a subsetting feature are among those of the subsetted feature, so its multiplicity upper bound should not exceed the subsetted feature's. " +
			"Lower the bound, or widen the subsetted feature's.",
		Spec: "KerML 1.0 validateSubsettingMultiplicityConformance",
	},
	{
		Code: "redefinition-multiplicity", Tier: TierConstraint, Severity: "warning",
		Summary: "a redefining feature with a smaller lower bound than the redefined one",
		Meaning: "A redefinition narrows the redefined feature, so its multiplicity lower bound should not fall below the redefined feature's. " +
			"Raise the lower bound, or lower the redefined feature's.",
		Spec: "KerML 1.0 validateRedefinitionMultiplicityConformance",
	},
	{
		Code: "subsetting-uniqueness-conformance", Tier: TierConstraint, Severity: "error",
		Summary: "a nonunique feature subsetting or redefining a unique one",
		Meaning: "A subsetting or redefining feature cannot be `nonunique` when the feature it subsets or redefines is unique: its values are among the other's, which admit no repetition. " +
			"Drop `nonunique`, or declare the subsetted feature `nonunique`.",
		Spec: "KerML 1.0 validateSubsettingUniquenessConformance",
	},
	{
		Code: "subsetting-constancy-conformance", Tier: TierConstraint, Severity: "error",
		Summary: "a variable feature subsetting or redefining a constant one",
		Meaning: "A subsetting or redefining feature must be constant when the feature it subsets or redefines is constant. " +
			"Declare the feature `const`, or make the subsetted one variable.",
		Spec: "KerML 1.0 validateSubsettingConstantConformance",
	},
	{
		Code: "redefinition-direction-conformance", Tier: TierConstraint, Severity: "error",
		Summary: "a redefining feature whose direction does not fit the redefined one",
		Meaning: "A redefining feature keeps a direction compatible with the redefined feature's: `in` redefines `in` or `inout`, `out` redefines `out` or `inout`, an undirected feature redefines an undirected one. " +
			"Match the direction.",
		Spec: "KerML 1.0 validateRedefinitionDirectionConformance",
	},
	{
		Code: "redefinition-not-end", Tier: TierConstraint, Severity: "error",
		Summary: "a non-end feature of an association or connector redefining an end feature",
		Meaning: "A feature of an association or a connector that redefines an end feature is itself an end feature; a feature of any other type may redefine an end without being one. " +
			"Write `end` before the redefining feature.",
		Spec: "KerML 1.1 Beta 2 §8.3.3.3.8 validateRedefinitionEndConformance",
	},
	{
		Code: "redefinition-package-level", Tier: TierConstraint, Severity: "error",
		Summary: "a redefinition of a package-level feature",
		Meaning: "A feature owned directly by a package has no featuring type, so nothing can redefine it: redefinition replaces a feature in the context of a specializing type. " +
			"Subset the feature instead, or move it into a type.",
		Spec: "KerML 1.0 validateRedefinitionFeaturingTypes",
	},
	{
		Code: "redefinition-same-featuring", Tier: TierConstraint, Severity: "error",
		Summary: "a redefinition between features of one featuring type",
		Meaning: "A redefining feature and the feature it redefines are featured by different types: redefinition replaces an inherited feature, and two features of one type cannot redefine each other. " +
			"Subset the feature instead, or redefine from a specializing type.",
		Spec: "KerML 1.0 validateRedefinitionFeaturingTypes",
	},
	{
		Code: "redefinition-no-inherited", Tier: TierConstraint, Severity: "error",
		Summary: "`:>>` naming a feature the owner does not inherit",
		Meaning: "A redefinition replaces a feature the redefining feature's owner inherits; the feature named is a member of a type the owner does not specialize. " +
			"Specialize the type that owns the redefined feature, or subset the feature instead.",
		Spec: "KerML 1.0 §8.3.3.3.6 Redefinition",
	},
	{
		Code: "redefinition-no-derived-name", Tier: TierConstraint, Severity: "warning",
		Summary: "an unnamed redefinition given a value in a context that names it differently",
		Meaning: "A member written `:>> x = v` takes the name of what it redefines; here the redefined feature's name differs from what the owner's other members expect, so the value lands on a member the model names otherwise. " +
			"Name the member, or redefine the feature the name was meant for.",
		Spec: "",
	},
	{
		Code: "redefinition-type-mismatch", Tier: TierConstraint, Severity: "warning",
		Summary: "a redefining feature typed by a type unrelated to the redefined feature's",
		Meaning: "The redefining feature is typed by both its declared type and the redefined feature's, so two unrelated types leave it with no possible value — almost always a slip, though no normative constraint forbids it. " +
			"Type the redefinition by a specialization of the redefined feature's type.",
		Spec: "KerML 1.0 §8.3.3.3.4 (a feature's types are its typings and its subsetted features'); an OpenSysML advisory",
	},
	{
		Code: "redefinition-through-reference", Tier: TierConstraint, Severity: "error",
		Summary: "a chain redefinition stepping through a feature that owns no object",
		Meaning: "`:>> a.b` redefines `b` below `a` in this type's context, which needs `a` to own its values; a reference, subject or port usage owns none, so the redefinition has nothing to apply to. " +
			"Redefine the feature where it is owned, or make the intermediate feature composite.",
		Spec: "KerML 1.0 §8.3.3.3.6 Redefinition",
	},
	{
		Code: "specialization-cycle", Tier: TierConstraint, Severity: "error",
		Summary: "a type that specializes itself, directly or through others",
		Meaning: "Specialization is acyclic: following `:>` from this type leads back to it. " +
			"Break the cycle by dropping one specialization.",
		Spec: "KerML 1.0 §8.3.3.1 (specialization is a partial order)",
	},
	{
		Code: "multiplicity-range", Tier: TierConstraint, Severity: "error",
		Summary: "a multiplicity whose lower bound exceeds its upper bound",
		Meaning: "`[lo..hi]` requires lo ≤ hi; this range admits no count. " +
			"Swap or correct the bounds.",
		Spec: "KerML 1.0 §8.3.3.3.5 MultiplicityRange",
	},
	{
		Code: "multiplicity-bound-natural", Tier: TierConstraint, Severity: "error",
		Summary: "a multiplicity bound that is no Natural",
		Meaning: "The bounds of a multiplicity range are Natural numbers (or `*` for the upper bound); this one is negative, a Real, or a feature of another type. " +
			"Write a Natural literal or name a feature typed by Natural.",
		Spec: "KerML 1.0 validateMultiplicityRangeBoundResultTypes",
	},
	{
		Code: "multiplicity-featuring-type", Tier: TierConstraint, Severity: "error",
		Summary: "a multiplicity featured by a type other than its feature's",
		Meaning: "A multiplicity declared as a separate feature must have the same featuring types as the feature it bounds. " +
			"Declare the multiplicity beside its feature, or bound the feature inline.",
		Spec: "KerML 1.0 validateFeatureMultiplicityDomain",
	},
	{
		Code: "only-one-multiplicity", Tier: TierConstraint, Severity: "error",
		Summary: "a type with more than one owned multiplicity",
		Meaning: "A type owns at most one multiplicity. " +
			"Keep one `[..]`.",
		Spec: "KerML 1.0 validateTypeOwnedMultiplicity",
	},
	{
		Code: "end-feature-multiplicity", Tier: TierConstraint, Severity: "warning",
		Summary: "an end feature whose multiplicity is not 1",
		Meaning: "An end relates exactly one thing per link, so its own multiplicity is 1; the range written says otherwise (the multiplicity of the *other* ends is what `[n]` on an end usually means). " +
			"Write `[1]` or take the multiplicity from a feature the end subsets or redefines.",
		Spec: "KerML 1.0 validateFeatureEndFeatureMultiplicity",
	},
	{
		Code: "end-feature-direction", Tier: TierConstraint, Severity: "error",
		Summary: "an end feature with a direction",
		Meaning: "An end feature has no direction: `in`, `out` and `inout` are for parameters. " +
			"Drop the direction.",
		Spec: "KerML 1.0 validateFeatureEndNoDirection",
	},
	{
		Code: "end-feature-kind", Tier: TierConstraint, Severity: "error",
		Summary: "an end feature that is derived, abstract, composite or a portion",
		Meaning: "An end feature cannot be `derived`, `abstract`, `composite` or `portion`: it relates, it does not own. " +
			"Drop the modifier.",
		Spec: "KerML 1.0 validateFeatureEndNotDerivedAbstractCompositeOrPortion",
	},
	{
		Code: "type-unioning-not-one", Tier: TierConstraint, Severity: "error",
		Summary: "`unions` with one type only",
		Meaning: "A type is the union of two or more types or of none; a unioning of one is the type itself. " +
			"Union two or more types, or drop the clause.",
		Spec: "KerML 1.0 validateTypeOwnedUnioningNotOne",
	},
	{
		Code: "type-unioning-self", Tier: TierConstraint, Severity: "error",
		Summary: "a type unioning itself",
		Meaning: "A type cannot be one of its own unioning types. " +
			"Union other types.",
		Spec: "KerML 1.0 validateTypeUnioningTypesNotSelf",
	},
	{
		Code: "type-intersecting-not-one", Tier: TierConstraint, Severity: "error",
		Summary: "`intersects` with one type only",
		Meaning: "A type is the intersection of two or more types or of none; an intersecting of one is the type itself. " +
			"Intersect two or more types, or drop the clause.",
		Spec: "KerML 1.0 validateTypeOwnedIntersectingNotOne",
	},
	{
		Code: "type-intersecting-self", Tier: TierConstraint, Severity: "error",
		Summary: "a type intersecting itself",
		Meaning: "A type cannot be one of its own intersecting types. " +
			"Intersect other types.",
		Spec: "KerML 1.0 validateTypeIntersectingTypesNotSelf",
	},
	{
		Code: "type-differencing-not-one", Tier: TierConstraint, Severity: "error",
		Summary: "`differences` with one type only",
		Meaning: "A type is the difference of two or more types or of none; a differencing of one is the type itself. " +
			"Difference two or more types, or drop the clause.",
		Spec: "KerML 1.0 validateTypeOwnedDifferencingNotOne",
	},
	{
		Code: "type-differencing-self", Tier: TierConstraint, Severity: "error",
		Summary: "a type differencing itself",
		Meaning: "A type cannot be one of its own differencing types. " +
			"Difference other types.",
		Spec: "KerML 1.0 validateTypeDifferencingTypesNotSelf",
	},
	{
		Code: "type-conjugators", Tier: TierConstraint, Severity: "error",
		Summary: "a type with more than one conjugator",
		Meaning: "A type conjugates at most one type. " +
			"Remove the other `~`.",
		Spec: "KerML 1.0 validateTypeAtMostOneConjugator",
	},
	{
		Code: "cross-subsetting-crossing-feature", Tier: TierConstraint, Severity: "error",
		Summary: "a cross subsetting owned by something other than an end feature",
		Meaning: "A cross subsetting (`crosses`) is owned by an end feature of a type with two or more ends: it relates that end to the opposite end's feature. " +
			"Write `crosses` on an end feature of an association or connector.",
		Spec: "KerML 1.0 validateCrossSubsettingCrossingFeature",
	},
	{
		Code: "cross-subsetting-at-most-one", Tier: TierConstraint, Severity: "error",
		Summary: "an end feature with more than one cross subsetting",
		Meaning: "An end feature owns at most one cross subsetting. " +
			"Keep one `crosses`.",
		Spec: "KerML 1.0 validateFeatureOwnedCrossSubsetting",
	},
	{
		Code: "cross-subsetting-chain", Tier: TierConstraint, Severity: "error",
		Summary: "a cross subsetting that does not chain through an opposite end",
		Meaning: "The crossed feature is a chain whose first feature is another end of the same type (`crosses other.x`), reaching what that end relates. " +
			"Chain through an opposite end feature.",
		Spec: "KerML 1.0 validateCrossSubsettingCrossedFeature",
	},
	{
		Code: "cross-feature-type", Tier: TierConstraint, Severity: "error",
		Summary: "a cross feature typed differently from its end feature",
		Meaning: "The cross feature of an end has the same type as the end feature it belongs to. " +
			"Type both alike.",
		Spec: "KerML 1.0 validateFeatureCrossFeatureType",
	},
	{
		Code: "cross-feature-specialization", Tier: TierConstraint, Severity: "error",
		Summary: "a cross feature that does not specialize the redefined ends' cross features",
		Meaning: "When an end redefines another end, its cross feature specializes the redefined end's cross feature. " +
			"Subset the redefined cross feature, or drop the redefinition.",
		Spec: "KerML 1.0 validateFeatureCrossFeatureSpecialization",
	},
	{
		Code: "declared-cross-feature", Tier: TierConstraint, Severity: "error",
		Summary: "an explicitly declared cross feature that is not the end's",
		Meaning: "A feature declared as the cross feature of an end must be the one the end's cross subsetting derives. " +
			"Let the cross feature be derived, or declare the one `crosses` names.",
		Spec: "KerML 1.0 validateFeatureCrossFeature",
	},
	{
		Code: "variable-feature-owner", Tier: TierConstraint, Severity: "error",
		Summary: "a variable (`var`) feature owned by a non-occurrence type",
		Meaning: "A feature that varies over time (`var`) is owned by an occurrence type, whose instances have a lifetime for it to vary over. " +
			"Drop `var`, or move the feature into an occurrence, item or part definition.",
		Spec: "KerML 1.0 validateFeatureIsVariable",
	},
	{
		Code: "constant-feature-not-variable", Tier: TierConstraint, Severity: "error",
		Summary: "`const` on a feature that is not variable",
		Meaning: "`const` fixes a variable feature's value for its lifetime; a feature that is not `var` is already constant. " +
			"Drop `const`, or declare the feature `var const`.",
		Spec: "KerML 1.0 validateFeatureConstantIsVariable",
	},
	{
		Code: "feature-portion-not-variable", Tier: TierConstraint, Severity: "error",
		Summary: "a portion feature that is variable",
		Meaning: "A portion of an occurrence cannot be `var`: a portion is fixed for the occurrence it is part of. " +
			"Drop `var` or `portion`.",
		Spec: "KerML 1.0 validateFeaturePortionNotVariable",
	},
	{
		Code: "initial-value-not-variable", Tier: TierConstraint, Severity: "error",
		Summary: "an initial value (`:=`) on a feature that is not variable",
		Meaning: "An initial value is the value a variable feature starts with; a feature that is not `var` has one value for its lifetime. " +
			"Write `=` for a fixed value, or declare the feature `var`.",
		Spec: "KerML 1.0 validateFeatureValueIsInitial",
	},
	{
		Code: "feature-value-overriding", Tier: TierConstraint, Severity: "error",
		Summary: "a redefinition giving a value to a feature bound with `=`",
		Meaning: "A feature bound with `=` is fixed; a feature that redefines it, directly or through further redefinitions, cannot give it another value. Only `default =` values may be overridden. " +
			"Write `default =` on the redefined feature, or drop the value on the redefining one.",
		Spec: "KerML 1.0 §8.3.4.10.2 validateFeatureValueOverriding",
	},
	{
		Code: "reference-subsetting-at-most-one", Tier: TierConstraint, Severity: "error",
		Summary: "a feature with more than one reference subsetting",
		Meaning: "A feature owns at most one reference subsetting (`::>` or `references`). " +
			"Keep one.",
		Spec: "KerML 1.0 validateFeatureOwnedReferenceSubsetting",
	},
	{
		Code: "related-elements", Tier: TierConstraint, Severity: "error",
		Summary: "an association with fewer than two related types",
		Meaning: "An association relates at least two types, one per end. " +
			"Declare two or more ends.",
		Spec: "KerML 1.0 validateAssociationRelatedTypes",
	},
	{
		Code: "result-expression-at-most-one", Tier: TierConstraint, Severity: "error",
		Summary: "a function or expression with more than one result expression",
		Meaning: "A function, expression or calculation body ends in at most one result expression, owned or inherited; this one has several. " +
			"Keep one result, and bind the others to `out` parameters.",
		Spec: "KerML 1.0 validateFunctionResultExpressionMembership and validateExpressionResultExpressionMembership",
	},
	{
		Code: "return-parameter-owner", Tier: TierConstraint, Severity: "error",
		Summary: "`return` outside a function, expression or calculation",
		Meaning: "Only a function or an expression — a calculation or a constraint in SysML — declares a result parameter; an action, a part or a package has none. " +
			"Write `out` for an output, or make the owner a calculation.",
		Spec: "KerML 1.0 validateReturnParameterMembershipOwningType",
	},
	{
		Code: "import-top-level-visibility", Tier: TierConstraint, Severity: "error",
		Summary: "a top-level import that is not private",
		Meaning: "An import owned by the root namespace of a file is private: there is no namespace for it to re-export into. " +
			"Write `private import`, or move the import into a package.",
		Spec: "KerML 1.0 validateImportTopLevelVisibility",
	},
	{
		Code: "metadata-annotated-element", Tier: TierConstraint, Severity: "error",
		Summary: "metadata annotating an element its metaclass does not admit",
		Meaning: "A metadata definition may restrict, through its `annotatedElement` feature, what kind of element it annotates; this usage is about an element of another kind. " +
			"Annotate an element the metaclass admits, or widen the definition's `annotatedElement`.",
		Spec: "KerML 1.0 validateMetadataFeatureAnnotatedElement",
	},
	{
		Code: "metadata-metaclass", Tier: TierConstraint, Severity: "error",
		Summary: "a metadata usage typed by none or several metaclasses",
		Meaning: "A metadata usage (`@M` or `metadata m : M`) is typed by exactly one metadata definition or metaclass. " +
			"Type it by one.",
		Spec: "KerML 1.0 validateMetadataFeatureMetaclass",
	},
	{
		Code: "metadata-concrete-type", Tier: TierConstraint, Severity: "error",
		Summary: "a metadata usage typed by an abstract metaclass",
		Meaning: "A metadata usage is typed by a concrete metadata definition: an abstract one has no instances to annotate with. " +
			"Type it by a concrete specialization.",
		Spec: "KerML 1.0 validateMetadataFeatureMetaclassNotAbstract",
	},
	{
		Code: "metadata-body-feature", Tier: TierConstraint, Severity: "error",
		Summary: "a metadata body member redefining no feature of the metaclass",
		Meaning: "Each feature in the body of a metadata usage redefines a feature of its metadata definition, giving it a value. " +
			"Redefine (`:>>`) a feature the metaclass declares.",
		Spec: "SysML v2 validateMetadataUsageBodyFeature and KerML 1.0 validateMetadataFeatureBody",
	},
	{
		Code: "metadata-owning-type-feature", Tier: TierConstraint, Severity: "error",
		Summary: "a metadata body feature that is not owned by the metaclass",
		Meaning: "A feature in a metadata body redefines a feature the metadata definition itself owns or inherits; this one names a feature of another type. " +
			"Redefine a feature of the metaclass.",
		Spec: "KerML 1.0 validateMetadataFeatureBody",
	},
	{
		Code: "metadata-value-not-evaluable", Tier: TierConstraint, Severity: "error",
		Summary: "a metadata feature value that is not model-level evaluable",
		Meaning: "The value of a feature in a metadata body is evaluated over the model itself, so it is a literal, a metadata feature reference or an operator over those; this one calls or reads something evaluation cannot reach. " +
			"Write a model-level evaluable value.",
		Spec: "KerML 1.0 validateMetadataFeatureBody and §8.3.4.8.7 (model-level evaluability)",
	},
	{
		Code: "part-usage-part-definition", Tier: TierConstraint, Severity: "error",
		Summary: "a part usage typed by no part definition",
		Meaning: "A part is typed by at least one part definition, directly or through what it subsets; an item definition alone does not make a part. " +
			"Type the part by a part definition.",
		Spec: "SysML v2 validatePartUsagePartDefinition",
	},
	{
		Code: "portion-owner", Tier: TierConstraint, Severity: "error",
		Summary: "a portion (snapshot or timeslice) owned by a non-occurrence",
		Meaning: "A `snapshot` or `timeslice` is a portion of the occurrence that owns it. " +
			"Declare it in an occurrence definition or usage.",
		Spec: "SysML v2 validateOccurrenceUsageIsPortion",
	},
	{
		Code: "only-one-subject", Tier: TierConstraint, Severity: "error",
		Summary: "a requirement or case with more than one subject",
		Meaning: "A requirement or case definition or usage has one subject, owned or inherited. " +
			"Keep one `subject`, redefining an inherited one where its type is to change.",
		Spec: "SysML v2 validateRequirementDefinitionOnlyOneSubject, validateRequirementUsageOnlyOneSubject, validateCaseDefinitionOnlyOneSubject, validateCaseUsageOnlyOneSubject",
	},
	{
		Code: "only-one-objective", Tier: TierConstraint, Severity: "error",
		Summary: "a case with more than one objective",
		Meaning: "A case definition or usage has one objective. " +
			"Keep one `objective`, merging the requirements into it.",
		Spec: "SysML v2 validateCaseDefinitionOnlyOneObjective and validateCaseUsageOnlyOneObjective",
	},
	{
		Code: "subject-parameter-position", Tier: TierConstraint, Severity: "error",
		Summary: "a subject that is not the first parameter",
		Meaning: "The subject of a requirement or case is its first parameter; here another `in` or `out` parameter is declared before it. " +
			"Declare the subject before the other parameters.",
		Spec: "SysML v2 validateRequirementDefinitionSubjectParameterPosition, validateRequirementUsageSubjectParameterPosition, validateCaseDefinitionSubjectParameterPosition, validateCaseUsageSubjectParameterPosition",
	},
	{
		Code: "verification-owning-type", Tier: TierConstraint, Severity: "error",
		Summary: "`verify` outside the objective of a verification case",
		Meaning: "A requirement verification belongs in the objective of a verification case definition or usage. " +
			"Move the `verify` into the objective, or write a `satisfy`.",
		Spec: "SysML v2 validateRequirementVerificationMembershipOwningType",
	},
	{
		Code: "variant-outside-variation", Tier: TierConstraint, Severity: "error",
		Summary: "a variant that is not an owned member of a variation",
		Meaning: "`variant` marks a member of a variation definition or usage; this one is declared elsewhere, or in an enumeration definition, whose enumerated values are variants already. " +
			"Move the member into the variation, or drop the keyword.",
		Spec: "SysML v2 validateVariationMembershipOwningNamespace",
	},
	{
		Code: "variation-member-not-variant", Tier: TierConstraint, Severity: "error",
		Summary: "a variation owning a usage that is no variant",
		Meaning: "Every owned usage of a variation definition or usage is a variant (an enumeration definition's every member an enumerated value). " +
			"Write `variant` before the usage, or declare it in a definition the variation specializes.",
		Spec: "SysML v2 validateDefinitionVariationMembership and validateUsageVariationMembership",
	},
	{
		Code: "variation-specialization", Tier: TierConstraint, Severity: "error",
		Summary: "a variation specializing another variation",
		Meaning: "A variation does not specialize another variation; an enumeration definition, a variation by nature, does not specialize another enumeration. " +
			"Specialize a non-variation definition, and declare the variants directly.",
		Spec: "SysML v2 validateDefinitionVariationSpecialization and validateUsageVariationSpecialization",
	},
	{
		Code: "flow-end-subsetting", Tier: TierConstraint, Severity: "error",
		Summary: "a flow end that does not name the feature the payload flows from or to",
		Meaning: "Each end of a flow names an item or port feature the payload leaves or reaches, with dot notation for a feature of a part (`flow from a.out to b.in`); this end names a part itself, or a feature not reachable from the flow's context. " +
			"Name the flowing feature with dot notation.",
		Spec: "SysML v2 validateFlowEndSubsetting and validateFlowEndImplicitSubsetting",
	},
	{
		Code: "port-conjugation", Tier: TierConstraint, Severity: "warning",
		Summary: "an interface joining two ports whose directed features are not conjugate",
		Meaning: "The two ends of an interface are typed by ports whose `in` and `out` features point the same way, so what one end sends the other cannot receive. " +
			"Type one end by the conjugate port (`~P`).",
		Spec: "SysML v2 §7.12.2 (port conjugation)",
	},
	{
		Code: "view-satisfy-viewpoint", Tier: TierConstraint, Severity: "error",
		Summary: "`satisfy` in a view body naming no viewpoint",
		Meaning: "A view satisfies viewpoints, which frame the concerns it is to address; the element named is a requirement or something else. " +
			"Name a viewpoint usage.",
		Spec: "SysML v2 §7.24 (views satisfy viewpoints)",
	},
	{
		Code: "only-one-view-rendering", Tier: TierConstraint, Severity: "error",
		Summary: "a view with more than one rendering",
		Meaning: "A view definition or usage has at most one view rendering. " +
			"Keep one `render`.",
		Spec: "SysML v2 validateViewDefinitionOnlyOneViewRendering and validateViewUsageOnlyOneRendering",
	},
	{
		Code: "assignment-referent", Tier: TierConstraint, Severity: "error",
		Summary: "an assignment whose target is no feature",
		Meaning: "`assign x := v` writes a feature; `x` resolves to a definition, a package or another non-feature. " +
			"Assign to a usage.",
		Spec: "SysML v2 validateAssignmentActionUsageReferent",
	},
	{
		Code: "assignment-referent-read-only", Tier: TierConstraint, Severity: "error",
		Summary: "an assignment to a constant or derived feature",
		Meaning: "The assigned feature is `const`, `derived` or bound with `=`, so a run cannot write it. " +
			"Assign to a variable feature, or declare the feature `var`.",
		Spec: "SysML v2 validateAssignmentActionUsageReferentIsTimeVarying",
	},
	{
		Code: "assignment-referent-time-varying", Tier: TierConstraint, Severity: "error",
		Summary: "an assignment to a feature that is not time-varying",
		Meaning: "The target of an assignment is a time-varying feature: a `var` feature, or a feature of an occurrence that can change over its lifetime. " +
			"Declare the feature `var`.",
		Spec: "SysML v2 validateAssignmentActionUsageReferentIsTimeVarying",
	},
	{
		Code: "assignment-chain-in-calc", Tier: TierConstraint, Severity: "error",
		Summary: "a calculation assigning a feature of another object",
		Meaning: "A calculation computes a result and has no side effects; an assignment whose target chains into another object (`assign a.x := v`) would write that object. " +
			"Move the assignment into an action, or compute the value as the calculation's result.",
		Spec: "KerML 1.0 §8.4.4.9 (functions have no side effects)",
	},
	{
		Code: "assignment-chain-step-not-one-object", Tier: TierConstraint, Severity: "error",
		Summary: "an assignment target stepping through a feature that may hold several objects",
		Meaning: "`assign a.x := v` writes `x` of the one object `a` holds; a feature with multiplicity above 1 in the chain leaves it unclear which object is written. " +
			"Index the step (`a#(1).x`) or assign within an action performed per element.",
		Spec: "SysML v2 §7.17 AssignmentActionUsage",
	},
	{
		Code: "identity-duplicate-id", Tier: TierConstraint, Severity: "error",
		Summary: "two elements with the same effective element id",
		Meaning: "An `ElementId` annotation gives an element the identity it keeps across interchange; two elements here resolve to one id, so an API or XMI reader cannot tell them apart. " +
			"Give each element its own id.",
		Spec: specElementIdentity,
	},
	{
		Code: "identity-conflicting-ids", Tier: TierConstraint, Severity: "error",
		Summary: "one element annotated with several different ids",
		Meaning: "Several `ElementId` annotations about one element disagree. " +
			"Keep one id.",
		Spec: specElementIdentity,
	},
	{
		Code: "identity-conflicting-projects", Tier: TierConstraint, Severity: "error",
		Summary: "one element placed in several projects",
		Meaning: "Several `ProjectRef` annotations about one element name different projects. " +
			"Keep one project reference.",
		Spec: "SysML v2 API and Services (projects)",
	},
	{
		Code: "identity-id-shape", Tier: TierConstraint, Severity: "error",
		Summary: "an element id that is empty or holds a character outside [a-zA-Z0-9_-]",
		Meaning: "An element id is a non-empty string of letters, digits, `_` and `-`, so it travels through URLs, XMI and the API unchanged. " +
			"Write an id of that shape.",
		Spec: specElementIdentity,
	},
	{
		Code: "identity-unscoped-id", Tier: TierConstraint, Severity: "error",
		Summary: "an element id with no enclosing project to resolve against",
		Meaning: "An `ElementId` is scoped by the project its element belongs to; this element lies under no `ProjectRef`. " +
			"Annotate an enclosing namespace with `ProjectRef`.",
		Spec: "SysML v2 API and Services (projects)",
	},
	{
		Code: "diagram-layout-unplaced", Tier: TierConstraint, Severity: "warning",
		Summary: "a layout annotation on an element no rendering draws",
		Meaning: "A `Position`, `Size` or `Route` annotation places an element on a view, but no rendering of the view draws that element in the role the annotation assumes, so the layout applies to nothing. " +
			"Annotate an element the rendering draws, or expose it in the view.",
		Spec: specDiagramLayout,
	},
	{
		Code: "diagram-layout-value", Tier: TierConstraint, Severity: "error",
		Summary: "a layout annotation with an ill-formed value",
		Meaning: "The coordinates, size or route points of a layout annotation are not of the form the layout library declares (numbers in the view's units, a route of at least two points). " +
			"Write the value the message names.",
		Spec: specDiagramLayout,
	},
	{
		Code: "diagram-layout-canvas", Tier: TierConstraint, Severity: "error",
		Summary: "a Canvas or Picture annotation outside the view it belongs to",
		Meaning: "A `Canvas` sizes the view whose body it sits in, and a `Picture` is drawn on the surface of the view it annotates; this one annotates something that is no view, or sizes a view from outside its body. " +
			"Move the annotation into the view's body.",
		Spec: specDiagramLayout,
	},
	{
		Code: "diagram-layout-duplicate", Tier: TierConstraint, Severity: "warning",
		Summary: "a layout annotation stated twice about one element",
		Meaning: "Two annotations of one kind place the same element on the same view; the first stated applies. " +
			"Drop the second.",
		Spec: specDiagramLayout,
	},
	{
		Code: "control-node-owner", Tier: TierConstraint, Severity: "error",
		Summary: "a control node outside an action",
		Meaning: "A fork, join, merge or decision node is a member of an action definition or usage. " +
			"Declare it in the action whose flow it controls.",
		Spec: "SysML v2 validateControlNodeOwningType",
	},
	{
		Code: "control-node-incoming-multiplicity", Tier: TierConstraint, Severity: "error",
		Summary: "a succession into a control node with a target multiplicity other than 1",
		Meaning: "Each succession into a control node reaches exactly one occurrence of it. " +
			"Drop the multiplicity on the target end, or write `[1]`.",
		Spec: "SysML v2 validateControlNodeIncomingSuccessions",
	},
	{
		Code: "control-node-outgoing-multiplicity", Tier: TierConstraint, Severity: "error",
		Summary: "a succession out of a control node with a source multiplicity other than 1",
		Meaning: "Each succession out of a control node leaves exactly one occurrence of it. " +
			"Drop the multiplicity on the source end, or write `[1]`.",
		Spec: "SysML v2 validateControlNodeOutgoingSuccessions",
	},
	{
		Code: "fork-incoming-successions", Tier: TierConstraint, Severity: "error",
		Summary: "a fork node with more than one incoming succession",
		Meaning: "A fork splits one flow into several, so exactly one succession enters it. " +
			"Join the incoming flows first, or add a merge before the fork.",
		Spec: "SysML v2 validateForkNodeIncomingSuccessions",
	},
	{
		Code: "join-outgoing-successions", Tier: TierConstraint, Severity: "error",
		Summary: "a join node with more than one outgoing succession",
		Meaning: "A join waits for several flows and continues as one, so exactly one succession leaves it. " +
			"Fork after the join where several flows are meant.",
		Spec: "SysML v2 validateJoinNodeOutgoingSuccessions",
	},
	{
		Code: "merge-incoming-multiplicity", Tier: TierConstraint, Severity: "error",
		Summary: "a succession into a merge node with a source multiplicity other than 0..1",
		Meaning: "A merge continues when any one of its incoming flows arrives, so each incoming succession has a source multiplicity of `[0..1]`. " +
			"Drop the source multiplicity, or write `[0..1]`.",
		Spec: "SysML v2 validateMergeNodeIncomingSuccessions",
	},
	{
		Code: "merge-outgoing-successions", Tier: TierConstraint, Severity: "error",
		Summary: "a merge node with more than one outgoing succession",
		Meaning: "A merge continues as one flow, so exactly one succession leaves it. " +
			"Fork after the merge where several flows are meant.",
		Spec: "SysML v2 validateMergeNodeOutgoingSuccessions",
	},
	{
		Code: "decision-incoming-successions", Tier: TierConstraint, Severity: "error",
		Summary: "a decision node with more than one incoming succession",
		Meaning: "A decision chooses one of several outgoing flows for one incoming flow, so exactly one succession enters it. " +
			"Merge the incoming flows before the decision.",
		Spec: "SysML v2 validateDecisionNodeIncomingSuccessions",
	},
	{
		Code: "decision-outgoing-multiplicity", Tier: TierConstraint, Severity: "error",
		Summary: "a succession out of a decision node with a target multiplicity other than 0..1",
		Meaning: "A decision takes at most one of its outgoing flows, so each outgoing succession has a target multiplicity of `[0..1]`. " +
			"Drop the target multiplicity, or write `[0..1]`.",
		Spec: "SysML v2 validateDecisionNodeOutgoingSuccessions",
	},
	{
		Code: "endpoint-not-a-node", Tier: TierConstraint, Severity: "error",
		Summary: "a succession endpoint in an action that names no action node",
		Meaning: "The source or target of a succession (`first a then b`) in an action body names an action node: an action usage, a control node, an accept or send, or `start`/`done`; this one resolves to an attribute, a definition or another element. " +
			"Name an action node of the enclosing action.",
		Spec: "SysML v2 §7.17 (successions between action nodes)",
	},
	{
		Code: "endpoint-not-of-machine", Tier: TierConstraint, Severity: "error",
		Summary: "a transition endpoint naming a vertex of another state machine",
		Meaning: "A transition's source and target are states or pseudostates of the state machine the transition belongs to; this endpoint names a vertex another state definition or usage owns. " +
			"Name a vertex of the enclosing state, or move the transition.",
		Spec: "UML 2.5.1 §14.2.3.9 (transitions relate vertices of one region), SysML v2 §7.19",
	},
	{
		Code: "no-outgoing-transition", Tier: TierConstraint, Severity: "error",
		Summary: "a routing pseudostate no transition leaves",
		Meaning: "A choice, fork, join or junction pseudostate routes a transition on; one no transition leaves terminates every transition reaching it nowhere. " +
			"Add an outgoing transition, or remove the pseudostate.",
		Spec: "UML 2.5.1 §15.7.18 (pseudostates), SysML v2 §7.19",
	},
	{
		Code: "parallel-state-transition", Tier: TierConstraint, Severity: "error",
		Summary: "a transition or succession between the direct substates of a parallel state",
		Meaning: "The direct substates of a `parallel` state run concurrently, so no transition or succession orders them. " +
			"Drop the transition, or nest the ordered states in one region (a non-parallel substate).",
		Spec: "SysML v2 validateStateDefinitionParallelSubactions and validateStateUsageParallelSubactions",
	},
	{
		Code: "accepter-source-not-state", Tier: TierConstraint, Severity: "error",
		Summary: "a transition with a trigger whose source is not a state",
		Meaning: "A transition's accepter waits in its source for the trigger, which a state can do and a pseudostate cannot. " +
			"Move the trigger to a transition out of a state, or drop it from the pseudostate's transition.",
		Spec: "SysML v2 validateTransitionUsageTriggerActions",
	},
	{
		Code: "no-transition-source", Tier: TierConstraint, Severity: "error",
		Summary: "a transition written without a source that no member precedes",
		Meaning: "`then <target>;` takes the member declared before it in the body as its source; this one is the first member, so nothing precedes it. " +
			"Write the source (`transition first s then t;`), or declare the source state before it.",
		Spec: "SysML v2 §7.18.3 TargetTransitionUsage",
	},
	{
		Code: "transition-source-not-vertex", Tier: TierConstraint, Severity: "error",
		Summary: "a transition written without a source whose preceding member is no vertex",
		Meaning: "`then <target>;` takes the member declared before it as its source, and that member is an action, an attribute or another non-vertex of the state machine. " +
			"Write the source explicitly, or declare the transition right after the state it leaves.",
		Spec: "SysML v2 §7.18.3 TargetTransitionUsage",
	},
	{
		Code: "entry-transition-shape", Tier: TierConstraint, Severity: "error",
		Summary: "an entry transition with a trigger or an effect",
		Meaning: "A transition out of the entry action (`entry; then s;`) chooses the starting state by its guard alone: it accepts no trigger and performs no effect. " +
			"Drop the trigger and effect, or start in a state whose transitions carry them.",
		Spec: "SysML v2 §7.18.3 EntryTransitionMember",
	},
	{
		Code: "entry-transition-target", Tier: TierConstraint, Severity: "error",
		Summary: "an entry transition targeting something the body cannot start in",
		Meaning: "A transition out of the entry action targets a state of the body; this one targets a pseudostate or an action the state machine cannot start in. " +
			"Target a substate.",
		Spec: "SysML v2 §7.18.3 EntryTransitionMember",
	},
	{
		Code: "send-payload-missing", Tier: TierConstraint, Severity: "error",
		Summary: "a send as a state subaction or transition effect with no payload",
		Meaning: "A send written as an entry, do or exit action or as a transition effect must state what it sends (`send Sig() to x`); without a payload there is no signal to deliver. " +
			"Write the payload expression.",
		Spec: "SysML v2 validateSendActionUsagePayloadArgument",
	},
	{
		Code: "send-to-port", Tier: TierConstraint, Severity: "warning",
		Summary: "`send ... to <port>`, where `via` is meant",
		Meaning: "`to` names the occurrence that receives a signal; a port is the path it travels, which `via` names. " +
			"Write `send Sig() via port`.",
		Spec: "SysML v2 validateSendActionUsageReceiver",
	},
	{
		Code: "send-receiver-not-occurrence", Tier: TierConstraint, Severity: "warning",
		Summary: "a send whose receiver is no occurrence",
		Meaning: "`send ... to x` delivers to an occurrence; `x` is typed by an attribute definition or another non-occurrence, which cannot accept a signal. " +
			"Name a part, action or other occurrence.",
		Spec: "SysML v2 validateSendActionUsageReceiver",
	},
	{
		Code: "send-sender-not-occurrence", Tier: TierConstraint, Severity: "warning",
		Summary: "a send whose sender is no occurrence",
		Meaning: "`send ... from x` routes the signal through an occurrence of the sender; `x` is typed by a non-occurrence. " +
			"Name a part, action or other occurrence, or drop the `from` clause.",
		Spec: "SysML v2 §7.17 SendActionUsage",
	},
	{
		Code: "succession-order-cycle", Tier: TierConstraint, Severity: "error",
		Summary: "successions that order performances in a cycle",
		Meaning: "Following the successions of the body leads from a performance back to itself, so no order satisfies them and a run of the body would wait forever. " +
			"Drop one succession, or make the loop explicit (`loop`, `while`).",
		Spec: "KerML 1.0 §7.4.6 (successions are a partial order of occurrences)",
	},
	{
		Code: "succession-orders-nothing", Tier: TierConstraint, Severity: "warning",
		Summary: "a succession whose end is no performance of the run",
		Meaning: "One end of the succession names a requirement, a constraint or another usage whose evaluations are checked as verdicts rather than performed as steps, so the succession orders nothing a run does. " +
			"Order actions, or drop the succession.",
		Spec: "KerML 1.0 §7.4.6 Succession",
	},
}
