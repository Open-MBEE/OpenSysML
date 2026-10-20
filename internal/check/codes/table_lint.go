package codes

// lintCodes are the lints: warnings about a model the specification accepts,
// each of which a surface may switch off by its code.
var lintCodes = []Entry{
	{
		Code: "undeclared-signal", Tier: TierLint, Severity: "warning",
		Summary: "`when <name>` naming no visible declaration and no sent signal",
		Meaning: "A transition's `accept when <name>` or `when <name>` trigger names something no declaration visible where it is written matches and no `send` in the model emits, so only a signal injected from outside by that name ever fires it — almost always a misspelling. " +
			"Name the signal's attribute or item definition, or declare it. Switch the lint off with `-disable-lint undeclared-signal`.",
		Spec: "",
	},
	{
		Code: "port-type-mismatch", Tier: TierLint, Severity: "warning",
		Summary: "a connection, interface or flow between ports of unrelated definitions",
		Meaning: "The two ports joined are typed by port definitions neither of which specializes or conjugates the other, so nothing relates what one sends to what the other receives. " +
			"Type one end by the conjugate (`~P`) of the other's definition, or relate the definitions. Switch the lint off with `-disable-lint port-type-mismatch`.",
		Spec: "",
	},
	{
		Code: "deferred-keeper-unmarked", Tier: TierLint, Severity: "warning",
		Summary: "an accept of a deferred signal not marked as the keeping accept",
		Meaning: "In a state annotated `MigrationMetadata::DeferredEvent`, the accept at the root of the do action that buffers the deferred signal is expected to be marked `MigrationMetadata::DeferredKeeper`, so the migration tooling and readers can tell the keeper from an ordinary accept. " +
			"Add the `@DeferredKeeper` annotation. Switch the lint off with `-disable-lint deferred-keeper-unmarked`.",
		Spec: "",
	},
	{
		Code: "rounded-real-literal", Tier: TierLint, Severity: "warning",
		Summary: "a decimal literal a Real feature holds rounded (opt-in)",
		Meaning: "A decimal literal bound to a feature typed by `Real` equals no binary64 value, so the feature holds the nearest double rather than the number written; exact comparisons against it may fail. " +
			"Use a `Rational` feature where exactness matters, or accept the rounding. This lint is off until enabled with `-enable-lint rounded-real-literal`.",
		Spec: "",
	},
}

// The extension libraries the method codes cite.
const (
	specMOSA  = "MOSA extension library"
	specOOSEM = "OOSEM extension library"
)

// methodCodes are the codes of the method rules: findings of the MOSA and OOSEM
// self-check passes, which read a model against a modeling method it declares.
var methodCodes = []Entry{
	{
		Code: "mosa-interface-no-standard", Tier: TierMethod, Severity: "warning",
		Summary: "a modular system interface conforming to no standard",
		Meaning: "The model declares standards, so MOSA expects every modular system interface to conform to one: a `#conformance` connection naming the interface at a `#conformant` end and a standard at a `#conformsTo` end, or a `@Proprietary` annotation with the rationale. " +
			"Add the conformance connection or the annotation.",
		Spec: "MOSA (Modular Open Systems Approach) extension library",
	},
	{
		Code: "mosa-interface-no-control", Tier: TierMethod, Severity: "warning",
		Summary: "a modular system interface naming no interface control authority",
		Meaning: "The model names an interface control authority for other interfaces, so MOSA expects one on this interface or its definition. " +
			"Add an `@InterfaceControl` annotation.",
		Spec: specMOSA,
	},
	{
		Code: "mosa-interface-not-traced", Tier: TierMethod, Severity: "warning",
		Summary: "a modular system interface satisfying no requirement",
		Meaning: "The model declares interface requirements, so MOSA expects each modular system interface, or its definition, to be named by a `satisfy ... by` statement. " +
			"Add the satisfaction.",
		Spec: specMOSA,
	},
	{
		Code: "mosa-component-no-data-rights", Tier: TierMethod, Severity: "warning",
		Summary: "a major system component recording no data rights",
		Meaning: "The model records data rights elsewhere, so MOSA expects a `@DataRights` annotation on this component or its definition stating the rights the program holds in its technical data. " +
			"Add the annotation.",
		Spec: specMOSA,
	},
	{
		Code: "mosa-proprietary-no-rationale", Tier: TierMethod, Severity: "warning",
		Summary: "an element marked `@Proprietary` with no rationale",
		Meaning: "MOSA expects the justification for each proprietary element to be recorded with it. " +
			"Give the annotation a `rationale`.",
		Spec: specMOSA,
	},
	{
		Code: "mosa-boundary-not-designated", Tier: TierMethod, Severity: "warning",
		Summary: "a connection between major components that is no modular system interface",
		Meaning: "The connection joins two major system components (or modular systems), a boundary MOSA expects to be designated a modular system interface. " +
			"Type the connection by a `#modularSystemInterface` definition or mark it so.",
		Spec: specMOSA,
	},
	{
		Code: "oosem-requirement-not-derived", Tier: TierMethod, Severity: "warning",
		Summary: "a requirement derived from no higher-level requirement",
		Meaning: "The model declares requirements at several levels and states derivations, so OOSEM expects this one to derive from one: a `#derivation` connection naming it at a `#derive` end and the original at an `#original` end. " +
			"Add the derivation.",
		Spec: "OOSEM (Object-Oriented Systems Engineering Method) extension library",
	},
	{
		Code: "oosem-requirement-not-satisfied", Tier: TierMethod, Severity: "warning",
		Summary: "a requirement nothing satisfies",
		Meaning: "The model states satisfactions, so OOSEM expects each requirement to be named by a `satisfy` statement. " +
			"Add the satisfaction, or mark the requirement as not yet allocated.",
		Spec: specOOSEM,
	},
	{
		Code: "oosem-logical-component-not-allocated", Tier: TierMethod, Severity: "warning",
		Summary: "a logical component allocated to nothing",
		Meaning: "The model declares nodes or physical components, so OOSEM expects each logical component, or its type, to be allocated to one. " +
			"Add an `allocate` from the component to a node or physical component.",
		Spec: specOOSEM,
	},
	{
		Code: "oosem-use-case-subject", Tier: TierMethod, Severity: "warning",
		Summary: "a use case whose subject is untyped",
		Meaning: "The subject of a use case is the system it serves; OOSEM expects it typed by that system's definition. " +
			"Type the subject.",
		Spec: specOOSEM,
	},
	{
		Code: "self-check-rule", Tier: TierMethod, Severity: "error",
		Summary: "a model's own rule failing for one of its elements",
		Meaning: "A `-self-check-package` runs each constraint of the named package as a predicate over every element of the model whose metaclass its parameter accepts, and this element makes the rule false. " +
			"Change the element so the rule holds, or amend the rule in the package.",
		Spec: "Model-defined rule; the predicate is a KerML constraint evaluated over the model's metaobjects",
	},
}
