package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
)

// checks are the model checks and runs named on the command line, in the order
// they are carried out: objects are created first, so a verdict is about them,
// and behavior runs after the conditions the model states about it.
type checks struct {
	validate     optionalNames
	instantiate  stringSlice
	constraints  stringSlice
	requirements stringSlice
	satisfy      optionalNames
	calcs        stringSlice
	analyses     stringSlice
	sweeps       stringSlice
	samples      sweepCount
	seed         sweepSeed
	draws        drawPolicy
	clockStep    clockStep
	runs         runCount
	observe      stringSlice
	compare      string
	queries      stringSlice
	actions      stringSlice
	states       stringSlice
	advance      advanceTime
	jsonOut      bool
	checker      checkerOptions
}

// checkerOptions are the -check-* flags: what the check and smt engines are asked
// beside -engine check -action or -state and -engine smt -action, and the bounds
// they run under.
type checkerOptions struct {
	diverge    stringSlice
	properties stringSlice
	inputs     stringSlice
	assume     stringSlice
	witness    string
	depth      positiveCount
	states     positiveCount
	unroll     positiveCount
	timeout    checkTimeout
}

// given reports whether any -check-* flag was written.
func (o *checkerOptions) given() bool {
	return len(o.diverge) > 0 || len(o.properties) > 0 || len(o.inputs) > 0 || len(o.assume) > 0 ||
		o.witness != "" || o.depth.given || o.states.given || o.unroll.given || o.timeout.given
}

// positiveCount is a -check-depth, -check-states or -check-unroll value as written: a bound of
// at least one, parsed where a bad value is reported in the caller's own form.
type positiveCount struct {
	flag  string
	value int
	text  string
	given bool
}

func (c *positiveCount) String() string { return c.text }

func (c *positiveCount) Set(value string) error {
	c.text, c.given = value, true
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fmt.Errorf("-%s takes a bound of at least one, not %q", c.flag, value)
	}
	c.value = n
	return nil
}

// checkTimeout is -check-timeout as written: the time a check's plan may run
// for, as Go spells a duration.
type checkTimeout struct {
	value time.Duration
	text  string
	given bool
}

func (c *checkTimeout) String() string { return c.text }

func (c *checkTimeout) Set(value string) error {
	c.text, c.given = value, true
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return fmt.Errorf("-check-timeout takes a duration such as 30s or 2m, not %q", value)
	}
	c.value = d
	return nil
}

// sweepCount is -samples as written: the number of values to draw for each
// range given, parsed where a bad value is reported in the caller's own form.
type sweepCount struct {
	value int64
	text  string
	given bool
}

func (s *sweepCount) String() string { return s.text }

func (s *sweepCount) Set(value string) error {
	s.text, s.given = value, true
	count, err := strconv.ParseInt(value, 10, 64)
	if err != nil || count <= 0 {
		return fmt.Errorf("-samples takes the number of values to draw, not %q", value)
	}
	s.value = count
	return nil
}

// sweepSeed is -seed as written: the seed a sampled sweep or a Monte Carlo draws
// from, which is required rather than defaulted so a table is reproducible.
type sweepSeed struct {
	value uint64
	text  string
	given bool
}

func (s *sweepSeed) String() string { return s.text }

func (s *sweepSeed) Set(value string) error {
	s.text, s.given = value, true
	seed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("-seed takes a whole number to draw from, not %q", value)
	}
	s.value = seed
	return nil
}

// seed is the seed as given, nil when -seed was not written.
func (s *sweepSeed) seed() *uint64 {
	if !s.given {
		return nil
	}
	return &s.value
}

// drawPolicy is -draws as written: how every run resolves the RandomFunctions
// draws — at random from the seed, or at each call's min, max or average.
type drawPolicy struct {
	value runtime.DrawPolicy
	text  string
}

func (d *drawPolicy) String() string { return d.text }

func (d *drawPolicy) Set(value string) error {
	policy, err := runtime.ParseDrawPolicy(value)
	if err != nil {
		return err
	}
	d.value, d.text = policy, value
	return nil
}

// clockStep is -clock-step as written: the step, in seconds, the clock of every
// run ticks by; 0, the default, is a continuous clock.
type clockStep struct {
	value float64
	text  string
	given bool
}

func (c *clockStep) String() string { return c.text }

func (c *clockStep) Set(value string) error {
	step, err := runtime.ParseClockStep(value)
	if err != nil {
		return fmt.Errorf("-clock-step: %w", err)
	}
	c.value, c.text, c.given = step, value, true
	return nil
}

// runCount is -runs as written: how many times to run the action named, parsed
// where a bad value is reported in the caller's own form.
type runCount struct {
	value int64
	text  string
	given bool
}

func (r *runCount) String() string { return r.text }

func (r *runCount) Set(value string) error {
	r.text, r.given = value, true
	count, err := strconv.ParseInt(value, 10, 64)
	if err != nil || count <= 0 {
		return fmt.Errorf("-runs takes the number of runs to make, not %q", value)
	}
	r.value = count
	return nil
}

// advanceTime is -advance as written, parsed where a bad value is reported in
// whichever form the caller asked for. An empty value is a misuse rather than
// silently no advance, so it records that the flag was written.
type advanceTime struct {
	value string
	given bool
}

func (a *advanceTime) String() string { return a.value }

func (a *advanceTime) Set(value string) error {
	a.value, a.given = value, true
	return nil
}

// requested reports whether checking mode was asked for rather than a prompt.
// -json and -advance check nothing themselves, but are included so their misuse
// is reported rather than leaving a script at a prompt it cannot answer.
func (c *checks) requested() bool {
	return c.validate.given || c.jsonOut || c.advance.given || c.satisfy.given || len(c.instantiate) > 0 ||
		len(c.constraints) > 0 || len(c.requirements) > 0 || len(c.calcs) > 0 || len(c.analyses) > 0 ||
		len(c.queries) > 0 || len(c.actions) > 0 || len(c.states) > 0 ||
		c.sweeping() || c.running() || c.compare != "" || c.checker.given()
}

// explicitOnly names the -check-* flags written that the check engine alone reads.
func (o *checkerOptions) explicitOnly() []string {
	var written []string
	if o.states.given {
		written = append(written, "-check-states")
	}
	return written
}

// symbolicOnly names the -check-* flags written that the smt engine alone reads.
func (o *checkerOptions) symbolicOnly() []string {
	var written []string
	if len(o.inputs) > 0 {
		written = append(written, "-check-input")
	}
	if len(o.assume) > 0 {
		written = append(written, "-check-assume")
	}
	if o.unroll.given {
		written = append(written, "-check-unroll")
	}
	return written
}

// checkerMisuse reports why the -check-* flags written check nothing under the
// engine selected, and "" when they check a behavior: under -engine check or
// -engine smt always, each reading the flags it has, under -engine all when one
// of them is written. The smt engine searches an action's schedules alone; the
// check engine's take in state machines and the clock -advance moves.
func (c *checks) checkerMisuse(engine string) string {
	selection := analysis.ParseSelection(engine)
	checkOnly, symbolic := selection == analysis.Only(analysis.CheckEngineName), selection == analysis.Only(analysis.SMTEngineName)
	checking := checkOnly || symbolic || selection.Mode == analysis.SelectAll && c.checker.given()
	switch {
	case c.checker.given() && !checking:
		return "-check-diverge, -check-property, -check-input, -check-assume, -check-witness, -check-depth, -check-states, -check-unroll and -check-timeout are the check and smt engines'; select one, as -engine check or -engine smt, or every engine, as -engine all"
	case checkOnly && len(c.checker.symbolicOnly()) > 0:
		return flagMisuse(c.checker.symbolicOnly(), analysis.SMTEngineName, analysis.CheckEngineName)
	case symbolic && len(c.checker.explicitOnly()) > 0:
		return flagMisuse(c.checker.explicitOnly(), analysis.CheckEngineName, analysis.SMTEngineName)
	case symbolic && c.checker.given() && len(c.actions) == 0:
		return "the -check-* flags search an action's schedules under -engine smt; name one, as -action <name>"
	case c.checker.given() && len(c.actions) == 0 && len(c.states) == 0:
		return "the -check-* flags search a behavior's schedules; name one, as -action <name> or -state <name>"
	case symbolic && c.advance.given:
		return "-advance runs behaviors on one clock, which -engine smt's search of an action's schedules does not; drop one of them"
	}
	return ""
}

// flagMisuse spells the flags written that reader alone reads, which -engine
// selected leaves out.
func flagMisuse(written []string, reader, selected string) string {
	verb := "are"
	if len(written) == 1 {
		verb = "is"
	}
	return fmt.Sprintf("%s %s the %s engine's, which -engine %s leaves out; select it, as -engine %s, or every engine, as -engine all",
		spelled(written), verb, reader, selected, reader)
}

// spelled lists names as prose: `a`, `a and b`, `a, b and c`.
func spelled(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// sweeping reports whether a sweep or a sample of one was asked for. A seed
// alone asks for no sweep: it seeds the model's own draws in whatever runs.
func (c *checks) sweeping() bool {
	return len(c.sweeps) > 0 || c.samples.given
}

// running reports whether a Monte Carlo was asked for.
func (c *checks) running() bool {
	return c.compare == "" && (c.runs.given || len(c.observe) > 0)
}

// runsMisuse reports why the flags a Monte Carlo was asked for with run none,
// and "" when they run one: -runs needs a single -action or -analysis and, under
// the random draw policy, -seed; -observe names what the runs of an action report.
func (c *checks) runsMisuse() string {
	if c.compare != "" {
		return c.compareMisuse()
	}
	if !c.running() {
		return ""
	}
	switch {
	case !c.runs.given:
		return "-observe names what -runs reports; ask for the runs, as -runs <number>"
	case len(c.actions) == 0 && len(c.analyses) == 0:
		return "-runs runs an action or a Simulation::MonteCarlo analysis case; name one, as -action <name> or -analysis <name>"
	case len(c.actions)+len(c.analyses) > 1:
		return "-runs runs one action or analysis case; name a single -action or -analysis"
	case len(c.analyses) > 0 && len(c.observe) > 0:
		return "-runs of an analysis case observes what the case declares as observed; -observe names the features of an -action"
	case len(c.states) > 0:
		return "-runs runs an action; a state machine is run once, as -state <name> without -runs"
	case !c.seed.given && !c.draws.value.Fixed():
		return "-runs draws each run's randomness from a seed; name one, as -seed <number>, or fix the draws, as -draws min|max|average"
	case c.sweeping():
		return "-runs runs an action; -sweep and -samples run an analysis case or calc; ask for one of them"
	case c.advance.given:
		return "-runs runs the action to completion; -advance runs it for a time; ask for one of them"
	case c.checker.given():
		return "-runs makes concrete runs; the -check-* flags search schedules; ask for one of them"
	}
	return ""
}

// compareMisuse reports why the flags -compare-results was written with compare
// nothing, and "" when they do: it runs the configurations the results index, so
// -runs, -seed, -draws and -observe shape the runs and -action names configurations.
func (c *checks) compareMisuse() string {
	switch {
	case len(c.states) > 0 || c.sweeping() || c.advance.given || c.checker.given() ||
		c.validate.given || c.satisfy.given || len(c.instantiate) > 0 || len(c.constraints) > 0 ||
		len(c.requirements) > 0 || len(c.calcs) > 0 || len(c.analyses) > 0 || len(c.queries) > 0:
		return "-compare-results runs the migrated configurations the results index and compares the runs with the tool's; the other checks are made in a run of their own"
	}
	for _, pair := range c.observe {
		if stored, _, _ := strings.Cut(pair, "="); strings.TrimSpace(stored) == "" {
			return fmt.Sprintf("-observe %q names no stored observable; with -compare-results write it as -observe <observable> or -observe <observable>=<feature>", pair)
		}
	}
	return ""
}

// readResults reads the -compare-results sidecar, naming the file in what went wrong.
func readResults(path string) (*simresults.Results, error) {
	f, err := os.Open(path) // #nosec G304 -- the operator names the sidecar on the command line
	if err != nil {
		return nil, fmt.Errorf("-compare-results: %w", err)
	}
	defer f.Close()
	results, err := simresults.Read(f)
	if err != nil {
		return nil, fmt.Errorf("-compare-results %s: %w", path, err)
	}
	return results, nil
}

// compareOptions are the -compare-results runs as the flags shape them.
func (c *checks) compareOptions() repl.CompareOptions {
	opts := repl.CompareOptions{Seed: c.seed.seed(), Only: c.actions}
	if c.runs.given {
		opts.Runs = c.runs.value
	}
	if flagGiven("draws") {
		policy := c.draws.value
		opts.Draws = &policy
	}
	if c.clockStep.given {
		step := c.clockStep.value
		opts.ClockStep = &step
	}
	for _, pair := range c.observe {
		stored, feature, _ := strings.Cut(pair, "=")
		opts.Observe = append(opts.Observe, repl.ObservablePair{Stored: stored, Feature: feature})
	}
	return opts
}

// sweepMisuse reports why the flags a sweep was asked for with make no sweep,
// and "" when they make one.
func (c *checks) sweepMisuse() string {
	if !c.sweeping() {
		return ""
	}
	targets := len(c.calcs) + len(c.analyses)
	switch {
	case targets == 0:
		return "-sweep runs an analysis case or a calc; name one, as -analysis <name> or -calc <name>"
	case targets > 1:
		return "-sweep runs one analysis case or calc; name a single -analysis or -calc"
	case len(c.sweeps) == 0:
		return "-samples draws from a range; name one, as -sweep <parameter>=<from>..<to>"
	case c.samples.given && !c.seed.given:
		return "-samples draws from a seed; name one, as -seed <number>"
	}
	return ""
}

// instantiatesOnly reports whether the run creates objects and decides nothing
// about them, so a document can be rendered over what it holds.
func (c *checks) instantiatesOnly() bool {
	return len(c.instantiate) > 0 && !c.validate.given && !c.jsonOut && !c.advance.given && !c.satisfy.given &&
		len(c.constraints) == 0 && len(c.requirements) == 0 && len(c.calcs) == 0 && len(c.analyses) == 0 &&
		len(c.queries) == 0 && len(c.actions) == 0 && len(c.states) == 0 && !c.sweeping() && !c.running() && c.compare == "" && !c.checker.given()
}

// checksOnly reports whether anything was asked about the model itself, as
// against how to report the answer.
func (c *checks) checksOnly() bool {
	return len(c.validate.targets) > 0 || len(c.instantiate) > 0 || len(c.constraints) > 0 ||
		len(c.requirements) > 0 || len(c.satisfy.targets) > 0 || len(c.calcs) > 0 || len(c.analyses) > 0 ||
		len(c.queries) > 0 || len(c.actions) > 0 || len(c.states) > 0 || c.compare != ""
}

// optionalNames collects the values of a flag that takes an optional name, as
// -satisfy and -validate do: bare, it is about the whole model, and with =<name>
// about the element or object named. Go's flag package passes "true" for the
// valueless spelling, which no name can be mistaken for because `true` is a
// literal keyword rather than a declarable name.
type optionalNames struct {
	// targets are the names given, the bare spelling recorded as "".
	targets []string
	// given records the flag written at all, =false included, so a script that
	// wrote it is answered rather than left at a prompt.
	given bool
}

func (t *optionalNames) String() string { return fmt.Sprint(t.targets) }

// IsBoolFlag makes the value optional, so the flag alone is accepted.
func (t *optionalNames) IsBoolFlag() bool { return true }

func (t *optionalNames) Set(value string) error {
	t.given = true
	switch value {
	case "true":
		t.targets = append(t.targets, "")
	case "false":
		// The off spelling of a flag declared boolean, so -satisfy=$on works: it
		// withdraws a bare request written before it and leaves the names given.
		t.targets = t.names()
	default:
		t.targets = append(t.targets, value)
	}
	return nil
}

// tookNoValue reports whether the flag was given without a value. A name written
// after it (`-satisfy Landing::touchdown`) is then a positional argument, i.e. a
// file to load, which is worth explaining when no such file exists.
func (t *optionalNames) tookNoValue() bool {
	return slices.Contains(t.targets, "")
}

// names are the values given, the bare spelling left out.
func (t *optionalNames) names() []string {
	return slices.DeleteFunc(slices.Clone(t.targets), func(name string) bool { return name == "" })
}

// valueMisuse explains a flag whose name was written as a positional argument
// that names no file, for whichever of -satisfy and -validate was so written.
func (c *checks) valueMisuse(path string) string {
	if fileExists(path) {
		return ""
	}
	switch {
	case c.satisfy.tookNoValue():
		return fmt.Sprintf("%s is read as a file to load; -satisfy takes a name as -satisfy=%s", path, path)
	case c.validate.tookNoValue() && len(c.instantiate) > 0:
		return fmt.Sprintf("%s is read as a file to load; -validate takes an object as -validate=%s", path, path)
	}
	return ""
}

// refuse reports a misused flag in whichever form the caller asked for, so a
// script reading the JSON document reads why no check was made.
func refuse(c checks, message string) int {
	rep := newReporter(c.jsonOut)
	rep.failed(message)
	return rep.finish()
}

// runChecks loads the model, creates the objects asked for, evaluates the checks
// and runs the behavior named on the command line, reporting each outcome as it
// is decided. The result is the exit status: every verdict held, one of them
// failed, or a check could not be made at all.
func runChecks(files []string, exprs []string, c checks) int {
	rep := newReporter(c.jsonOut)

	var advance float64
	if c.advance.given {
		if len(c.actions) == 0 && len(c.states) == 0 {
			rep.failed("-advance is the time a behavior runs for; name one, as -action <name> or -state <name>")
			return rep.finish()
		}
		duration, err := parseAdvance(c.advance.value)
		if err != nil {
			rep.failed(err.Error())
			return rep.finish()
		}
		advance = duration
	}
	if message := c.sweepMisuse(); message != "" {
		rep.failed(message)
		return rep.finish()
	}
	if message := c.runsMisuse(); message != "" {
		rep.failed(message)
		return rep.finish()
	}
	if message := c.checkerMisuse(engine.text); message != "" {
		rep.failed(message)
		return rep.finish()
	}
	if !c.checksOnly() {
		if c.jsonOut {
			rep.failed("-json reports a check; name one, as -validate or -constraint <name>")
		} else {
			rep.failed("no check was named; name one, as -validate or -constraint <name>")
		}
		return rep.finish()
	}
	if len(files) == 0 {
		rep.failed("no model to check; name the file the checked elements are declared in")
		return rep.finish()
	}

	sess := newSession()
	sess.SetCheckDiverge(c.checker.diverge)
	sess.SetCheckProperties(c.checker.properties)
	sess.SetCheckInputs(c.checker.inputs)
	sess.SetCheckAssume(c.checker.assume)
	sess.SetCheckWitnessDir(c.checker.witness)
	sess.SetCheckBounds(c.checker.depth.value, c.checker.states.value, c.checker.unroll.value, c.checker.timeout.value)

	// A checked model may be named as a directory or a glob as well as by file, so
	// the paths are expanded to the files they stand for before loading.
	paths, err := repl.ExpandPaths(files)
	if err != nil {
		rep.failed(err.Error())
		if len(files) == 1 {
			if message := c.valueMisuse(files[0]); message != "" {
				rep.failed(message)
			}
		}
		return rep.finish()
	}

	// The files are loaded as one submission, indexed and analyzed once, and
	// each is still summarized on its own.
	loaded, err := sess.LoadFilesSummary(paths)
	if err != nil {
		rep.failed(err.Error())
		var read *repl.ReadError
		if errors.As(err, &read) {
			if message := c.valueMisuse(read.Path); message != "" {
				rep.failed(message)
			}
		}
		return rep.finish()
	}

	// A model that did not analyse cleanly answers nothing, so its diagnostics end
	// the run rather than a verdict being reported about a model nobody could read.
	// One file's reference to another only resolves once every file is loaded, so
	// the gate is about the whole model rather than about each file in turn.
	if sess.HasErrors() {
		rep.diags(sess.LocatedDiagnostics())
		rep.problem(sess.DiagnosticLines())
		rep.failed(fmt.Sprintf("%s did not analyse cleanly; no check was made", namedModels(files)))
		return rep.finish()
	}
	// A clean model's warnings are still findings rather than results, so they
	// are kept off the stream the verdicts are reported on, and printed before
	// the load summary they qualify.
	rep.problem(sess.DiagnosticLines())

	// What analysis found is reported as data whatever was checked, so a caller
	// parsing the report reads the warnings the printed load output carries.
	rep.diags(sess.LocatedDiagnostics())

	rep.info(loaded)

	// The configurations a migration indexed results for are run against those
	// results, and nothing else is asked of the model in the same run.
	if c.compare != "" {
		results, err := readResults(c.compare)
		if err != nil {
			rep.failed(err.Error())
			return rep.finish()
		}
		for _, v := range sess.CompareResults(results, c.compareOptions()) {
			rep.verdict(v)
		}
		return rep.finish()
	}

	// An object first: a constraint, requirement or expression about a feature of
	// a part is answered about the object that carries it, and only an existing
	// one can be. Creating it materializes its feature values, so a default that does not
	// conform to its feature's multiplicity is a diagnostic of this run rather
	// than one left to whoever reads the feature value next.
	bounded := false
	for _, name := range c.instantiate {
		report, err := sess.InstantiateReport(name)
		if err != nil {
			rep.failed(err.Error())
			return rep.finish()
		}
		rep.info(report.Lines)
		for _, fvErr := range report.FeatureValueErrors {
			rep.finding(fvErr)
		}
		// Materializing a wide or recursive model costs an object per value, so the
		// check is bounded; what it did not reach is unchecked rather than clean.
		if report.Bounded {
			bounded = true
			rep.warn(fmt.Sprintf("%s: materialization is bounded; not every feature value was checked", name))
		}
	}

	// The model is only reported clean once the objects asked for were created:
	// what materializing them found is a diagnostic about the model, so a run
	// that produced one must not also report that there were none.
	if c.validate.tookNoValue() {
		switch {
		case rep.clean() && bounded:
			rep.info([]string{fmt.Sprintf("✓ %s: no errors in the feature values checked", namedModels(files))})
		// An error the check ran through was reported above, so the model is not
		// reported free of errors: what it is free of is one that stops a check.
		case rep.clean() && reportedErrors(sess.LocatedDiagnostics()):
			rep.info([]string{fmt.Sprintf("✓ %s: no error that stops a check; the notation reported above does not conform",
				namedModels(files))})
		case rep.clean():
			rep.info([]string{fmt.Sprintf("✓ %s: no errors", namedModels(files))})
		default:
			rep.failed(fmt.Sprintf("%s did not materialize cleanly", namedModels(files)))
		}
	}

	for _, expr := range exprs {
		output, err := sess.EvalExpr(expr)
		if err != nil {
			rep.failed(fmt.Sprintf("%s: %v", expr, err))
			return rep.finish()
		}
		rep.info(output)
	}

	// An object is validated as a whole: every assertion about it and about the
	// objects it holds, then a summary verdict about the object.
	for _, object := range c.validate.names() {
		for _, v := range sess.ValidateObject(object) {
			rep.verdict(v)
		}
	}

	for _, name := range c.constraints {
		rep.verdict(sess.CheckConstraint(name))
	}
	for _, name := range c.requirements {
		rep.verdict(sess.CheckRequirement(name))
	}
	for _, target := range c.satisfy.targets {
		for _, v := range sess.CheckSatisfy(target) {
			rep.verdict(v)
		}
	}
	for _, invocation := range c.calcs {
		if c.sweeping() {
			rep.verdict(c.sweep(sess, invocation))
			continue
		}
		rep.verdict(sess.RunCalc(invocation))
	}
	for _, invocation := range c.analyses {
		switch {
		case c.sweeping():
			rep.verdict(c.sweep(sess, invocation))
		case c.runs.given:
			rep.verdict(sess.RunMonteCarlo(invocation, c.runs.value, c.seed.seed()))
		default:
			rep.verdict(sess.RunAnalysis(invocation))
		}
	}
	// With -advance every behavior named is started first and the clock they share
	// is moved once, so an action's signal reaches a machine that accepts it later;
	// under the check engine it bounds the search of the invocation's schedules.
	if c.advance.given {
		for _, v := range sess.RunFor(behaviors(c.actions), behaviors(c.states), advance) {
			rep.verdict(v)
		}
		c.runQueries(sess, rep)
		return rep.finish()
	}
	for _, value := range c.actions {
		name, performer := repl.SplitBehavior(value)
		if c.runs.given {
			rep.verdict(sess.RunRuns(name, performer, c.runs.value, c.seed.seed(), c.observe))
			continue
		}
		rep.verdict(sess.RunAction(name, performer...))
	}
	for _, value := range c.states {
		name, performer := repl.SplitBehavior(value)
		rep.verdict(sess.RunStateMachine(name, performer...))
	}
	c.runQueries(sess, rep)

	return rep.finish()
}

// runQueries executes each -run-query after the behaviors named have run, so a
// query over the session's states or trace reads what the run did.
func (c *checks) runQueries(sess *repl.Session, rep *reporter) {
	for _, invocation := range c.queries {
		rep.verdict(sess.RunDocumentQuery(invocation))
	}
}

// behaviors reads `-action`/`-state` values as the behaviors they name.
func behaviors(values []string) []repl.Behavior {
	out := make([]repl.Behavior, 0, len(values))
	for _, value := range values {
		name, performer := repl.SplitBehavior(value)
		out = append(out, repl.Behavior{Name: name, Performer: performer})
	}
	return out
}

// sweep runs one invocation once per row of the ranges given: over every value
// of each range, or over values drawn from them when -samples was asked for.
func (c *checks) sweep(sess *repl.Session, invocation string) repl.Verdict {
	if c.samples.given {
		return sess.RunSamples(invocation, c.sweeps, c.samples.value, c.seed.value)
	}
	return sess.RunSweep(invocation, c.sweeps)
}

// reportedErrors reports whether analysis found an error, which a check runs
// through only when it is about the notation (see diag.Diagnostic.Blocking).
func reportedErrors(diags []repl.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseAdvance reads the -advance value, which is the simulated time the
// behaviors named are run for.
func parseAdvance(value string) (float64, error) {
	duration, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("-advance takes a number of time units, not %q", value)
	}
	if math.IsNaN(duration) || math.IsInf(duration, 0) {
		return 0, fmt.Errorf("-advance takes a duration to run for, and %q is not one", value)
	}
	if duration < 0 {
		return 0, fmt.Errorf("-advance takes a duration to run for, and %v runs backwards", duration)
	}
	return duration, nil
}
