package sessiontools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtrace"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// RunInput names a behavior to run.
type RunInput struct {
	SessionName
	Kind      string   `json:"kind" jsonschema:"what to run: action, state (a state machine), calc or analysis (an analysis, verification or trade-study case)"`
	Name      string   `json:"name" jsonschema:"qualified name of the behavior"`
	Arguments []string `json:"arguments,omitempty" jsonschema:"argument expressions: positional for a calc; positional, in parameter order, or written parameter=expression for an action's inputs or an analysis case"`
	Object    string   `json:"object,omitempty" jsonschema:"an object the session holds (name, #id or path such as car.fl): the performer of an action or state machine, the subject of a case"`
	Duration  *float64 `json:"duration,omitempty" jsonschema:"for state: advance the machine's clock by this many time units after its initial transition; omit to take the initial transition alone"`
	Schedule  string   `json:"schedule,omitempty" jsonschema:"scheduling policy for this run alone: declared, reverse, seed:<n> or explore[:runs=N,depth=D]; omit for the session's"`
}

// SweepInput names an invocation to run once per row of its ranges.
type SweepInput struct {
	SessionName
	Name      string   `json:"name" jsonschema:"qualified name of the calc or analysis case"`
	Arguments []string `json:"arguments,omitempty" jsonschema:"argument expressions fixed for every row, positional or parameter=expression"`
	Object    string   `json:"object,omitempty" jsonschema:"the case's subject, an object the session holds"`
	Ranges    []string `json:"ranges" jsonschema:"one per swept parameter, written parameter=from..to[:step]; the rows are the cartesian product"`
	Samples   *Samples `json:"samples,omitempty" jsonschema:"draw rows uniformly from the ranges instead of stepping them"`
}

// Samples is how many rows a sampled sweep draws and from what seed.
type Samples struct {
	Count int64  `json:"count" jsonschema:"rows to draw"`
	Seed  uint64 `json:"seed" jsonschema:"the same seed draws the same rows"`
}

// RunsInput names an action to run repeatedly.
type RunsInput struct {
	SessionName
	Kind        string   `json:"kind,omitempty" jsonschema:"action (default): run an action count times and table the observables; case: a Simulation::MonteCarlo analysis case, run count times on fresh subjects and concluded"`
	Name        string   `json:"name" jsonschema:"qualified name of the action or case"`
	Arguments   []string `json:"arguments,omitempty" jsonschema:"for case: argument expressions"`
	Object      string   `json:"object,omitempty" jsonschema:"for action: the object performing it; for case: its subject"`
	Count       int64    `json:"count" jsonschema:"how many runs"`
	Seed        *uint64  `json:"seed,omitempty" jsonschema:"seed each run's modeled randomness derives from; omit for the session's"`
	Observables []string `json:"observables,omitempty" jsonschema:"for action: the features to table with their distributions; every feature the action holds and the clock when omitted"`
}

// RenderRunInput asks for the recorded run drawn.
type RenderRunInput struct {
	SessionName
	Kind string `json:"kind" jsonschema:"timeline (state occupancy per object machine over clock time) or sequence (the messages sent)"`
	Form string `json:"form,omitempty" jsonschema:"text (default), mermaid, plantuml, svg (a self-contained SVG drawing) or svg-animated (the SVG replaying the run with SMIL)"`
}

// RenderedRun is a run's trace drawn.
type RenderedRun struct {
	Kind      string  `json:"kind"`
	Form      string  `json:"form"`
	Until     float64 `json:"until" jsonschema:"the clock instant the rendering runs through"`
	Text      string  `json:"text" jsonschema:"the rendering in the form asked for"`
	Rendering any     `json:"rendering" jsonschema:"the rendering as data: lanes with their state spans, transitions and marks for a timeline; nodes and edges for a sequence"`
}

// invocation spells a behavior invocation as %calc and %analysis read one:
// `Name(args) object`.
func invocation(name string, args []string, object string) string {
	inv := strings.TrimSpace(name)
	if len(args) > 0 {
		inv += "(" + strings.Join(args, ", ") + ")"
	}
	if object = strings.TrimSpace(object); object != "" {
		inv += " " + object
	}
	return inv
}

// performer is the object an action or machine is performed by, as the session
// API takes it: none, or one.
func performer(object string) []string {
	if object = strings.TrimSpace(object); object != "" {
		return []string{object}
	}
	return nil
}

// withSchedule runs fn under the scheduling policy spelled, when one is, and
// restores the session's own afterwards.
func withSchedule(sess *repl.Session, spelling string, fn func() repl.Verdict) (repl.Verdict, error) {
	if strings.TrimSpace(spelling) == "" {
		return fn(), nil
	}
	policy, err := runtime.ParseSchedulePolicy(spelling)
	if err != nil {
		return repl.Verdict{}, err
	}
	was := sess.Schedule()
	if err := sess.SetSchedule(policy); err != nil {
		return repl.Verdict{}, err
	}
	defer func() { _ = sess.SetSchedule(was) }()
	return fn(), nil
}

// sweep runs a calc or analysis case over the rows of the ranges given, or
// over rows sampled from them.
func sweep(_ context.Context, sess *repl.Session, in SweepInput) (Verdict, error) {
	if strings.TrimSpace(in.Name) == "" {
		return Verdict{}, errors.New("name is required")
	}
	if len(in.Ranges) == 0 {
		return Verdict{}, errors.New("ranges is required: at least one parameter=from..to[:step]")
	}
	inv := invocation(in.Name, in.Arguments, in.Object)
	if in.Samples != nil {
		if in.Samples.Count <= 0 {
			return Verdict{}, errors.New("samples.count must be positive")
		}
		return verdict(sess.RunSamples(inv, in.Ranges, in.Samples.Count, in.Samples.Seed)), nil
	}
	return verdict(sess.RunSweep(inv, in.Ranges)), nil
}

// renderRun draws the run the session recorded in the form asked for.
func renderRun(_ context.Context, sess *repl.Session, in RenderRunInput) (RenderedRun, error) {
	kind, ok := runtrace.ParseKind(strings.TrimSpace(in.Kind))
	if !ok {
		return RenderedRun{}, fmt.Errorf("kind %q is unknown; want timeline or sequence", in.Kind)
	}
	form, err := renderRunForm(in.Form)
	if err != nil {
		return RenderedRun{}, err
	}
	rendering, err := sess.RunTraceRendering(kind)
	if errors.Is(err, runtrace.ErrNoTrace) {
		return RenderedRun{}, errors.New("the session records no trace; run repl_command `%trace on` before the run")
	}
	if err != nil {
		return RenderedRun{}, err
	}
	drawn, err := rendering.WriteWith(form, view.Options{})
	if err != nil {
		return RenderedRun{}, err
	}
	raw, err := json.Marshal(rendering)
	if err != nil {
		return RenderedRun{}, fmt.Errorf("encoding the rendering: %w", err)
	}
	data, err := decoded(raw)
	if err != nil {
		return RenderedRun{}, err
	}
	return RenderedRun{Kind: string(kind), Form: string(form), Until: rendering.RunUntil, Text: drawn, Rendering: data}, nil
}

// repeatedRuns runs an action count times, or concludes a Monte Carlo case over
// count runs, as the kind names.
func repeatedRuns(sess *repl.Session, in RunsInput) (Verdict, error) {
	switch strings.ToLower(strings.TrimSpace(in.Kind)) {
	case "", "action":
		return verdict(sess.RunRuns(in.Name, performer(in.Object), in.Count, in.Seed, in.Observables)), nil
	case "case":
		return verdict(sess.RunMonteCarlo(invocation(in.Name, in.Arguments, in.Object), in.Count, in.Seed)), nil
	}
	return Verdict{}, fmt.Errorf("kind %q is unknown; want action or case", in.Kind)
}

// renderRunForm is the form render_run draws in: text when none is named.
func renderRunForm(name string) (view.Form, error) {
	f := strings.TrimSpace(name)
	if f == "" {
		return view.FormText, nil
	}
	form := view.Form(f)
	if !slices.Contains(view.Forms(), form) {
		return "", fmt.Errorf("form %q is unknown; want one of %v", name, view.Forms())
	}
	return form, nil
}

// registerRuns adds the tools that run behaviors and table their runs.
func registerRuns(s *mcp.Server, m *Manager) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_run",
		Description: "Run a behavior in a session to completion and report what it produced: an action (%action then %continue), a state machine (%state, then %advance when a duration is given), a calc (%calc) or an analysis case (%analysis). Objects the run creates stay in the session, and the run's executor stays inspectable with debug_current until the next run or debug_stop. Under %engine check the behavior is checked over every schedule instead.",
		Annotations: toolkit.Mutating("Run behavior", false),
	}, handler(m, func(_ context.Context, sess *repl.Session, in RunInput) (Verdict, error) {
		if strings.TrimSpace(in.Name) == "" {
			return Verdict{}, errors.New("name is required")
		}
		var run func() repl.Verdict
		switch strings.ToLower(strings.TrimSpace(in.Kind)) {
		case "action":
			run = func() repl.Verdict { return sess.RunActionWithInputs(in.Name, in.Arguments, performer(in.Object)...) }
		case "state":
			run = func() repl.Verdict {
				if in.Duration != nil {
					return sess.RunStateMachineFor(in.Name, *in.Duration, performer(in.Object)...)
				}
				return sess.RunStateMachine(in.Name, performer(in.Object)...)
			}
		case "calc":
			run = func() repl.Verdict { return sess.RunCalc(invocation(in.Name, in.Arguments, in.Object)) }
		case "analysis":
			run = func() repl.Verdict { return sess.RunAnalysis(invocation(in.Name, in.Arguments, in.Object)) }
		default:
			return Verdict{}, fmt.Errorf("kind %q is unknown; want action, state, calc or analysis", in.Kind)
		}
		v, err := withSchedule(sess, in.Schedule, run)
		if err != nil {
			return Verdict{}, err
		}
		return verdict(v), nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "sweep",
		Description: "Run a calc or analysis case once per row of the ranges given (%sweep), or over rows drawn from them (%samples), and return the table: each row's inputs, outputs, verdicts and timing. The verdict holds when every run completed and every objective was satisfied.",
		Annotations: toolkit.Mutating("Sweep parameters", false),
	}, handler(m, sweep))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "runs",
		Description: "Run an action count times, each run's modeled randomness seeded on its own, and return the table of the observables with each one's distribution (%runs); or run a Simulation::MonteCarlo analysis case count times on fresh subjects and conclude it.",
		Annotations: toolkit.Mutating("Repeat runs", false),
	}, handler(m, func(_ context.Context, sess *repl.Session, in RunsInput) (Verdict, error) {
		if strings.TrimSpace(in.Name) == "" {
			return Verdict{}, errors.New("name is required")
		}
		if in.Count <= 0 {
			return Verdict{}, errors.New("count must be positive")
		}
		return repeatedRuns(sess, in)
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "render_run",
		Description: "Render the run a session recorded as a state timeline or a message sequence (%render-run), in a text, diagram or SVG form. Recording must be on before the run: repl_command `%trace on`.",
		Annotations: toolkit.ReadOnly("Render run"),
	}, handler(m, renderRun))
}
