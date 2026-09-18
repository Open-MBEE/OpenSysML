package semantics

import "github.com/Open-MBEE/OpenSysML/internal/core/symbols"

// ProbabilityFQN names the OpenSysML metadata type by which a model weights the
// successions out of a decision node; ProbabilityFeature is the weight it binds.
const (
	ProbabilityFQN     = "Stochastic::Probability"
	ProbabilityFeature = "p"
)

// RunDecidedMetadataFeature reports a metadata feature whose value the run reads
// rather than the model: Probability::p is read when its decision is reached.
func RunDecidedMetadataFeature(feature *symbols.Symbol) bool {
	return feature != nil && symbols.FQNOf(feature) == ProbabilityFQN+"::"+ProbabilityFeature
}
