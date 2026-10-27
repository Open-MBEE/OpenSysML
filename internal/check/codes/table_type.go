package codes

// specStateSubactionKind is the constraint pair the state subaction codes cite.
const specStateSubactionKind = "SysML v2 validateStateDefinitionSubactionKind and validateStateUsageSubactionKind"

// typeCodes are the codes of the typing tier: what a declaration may be typed
// by, reference or specialize, and what an expression evaluates to.
var typeCodes = []Entry{
	{
		Code: "usage-typing", Tier: TierType, Severity: "error",
		Summary: "a usage typed by something other than a definition of its kind",
		Meaning: "A usage is typed by definitions of its own kind: a part by part definitions, an attribute by attribute definitions, an action by action definitions, and so on; the type written is of another kind or is not a definition. " +
			"Type the usage by a definition of its kind, or change the usage keyword to the kind of the type.",
		Spec: "SysML v2 validateUsageType and the per-kind constraints (validatePartUsageType, validateAttributeUsageType, validateActionUsageType …)",
	},
	{
		Code: "one-type", Tier: TierType, Severity: "error",
		Summary: "a usage that admits one type declared with several",
		Meaning: "A calculation, case, view, viewpoint, rendering, requirement or enumeration usage is typed by exactly one definition, and this one declares more. " +
			"Keep the one definition meant; where two are needed, declare a definition that specializes both and type the usage by it.",
		Spec: "SysML v2 validateCalculationUsageType, validateCaseUsageType, validateViewUsageType, validateRequirementUsageType, validateEnumerationUsageType …",
	},
	{
		Code: "usage-reference-kind", Tier: TierType, Severity: "error",
		Summary: "a reference form (`perform`, `exhibit`, `include`, `satisfy`, `assert`) naming the wrong kind of usage",
		Meaning: "Each reference form admits one kind of referent: `perform` an action, `exhibit` a state, `include` a use case, `satisfy` a requirement, `assert` a constraint; the element named is of another kind. " +
			"Name a usage of the kind the form requires, or use the form that fits the element.",
		Spec: "SysML v2 validatePerformActionUsageReference, validateExhibitStateUsageReference, validateIncludeUseCaseUsageReference, validateSatisfyRequirementUsageReference, validateAssertConstraintUsageReference",
	},
	{
		Code: "relationship-target-kind", Tier: TierType, Severity: "error",
		Summary: "a specialization, subsetting, redefinition or reference whose target is of the wrong kind",
		Meaning: "The element named after `:>`, `:>>`, `::>` or `references` is not of a kind the relationship admits for this declaration: a definition specializing a usage, a usage subsetting a definition, a reference to a package. " +
			"Name an element of the kind the declaration relates to, or change the relationship keyword.",
		Spec: "KerML 1.0 §8.3.3 (specialization relates types to types, subsetting features to features)",
	},
	{
		Code: "relationship-end-kind", Tier: TierType, Severity: "error",
		Summary: "a connector, flow, allocation or succession end that names the wrong kind of element",
		Meaning: "An end of a relationship usage (`connect`, `flow`, `allocate`, `succession` …) names an element that is not of the kind the relationship relates: a definition where a usage is required, an attribute where an occurrence is. " +
			"Name a usage of the required kind, nested with dot notation where it sits inside another.",
		Spec: "SysML v2 §7.13 (connections) and §7.17 (successions) end rules",
	},
	{
		Code: "feature-chain-segment", Tier: TierType, Severity: "error",
		Summary: "a feature chain stepping through something that is not a feature",
		Meaning: "Every segment of a feature chain (`a.b.c`) names a feature of what precedes it; one segment resolves to a definition, a package or another non-feature. " +
			"Chain through usages only, and qualify the first segment (`A::a.b`) where a definition's member is meant.",
		Spec: "KerML 1.0 §8.3.3.3 feature chaining",
	},
	{
		Code: "subsetting-feature", Tier: TierType, Severity: "error",
		Summary: "a subsetting or redefinition whose target is not a feature",
		Meaning: "Subsetting and redefinition relate features; the target written after `:>` or `:>>` is a classifier, a definition or another non-feature. " +
			"Name a feature (a usage) as the target, or write a specialization (`:>`) between definitions.",
		Spec: "KerML 1.0 §8.3.3.3 Subsetting and Redefinition",
	},
	{
		Code: "conjugated-port-definition", Tier: TierType, Severity: "error",
		Summary: "`~` applied to something other than a port definition, or typing something other than a port",
		Meaning: "`~P` names the conjugated port definition of the port definition `P`, and only a port usage or a connector end may be typed by it. " +
			"Apply `~` to a port definition, and type ports alone by the conjugate.",
		Spec: "SysML v2 §7.12.2 ConjugatedPortDefinition",
	},
	{
		Code: "specialization-kind", Tier: TierType, Severity: "error",
		Summary: "a KerML classifier specializing one of another family",
		Meaning: "A class specializes classes, a data type data types, a structure structures, a behavior behaviors, an association associations; the general type written belongs to another family. " +
			"Specialize a classifier of the same family, or change the declaration's kind.",
		Spec: "KerML 1.0 validateClassSpecialization, validateDataTypeSpecialization, validateStructureSpecialization, validateBehaviorSpecialization, validateAssociationRelatedTypes",
	},
	{
		Code: "specialization-specific-conjugated", Tier: TierType, Severity: "error",
		Summary: "a conjugated type that specializes",
		Meaning: "A conjugated type (`~T`) derives its features from the type it conjugates and cannot itself be the specific type of a specialization. " +
			"Specialize the original type, or declare a type that conjugates the general one.",
		Spec: "KerML 1.0 validateSpecializationSpecificNotConjugated",
	},
	{
		Code: "individual-typing", Tier: TierType, Severity: "error",
		Summary: "an individual usage typed by none or several individual definitions",
		Meaning: "An individual occurrence usage is typed by exactly one individual definition, which names the individual it is a portion or snapshot of. " +
			"Type the usage by one `individual` definition.",
		Spec: "SysML v2 validateOccurrenceUsageIndividualDefinition",
	},
	{
		Code: "occurrence-usage-type", Tier: TierType, Severity: "error",
		Summary: "an occurrence, item or part typed by a non-occurrence definition",
		Meaning: "An occurrence usage (an item, part, action or state included) is typed by occurrence definitions; the type written is an attribute definition, an enumeration or another classifier that is no occurrence. " +
			"Type the usage by an occurrence, item or part definition.",
		Spec: "SysML v2 validateOccurrenceUsageType",
	},
	{
		Code: "event-reference-occurrence", Tier: TierType, Severity: "error",
		Summary: "an event occurrence referencing something that is no occurrence",
		Meaning: "`event occurrence e references x` names an occurrence usage; `x` resolves to an attribute, a definition or another element that is no occurrence. " +
			"Reference an occurrence usage (a part, action, state or occurrence).",
		Spec: "SysML v2 validateEventOccurrenceUsageReferent",
	},
	{
		Code: "instantiation-not-type", Tier: TierType, Severity: "error",
		Summary: "`new X(...)` where `X` is not a type",
		Meaning: "An instantiation expression names the type whose instance it constructs; `X` resolves to a feature, a package or another non-type. " +
			"Name a definition or classifier after `new`.",
		Spec: "KerML 1.0 §8.3.4.8 InstantiationExpression (Must have an invoked/instantiated type)",
	},
	{
		Code: "invocation-not-behavior", Tier: TierType, Severity: "error",
		Summary: "a call of something that is neither a behavior nor a behavioral feature",
		Meaning: "`f(x)` invokes a function, a calculation, an action or a behavioral feature; `f` resolves to an attribute, a part or a definition that is not a behavior. " +
			"Call a calculation, function or action, or write a feature reference where no call is meant.",
		Spec: "KerML 1.0 §8.3.4.8 InvocationExpression",
	},
	{
		Code: "invocation-ambiguous", Tier: TierType, Severity: "warning",
		Summary: "a call whose name several behaviors answer to",
		Meaning: "The callee's name resolves to more than one calculation or function visible at the call, so which one the arguments bind to depends on the resolution order. " +
			"Qualify the callee (`Pkg::f(x)`), or rename one of the candidates.",
		Spec: "",
	},
	{
		Code: "invocation-argument", Tier: TierType, Severity: "error",
		Summary: "an argument of a call that binds to no input parameter",
		Meaning: "A call's positional arguments bind to the callee's `in` parameters in order and its named arguments by name; this argument has no parameter to bind to, names one twice, or names one that is not an input. " +
			"Match the arguments to the callee's parameters, listed in the message.",
		Spec: "KerML 1.0 §8.3.4.8.8 (invocation argument binding)",
	},
	{
		Code: "unbound-parameter", Tier: TierType, Severity: "warning",
		Summary: "a call leaving a parameter without default unbound",
		Meaning: "The callee has an `in` parameter with no default value that the call does not bind, so the call is well formed but cannot be evaluated. " +
			"Pass the argument, or give the parameter a default.",
		Spec: "KerML 1.0 §8.3.4.8.8",
	},
	{
		Code: "constructor-argument", Tier: TierType, Severity: "error",
		Summary: "an argument of `new X(...)` that fits no feature of `X`",
		Meaning: "The arguments of a constructor expression bind to the features of the instantiated type, positional ones in order and named ones by name; this one names no feature of the type, is one too many, or has a value the feature's type does not admit. " +
			"Bind each argument to a feature of the type with a value of its type.",
		Spec: "KerML 1.0 §8.3.4.8 (constructor expressions)",
	},
	{
		Code: "constructor-expression-no-duplicate-feature-redefinition", Tier: TierType, Severity: "error",
		Summary: "`new X(...)` binding one feature twice",
		Meaning: "A constructor expression binds each feature of the instantiated type at most once; a named argument repeats a feature a positional or earlier named argument already bound. " +
			"Drop the repeated binding.",
		Spec: "KerML 1.0 validateConstructorExpressionNoDuplicateFeatureRedefinition",
	},
	{
		Code: "binding-connector-type-conformance", Tier: TierType, Severity: "error",
		Summary: "a value bound to a feature whose type does not admit it",
		Meaning: "A feature value (`x : Integer = \"a\"`), a default and an initial value alike, binds the value to the feature, and a feature's values are instances of its types: here the value's type does not conform to the feature's. " +
			"Write a value of the feature's type, convert it with a library function, or type the feature by what the value is.",
		Spec: "KerML 1.0 validateBindingConnectorTypeConformance and §8.3.3.3.4 (a feature's values are instances of its types); a `*` value is a Positive (checkLiteralInfinitySpecialization)",
	},
	{
		Code: "feature-value-count", Tier: TierType, Severity: "error",
		Summary: "a feature value holding a number of elements its multiplicity excludes",
		Meaning: "The value bound to the feature is a sequence of a known length outside the feature's multiplicity: a list bound to a single-valued feature, or one element short of a lower bound. " +
			"Write a value within the multiplicity, or widen the multiplicity (`[*]`, `[0..1]`).",
		Spec: "KerML 1.0 §8.3.3.3.5 Multiplicity",
	},
	{
		Code: "unique-feature-values", Tier: TierType, Severity: "error",
		Summary: "a repeated element in the value of a unique feature",
		Meaning: "A feature is unique unless declared `nonunique`, and the value written repeats one element at two positions, which a unique feature cannot hold. " +
			"Drop the repetition, or declare the feature `nonunique` where repetition is meant.",
		Spec: "KerML 1.0 §7.3.4.3 (isUnique)",
	},
	{
		Code: "incommensurable-binding", Tier: TierType, Severity: "error",
		Summary: "a quantity bound to a feature typed by a quantity of another dimension",
		Meaning: "The value is a quantity of one kind (a length, say) and the feature is typed by a quantity value type of another (a mass), so no unit conversion relates them. " +
			"Write a value of the feature's quantity kind, or type the feature by the value's.",
		Spec: "SysML v2 Quantities and Units Domain Library (QuantityValue, MeasurementReference)",
	},
	{
		Code: "incommensurable-operands", Tier: TierType, Severity: "warning",
		Summary: "an operator combining quantities whose dimensions do not fit",
		Meaning: "`+`, `-` and comparisons take quantities of one kind; the operands are of different kinds, or one is a point on a measurement scale (a date, a temperature in °C) for which the operation has no meaning. " +
			"Convert one operand, take the difference of two scale points instead of their sum, or check the units written.",
		Spec: "SysML v2 Quantities and Units Domain Library",
	},
	{
		Code: "operator-operand-type", Tier: TierType, Severity: "error",
		Summary: "an operator applied to operands of a type it is not defined for",
		Meaning: "`not` takes a Boolean, `and`/`or`/`xor`/`implies` Booleans, `+`, `-`, `*`, `/`, `**` and `%` numbers (or quantities), and `+` also strings; one operand has another type, or the pair has no definition in the kernel function libraries. " +
			"Write operands of the type the operator takes, or call the library function meant.",
		Spec: "KerML 1.0 §9.4 Kernel Function Library (BaseFunctions, ScalarFunctions, BooleanFunctions …)",
	},
	{
		Code: "comparison-constant", Tier: TierType, Severity: "warning",
		Summary: "a comparison of two values whose types never meet",
		Meaning: "`==` or `!=` compares values of types with no common instance (a String with a Natural), so the result is always the same and the condition is likely a slip. " +
			"Compare values of one type, or convert one side.",
		Spec: "",
	},
	{
		Code: "sequence-index", Tier: TierType, Severity: "error",
		Summary: "a sequence index that is no Integer, or outside the sequence",
		Meaning: "`x#(i)` selects the i-th element of a sequence, counting from 1; the index written is not an Integer, is 0, or exceeds the sequence's known length. " +
			"Write an Integer index between 1 and the length.",
		Spec: "KerML 1.0 §8.3.4.8 (index expressions) and §9.4.4 SequenceFunctions",
	},
	{
		Code: "connector-end-index", Tier: TierType, Severity: "error",
		Summary: "a connector end selecting an element outside the feature's multiplicity",
		Meaning: "`connect a#(i) to b` selects the i-th element of `a`; the index is no Integer, is 0, or lies outside the multiplicity of the feature it selects from. " +
			"Index from 1 within the multiplicity, or connect the whole feature.",
		Spec: "SysML v2 §7.13 (connector end indexing)",
	},
	{
		Code: "condition-boolean", Tier: TierType, Severity: "error",
		Summary: "a condition that is not Boolean",
		Meaning: "The expression stands where a Boolean is required — a constraint or requirement body, `if`, `while`, `until`, a select predicate — and its type is another: a number, a string, a part. " +
			"Write a comparison or a Boolean feature, or call a predicate.",
		Spec: "KerML 1.0 §8.3.4.7 (a constraint's result is Boolean) and §8.3.4.8 ConditionalExpression",
	},
	{
		Code: "transition-feature-membership-guard-expression", Tier: TierType, Severity: "error",
		Summary: "a transition guard that is not Boolean",
		Meaning: "The `if` guard of a transition or succession must evaluate to a Boolean; this one yields a value of another type, or names a feature no Boolean can come from. " +
			"Compare the value (`if x > 3`) or name a Boolean feature.",
		Spec: "SysML v2 validateTransitionFeatureMembershipGuardExpression",
	},
	{
		Code: "trigger-after-duration", Tier: TierType, Severity: "error",
		Summary: "an `after` trigger whose delay is no DurationValue",
		Meaning: "`accept after d` waits for a duration; the delay written is a plain number or a quantity of another kind. " +
			"Write it with a duration unit (`after 5 [s]`) or name a feature typed by `DurationValue`.",
		Spec: "SysML v2 validateTriggerInvocationActionAfterArgument",
	},
	{
		Code: "trigger-at-time-instant", Tier: TierType, Severity: "error",
		Summary: "an `at` trigger whose time is no TimeInstantValue",
		Meaning: "`accept at t` waits for a time instant; the time written is a plain number or a quantity of another kind. " +
			"Name a feature typed by `TimeInstantValue` or a clock reading such as `TimeOf(this)`.",
		Spec: "SysML v2 validateTriggerInvocationActionAtArgument",
	},
	{
		Code: "trigger-when-boolean", Tier: TierType, Severity: "error",
		Summary: "a `when` trigger whose condition is not Boolean",
		Meaning: "`accept when c` waits for a condition to become true; the expression written is of another type. " +
			"Compare the value (`when x > 3`) or name a feature typed by Boolean.",
		Spec: "SysML v2 validateTriggerInvocationActionWhenArgument",
	},
	{
		Code: "filter-not-boolean", Tier: TierType, Severity: "error",
		Summary: "an element filter whose condition is not Boolean",
		Meaning: "A `filter` member or an import filter (`[@M]`) is a Boolean expression over each candidate element; this one yields another type. " +
			"Write a metadata test (`@M`) or a comparison that yields Boolean.",
		Spec: "KerML 1.0 validateElementFilterMembershipIsBoolean",
	},
	{
		Code: "filter-not-evaluable", Tier: TierType, Severity: "error",
		Summary: "an element filter that is not model-level evaluable",
		Meaning: "A filter condition is evaluated over the model itself, so it may use metadata tests, literals, the kernel's Boolean and comparison operators and metadata feature references only; this one calls or reads something evaluation cannot reach. " +
			"Restrict the condition to model-level evaluable expressions.",
		Spec: "KerML 1.0 validateElementFilterMembershipIsModelLevelEvaluable and §8.3.4.8.7 (model-level evaluability)",
	},
	{
		Code: "filter-not-evaluated", Tier: TierType, Severity: "warning",
		Summary: "an element filter OpenSysML does not evaluate",
		Meaning: "The filter condition is well formed but uses a form OpenSysML's model-level evaluator does not implement yet, so every candidate is kept as if the filter were absent. " +
			"Rewrite the condition with metadata tests and comparisons the evaluator covers, or expect the unfiltered members.",
		Spec: "",
	},
	{
		Code: "bracket-operator", Tier: TierType, Severity: "warning",
		Summary: "`x[i]` written as an index",
		Meaning: "In KerML `[` is the operator `BaseFunctions::'['`, which the kernel library leaves abstract: `x[i]` indexes nothing, and in SysML it reads a unit (`5 [m]`). " +
			"Index a sequence with `x#(i)`.",
		Spec: "KerML 1.0 validateOperatorExpressionBracket and §9.4.2 BaseFunctions",
	},
	{
		Code: "quantity-unit", Tier: TierType, Severity: "warning",
		Summary: "a quantity whose unit is no measurement reference",
		Meaning: "`5 [u]` makes a quantity whose unit `u` must be a measurement reference (a unit of the ISQ or SI libraries, or a feature typed by `MeasurementUnit`); `u` is something else. " +
			"Write a unit such as `[m]` or name a feature typed by a measurement reference.",
		Spec: "SysML v2 validateOperatorExpressionQuantity",
	},
	{
		Code: "cast-conformance", Tier: TierType, Severity: "warning",
		Summary: "a cast between unrelated types",
		Meaning: "`x as T` selects the values of `x` that are instances of `T`; the argument's type and `T` are unrelated, neither specializing the other, so the cast selects no value. " +
			"Cast to a type related to the argument's, or convert with a library function.",
		Spec: "KerML 1.0 §8.3.4.8 (cast expressions select conforming values)",
	},
	{
		Code: "interface-end-not-port", Tier: TierType, Severity: "error",
		Summary: "an interface end that is no port",
		Meaning: "The ends of an interface definition or usage are ports; this end names a part, an attribute or another usage. " +
			"Connect ports, or write a `connection` where arbitrary usages are joined.",
		Spec: "SysML v2 validateInterfaceDefinitionEnd and validateInterfaceUsageEnd",
	},
	{
		Code: "connection-def-end-not-usage", Tier: TierType, Severity: "error",
		Summary: "a connection definition end that is no usage",
		Meaning: "Every end feature of a connection, interface or allocation definition is a usage, the inherited ones included; " +
			"this definition inherits a KerML end feature from a kernel association such as Links::BinaryLink or Occurrences::HappensLink. " +
			"Specialize Connections::Connection or BinaryConnection instead, or declare usage ends that redefine the inherited ones.",
		Spec: "SysML v2.1 Beta 2 §8.3.13.3 (connectionEnd : Usage redefines associationEnd); pilot validateConnectionDefinitionEndsAreUsages",
	},
	{
		Code: "flow-def-end-not-usage", Tier: TierType, Severity: "error",
		Summary: "a flow definition end that is no usage",
		Meaning: "Every end feature of a flow definition is a usage, the inherited ones included; " +
			"this definition inherits a KerML end feature from a kernel association such as Transfers::Transfer. " +
			"Specialize Flows::Flow or Flows::Message instead, or declare usage ends that redefine the inherited ones.",
		Spec: "SysML v2.1 Beta 2 §8.3.16.2 (flowEnd : Usage redefines associationEnd); pilot validateFlowDefinitionEndsAreUsages",
	},
	{
		Code: "flow-def-too-many-ends", Tier: TierType, Severity: "error",
		Summary: "a flow definition with more than two ends",
		Meaning: "A flow definition relates a source and a target: it can have at most two ends. " +
			"Drop the extra ends, or declare a connection definition for an n-ary relationship.",
		Spec: "SysML v2 validateFlowDefinitionConnectionEnds",
	},
	{
		Code: "port-owned-usage-composite", Tier: TierType, Severity: "error",
		Summary: "a composite usage owned by a port",
		Meaning: "The usages a port definition or port usage owns, other than nested ports, are referential: a port exposes features of the part it belongs to rather than owning them. " +
			"Write `ref` before the usage, or declare it in the owning part.",
		Spec: "SysML v2 validatePortDefinitionOwnedUsagesNotComposite and validatePortUsageNestedUsagesNotComposite",
	},
	{
		Code: "variant-port-composite", Tier: TierType, Severity: "error",
		Summary: "a variant port that is composite",
		Meaning: "A port usage, a variant port included, is referential: it exposes rather than owns. " +
			"Write `ref port` or drop the composite prefix.",
		Spec: "SysML v2 validatePortUsageIsReference",
	},
	{
		Code: "state-entry-action", Tier: TierType, Severity: "error",
		Summary: "a state with more than one entry action",
		Meaning: "A state definition or usage declares at most one `entry` action. " +
			"Merge the entry actions into one, sequenced in its body.",
		Spec: specStateSubactionKind,
	},
	{
		Code: "state-do-action", Tier: TierType, Severity: "error",
		Summary: "a state with more than one do action",
		Meaning: "A state definition or usage declares at most one `do` action. " +
			"Merge the do actions into one, with parallel or sequenced steps in its body.",
		Spec: specStateSubactionKind,
	},
	{
		Code: "state-exit-action", Tier: TierType, Severity: "error",
		Summary: "a state with more than one exit action",
		Meaning: "A state definition or usage declares at most one `exit` action. " +
			"Merge the exit actions into one, sequenced in its body.",
		Spec: specStateSubactionKind,
	},
	{
		Code: "only-one-return-parameter", Tier: TierType, Severity: "error",
		Summary: "a calculation or function with more than one return parameter",
		Meaning: "A function, expression or calculation has one result parameter; a second `return` declares another. " +
			"Keep one return, and declare the other results as `out` parameters.",
		Spec: "KerML 1.0 validateFunctionResultParameterMembership and validateExpressionResultParameterMembership",
	},
	{
		Code: "bound-feature-types", Tier: TierType, Severity: "warning",
		Summary: "a binding connector joining features of unrelated types",
		Meaning: "`bind a = b` asserts the two features hold the same values, which needs types with common instances; the types written are unrelated, so the binding can hold only when both are empty. " +
			"Bind features of related types, or type one by the other's type.",
		Spec: "KerML 1.0 validateBindingConnectorTypeConformance",
	},
}
