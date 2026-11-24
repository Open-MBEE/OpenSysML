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
	specNPR   = "NPR7123 extension library (NASA NPR 7123.1D)"
	specMS    = "NASASTD7009 extension library (NASA-STD-7009B)"
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
		Code: "nasastd7009-assessment-missing-factor", Tier: TierMethod, Severity: "warning",
		Summary: "an M&S capability or results assessment recording no level for one of its factors",
		Meaning: "NASA-STD-7009B [M&S 48] has the assessed M&S capability recorded according to each of the five factors of Appendix E.3, and [M&S 31] the assessed M&S results according to each of the six factors of Appendix E.4, so an `MSCapabilityAssessment` or `MSResultsAssessment` usage is expected to nest one factor assessment per factor stating its `level`. " +
			"Nest the missing factors (`#capabilityFactor item pedigree { :>> factor = CapabilityFactor::msDataPedigree; :>> level = FactorLevel::level2; }`).",
		Spec: specMS,
	},
	{
		Code: "nasastd7009-report-missing-content", Tier: TierMethod, Severity: "warning",
		Summary: "a report of M&S results omitting something section 4.3.8 has it include",
		Meaning: "NASA-STD-7009B section 4.3.8 has a report of M&S results to decision makers include the outcome of the M&S capability assessment [M&S 50] and of the M&S results assessment [M&S 35], an estimate of results uncertainty or the statement that none is available [M&S 33] with the processes that obtained it [M&S 34], the qualifications of the developers, users, operators or analysts [M&S 37], and the assessment of, and rationale for, acceptance of the risks [M&S 39]. " +
			"Nest or subset the two assessments under the `MSResultsReport` usage and state its `uncertaintyEstimate`, `uncertaintyProcess`, `qualifications` and `riskAcceptance`.",
		Spec: specMS,
	},
	{
		Code: "nasastd7009-report-missing-warning", Tier: TierMethod, Severity: "warning",
		Summary: "a report of M&S results with a factor assessed below its threshold and no warning of it",
		Meaning: "NASA-STD-7009B [M&S 32] a has unachieved acceptance criteria reported to decision makers with an explicit warning, and [M&S 43] e makes the M&S assessment level thresholds acceptance criteria, so a report holding a factor assessed below its `threshold` is expected to nest a `ResultsWarning` of kind `unachievedAcceptanceCriteria`. " +
			"Add the warning with its `impact`; the Standard sets no threshold of its own.",
		Spec: specMS,
	},
	{
		Code: "nasastd7009-warning-missing-impact", Tier: TierMethod, Severity: "warning",
		Summary: "a warning of a report of M&S results stating no impact",
		Meaning: "NASA-STD-7009B [M&S 32] has each warning reported to decision makers accompanied by at least a qualitative estimate of the impact of the occurrence. " +
			"Give the `ResultsWarning` an `impact`.",
		Spec: specMS,
	},
	{
		Code: "nasastd7009-criticality-rating-differs-from-matrix", Tier: TierMethod, Severity: "warning",
		Summary: "a criticality rating that differs from Figure 1 with no method recorded",
		Meaning: "NASA-STD-7009B section 4.1.1.2 has the method of the criticality assessment [M&S 6] recorded, Appendix D as is or modified or another method, so a `@CriticalityAssessment` whose `rating` is not the cell of the Appendix D matrix (Figure 1) for its `consequence` and `influence` is expected to state its `method`. " +
			"Correct the `rating` or record the `method` the project uses.",
		Spec: specMS,
	},
	{
		Code: "nasastd7009-requirement-entry-incomplete", Tier: TierMethod, Severity: "warning",
		Summary: "a matrix entry not applicable with no comments, or applicable with no record location",
		Meaning: "Appendix A of NASA-STD-7009B has the Comments column of the Requirements Identification Matrix explain or justify a requirement that is not applicable, and section 4.3.8.8 adds that the location of applicable/required records is also necessary for compliance. " +
			"Give the `@Applicability` annotation its `comments` when not applicable, and a requirement's entry its `recordLocation` when applicable.",
		Spec: specMS,
	},
	{
		Code: "nasastd7009-matrix-missing-requirement", Tier: TierMethod, Severity: "warning",
		Summary: "a Requirements Identification Matrix whose document has no entry for a requirement",
		Meaning: "Table 1 of NASA-STD-7009B lists every `[M&S n]` requirement, so a `RequirementsIdentificationMatrix` usage expects its document to hold an `@Applicability`-annotated usage of each definition in `NASASTD7009::MSRequirements`. " +
			"Add the missing entries, marking each applicable or not.",
		Spec: specMS,
	},
	{
		Code: "npr7123-review-missing-entrance-or-success-criteria", Tier: TierMethod, Severity: "warning",
		Summary: "a life-cycle review stating no entrance criteria or no success criteria",
		Meaning: "NPR 7123.1D section 5.2.2.1 (SE-34) has the technical team define entrance and success criteria for each life-cycle review, so a `LifeCycleReview` usage is expected to nest `EntranceCriterion` and `SuccessCriterion` requirement usages. " +
			"Subset the transcribed criteria of the library's `ReviewCriteria` package into the review's `entranceCriteria` and `successCriteria`, tailoring them as section G.1.2 expects.",
		Spec: specNPR,
	},
	{
		Code: "npr7123-review-missing-minimum-product", Tier: TierMethod, Severity: "warning",
		Summary: "a section 5.2.2.2 minimum product absent from its review or below the required maturity",
		Meaning: "NPR 7123.1D section 5.2.2.2 (SE-35 to SE-56, SE-66 to SE-69) names the minimum products the technical team provides at each life-cycle review and their maturity, recorded on the library's work product definitions as `@MaturityExpectation { required = true }`. " +
			"Nest an item of that product subsetting `reviewedProducts` under the review, with a `maturity` at or above the one asked for, or document the tailoring in the SEMP.",
		Spec: specNPR,
	},
	{
		Code: "npr7123-compliance-missing-justification", Tier: TierMethod, Severity: "warning",
		Summary: "a compliance entry marked tailored or not applicable with no justification",
		Meaning: "Table H-1 of NPR 7123.1D asks for a justification wherever a requirement is tailored (`T`) or not applicable (`NA`). " +
			"Give the `@Compliance` annotation a `justification`.",
		Spec: specNPR,
	},
	{
		Code: "npr7123-compliance-matrix-missing-requirement", Tier: TierMethod, Severity: "warning",
		Summary: "a compliance matrix whose document has no compliance entry for an active SE requirement",
		Meaning: "Table H-1 of NPR 7123.1D covers every active `[SE-nn]` requirement, so a `ComplianceMatrix` usage expects its document to hold a `@Compliance`-annotated usage of each definition in `NPR7123::SERequirements`. " +
			"Add the missing entries, marking each FC, T or NA.",
		Spec: specNPR,
	},
	{
		Code: "npr7123-semp-missing-process", Tier: TierMethod, Severity: "warning",
		Summary: "a SEMP not addressing all 17 common technical processes",
		Meaning: "NPR 7123.1D section 6.2.3 (SE-58) has the SEMP define how each of the 17 common technical processes, as tailored, is applied, so a `SEMP` usage expects one nested usage per process definition of the library. " +
			"Nest the missing processes (`#technicalPlanning action plan;`) with their `tailoring`.",
		Spec: specNPR,
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
