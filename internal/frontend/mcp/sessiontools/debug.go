package sessiontools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// DebugStartInput names the behavior to debug.
type DebugStartInput struct {
	SessionName
	Kind   string `json:"kind" jsonschema:"action, or state for a state machine"`
	Name   string `json:"name" jsonschema:"qualified name of the action or machine; for state, also an object the session holds whose exhibited machine to attach to"`
	Object string `json:"object,omitempty" jsonschema:"an object the session holds (name, #id or path) performing the behavior"`
}

// BreakpointsInput sets breakpoints on the debugged action.
type BreakpointsInput struct {
	SessionName
	Set   []string `json:"set,omitempty" jsonschema:"node names to break at; debug_continue then stops when a token reaches one"`
	Clear bool     `json:"clear,omitempty" jsonschema:"remove the breakpoints set before, before setting the new ones"`
}

// SendInput is a signal to send.
type SendInput struct {
	SessionName
	Signal    string   `json:"signal" jsonschema:"the signal's definition, qualified if needed"`
	Arguments []string `json:"arguments,omitempty" jsonschema:"the signal's feature values, written parameter=expression"`
	To        string   `json:"to,omitempty" jsonschema:"the receiving object (name, #id or path); omit for the object whose behavior is being debugged"`
}

// AdvanceInput moves the clock.
type AdvanceInput struct {
	SessionName
	Time float64 `json:"time" jsonschema:"time units (seconds) to advance the debuggers' clock by, running every event, token and do behavior that comes due; 0 dispatches what is already due"`
}

// Token is one token of the debugged action.
type Token struct {
	ID          int64    `json:"id"`
	Node        string   `json:"node" jsonschema:"the node the token sits at"`
	ArrivedFrom string   `json:"arrivedFrom,omitempty" jsonschema:"at a join, the node the token came from"`
	Awaiting    []string `json:"awaiting,omitempty" jsonschema:"at a join, the nodes whose tokens are still awaited"`
}

// DebugState is where a debugging session stands.
type DebugState struct {
	Kind      string `json:"kind" jsonschema:"action or state for the executor described (with both debugged, the one still running, else the action); check when the behavior was checked over every schedule rather than stepped; empty when no debugging session runs"`
	Name      string `json:"name,omitempty" jsonschema:"the behavior as named"`
	Qualified string `json:"qualified,omitempty" jsonschema:"its qualified name"`
	Performer string `json:"performer,omitempty" jsonschema:"the object performing it"`
	State     string `json:"state,omitempty" jsonschema:"the executor's state as the prompt prints it: Ready, Running, Waiting, Suspended, Completed or Terminated"`
	Ended     bool   `json:"ended" jsonschema:"completed or terminated"`

	PausedAt    string       `json:"pausedAt,omitempty" jsonschema:"action: the breakpoint the run last stopped at"`
	Breakpoints []string     `json:"breakpoints,omitempty" jsonschema:"action: the nodes breakpoints are set at"`
	Tokens      []Token      `json:"tokens,omitempty" jsonschema:"action: the active tokens"`
	Results     []NamedValue `json:"results,omitempty" jsonschema:"action: the values its features hold, outputs included"`

	CurrentState    string       `json:"currentState,omitempty" jsonschema:"state machine: the active state, or one per orthogonal region"`
	ActiveStates    []string     `json:"activeStates,omitempty"`
	Time            float64      `json:"time,omitempty" jsonschema:"state machine: the clock"`
	LastEventAt     float64      `json:"lastEventAt,omitempty"`
	QueuedEvents    int          `json:"queuedEvents,omitempty"`
	SignalsInFlight []string     `json:"signalsInFlight,omitempty" jsonschema:"signals sent and not yet dispatched to the machine"`
	SuspendReason   string       `json:"suspendReason,omitempty" jsonschema:"why the machine cannot progress"`
	StateStack      []string     `json:"stateStack,omitempty" jsonschema:"the active configuration, outermost first"`
	StateData       []NamedValue `json:"stateData,omitempty"`

	Text string `json:"text" jsonschema:"what the prompt printed"`
}

func debugState(r repl.DebugReport) DebugState {
	out := DebugState{
		Kind: r.Kind, Name: r.Name, Qualified: r.Qualified, Performer: r.Performer, State: r.State, Ended: r.Ended,
		PausedAt: r.PausedAt, Breakpoints: r.Breakpoints, Results: namedValues(r.Results),
		CurrentState: r.CurrentState, ActiveStates: r.ActiveStates, Time: r.Time, LastEventAt: r.LastEventAt,
		QueuedEvents: r.QueuedEvents, SignalsInFlight: r.SignalsInFlight, SuspendReason: r.SuspendReason,
		StateStack: r.StateStack, StateData: namedValues(r.StateData), Text: text(r.Lines),
	}
	for _, t := range r.Tokens {
		out.Tokens = append(out.Tokens, Token{ID: t.ID, Node: t.Node, ArrivedFrom: t.ArrivedFrom, Awaiting: t.Awaiting})
	}
	return out
}

// Output is what a command printed.
type Output struct {
	Text string `json:"text"`
}

// debugTool registers a tool driving the debuggers, whose answer is where they stand.
func debugTool[In sessionNamer](s *mcp.Server, m *Manager, name, title, description string, mutating bool, step func(*repl.Session, In) (repl.DebugReport, error)) {
	annotations := toolkit.ReadOnly(title)
	if mutating {
		annotations = toolkit.Mutating(title, false)
	}
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: description, Annotations: annotations},
		handler(m, func(_ context.Context, sess *repl.Session, in In) (DebugState, error) {
			report, err := step(sess, in)
			if err != nil {
				return DebugState{}, failed(err, report.Lines)
			}
			return debugState(report), nil
		}))
}

// registerDebug adds the action and state machine debugger tools.
func registerDebug(s *mcp.Server, m *Manager) {
	debugTool(s, m, "debug_start", "Start debugging",
		"Start a debugging session on an action (%action) or a state machine (%state), performed by an object when one is named. The action is not stepped: debug_step advances one token step, debug_continue runs to completion or a breakpoint. A machine takes its entry; debug_step, advance and send drive it. Starting again replaces the session.",
		true, func(sess *repl.Session, in DebugStartInput) (repl.DebugReport, error) {
			if strings.TrimSpace(in.Name) == "" {
				return repl.DebugReport{}, errors.New("name is required")
			}
			switch strings.ToLower(strings.TrimSpace(in.Kind)) {
			case "action":
				return sess.DebugAction(in.Name, performer(in.Object))
			case "state":
				return sess.DebugState(in.Name, performer(in.Object))
			}
			return repl.DebugReport{}, fmt.Errorf("kind %q is unknown; want action or state", in.Kind)
		})
	debugTool(s, m, "debug_step", "Step",
		"Advance the debugged action one token step, or the debugged state machine one step (%step): one event dispatched, one change event, one round of do actions. A token waiting only on the clock is not stepped; advance moves the clock.",
		true, func(sess *repl.Session, _ SessionName) (repl.DebugReport, error) { return sess.DebugStep() })
	debugTool(s, m, "debug_continue", "Continue",
		"Run the debugged action to completion, or to the first breakpoint a token reaches (%continue), and report its results.",
		true, func(sess *repl.Session, _ SessionName) (repl.DebugReport, error) { return sess.DebugContinue() })
	debugTool(s, m, "debug_current", "Debugger state",
		"Report where the debugging session stands without changing it: the action's execution state, tokens, breakpoints and feature values (%tokens), or the machine's current state, clock, event queue, signals in flight and state data (%current, %events).",
		false, func(sess *repl.Session, _ SessionName) (repl.DebugReport, error) {
			report, ok := sess.DebugStatus()
			if !ok {
				return repl.DebugReport{}, errors.New("no debugging session runs; debug_start opens one")
			}
			return report, nil
		})
	debugTool(s, m, "debug_breakpoints", "Breakpoints",
		"Set breakpoints at nodes of the debugged action (%break), optionally clearing the earlier ones, and report the breakpoints it then has; with nothing to set, lists them. A node the action has none of fails, naming its nodes.",
		true, func(sess *repl.Session, in BreakpointsInput) (repl.DebugReport, error) {
			if len(in.Set) == 0 && !in.Clear {
				report, ok := sess.DebugStatus()
				if !ok || report.Kind != "action" {
					return repl.DebugReport{}, errors.New("no action is being debugged; debug_start with kind action first")
				}
				return report, nil
			}
			return sess.SetBreakpoints(in.Set, in.Clear)
		})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "debug_stop",
		Description: "End the debugging session (%stop), letting go of its run; objects the session holds stay.",
		Annotations: toolkit.Mutating("Stop debugging", true),
	}, handler(m, func(_ context.Context, sess *repl.Session, _ SessionName) (Output, error) {
		lines, err := sess.DebugStop()
		if err != nil {
			return Output{}, failed(err, lines)
		}
		return Output{Text: text(lines)}, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "send",
		Description: "Send a signal to an object through the runtime's message bus (%send), for the state machine it exhibits or the action parked at a matching accept to take; by default to the object whose behavior is being debugged. The signal is refused when no behavior of the object accepts it now. It is dispatched by the next debug_step or advance.",
		Annotations: toolkit.Mutating("Send signal", false),
	}, handler(m, func(_ context.Context, sess *repl.Session, in SendInput) (Output, error) {
		signal := strings.TrimSpace(in.Signal)
		if signal == "" {
			return Output{}, errors.New("signal is required")
		}
		line := signal
		if len(in.Arguments) > 0 {
			line += "(" + strings.Join(in.Arguments, ", ") + ")"
		}
		if to := strings.TrimSpace(in.To); to != "" {
			line += " to " + to
		}
		lines, err := sess.SendSignal(line)
		if err != nil {
			return Output{}, failed(err, lines)
		}
		return Output{Text: text(lines)}, nil
	}))
	debugTool(s, m, "advance", "Advance clock",
		"Advance the debuggers' simulation clock (%advance), running every state event, action token, change-condition poll and do behavior that comes due, in due order, and report what ran and where the debuggers stand. The clock is the session's: a debugged action and machine both move.",
		true, func(sess *repl.Session, in AdvanceInput) (repl.DebugReport, error) { return sess.Advance(in.Time) })
	registerInspect(s, m)
}

// FeaturesInput names an object to inspect.
type FeaturesInput struct {
	SessionName
	Object string `json:"object" jsonschema:"an object the session holds: its name, #id, or a path such as car.fl or #1.wheels[2]"`
	Depth  *int   `json:"depth,omitempty" jsonschema:"expand nested objects to this depth without a size bound; omit for the prompt's bounded listing"`
	All    bool   `json:"all,omitempty" jsonschema:"expand every nested object; overrides depth"`
}

// Features is an object's feature values.
type Features struct {
	Text  string `json:"text" jsonschema:"the listing as %features prints it: each feature's value and what the object's behaviors are doing"`
	Graph any    `json:"graph,omitempty" jsonschema:"the object and everything reachable from it as the runtime API's Instantiate writes it (opensysml.v1 JSON); absent when the build does not link the graph writer"`
}

// Instance is one object the session holds.
type Instance struct {
	Name          string `json:"name,omitempty" jsonschema:"the name %instantiate created it under; empty for an object a later instantiation displaced"`
	ID            int64  `json:"id" jsonschema:"addresses the object as #<id>"`
	Type          string `json:"type" jsonschema:"qualified name of what it instantiates"`
	DisplacedFrom string `json:"displacedFrom,omitempty" jsonschema:"the name a later instantiation took from it"`
	Life          string `json:"life,omitempty" jsonschema:"destroyed or ended when its lifetime is over"`
}

// Instances are the objects a session holds.
type Instances struct {
	Instances []Instance `json:"instances"`
	Text      string     `json:"text" jsonschema:"what %instances printed"`
}

// registerInspect adds the tools reading the instance graph.
func registerInspect(s *mcp.Server, m *Manager) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "features",
		Description: "Show an object's feature values and what its behaviors are doing (%features): the values its attributes hold, the objects its parts and ports hold, its connectors, and the state of each machine it exhibits and action it performs. Objects come from repl_command `%instantiate <name>` or from runs.",
		Annotations: toolkit.ReadOnly("Object features"),
	}, handler(m, func(_ context.Context, sess *repl.Session, in FeaturesInput) (Features, error) {
		if strings.TrimSpace(in.Object) == "" {
			return Features{}, errors.New("object is required")
		}
		listing := repl.FeatureListing{Depth: -1, All: in.All}
		if in.Depth != nil && !in.All {
			if *in.Depth < 0 {
				return Features{}, errors.New("depth must be non-negative")
			}
			listing.Depth = *in.Depth
		}
		report, err := sess.Features(in.Object, listing)
		if err != nil {
			return Features{}, failed(err, report.Lines)
		}
		out := Features{Text: text(report.Lines)}
		if report.Graph != nil {
			graph, err := decoded(report.Graph)
			if err != nil {
				return Features{}, err
			}
			out.Graph = graph
		}
		return out, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "instances",
		Description: "List the objects a session holds (%instances): each with its id, type and lifetime, the ones a later instantiation displaced included.",
		Annotations: toolkit.ReadOnly("List instances"),
	}, handler(m, func(_ context.Context, sess *repl.Session, in SessionName) (Instances, error) {
		held, lines := sess.Instances()
		out := Instances{Instances: make([]Instance, 0, len(held)), Text: text(lines)}
		for _, h := range held {
			out.Instances = append(out.Instances, Instance{Name: h.Name, ID: h.ID, Type: h.Type, DisplacedFrom: h.DisplacedFrom, Life: h.Life})
		}
		return out, nil
	}))
}
