package querytranslate

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/suggest"
)

// OutcomeOf is the QVTS record of a translator refusal that has one: an unknown property is a
// path error whose frontier lists the properties the query could have named, with the nearest.
func OutcomeOf(err error) *queryexec.Outcome {
	var terr *Error
	if !errors.As(err, &terr) || terr.Kind != ErrUnknownProperty {
		return nil
	}
	frontier := queryexec.Frontier{Features: make([]queryexec.FeatureRef, 0, len(terr.Candidates))}
	for _, candidate := range terr.Candidates {
		frontier.Features = append(frontier.Features, queryexec.FeatureRef{Name: candidate})
	}
	if nearest := suggest.Nearest(terr.Property, terr.Candidates); len(nearest) > 0 {
		frontier.Nearest = nearest[0]
	}
	return &queryexec.Outcome{Path: &queryexec.PathError{Failures: []queryexec.Failure{{
		Step:     queryexec.Step{Kind: queryexec.StepFeature, Argument: terr.Property},
		Expected: "a property of the metamodel",
		Frontier: frontier,
	}}}}
}
