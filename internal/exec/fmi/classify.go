package fmi

// Reserved ToolVariable names are selectors, but one classifier gives an exact
// FMU variable name first claim, on every surface that resolves a name.

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

// classifyOutput gives an exact variable first claim over a reserved spelling:
// an output named `fmi:series:y` reads that variable, not a trajectory of `y`.
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
