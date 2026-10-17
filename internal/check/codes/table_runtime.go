package codes

// runtimeCodes are the codes of the runtime tier: what the executor finds while
// lowering a behavior, running it or materializing a feature value.
var runtimeCodes = []Entry{
	{
		Code: "action-step-multiplicity-not-fixed", Tier: TierRuntime, Severity: "warning",
		Summary: "a repeated action step with no exact finite count",
		Meaning: "A step with a multiplicity other than 1 is performed that many times; a bound of `*` or a range (`[1..3]`) fixes no count the executor can run. " +
			"Write an exact multiplicity (`[3]`), or loop explicitly.",
		Spec: "",
	},
	{
		Code: "action-step-multiplicity-unsupported", Tier: TierRuntime, Severity: "warning",
		Summary: "a repeated action step linked in a shape the executor does not run",
		Meaning: "The repeated step is joined by flows or successions the executor cannot unroll per performance (a fork driving it, a flow into one of its pins). " +
			"Write the repetition as a `loop` or `for` body.",
		Spec: "",
	},
	{
		Code: "action-step-order-unsatisfiable", Tier: TierRuntime, Severity: "warning",
		Summary: "succession multiplicities that exclude the step's count",
		Meaning: "The multiplicities written on the successions into and out of a repeated step admit no number of performances that the step's own multiplicity requires. " +
			"Make the succession and step multiplicities agree.",
		Spec: "",
	},
	{
		Code: "action-step-order-open", Tier: TierRuntime, Severity: "warning",
		Summary: "a succession that does not order a step's repetitions",
		Meaning: "A succession touching a repeated step says nothing about how its performances follow one another, so the executor runs them in an order the model does not fix. " +
			"Write the end multiplicities (`first [1] a then [1] b`) that order them, or declare the step `ordered`.",
		Spec: "",
	},
	{
		Code: "action-step-lowering", Tier: TierRuntime, Severity: "warning",
		Summary: "an action body the executor cannot lower",
		Meaning: "The action body uses a shape the lowering to the executor's graph does not implement, named in the message; the body is checked but cannot be run. " +
			"Rewrite the body with the forms the message suggests.",
		Spec: "",
	},
	{
		Code: "succession-order-violated", Tier: TierRuntime, Severity: "error",
		Summary: "a run that began a performance before one a succession orders first",
		Meaning: "A succession orders one performance before another, and the run began the later one while the earlier had not started: the successions and the behavior's own control flow disagree. " +
			"Align the successions with the body's flow.",
		Spec: "KerML 1.0 §7.4.6 Succession",
	},
	{
		Code: "choice-point", Tier: TierRuntime, Severity: "warning",
		Summary: "a run that chose among several enabled transitions or successions",
		Meaning: "Several transitions or successions were enabled at once and the model does not say which to take; the run took the one the message names. " +
			"Add guards that make the choice, or accept the recorded choice.",
		Spec: "",
	},
	{
		Code: "guard-unevaluable", Tier: TierRuntime, Severity: "warning",
		Summary: "a guard the run could not evaluate",
		Meaning: "A transition or succession guard reads a feature that holds no value yet or calls what the runtime cannot evaluate, so the run treated it as not taken. " +
			"Bind the features the guard reads, or simplify the guard.",
		Spec: "",
	},
	{
		Code: "random-draw", Tier: TierRuntime, Severity: "warning",
		Summary: "a run that drew a random value",
		Meaning: "The model called a random function; the run records the draw so the result can be reproduced from its seed. " +
			"Fix the seed for a reproducible run.",
		Spec: "",
	},
	{
		Code: "tool-divergence", Tier: TierRuntime, Severity: "warning",
		Summary: "an external tool whose result diverged from the model's",
		Meaning: "A tool invoked by the run returned a value that disagrees with what the model's own calculation gives for the same inputs. " +
			"Check the tool's binding and units against the calculation.",
		Spec: "",
	},
	{
		Code: "runtime.materialize", Tier: TierRuntime, Severity: "error",
		Summary: "a feature value the run could not materialize",
		Meaning: "A feature read by the run has a value expression the runtime could not evaluate: an unbound parameter, an unsupported function, a value of the wrong type. " +
			"Bind what the expression reads, or rewrite it with supported forms.",
		Spec: "",
	},
	{
		Code: "runtime.materialize.bounded", Tier: TierRuntime, Severity: "warning",
		Summary: "a check that was bounded rather than completed",
		Meaning: "A constraint or requirement check over an unbounded domain was cut off at the run's bound, so what it reports holds of the cases examined only. " +
			"Raise the bound, or restrict the domain.",
		Spec: "",
	},
}
