package apijson

// Nondeterminism declares a property the pinned pilot exports differently
// from one JVM to the next, with the evidence. Its scorecard row is scored in
// the pilot-nondeterministic column, so the instability is visible and the
// baseline reproduces on every JVM.
type Nondeterminism struct {
	Metaclass string `json:"metaclass"`
	Property  string `json:"property"`
	Evidence  string `json:"evidence"`
}

// pilotNondeterminism is the declared list. Every entry must be shown by two
// exports of one corpus at the pin disagreeing on no other property, and is
// removed when a pin stops reproducing the disagreement.
var pilotNondeterminism = []Nondeterminism{
	{
		Metaclass: "FeatureChainExpression",
		Property:  "featuringType",
		Evidence: "At pin 2026-08 (0.62.0) two exports of the same corpus differ only here: under OpenJDK 21 the three " +
			"FeatureChainExpressions valuing the event occurrences of ServerSequenceModelOutside::PubSubSequenceOutside " +
			"(examples/pilot-corpora/sysml-examples/Interaction Sequencing Examples/ServerSequenceModelOutside.sysml, " +
			"re-exported by ServerSequenceOutsideRealization-2.sysml and -3.sysml) have no featuringType; under OpenJDK 25 " +
			"each has one, the enclosing SuccessionAsUsage's defaultFeaturingType, an element the pilot neither exports nor " +
			"keys. The pilot derives an expression's featuringType from its adapter's implicit featuring types, filled while " +
			"the owner is transformed, so the value depends on transformation order rather than on the model.",
	},
	{
		Metaclass: "InvocationExpression",
		Property:  "featuringType",
		Evidence: "At pin 2026-09 (0.63.0) two full exports of the same checkout under OpenJDK 27 differ only in " +
			"InvocationExpression.featuringType in examples/pilot-corpora/sysml-validation: one comparison moves " +
			"between ours-missing and absent-both, and the unkeyed-reference count moves by one. The whole row " +
			"holds 480 comparisons, 408 ours-missing and 72 absent-both in one run, 407 and 73 in the other. " +
			"A third export of that root reproduces the first counts; no source, element match or other property changes.",
	},
	{
		Metaclass: "OperatorExpression",
		Property:  "featuringType",
		Evidence: "At pin 2026-09 (0.63.0) full exports of the same checkout under OpenJDK 27 move four comparisons " +
			"of OperatorExpression.featuringType between ours-missing and absent-both: 875 and 458 in one run, " +
			"871 and 462 in another, over 1333 comparisons. Apart from the separately declared invocation-expression " +
			"row, every other property row and every element match is unchanged.",
	},
}

// isPilotNondeterministic reports whether the (metaclass, property) row is declared.
func isPilotNondeterministic(metaclass, property string) bool {
	for _, n := range pilotNondeterminism {
		if n.Metaclass == metaclass && n.Property == property {
			return true
		}
	}
	return false
}
