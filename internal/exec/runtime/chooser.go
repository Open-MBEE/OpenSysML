package runtime

// Chooser resolves the choice points of a run as the scheduler meets them: a
// pick among alternatives given in declaration order, and the one move of a
// step among the tokens able to act. An exploration implements it to drive the
// runs it makes; the context's runs consult it through BeginExploration.
type Chooser interface {
	// Pick resolves a choice among n alternatives and returns the index taken.
	Pick(n int) int
	// Describe attaches to the last pick how the run reports it.
	Describe(c ChoicePoint)
	// BeginStep opens one step's choice among its tokens.
	BeginStep(tokens StepTokens) StepChooser
	// Mark records the chooser's state and returns what restores it, as a
	// probe that must leave no trace brackets its run.
	Mark() func()
}

// StepChooser hands one step its tokens until one acts.
type StepChooser interface {
	// Next is the token to try next; false once the step tried them all.
	Next() (int64, bool)
	// Acted tells the chooser whether the token last handed out did something.
	Acted(id int64, acted bool)
	// Choice is the pick the move resolved: the tokens able to act as the
	// trace names them and the index of the one moved; false when one token
	// alone could act.
	Choice() (alternatives []string, taken int, ok bool)
}

// IDs are the tokens the step may try, in spawn order.
func (t StepTokens) IDs() []int64 { return t.ids }

// Held reports a token that collapses into another's synchronization, so it
// never acts on its own.
func (t StepTokens) Held(id int64) bool { return t.held[id] }

// Enabled reports whether the token is able to act at this moment.
func (t StepTokens) Enabled(id int64) bool { return t.enabled(id) }

// Label names the token as the trace does.
func (t StepTokens) Label(id int64) string { return t.label(id) }

// Step is the step's number in its executor's run.
func (t StepTokens) Step() int { return t.step }
