package runtime

import (
	"fmt"
	"strings"
)

// ChoiceTaken is one choice point of a run and the alternative it took, as the
// witness of an outcome lists them.
type ChoiceTaken struct {
	Kind ChoiceKind
	// Step is the action step the choice was made in, 0 for a state machine.
	Step int
	// Where names the decision node or the state and event; empty for a token order.
	Where string
	// Alternatives is how many the point had; Taken indexes the one taken.
	Alternatives int
	Taken        int
	// Among names the alternatives as a trace does; Took is the one taken: the
	// token tried next, a branch or a transition by declared position and target,
	// or the state whose transition fired first.
	Among []string
	Took  string
	// Weights are the probabilities the model states for the alternatives, one
	// per alternative, for a weighted decision; nil for a choice it does not weight.
	Weights []float64
	// Drew is the unit draw that selected Taken among the Weights, when Drawn.
	Drew  float64
	Drawn bool
}

// weighted reports whether the model weights the alternatives.
func (c ChoiceTaken) Weighted() bool { return len(c.Weights) == c.Alternatives && c.Alternatives > 0 }

// String renders the choice for a table or a failure message, as ParseChoice
// reads it back: a name the line's own punctuation occurs in is quoted, and a
// weighted branch carries the alternatives with their weights and the draw.
func (c ChoiceTaken) String() string {
	switch c.Kind {
	case ChoiceTokenOrder:
		return fmt.Sprintf("step %d: %s first of %s", c.Step, choiceLabel(c.Took), choiceLabels(c.Among))
	case ChoiceDecisionBranch:
		return fmt.Sprintf("step %d: %s -> %s%s", c.Step, choiceLabel(c.Where), choiceLabel(c.Took), c.weightedTail())
	case ChoiceTransition:
		return fmt.Sprintf("%s -> %s%s", choiceLabel(c.Where), choiceLabel(c.Took), c.weightedTail())
	case choiceStatementOrder, choiceGuardOrder:
		if c.Step > 0 {
			return fmt.Sprintf("step %d: %s: %s first of %s", c.Step, choiceLabel(c.Where), choiceLabel(c.Took), choiceLabels(c.Among))
		}
		return fmt.Sprintf("%s: %s first of %s", choiceLabel(c.Where), choiceLabel(c.Took), choiceLabels(c.Among))
	case ChoiceRegionOrder, ChoiceDueOrder, choiceDispatchOrder, choiceEntryOrder, choiceExitOrder, choiceStepOrder, choiceEntryStep:
		return fmt.Sprintf("%s: %s first of %s", choiceLabel(c.Where), choiceLabel(c.Took), choiceLabels(c.Among))
	}
	return fmt.Sprintf("%s -> %s", c.Kind, choiceLabel(c.Took))
}

// weightedTail spells what a weighted choice carries past the branch taken —
// ` among <alt> p=<w>, …` and ` drew <u>` when a draw selected it — "" for an unweighted one.
func (c ChoiceTaken) weightedTail() string {
	if !c.Weighted() || len(c.Among) != len(c.Weights) {
		return ""
	}
	tail := markAmong + weightedLabels(c.Among, c.Weights)
	if c.Drawn {
		tail += markDrew + FormatWeight(c.Drew)
	}
	return tail
}

// weightedLabels spells the alternatives with their weights, `<alt> p=<w>, …`.
func weightedLabels(among []string, weights []float64) string {
	parts := make([]string, len(among))
	for i, alt := range among {
		parts[i] = choiceLabel(alt) + markWeight + FormatWeight(weights[i])
	}
	return strings.Join(parts, markList)
}

// FormatChoices renders a witness as one line, its choices in run order.
func FormatChoices(choices []ChoiceTaken) string {
	if len(choices) == 0 {
		return "no choice points"
	}
	parts := make([]string, len(choices))
	for i, c := range choices {
		parts[i] = c.String()
	}
	return strings.Join(parts, "; ")
}
