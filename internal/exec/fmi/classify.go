package fmi

// The engine's reserved ToolVariable names are selectors, not model variables.
// An imported variable whose archive name is exactly one of those spellings is
// still a model variable: classify* resolves that ambiguity by giving the FMU's
// own names first claim, and every surface shares the decision.

// outputKind is what a call output's ToolVariable names: an FMU variable read
// at stopTime, the reply's time, the sample times, a trajectory, or nothing.
type outputKind int

const (
	unknownOutput outputKind = iota
	fmuOutput                // a variable of the FMU, by its exact archive name
	replyTime                // fmi:time
	sampleTimes              // fmi:seriesTime
	sampleSeries             // fmi:series:<name>
)

// inputKind is what a call input's ToolVariable names: an FMU variable or a
// reserved experiment or sampling input.
type inputKind int

const (
	unknownInput  inputKind = iota
	fmuInput                // a variable of the FMU, by its exact archive name
	reservedInput           // an experiment or sampling input
)

// classifyOutput resolves one output name against the FMU's variables. An
// exact variable wins over a reserved spelling, so an imported output literally
// named `fmi:series:y` reads that variable at stopTime instead of sampling `y`.
func classifyOutput(name string, d *Description) (kind outputKind, variable Variable, seriesVar string) {
	if v, ok := d.Variable(name); ok {
		return fmuOutput, v, ""
	}
	switch name {
	case "fmi:time":
		return replyTime, Variable{}, ""
	case seriesTimeOutput:
		return sampleTimes, Variable{}, ""
	}
	if sampled, ok := seriesVariable(name); ok {
		return sampleSeries, Variable{}, sampled
	}
	return unknownOutput, Variable{}, ""
}

// classifyInput resolves one input name the same way: a variable named
// `fmi:startTime` by the archive is set on the FMU, not read as the experiment.
func classifyInput(name string, d *Description) (kind inputKind, variable Variable) {
	if v, ok := d.Variable(name); ok {
		return fmuInput, v
	}
	if reservedInputs[name] {
		return reservedInput, Variable{}
	}
	return unknownInput, Variable{}
}
