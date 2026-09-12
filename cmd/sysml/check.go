package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/repl"
)

// checks are the model checks and runs named on the command line, in the order
// they are carried out: objects are created first, so a verdict is about them,
// and behavior runs after the conditions the model states about it.
type checks struct {
	validate     bool
	instantiate  stringSlice
	constraints  stringSlice
	requirements stringSlice
	satisfy      satisfyTargets
	calcs        stringSlice
	analyses     stringSlice
	sweeps       stringSlice
	samples      sweepCount
	seed         sweepSeed
	queries      stringSlice
	actions      stringSlice
	states       stringSlice
	advance      advanceTime
	jsonOut      bool
	checker      checkerOptions
}

// checkerOptions are the -check-* flags: what the check engine is asked beside
// -engine check -action, and the bounds its search runs under.
type checkerOptions struct {
	diverge    stringSlice
	properties stringSlice
	witness    string
	depth      positiveCount
	states     positiveCount
	timeout    checkTimeout
}

// given reports whether any -check-* flag was written.
func (o *checkerOptions) given() bool {
	return len(o.diverge) > 0 || len(o.properties) > 0 || o.witness != "" ||
		o.depth.given || o.states.given || o.timeout.given
}

// positiveCount is a -check-depth or -check-states value as written: a bound of
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

// sweepSeed is -seed as written: the seed a sampled sweep draws from, which is
// required rather than defaulted so a table is reproducible.
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
	return c.validate || c.jsonOut || c.advance.given || c.satisfy.given || len(c.instantiate) > 0 ||
		len(c.constraints) > 0 || len(c.requirements) > 0 || len(c.calcs) > 0 || len(c.analyses) > 0 ||
		len(c.queries) > 0 || len(c.actions) > 0 || len(c.states) > 0 ||
		c.sweeping() || c.checker.given()
}

// checkerMisuse reports why the -check-* flags written check nothing under the
// engine selected, and "" when they check an action: under -engine check always,
// under -engine all when one of them is written.
func (c *checks) checkerMisuse(engine string) string {
	selection := analysis.ParseSelection(engine)
	checking := selection == analysis.Only(analysis.CheckEngineName) ||
		selection.Mode == analysis.SelectAll && c.checker.given()
	switch {
	case c.checker.given() && !checking:
		return "-check-diverge, -check-property, -check-witness, -check-depth, -check-states and -check-timeout are the check engine's; select it, as -engine check, or every engine, as -engine all"
	case c.checker.given() && len(c.actions) == 0:
		return "the -check-* flags search an action's schedules; name one, as -action <name>"
	case checking && c.advance.given:
		return "-advance runs behaviors on one clock, which a search of every schedule of an action does not; drop one of them"
	}
	return ""
}

// sweeping reports whether a sweep or a sample of one was asked for.
func (c *checks) sweeping() bool {
	return len(c.sweeps) > 0 || c.samples.given || c.seed.given
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
	case c.seed.given && !c.samples.given:
		return "-seed is the seed -samples draws from; name how many to draw, as -samples <number>"
	}
	return ""
}

// checksOnly reports whether anything was asked about the model itself, as
// against how to report the answer.
func (c *checks) checksOnly() bool {
	return c.validate || len(c.instantiate) > 0 || len(c.constraints) > 0 ||
		len(c.requirements) > 0 || len(c.satisfy.targets) > 0 || len(c.calcs) > 0 || len(c.analyses) > 0 ||
		len(c.queries) > 0 || len(c.actions) > 0 || len(c.states) > 0
}

// satisfyTargets collects -satisfy values. The flag takes an optional value: a
// bare -satisfy evaluates every satisfaction assertion in the model, and
// -satisfy=<name> evaluates the ones the named element states. Go's flag package
// passes "true" for the valueless spelling, which no name can be mistaken for
// because `true` is a literal keyword rather than a declarable name.
type satisfyTargets struct {
	targets []string
	given   bool
}

func (t *satisfyTargets) String() string { return fmt.Sprint(t.targets) }

// IsBoolFlag makes the value optional, so -satisfy alone is accepted.
func (t *satisfyTargets) IsBoolFlag() bool { return true }

func (t *satisfyTargets) Set(value string) error {
	t.given = true
	switch value {
	case "true":
		// Every assertion in the model, which CheckSatisfy names with "".
		t.targets = append(t.targets, "")
	case "false":
		// The off spelling of a flag declared boolean, so -satisfy=$on works.
	default:
		t.targets = append(t.targets, value)
	}
	return nil
}

// tookNoValue reports whether -satisfy was given without a value. A name written
// after it (`-satisfy Landing::touchdown`) is then a positional argument, i.e. a
// file to load, which is worth explaining when no such file exists.
func (t *satisfyTargets) tookNoValue() bool {
	for _, target := range t.targets {
		if target == "" {
			return true
		}
	}
	return false
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
	sess.SetCheckWitnessDir(c.checker.witness)
	sess.SetCheckBounds(c.checker.depth.value, c.checker.states.value, c.checker.timeout.value)

	// A checked model may be named as a directory or a glob as well as by file, so
	// the paths are expanded to the files they stand for before loading.
	paths, err := repl.ExpandPaths(files)
	if err != nil {
		rep.failed(err.Error())
		if c.satisfy.tookNoValue() && len(files) == 1 && !fileExists(files[0]) {
			rep.failed(fmt.Sprintf("%s is read as a file to load; -satisfy takes a name as -satisfy=%s", files[0], files[0]))
		}
		return rep.finish()
	}

	// The files are loaded as one submission, indexed and analyzed once, and
	// each is still summarized on its own.
	loaded, err := sess.LoadFilesSummary(paths)
	if err != nil {
		rep.failed(err.Error())
		var read *repl.ReadError
		if errors.As(err, &read) && c.satisfy.tookNoValue() && !fileExists(read.Path) {
			rep.failed(fmt.Sprintf("%s is read as a file to load; -satisfy takes a name as -satisfy=%s", read.Path, read.Path))
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
	if c.validate {
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
		if c.sweeping() {
			rep.verdict(c.sweep(sess, invocation))
			continue
		}
		rep.verdict(sess.RunAnalysis(invocation))
	}
	for _, invocation := range c.queries {
		rep.verdict(sess.RunDocumentQuery(invocation))
	}
	// With -advance every behavior named is started first and the clock they share
	// is moved once, so an action's signal reaches a machine that accepts it later.
	if c.advance.given {
		for _, v := range sess.RunFor(behaviors(c.actions), behaviors(c.states), advance) {
			rep.verdict(v)
		}
		return rep.finish()
	}
	for _, value := range c.actions {
		name, performer := splitPerformer(value)
		rep.verdict(sess.RunAction(name, performer...))
	}
	for _, value := range c.states {
		name, performer := splitPerformer(value)
		rep.verdict(sess.RunStateMachine(name, performer...))
	}

	return rep.finish()
}

// behaviors reads `-action`/`-state` values as the behaviors they name.
func behaviors(values []string) []repl.Behavior {
	out := make([]repl.Behavior, 0, len(values))
	for _, value := range values {
		name, performer := splitPerformer(value)
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
// through only when it is about the notation (see passes.Diagnostic.Blocking).
func reportedErrors(diags []repl.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

// splitPerformer splits a `-action`/`-state` value into the behavior's name and
// the object performing it, which is the word after it as `%action` takes it:
// `-action "Drive rover1"`.
func splitPerformer(value string) (string, []string) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
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
