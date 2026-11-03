package apijson

// Nondeterminism declares one property the pinned pilot exports differently
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
