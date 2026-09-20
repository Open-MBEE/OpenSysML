package semantics

import "github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"

// ProbabilityFQN names the OpenSysML metadata type by which a model weights the
// successions out of a decision node; ProbabilityFeature is the weight it binds.
const (
	ProbabilityFQN     = "Stochastic::Probability"
	ProbabilityFeature = "p"
)

// RunDecidedMetadataFeature reports a feature of the annotation type def whose
// value the run reads rather than the model: p of a Probability itself, which is
// read when its decision is reached. A subtype has no runtime reader, so none of its.
func RunDecidedMetadataFeature(def, feature *symbols.Symbol) bool {
	return def != nil && feature != nil &&
		symbols.FQNOf(def) == ProbabilityFQN &&
		symbols.FQNOf(feature) == ProbabilityFQN+"::"+ProbabilityFeature
}
