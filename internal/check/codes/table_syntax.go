package codes

// syntaxCodes are the codes of the syntax tier: the parser's findings and the
// notation rules that read the tree before any name is resolved.
var syntaxCodes = []Entry{
	{
		Code: "syntax", Tier: TierSyntax, Severity: "error",
		Summary: "the text does not parse as KerML or SysML v2 notation",
		Meaning: "The parser could not read the text as a production of the grammar: a token is missing, misplaced or unexpected, and the message names what was expected at that point. " +
			"The parser recovers and reads on, so one slip can be followed by further findings; fix the first one reported and re-check. " +
			"Every other syntax-tier code names a more specific rule; this one is the general ill-formed-parse finding.",
		Spec: "KerML 1.0 §8.2 and SysML v2 §8.2, the textual notation grammars",
	},
	{
		Code: "defer-notation-removed", Tier: TierSyntax, Severity: "error",
		Summary: "`defer <event>;` in a state body, a removed extension",
		Meaning: "The state body member `defer <event>;` was an OpenSysML extension that SysML v2 has no notation for, and it was removed. " +
			"Model a deferred signal explicitly: an ordered buffer (`item deferred : Sig[*] ordered;`), a do action whose accept loop keeps each occurrence while the state holds it, and a transition out of the state that replays the buffer.",
		Spec: "SysML v2 §7.16 (states have no deferral notation)",
	},
	{
		Code: "reserved-keyword-name", Tier: TierSyntax, Severity: "error",
		Summary: "a declared name spelled as a reserved keyword",
		Meaning: "An element is named by a word the grammar reserves (`part`, `action`, `end` …), which the ID terminal does not admit, so no reference can name it and the pilot rejects the file. " +
			"Rename the element, or quote the name as an unrestricted name (`'part'`) where the keyword spelling is what the model needs.",
		Spec: "KerML 1.0 §8.2.2.3 and SysML v2 §8.2.2 (the ID terminal and reserved words)",
	},
	{
		Code: "import-visibility", Tier: TierSyntax, Severity: "error",
		Summary: "an `import` written without `public`, `private` or `protected`",
		Meaning: "SysML v2 requires a visibility indicator before `import`; an import without one is KerML notation the SysML grammar does not admit. " +
			"Write `private import` for an import the namespace uses itself, `public import` to re-export the imported members to whoever imports the namespace.",
		Spec: "SysML v2 §8.2.2.4 ImportPrefix",
	},
	{
		Code: "enumeration-body-member", Tier: TierSyntax, Severity: "error",
		Summary: "a member an enumeration definition body does not admit",
		Meaning: "The body of an enumeration definition holds enumerated values, annotations and imports only; a part, attribute or other usage declared there is not admitted by the grammar. " +
			"Declare the member in an attribute definition the enumeration specializes, or drop the keyword to make the line an enumerated value.",
		Spec: "SysML v2 §8.2.2.8 EnumerationBody",
	},
	{
		Code: "nonstandard-notation", Tier: TierSyntax, Severity: "warning",
		Summary: "notation OpenSysML reads that no production of the SysML v2 grammar admits",
		Meaning: "The text is an OpenSysML extension: the parser reads it and the model is analyzed, but the pinned SysML v2 and KerML grammars have no production for it, so another tool will reject the file. " +
			"The message states the standard form to write instead. `-strict` promotes this finding to an error.",
		Spec: "the pinned SysML v2 and KerML Xtext grammars",
	},
	{
		Code: "kerml-notation", Tier: TierSyntax, Severity: "warning",
		Summary: "KerML notation in a SysML file",
		Meaning: "A `.sysml` file uses a KerML production (`namespace`, `featured by`, `class` …) or a KerML spelling " +
			"(`typed by` for a typing, which SysML spells `:` or `defined by`) that the SysML v2 grammar does not admit. " +
			"Move the declaration to a `.kerml` file, or write the SysML form the message names. `-strict` promotes this finding to an error.",
		Spec: "the pinned SysML v2 Xtext grammar",
	},
	{
		Code: "sysml-notation", Tier: TierSyntax, Severity: "error",
		Summary: "SysML notation in a KerML file",
		Meaning: "A `.kerml` file uses a SysML v2 declaration keyword (`part`, `attribute`, `action` …) that the KerML grammar has no production for. " +
			"Move the declaration to a `.sysml` file, or write the KerML form (`feature`, `classifier`, `behavior`).",
		Spec: "the pinned KerML Xtext grammar",
	},
	{
		Code: "undefined-operator", Tier: TierSyntax, Severity: "warning",
		Summary: "the unary `~` operator, which the kernel library leaves undefined",
		Meaning: "The unary `~` maps to `DataFunctions::'~'`, which the Kernel Function Library declares abstract and never defines, so the expression has no value in any conforming evaluator. " +
			"Write the operation meant explicitly (`not x` for a Boolean, a library function for a bitwise complement).",
		Spec: "KerML 1.0 §8.2.5.8.1 and §9.4.3 DataFunctions",
	},
	{
		Code: "subject-membership-owning-type", Tier: TierSyntax, Severity: "error",
		Summary: "`subject` outside a requirement or case body",
		Meaning: "A subject is a parameter of a requirement or a case: `subject` is admitted in the body of a requirement, a case or one of their usages only. " +
			"Move the member into the requirement or case it belongs to, or write it as an ordinary usage.",
		Spec: "SysML v2 validateSubjectMembershipOwningType",
	},
	{
		Code: "actor-membership-owning-type", Tier: TierSyntax, Severity: "error",
		Summary: "`actor` outside a requirement or case body",
		Meaning: "An actor is a parameter of a requirement or a case: `actor` is admitted in the body of a requirement, a case or one of their usages only. " +
			"Move the member into the requirement or case it belongs to, or write it as an ordinary part usage.",
		Spec: "SysML v2 validateActorMembershipOwningType",
	},
	{
		Code: "stakeholder-membership-owning-type", Tier: TierSyntax, Severity: "error",
		Summary: "`stakeholder` outside a requirement body",
		Meaning: "A stakeholder is a parameter of a requirement: `stakeholder` is admitted in the body of a requirement definition or usage only. " +
			"Move the member into the requirement it belongs to, or write it as an ordinary part usage.",
		Spec: "SysML v2 validateStakeholderMembershipOwningType",
	},
	{
		Code: "objective-membership-owning-type", Tier: TierSyntax, Severity: "error",
		Summary: "`objective` outside a case body",
		Meaning: "An objective is the requirement a case is to meet: `objective` is admitted in the body of a case definition or usage (an analysis, verification or use case included) only. " +
			"Move the member into the case it belongs to, or write it as an ordinary requirement usage.",
		Spec: "SysML v2 validateObjectiveMembershipOwningType",
	},
	{
		Code: "requirement-constraint-membership-owning-type", Tier: TierSyntax, Severity: "error",
		Summary: "`assume` or `require` outside a requirement or case body",
		Meaning: "An assumed or required constraint is a member of a requirement: the `assume` and `require` prefixes are admitted in the body of a requirement, a case or one of their usages only. " +
			"Move the constraint into the requirement it belongs to, or drop the prefix to declare an ordinary constraint usage.",
		Spec: "SysML v2 validateRequirementConstraintMembershipOwningType",
	},
	{
		Code: "state-subaction-membership-owning-type", Tier: TierSyntax, Severity: "error",
		Summary: "`entry`, `do` or `exit` outside a state body",
		Meaning: "An entry, do or exit action is a subaction of a state: these keywords are admitted in the body of a state definition or usage only. " +
			"Move the member into the state it belongs to, or drop the keyword to declare an ordinary action usage.",
		Spec: "SysML v2 validateStateSubactionMembershipOwningType",
	},
	{
		Code: "transition-membership-owning-type", Tier: TierSyntax, Severity: "error",
		Summary: "`transition` outside a state body",
		Meaning: "A transition usage is a member of a state: the `transition` keyword is admitted in the body of a state definition or usage only (a transition between action nodes is an OpenSysML extension the notation rules report separately). " +
			"Move the transition into the state whose substates it joins, or write a succession (`first a then b;`) between actions.",
		Spec: "SysML v2 §8.2.2.19 TransitionUsageMember (a StateBodyItem)",
	},
	{
		Code: "framed-concern-membership-owning-type", Tier: TierSyntax, Severity: "error",
		Summary: "`frame` outside a requirement, concern or viewpoint body",
		Meaning: "A framed concern is a member of a requirement: `frame` is admitted in the body of a requirement, concern or viewpoint definition or usage only. " +
			"Move the member into the requirement or viewpoint that frames the concern.",
		Spec: "SysML.xtext FramedConcernMember (a RequirementBodyItem)",
	},
	{
		Code: "element-filter-membership-owning-type", Tier: TierSyntax, Severity: "error",
		Summary: "`filter` outside a package or view body",
		Meaning: "An element filter is a member of a package or a view: `filter` is admitted at the file root, in a package body, or in the body of a view definition or usage only. " +
			"Move the member into the package or view whose members it selects.",
		Spec: "SysML.xtext ElementFilterMember (a PackageBodyElement, ViewDefinitionBodyItem and ViewBodyItem)",
	},
	{
		Code: "view-rendering-membership-owning-type", Tier: TierSyntax, Severity: "error",
		Summary: "`render` outside a view body",
		Meaning: "A rendering is a member of a view: `render` is admitted in the body of a view definition or usage only. " +
			"Move the member into the view it renders, or declare a rendering usage (`rendering r : R;`) elsewhere.",
		Spec: "SysML v2 validateViewRenderingMembershipOwningType",
	},
	{
		Code: "expose-owning-namespace", Tier: TierSyntax, Severity: "error",
		Summary: "`expose` outside a view body",
		Meaning: "An expose is an import a view usage owns: `expose` is admitted in the body of a view usage (and, as an OpenSysML extension the notation rules report, a view definition) only. " +
			"Move the expose into the view that shows the exposed elements, or write an `import` where a plain import is meant.",
		Spec: "SysML v2 validateExposeOwningNamespace",
	},
}

// nameResolutionCodes are the codes of the name-resolution tier.
var nameResolutionCodes = []Entry{
	{
		Code: "unresolved", Tier: TierNameRes, Severity: "error",
		Summary: "a name that resolves to no element visible where it is written",
		Meaning: "A qualified or simple name names no element the namespace it is written in can see: it is misspelled, declared in a namespace that is not imported, private where it is declared, or not declared at all. " +
			"The message offers the nearest visible names and, for a declared but invisible one, the import that would make it visible. " +
			"A member lookup that fails part way (`A::B::c`) reports the segment that did not resolve.",
		Spec: "KerML 1.0 §8.2.3.5 (name resolution)",
	},
	{
		Code: "ambiguous", Tier: TierNameRes, Severity: "error",
		Summary: "a name that resolves to several elements at once",
		Meaning: "Two or more elements visible where the name is written answer to it with equal standing, typically through several wildcard imports, so the reference cannot choose. " +
			"Qualify the name with the namespace meant, or narrow one of the imports.",
		Spec: "KerML 1.0 §8.2.3.5 (name resolution)",
	},
	{
		Code: "name-conflict", Tier: TierNameRes, Severity: "warning",
		Summary: "two members of one namespace with the same name",
		Meaning: "A namespace owns or inherits two members distinguishable by neither name nor short name: a duplicate owned member name, or an owned name that hides an inherited one without redefining it. " +
			"Rename one member, or redefine the inherited one (`:>>`) where the owned member is meant to replace it. " +
			"`-strict` reports the KerML-forbidden duplicate owned member names as errors.",
		Spec: "KerML 1.0 §8.3.2.5 validateNamespaceDistinguishability",
	},
	{
		Code: "not-a-vertex", Tier: TierNameRes, Severity: "error",
		Summary: "a transition endpoint that names no state or pseudostate",
		Meaning: "The source or target of a transition resolves to an element that is not a state usage or a pseudostate of its state machine (an action, an attribute, a definition). " +
			"Name a state of the enclosing state definition or usage, or an entry, exit, choice, fork, join or history pseudostate.",
		Spec: "SysML v2 §7.19.2 and validateTransitionUsageSuccession",
	},
	{
		Code: "library-root-name", Tier: TierNameRes, Severity: "warning",
		Summary: "a root namespace named like a standard library package",
		Meaning: "A top-level package of the model has the name of a standard library package (`Base`, `ScalarValues` …), so qualified names starting with it may resolve to the library rather than to the model, depending on which is imported. " +
			"Rename the package, or qualify the references that are meant for the model's package.",
		Spec: "",
	},
	{
		Code: "library-package", Tier: TierNameRes, Severity: "warning",
		Summary: "a user package marked `standard library package`",
		Meaning: "The `standard` prefix marks the normative library packages the specification publishes; a package of the model carrying it is read as library content by every consumer, which hides its findings and changes how tools load it. " +
			"Write `library package` for a user library, or a plain `package`.",
		Spec: "SysML v2 §7.3.2 and the pilot's validateLibraryPackageNotStandard",
	},
}
