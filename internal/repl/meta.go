package repl

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// renderUsage is how %render is written: a view, and the form to write it in,
// text when none is named.
const renderUsage = "usage: %render <name> [text|mermaid|markdown]"

// isMeta reports whether a trimmed input line is a meta command.
func isMeta(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "%")
}

// parseArgs splits a command line into arguments, handling quoted strings.
// A double-quoted string is one argument with its quotes removed, which is how a
// file path holding spaces is written; a SysML single-quoted name is one
// argument with its quotes kept, since the quotes are part of the notation the
// name is then parsed as ('My Pkg'::Car).
// Example: `%load "path with spaces/file.sysml"` -> ["%load", "path with spaces/file.sysml"]
func parseArgs(line string) []string {
	var args []string
	var current strings.Builder
	inQuote := false // inside a "…" string
	inName := false  // inside a '…' unrestricted name
	escaped := false

	runes := []rune(line)
	for i, r := range runes {
		switch {
		case escaped:
			// Previous char was backslash - add this char literally
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			// Inside a name the backslash is the notation's own escape, so it stays
			// with the character it escapes.
			if inName {
				current.WriteRune(r)
			}
			escaped = true
		case r == '"' && !inName:
			// Toggle quote mode
			inQuote = !inQuote
		case r == '\'' && !inQuote && inName:
			inName = false
			current.WriteRune(r)
		case r == '\'' && !inQuote && opensName(current.String(), runes[i+1:]):
			inName = true
			current.WriteRune(r)
		case (r == ' ' || r == '\t') && !inQuote && !inName:
			// Whitespace outside quotes - end current arg
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			// Regular character - add to current arg
			current.WriteRune(r)
		}
	}

	// Add final argument if any
	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// opensName reports whether a single quote begins an unrestricted name rather
// than being an apostrophe in ordinary text. A name starts an argument or follows
// a `::` qualifier or a `.` in an object path, and is closed later on the line;
// anything else — a path like o'brien/model.sysml — leaves the rest of the line
// split as it was.
func opensName(sofar string, rest []rune) bool {
	if sofar != "" && !strings.HasSuffix(sofar, "::") && !strings.HasSuffix(sofar, ".") {
		return false
	}
	for i, r := range rest {
		if r == '\'' && (i == 0 || rest[i-1] != '\\') {
			return true
		}
	}
	return false
}

// Literals the command table, its dispatch and the help headings share.
const (
	cmdQuery          = "%query"
	cmdAnalysis       = "%analysis"
	cmdSweep          = "%sweep"
	cmdSamples        = "%samples"
	cmdRunQuery       = "%run-query"
	cmdRenderDocument = "%render-document"
	argName           = "<name>"
	timeLabel         = "  Time: "

	groupLibrary    = "Library discovery:"
	groupRuntime    = "Runtime commands:"
	groupBehavioral = "Behavioral commands:"
	groupAction     = "Action debugging:"
	groupState      = "State machine debugging:"
)

// metaCommand is one prompt command. The table below is what the help text,
// tab completion and the unknown-command suggestion all read.
type metaCommand struct {
	name  string
	args  string // the arguments help shows after the name
	desc  string
	group string // help heading this command is listed under
	alias bool   // an alternative spelling, dispatched but not listed
}

var metaCommandTable = []metaCommand{
	{name: "%help", desc: "show this help"},
	{name: "%list", desc: "list current session declarations"},
	{name: "%clear", desc: "reset the session"},
	{name: "%load", args: "<path>...", desc: "submit the contents of files, directories or globs"},
	{name: "%print", args: "[name]", desc: "print the session model as SysML notation, or just the named element"},
	{name: "%save", args: "<file>", desc: "write the session model to a file (.sysml notation, or .ttl RDF — experimental)"},
	{name: cmdQuery, args: "<oslc-query>", desc: "identify model elements using OSLC Query text"},
	{name: "%verbosity", args: "[level]", desc: "show or set output level: quiet, normal or debug"},
	{name: "%trace", args: "[on|off]", desc: "show or set execution tracing (evaluation, calc, action and state steps)"},
	{name: "%strict", args: "[on|off]", desc: "show or set strict conformance: report notation no SysML v2 production admits as an error"},
	{name: "%schedule", args: "[<policy>]", desc: "show or set the scheduling policy runs started from here on resolve choice points under: declared, reverse or seed:<n>"},
	{name: "%budget", desc: "show the bounds one run may spend, and the variable raising each"},
	{name: "%quit", desc: "exit the REPL"},
	{name: "%exit", desc: "exit the REPL", alias: true},

	{group: groupLibrary, name: "%search", args: "<substring>", desc: "list the declared and library symbols whose qualified name contains <substring>"},
	{group: groupLibrary, name: "%builtins", desc: "list the library functions this build implements directly"},
	{group: groupLibrary, name: "%view", args: argName, desc: "show what a view exposes, and the views nested in it"},
	{group: groupLibrary, name: "%render", args: "<name> [form]", desc: "render a view as the rendering it states — as text, or as a Mermaid diagram or a Markdown table"},

	{group: groupRuntime, name: "%instantiate", args: argName, desc: "create an instance of a part def"},
	{group: groupRuntime, name: "%eval", args: "[in <name>|<path>|#<id> :] <expr>", desc: "evaluate an expression, in the named element or object when one is named"},
	{group: groupRuntime, name: "%features", args: "<object> [all|depth <n>] [json]", desc: "show an object's feature values and what its behaviors are doing, bounded unless all or a depth is asked for; json writes the object graph as the API does; an object is named, #<id>, or a path such as car.fl or #1.wheels[2]"},
	{group: groupRuntime, name: "%instances", desc: "list all instantiated objects"},
	{group: groupRuntime, name: "%invoke", args: "<object> <op> [<p>=<expr>]", desc: "invoke an operation of an object's type, performed by that object; an object is named, #<id>, or a path such as car.fl"},

	{group: groupBehavioral, name: "%calc", args: "<name> <args>", desc: "invoke a calculation with arguments"},
	{group: groupBehavioral, name: cmdAnalysis, args: "<name>[(<args>)] [<object>]", desc: "run an analysis case and report its outputs and the verdict of its objective; arguments bind its inputs and an object is its subject"},
	{group: groupBehavioral, name: cmdSweep, args: "<name>[(<args>)] [<object>] <p>=<from>..<to>[:<step>]...", desc: "run an analysis case or calc once per value of each range, one run per row of the cartesian product, and print the table"},
	{group: groupBehavioral, name: cmdSamples, args: "<n> <seed> <name>[(<args>)] [<object>] <p>=<from>..<to>...", desc: "run an analysis case or calc over <n> values drawn uniformly from each range with the given seed, and print the table"},
	{group: groupBehavioral, name: cmdRunQuery, args: "<name> [<p>=<expr>...]", desc: "execute a document query and print its rows, with each binding written as <parameter>=<expression>"},
	{group: groupBehavioral, name: cmdRenderDocument, args: argName, desc: "compile a document definition, run its queries and print the rendered Markdown"},
	{group: groupBehavioral, name: "%constraint", args: argName, desc: "evaluate a constraint definition"},
	{group: groupBehavioral, name: "%requirement", args: argName, desc: "evaluate a requirement definition"},
	{group: groupBehavioral, name: "%satisfy", args: "[name]", desc: "evaluate the satisfaction assertions of the model, or of one element"},
	{group: groupBehavioral, name: "%check", args: argName, desc: "ask an SMT solver whether a constraint, requirement or satisfaction can be satisfied (experimental)"},
	{group: groupBehavioral, name: "%explain", args: argName, desc: "ask an SMT solver which conditions of an unsatisfiable element conflict (experimental)"},
	{group: groupBehavioral, name: "%solve", args: argName, desc: "ask an SMT solver for values satisfying an element, keeping what is already fixed (experimental)"},
	{group: groupBehavioral, name: "%configure", args: "<name> [<variation>=<variant>...] [all [<count>]]", desc: "ask an SMT solver which variants an element's conditions permit (experimental)"},
	{group: groupBehavioral, name: "%optimize", args: argName, desc: "ask an SMT solver for the best values an analysis case's objectives admit (experimental)"},

	{group: groupAction, name: "%action", args: "<name> [<object>]", desc: "start action executor debugging session, performed by an object"},
	{group: groupAction, name: "%step", desc: "advance one token step, or one step of the state machine being debugged"},
	{group: groupAction, name: "%continue", desc: "run action to completion"},
	{group: groupAction, name: "%tokens", desc: "show active tokens"},
	{group: groupAction, name: "%break", args: "<node>", desc: "set breakpoint at node"},
	{group: groupAction, name: "%stop", desc: "stop current debugging session"},

	{group: groupState, name: "%state", args: "<name> [<object>]", desc: "debug the machine an object exhibits (naming that machine alone attaches to the one object exhibiting it; with none or several, name the object), or a state machine performed by an object; an object is named, #<id>, or a path such as car.fl"},
	{group: groupState, name: "%send", args: "<signal>[(<p>=<expr>, ...)] [to <object>]", desc: "send a signal to an object's machine, by default the one being debugged; an object is named, #<id>, or a path such as car.fl"},
	{group: groupState, name: "%events", desc: "show event queue and signals in flight"},
	{group: groupState, name: "%current", desc: "show current state and configuration"},
	{group: groupState, name: "%advance", args: "<time>", desc: "advance simulation time by <time> units, processing every event due"},
}

// helpText renders the command table, one line per command under its heading.
func helpText() []string {
	const width = 20
	var out []string
	group := ""
	for _, c := range metaCommandTable {
		if c.alias {
			continue
		}
		if c.group != group && c.group != "" {
			group = c.group
			out = append(out, "", group)
		}
		usage := strings.TrimSpace(c.name + " " + c.args)
		if pad := width - len(usage); pad > 0 {
			usage += strings.Repeat(" ", pad)
		} else {
			usage += "  "
		}
		out = append(out, usage+c.desc)
	}
	return out
}

// metaCommands returns every command name the prompt dispatches, aliases
// included, in table order.
func metaCommands() []string {
	out := make([]string, 0, len(metaCommandTable))
	for _, c := range metaCommandTable {
		out = append(out, c.name)
	}
	return out
}

// runMeta executes a meta command line. Returns lines to print, whether to quit,
// and an error only for unrecoverable I/O (unknown commands print guidance).
// RunMeta executes a meta-command (e.g., %eval, %load) and returns the output lines,
// a quit flag, and any error encountered.
func (s *Session) RunMeta(line string) (out []string, quit bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, quit, err = s.runMeta(line)
	return append(s.drainTrace(), out...), quit, err
}

func (s *Session) runMeta(line string) (out []string, quit bool, err error) {
	fields := parseArgs(strings.TrimSpace(line))
	if len(fields) == 0 {
		return nil, false, nil
	}
	for _, run := range []func([]string, string) (metaResult, bool){
		s.metaSessionCommand, s.metaModelCommand, s.metaDebugCommand,
	} {
		if res, ok := run(fields, line); ok {
			return res.out, res.quit, res.err
		}
	}
	return []string{unknownCommandLine(fields[0])}, false, nil
}

// metaResult is a meta command's outcome: lines to print, whether to quit, and
// an unrecoverable error.
type metaResult struct {
	out  []string
	quit bool
	err  error
}

func metaOut(out []string, quit bool, err error) metaResult {
	return metaResult{out: out, quit: quit, err: err}
}

// metaSessionCommand runs a session-level command, reporting whether the
// line named one.
func (s *Session) metaSessionCommand(fields []string, line string) (metaResult, bool) {
	switch fields[0] {
	case "%help":
		return metaOut(helpText(), false, nil), true
	case "%list":
		decls := s.list()
		if len(decls) == 0 {
			return metaOut([]string{"(empty session)"}, false, nil), true
		}
		return metaOut(decls, false, nil), true
	case "%clear":
		return metaOut(append([]string{"session cleared"}, s.clear()...), false, nil), true
	case "%load":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %load <file|dir|glob>..."}, false, nil), true
		}
		lines, lerr := s.loadPaths(pathArgs(fields[1:]))
		if lerr != nil {
			return metaOut(nil, false, lerr), true
		}
		return metaOut(lines, false, nil), true
	case "%print":
		if len(fields) < 2 {
			return metaOut(s.doPrint("")), true
		}
		return metaOut(s.doPrint(fields[1])), true
	case "%save":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %save <file.sysml|file.ttl>"}, false, nil), true
		}
		return metaOut(s.doSave(nameText(fields[1]))), true
	case "%verbosity":
		if len(fields) < 2 {
			return metaOut([]string{fmt.Sprintf("verbosity: %s", s.verbosity)}, false, nil), true
		}
		v, verr := ParseVerbosity(fields[1])
		if verr != nil {
			return metaOut([]string{errPrefix + verr.Error()}, false, nil), true
		}
		s.verbosity = v
		return metaOut([]string{fmt.Sprintf("verbosity: %s", v)}, false, nil), true
	case "%trace":
		if len(fields) >= 2 {
			switch fields[1] {
			case "on":
				s.setTracing(true)
			case "off":
				s.setTracing(false)
			default:
				return metaOut([]string{fmt.Sprintf("error: unknown trace setting %q (want on or off)", fields[1])}, false, nil), true
			}
		}
		return metaOut([]string{fmt.Sprintf("trace: %s", onOff(s.trace != nil))}, false, nil), true
	case "%strict":
		return metaOut(s.doStrict(fields[1:]), false, nil), true
	case "%schedule":
		return metaOut(s.doSchedule(fields[1:]), false, nil), true
	case "%budget":
		return metaOut(s.doBudget(), false, nil), true
	case "%search":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %search <substring>"}, false, nil), true
		}
		return metaOut(s.doSearch(nameText(fields[1]))), true
	case "%builtins":
		return metaOut(s.doBuiltins()), true
	case "%view":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %view <name>"}, false, nil), true
		}
		return metaOut(s.doView(fields[1])), true
	case "%render":
		if len(fields) < 2 || len(fields) > 3 {
			return metaOut([]string{renderUsage}, false, nil), true
		}
		form := view.FormText
		if len(fields) == 3 {
			form = view.Form(fields[2])
			if !slices.Contains(view.Forms(), form) {
				return metaOut([]string{fmt.Sprintf("unknown form %q; %s", fields[2], renderUsage)}, false, nil), true
			}
		}
		return metaOut(s.doRender(fields[1], form)), true
	case "%quit", "%exit":
		return metaOut([]string{"goodbye"}, true, nil), true
	}
	return metaResult{}, false
}

// metaModelCommand runs a model-level command, reporting whether the line
// named one.
func (s *Session) metaModelCommand(fields []string, line string) (metaResult, bool) {
	switch fields[0] {
	case "%instantiate":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %instantiate <name>"}, false, nil), true
		}
		return metaOut(s.doInstantiate(fields[1])), true
	case "%eval":
		if len(fields) < 2 {
			return metaOut([]string{evalUsage}, false, nil), true
		}
		tail := strings.TrimPrefix(strings.TrimSpace(line), "%eval")
		return metaOut(s.doEvalLine(strings.TrimSpace(tail))), true
	case "%features":
		if len(fields) < 2 {
			return metaOut([]string{featuresUsage}, false, nil), true
		}
		listing, perr := parseFeatureListing(fields[2:])
		if perr != nil {
			return metaOut([]string{errPrefix + perr.Error(), featuresUsage}, false, nil), true
		}
		return metaOut(s.doFeatures(fields[1], listing)), true
	case "%instances":
		return metaOut(s.doInstances()), true
	case "%calc":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %calc <name> [args...]"}, false, nil), true
		}
		name, argText := splitCalcArgs(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "%calc")))
		return metaOut(s.doCalc(name, argText)), true
	case cmdAnalysis:
		if len(fields) < 2 {
			return metaOut([]string{analysisUsage}, false, nil), true
		}
		return metaOut(s.doAnalysis(strings.TrimPrefix(strings.TrimSpace(line), cmdAnalysis))), true
	case cmdSweep:
		if len(fields) < 2 {
			return metaOut([]string{sweepUsage}, false, nil), true
		}
		return metaOut(s.doSweep(strings.TrimPrefix(strings.TrimSpace(line), cmdSweep))), true
	case cmdSamples:
		if len(fields) < 2 {
			return metaOut([]string{samplesUsage}, false, nil), true
		}
		return metaOut(s.doSamples(strings.TrimPrefix(strings.TrimSpace(line), cmdSamples))), true
	case cmdRunQuery:
		if len(fields) < 2 {
			return metaOut([]string{runQueryUsage}, false, nil), true
		}
		return metaOut(s.doRunQuery(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), cmdRunQuery)))), true
	case cmdRenderDocument:
		if len(fields) < 2 {
			return metaOut([]string{renderDocumentUsage}, false, nil), true
		}
		return metaOut(s.doRenderDocument(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), cmdRenderDocument)))), true
	case "%constraint":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %constraint <name>"}, false, nil), true
		}
		return metaOut(s.doConstraint(fields[1])), true
	case "%requirement":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %requirement <name>"}, false, nil), true
		}
		return metaOut(s.doRequirement(fields[1])), true
	case "%satisfy":
		return metaOut(s.doSatisfy(fields[1:])), true
	case "%check":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %check <name>"}, false, nil), true
		}
		return metaOut(s.doCheck(fields[1])), true
	case "%explain":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %explain <name>"}, false, nil), true
		}
		return metaOut(s.doExplain(fields[1])), true
	case "%solve":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %solve <name>"}, false, nil), true
		}
		return metaOut(s.doSolve(fields[1])), true
	case "%configure":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %configure <name> [<variation>=<variant>...] [all [<count>]]"}, false, nil), true
		}
		return metaOut(s.doConfigure(fields[1], fields[2:])), true
	case "%optimize":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %optimize <name>"}, false, nil), true
		}
		return metaOut(s.doOptimize(fields[1])), true
		// Action debugging
	}
	return metaResult{}, false
}

// metaDebugCommand runs an execution-debugging command, reporting whether
// the line named one.
func (s *Session) metaDebugCommand(fields []string, line string) (metaResult, bool) {
	switch fields[0] {
	case "%action":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %action <name> [<object>]"}, false, nil), true
		}
		return metaOut(s.doAction(fields[1], fields[2:])), true
	case "%step":
		return metaOut(s.doStep()), true
	case "%continue":
		return metaOut(s.doContinue()), true
	case "%tokens":
		return metaOut(s.doTokens()), true
	case "%break":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %break <node>"}, false, nil), true
		}
		return metaOut(s.doBreak(nameText(fields[1]))), true
	case "%stop":
		return metaOut(s.doStop()), true
	// State machine debugging
	case "%state":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %state <name> [<object>]"}, false, nil), true
		}
		return metaOut(s.doStateMachine(fields[1], fields[2:])), true
	case "%invoke":
		if len(fields) < 3 {
			return metaOut([]string{"usage: %invoke <object> <operation> [<parameter>=<expression> ...]"}, false, nil), true
		}
		return metaOut(s.doInvoke(fields[1], fields[2], fields[3:])), true
	case cmdQuery:
		if len(fields) < 2 {
			return metaOut([]string{"usage: %query <oslc-query>"}, false, nil), true
		}
		lines, err := s.query(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), cmdQuery)))
		if err != nil {
			return metaOut([]string{errPrefix + err.Error()}, false, nil), true
		}
		if len(lines) == 0 {
			return metaOut([]string{"no elements matched"}, false, nil), true
		}
		return metaOut(lines, false, nil), true
	case "%send":
		return metaOut(s.doSend(strings.TrimPrefix(strings.TrimSpace(line), "%send"))), true
	case "%events":
		return metaOut(s.doEvents()), true
	case "%current":
		return metaOut(s.doCurrent()), true
	case "%advance":
		if len(fields) < 2 {
			return metaOut([]string{"usage: %advance <time>"}, false, nil), true
		}
		return metaOut(s.doAdvance(fields[1])), true
	}
	return metaResult{}, false
}

// doInstantiate creates an instance of a part def. A runtime that cannot be
// created at all is unrecoverable, while a name the session cannot resolve is
// reported at the prompt.
func (s *Session) doInstantiate(name string) ([]string, bool, error) {
	if _, err := s.getOrCreateRuntime(); err != nil {
		return nil, false, fmt.Errorf("runtime init: %w", err)
	}

	lines, err := s.instantiateLines(name)
	if err != nil {
		return []string{errPrefix + err.Error()}, false, nil
	}
	return lines, false, nil
}

// evalUsage is what %eval accepts: an expression, optionally pinned to the
// context it is evaluated in.
const evalUsage = "usage: %eval [in <qualified-name> | <object-path> | #<id> :] <expression>"

// doEvalLine carries out %eval, in the context the line pins when it names one.
func (s *Session) doEvalLine(tail string) ([]string, bool, error) {
	rest, pinned := cutWord(tail, "in")
	if !pinned {
		// `in` alone is the pinned form with everything it needs left out.
		if strings.TrimSpace(tail) == "in" {
			return []string{evalUsage}, false, nil
		}
		return s.doEval(tail)
	}
	// `in` is a keyword, so it cannot start an expression: a tail beginning with
	// it is the pinned form, malformed or not.
	name, expr, ok := splitPinnedContext(rest)
	if !ok {
		return []string{evalUsage}, false, nil
	}
	lines, err := s.evalIn(name, expr)
	if err != nil {
		s.noteIfMaterializationFailure(err)
		return []string{errPrefix + err.Error()}, false, nil
	}
	return lines, false, nil
}

// cutWord removes a leading word from text, reporting whether it was there.
func cutWord(text, word string) (string, bool) {
	rest, ok := strings.CutPrefix(text, word)
	if !ok || rest == "" || !(rest[0] == ' ' || rest[0] == '\t') {
		return text, false
	}
	return strings.TrimSpace(rest), true
}

// splitPinnedContext reads `<qualified-name> : <expression>`, the tail of a
// pinned %eval. The separator is the first `:` outside a quoted name that is not
// part of a `::` qualifier.
func splitPinnedContext(tail string) (name, expr string, ok bool) {
	at := contextSeparator(tail)
	if at < 0 {
		return "", "", false
	}
	name = strings.TrimSpace(tail[:at])
	expr = strings.TrimSpace(tail[at+1:])
	return name, expr, name != "" && expr != ""
}

// contextSeparator locates the `:` that separates a pinned context from the
// expression, or -1 for a tail that states none.
func contextSeparator(tail string) int {
	inName, escaped := false, false
	for i := 0; i < len(tail); i++ {
		switch {
		case escaped:
			escaped = false
		case tail[i] == '\\':
			escaped = true
		case tail[i] == '\'':
			inName = !inName
		case inName || tail[i] != ':':
		case i+1 < len(tail) && tail[i+1] == ':':
			i++ // a qualifier, not the separator
		default:
			return i
		}
	}
	return -1
}

// evalIn evaluates an expression in the context the command pinned: the object
// the reference denotes (`ctx`, `#3`, `ctx.recv`), whose feature values it then
// reads as `%eval` does after `%instantiate`, or else the named element's own
// namespace. The expression is parsed before the reference materializes anything.
func (s *Session) evalIn(name, expr string) ([]string, error) {
	node, diags := parseExprAlone(expr)
	if len(diags) > 0 {
		return nil, exprError(expr, diags[0].Message, diags[0].Span, len(exprPrefix))
	}
	if node == nil {
		return nil, errors.New("could not parse expression")
	}
	inst, label, rerr := s.resolveObject(name)
	var noInstance *NotInstantiatedError
	if rerr != nil && (looksLikeObjectPath(name) || !errors.As(rerr, &noInstance)) {
		return nil, rerr
	}
	var (
		sym *symbols.Symbol
		fqn string
	)
	if inst != nil {
		sym, fqn = inst.Type, label
	} else {
		var lerr error
		if sym, fqn, lerr = s.lookupSymbol(name); lerr != nil {
			return nil, lerr
		}
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, err
	}
	scope := s.contextScope(sym)
	if scope == nil {
		return nil, fmt.Errorf("%s names no namespace to evaluate in", fqn)
	}
	if inst != nil {
		val, err := ctx.EvalWithScopeOn(node, scope, inst)
		if err != nil {
			return nil, evalError(expr, err, len(exprPrefix))
		}
		return []string{
			fmt.Sprintf("✓ %s%s", expr, onInstance(inst, label)),
			fmt.Sprintf("  = %s", formatValue(ctx, val)),
		}, nil
	}
	val, err := ctx.EvalWithScope(node, scope)
	if err != nil {
		// A feature the declarations give no value to reads as unset, as it does
		// on an object; an operation over one fails, and a name nothing declares
		// is an error.
		var noValue *runtime.NoValueError
		if !errors.As(err, &noValue) || !readsFeature(node, noValue.Ref) {
			return nil, evalError(expr, err, len(exprPrefix))
		}
		return []string{
			fmt.Sprintf("✓ %s (in %s)", expr, declarationNotation(sym)),
			fmt.Sprintf("  = %s", runtime.UnsetText),
		}, nil
	}
	return []string{
		fmt.Sprintf("✓ %s (in %s)", expr, declarationNotation(sym)),
		fmt.Sprintf("  = %s", formatValue(ctx, val)),
	}, nil
}

// contextScope is the namespace a pinned context evaluates in: the element's own
// scope, so its members are named without qualification, else the scope it was
// declared in, searched through both session documents.
func (s *Session) contextScope(sym *symbols.Symbol) *symbols.Scope {
	if sym == nil {
		return nil
	}
	if sym.Scope != nil {
		return sym.Scope
	}
	for _, doc := range s.sessionDocs() {
		if scope := declaringScope(sym, doc.Scope); scope != nil {
			return scope
		}
	}
	return nil
}

// pathArgs is a command's path arguments with any notation quoting removed, so
// a path holding a space can be written as a quoted name too.
func pathArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		out = append(out, nameText(arg))
	}
	return out
}

// nameText is the text a quoted argument names, for a command matching a name
// rather than resolving one: the quotes are notation, not part of the name.
func nameText(arg string) string {
	if plain, ok := plainName(arg); ok {
		return plain
	}
	return arg
}

// doEval evaluates an expression.
func (s *Session) doEval(expr string) ([]string, bool, error) {
	lines, err := s.evalExpr(expr)
	if err != nil {
		s.noteIfMaterializationFailure(err)
		return []string{errPrefix + err.Error()}, false, nil
	}
	return lines, false, nil
}

// evalExpr evaluates an expression, reporting a failure as an error rather than
// as a line of output so a caller outside the prompt can act on it.
func (s *Session) evalExpr(expr string) ([]string, error) {
	// Try literal evaluation first (works even with empty session)
	literalResult, isLiteral, litErr := s.tryEvalLiteral(expr)
	if isLiteral {
		return literalResult, litErr
	}

	doc := s.ws.Document(docName)

	// The library is indexed with or without session declarations, so a name it
	// declares is answered from it; only compound expressions, handled below,
	// need the session's own document.
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		if doc == nil || doc.Scope == nil {
			return nil, s.errWithoutDeclarations(expr)
		}
		return nil, err
	}

	// Try feature reference lookup, simple ("%eval x") or qualified
	// ("%eval Demo::Vehicle::mass"). A name the session did not declare may
	// still be reachable through an import, which the expression path below
	// resolves, so a lookup failure is only reported if that path fails too.
	var (
		sym       *symbols.Symbol
		fqn       string
		lookupErr error
	)
	if isSymbolReference(expr) {
		sym, fqn, lookupErr = s.lookupSymbol(expr)
		// A name several declarations answer to is reported, never resolved to
		// whichever of them the prompt's scope happens to reach.
		var ambiguous *AmbiguousNameError
		if errors.As(lookupErr, &ambiguous) {
			return nil, lookupErr
		}
	}
	if sym != nil {
		// An object carrying the feature makes this a question about that object,
		// as a check of a condition is: read the feature value, which holds the value the
		// object actually has, rather than the value the declaration defaults to.
		inst, owner, subjErr := s.subjectFor(expr, fqn, sym)
		if subjErr != nil {
			return nil, subjErr
		}
		if inst != nil {
			if _, ok := inst.FeatureValues[sym.Name]; ok {
				fv, err := inst.GetFeatureValue(ctx, sym.Name)
				if err != nil {
					return nil, fmt.Errorf("evaluation failed: %w", err)
				}
				return []string{
					fmt.Sprintf("✓ %s%s", expr, onInstance(inst, owner)),
					fmt.Sprintf("  = %s", formatFeatureValue(ctx, fv)),
				}, nil
			}
		}
		// An enumeration literal is a value even though it declares none.
		if val, isLiteral, err := ctx.EnumerationLiteralValue(sym); isLiteral {
			if err != nil {
				return nil, fmt.Errorf("evaluation failed: %w", err)
			}
			return []string{
				fmt.Sprintf("✓ %s", expr),
				fmt.Sprintf("  = %s", formatValue(ctx, val)),
			}, nil
		}
		// A measurement unit is the reference it declares, whatever defines it.
		if val, isUnit, err := ctx.MeasurementUnitValue(sym); isUnit {
			if err != nil {
				return nil, fmt.Errorf("evaluation failed: %w", err)
			}
			return []string{
				fmt.Sprintf("✓ %s", expr),
				fmt.Sprintf("  = %s", formatValue(ctx, val)),
			}, nil
		}
		// A calc definition is the function it declares.
		if val, isFunction, err := ctx.FunctionValue(sym); isFunction {
			if err != nil {
				return nil, fmt.Errorf("evaluation failed: %w", err)
			}
			return []string{
				fmt.Sprintf("✓ %s", expr),
				fmt.Sprintf("  = %s", formatValue(ctx, val)),
			}, nil
		}
		// A valueless usage may still name a value its own features shape.
		if _, isUsage := sym.Decl.(*ast.Usage); !isUsage && !declaresValue(sym) {
			return nil, fmt.Errorf("%q has no value to evaluate", expr)
		}
		// Read as the declaration is read: in its own scope, against its declared type.
		val, err := ctx.EvalDeclaredValue(sym)
		if err != nil {
			if !declaresValue(sym) && errors.Is(err, runtime.ErrNoValue) {
				return nil, fmt.Errorf("%q has no value to evaluate", expr)
			}
			return nil, fmt.Errorf("evaluation failed: %w", err)
		}
		return []string{
			fmt.Sprintf("✓ %s", expr),
			fmt.Sprintf("  = %s", formatValue(ctx, val)),
		}, nil
	}

	// A compound expression is evaluated in the session's own namespace; an empty
	// session has none, so only the library answers there.
	if doc == nil || doc.Scope == nil {
		return s.evalWithoutDeclarations(ctx, expr)
	}

	// Complex expression with feature refs - inject into session context
	tempSrc := s.joined() + fmt.Sprintf("\nattribute __eval__ = %s;", expr)
	p := parser.New(source.New("eval", []byte(tempSrc)))
	root := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		if lookupErr != nil {
			return nil, lookupErr
		}
		// These diagnostics describe the session fragment the expression was
		// appended to, so the expression is parsed on its own to report the
		// first error that is about what was typed.
		if _, diags := parseExprAlone(expr); len(diags) > 0 {
			return nil, exprError(expr, diags[0].Message, diags[0].Span, len(exprPrefix))
		}
		return nil, fmt.Errorf("parse failed: %s", p.Diagnostics[0].Message)
	}

	// Find __eval__ attribute (should be last member)
	var evalUsage *ast.Usage
	for i := len(root.Members) - 1; i >= 0; i-- {
		member := root.Members[i]
		if membership, ok := member.(*ast.Membership); ok {
			member = membership.Member
		}
		if usage, ok := member.(*ast.Usage); ok && usage.Value != nil {
			if usage.Ident.Name == "__eval__" || usage.Ident.ShortName == "__eval__" {
				evalUsage = usage
				break
			}
		}
	}

	if evalUsage == nil || evalUsage.Value == nil {
		return nil, errors.New("could not parse expression")
	}

	// Evaluated in the namespace the session is working in, so a compound
	// expression names what a member written there would: the session's
	// features and the units its imports bring in.
	val, err := ctx.EvalWithScope(evalUsage.Value, s.promptScope())
	if err != nil {
		// A name the lookup missed but the expression reached resolved after
		// all; finding no value there is not the unresolved name the lookup saw.
		var noValue *runtime.NoValueError
		if lookupErr != nil {
			if errors.As(err, &noValue) && readsFeature(evalUsage.Value, noValue.Ref) {
				return nil, fmt.Errorf("%q has no value to evaluate", expr)
			}
			return nil, lookupErr
		}
		return nil, evalError(expr, err, len(tempSrc)-len(expr)-1)
	}

	return []string{
		fmt.Sprintf("✓ %s", expr),
		fmt.Sprintf("  = %s", formatValue(ctx, val)),
	}, nil
}

// declaresValue reports whether sym's declaration binds a value of its own: a
// usage's `= expr`, or the value a require/assume constraint binds.
func declaresValue(sym *symbols.Symbol) bool {
	if oc, ok := ast.OwnedConstraintOf(sym.Decl); ok {
		return oc.Value != nil
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.Value != nil
}

// readsFeature reports whether an expression is a bare read (a name or a chain
// of names) with ref as one of its own links, not a name read inside a default.
func readsFeature(node ast.Node, ref *ast.QualifiedName) bool {
	if ref == nil {
		return false
	}
	switch n := node.(type) {
	case *ast.FeatureReference:
		return n.Name == ref
	case *ast.FeatureChainExpr:
		return n.Member == ref || readsFeature(n.Operand, ref)
	default:
		return false
	}
}

// exprPrefix wraps an expression as a declaration of its own, so parsing it
// alone reports positions the typed expression explains.
const exprPrefix = "attribute __lit__ = "

// parseExprAlone parses expr as the value of a declaration of its own, so its
// diagnostics are about the expression rather than about a session fragment it
// was appended to. Spans are offsets into exprPrefix + expr.
func parseExprAlone(expr string) (ast.Node, []parser.Diagnostic) {
	p := parser.New(source.New("literal", []byte(exprPrefix+expr+";")))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		return nil, p.Diagnostics
	}
	if len(root.Members) == 0 {
		return nil, nil
	}
	member := root.Members[0]
	if mem, ok := member.(*ast.Membership); ok {
		member = mem.Member
	}
	usage, ok := member.(*ast.Usage)
	if !ok {
		return nil, nil
	}
	return usage.Value, nil
}

// errWithoutDeclarations reports why an expression an empty session could not
// answer failed. Only a failure declarations would answer — a name nothing
// declares — is met with the no-declarations message, carrying the resolver's
// hint for the name; a syntax error or a real evaluation failure, such as a
// division by zero, is the answer itself.
func (s *Session) errWithoutDeclarations(expr string) error {
	value, diags := parseExprAlone(expr)
	if len(diags) > 0 {
		return exprError(expr, diags[0].Message, diags[0].Span, len(exprPrefix))
	}
	if value != nil {
		if ctx, err := emptyRuntime(s.budgets); err == nil {
			_, evalErr := ctx.Eval(value)
			if evalErr != nil && !declarationsWouldAnswer(evalErr) {
				return evalError(expr, evalErr, len(exprPrefix))
			}
			if errors.Is(evalErr, runtime.ErrUnresolvedReference) {
				return fmt.Errorf("%s: %w", noDeclarationsLoaded, evalErr)
			}
		}
	}
	return errors.New(noDeclarationsLoaded)
}

// evalWithoutDeclarations answers an expression over the library alone in the
// session's runtime, so an object it reaches is one %features shows.
func (s *Session) evalWithoutDeclarations(ctx *runtime.Context, expr string) ([]string, error) {
	value, diags := parseExprAlone(expr)
	if len(diags) > 0 {
		return nil, exprError(expr, diags[0].Message, diags[0].Span, len(exprPrefix))
	}
	if value == nil {
		return nil, errors.New(noDeclarationsLoaded)
	}
	val, err := ctx.Eval(value)
	switch {
	case err == nil:
		return []string{
			fmt.Sprintf("✓ %s", expr),
			fmt.Sprintf("  = %s", formatValue(ctx, val)),
		}, nil
	case !declarationsWouldAnswer(err):
		return nil, evalError(expr, err, len(exprPrefix))
	case errors.Is(err, runtime.ErrUnresolvedReference):
		return nil, fmt.Errorf("%s: %w", noDeclarationsLoaded, err)
	default:
		return nil, errors.New(noDeclarationsLoaded)
	}
}

// noDeclarationsLoaded is why an empty session cannot answer a name.
const noDeclarationsLoaded = "no declarations loaded (literals work, but feature references need declarations)"

// declarationsWouldAnswer reports whether err is a failure declarations would
// answer — a name, a unit or a value nothing declares — rather than the answer
// of the expression itself.
func declarationsWouldAnswer(err error) bool {
	return errors.Is(err, runtime.ErrUnresolvedReference) ||
		errors.Is(err, semantics.ErrNotAUnit) ||
		errors.Is(err, runtime.ErrNoValue)
}

// evalError reports an evaluation failure, giving one that carries the span of
// the expression it failed in the caret treatment declarations get. base is the
// offset expr starts at in the text the span was measured in.
func evalError(expr string, err error, base int) error {
	var operand *runtime.OperandTypeError
	// A span is a position in the typed expression only when the operator was
	// written there; one in a declaration the expression reached keeps the
	// wrapped message, which names the calc or feature it is in.
	if errors.As(err, &operand) && mismatchInExpr(expr, operand, base) {
		return exprError(expr, operand.Error(), operand.Span, base)
	}
	return fmt.Errorf("evaluation failed: %w", err)
}

// mismatchInExpr reports whether the mismatch was written in expr: its span has
// to land inside expr and the text there has to be the operator it names, since
// a span alone is an offset a declaration in another file could also occupy.
func mismatchInExpr(expr string, operand *runtime.OperandTypeError, base int) bool {
	start, end := operand.Span.Offset-base, operand.Span.End()-base
	if start < 0 || end > len(expr) || start >= end {
		return false
	}
	return strings.Contains(expr[start:end], operand.Op)
}

// emptyRuntime is a context over an empty model, which answers an expression of
// literals alone and nothing a session declares.
func emptyRuntime(budgets runtime.Budgets) (*runtime.Context, error) {
	emptyIdx := libs.NewModelIndex()
	emptyModel := semantics.NewModel(resolve.New(emptyIdx))
	ctx := runtime.NewContext(emptyModel, resolve.New(emptyIdx), budgets.MaxSteps)
	if err := ctx.SetBudgets(budgets); err != nil {
		return nil, err
	}
	return ctx, nil
}

// tryEvalLiteral attempts to evaluate standalone literal expressions. It reports
// whether the expression is one it answered, and what an answered one failed on.
func (s *Session) tryEvalLiteral(expr string) ([]string, bool, error) {
	// A name the session declares is answered by that declaration, so the empty
	// model this pass evaluates in must not answer for it: a library operation
	// reached by its unqualified name would otherwise stand in for a calc the
	// session wrote under the same name.
	if s.declaresANameIn(expr) {
		return nil, false, nil
	}

	// Parse as standalone attribute
	value, diags := parseExprAlone(expr)
	if len(diags) > 0 || value == nil {
		return nil, false, nil
	}

	// Use runtime context with empty model (no symbols needed for literals)
	ctx, err := emptyRuntime(s.budgets)
	if err != nil {
		return nil, false, nil
	}

	val, err := ctx.Eval(value)
	if err != nil {
		// A failure the session's declarations could answer — a name or a unit
		// this empty model knows nothing of — is not the answer, so the
		// expression is evaluated again in the session. A failure that is the
		// answer to an expression of literals alone, such as an index that names
		// no position, is reported here instead of being hidden behind "no
		// declarations loaded".
		if isLiteralAnswerError(err) {
			return nil, true, evalError(expr, err, len(exprPrefix))
		}
		return nil, false, nil
	}
	// An object is one of the session's, where %features can reach it, not of
	// this throwaway model: the session materializes the declaration itself.
	if ctx.HoldsObject(val) {
		return nil, false, nil
	}

	return []string{
		fmt.Sprintf("✓ %s", expr),
		fmt.Sprintf("  = %s", formatValue(ctx, val)),
	}, true, nil
}

// doBudget lists the bounds one run of this session may spend, each with the
// variable that raises it.
func (s *Session) doBudget() []string {
	b := s.budgets
	return []string{
		"budgets (each bounds one run, not the session):",
		fmt.Sprintf("  evaluation steps     %-10d %s", b.MaxSteps, runtime.MaxStepsEnvVar),
		fmt.Sprintf("  action steps         %-10d %s", b.MaxActionSteps, runtime.MaxActionStepsEnvVar),
		fmt.Sprintf("  state events         %-10d %s", b.MaxStateEvents, runtime.MaxStateEventsEnvVar),
		fmt.Sprintf("  do action steps      %-10d %s", b.MaxDoSteps, runtime.MaxDoStepsEnvVar),
		fmt.Sprintf("  collection elements  %-10d %s", b.MaxElements, runtime.MaxElementsEnvVar),
		fmt.Sprintf("  nested calc depth    %-10d %s", b.MaxCalcDepth, runtime.MaxCalcDepthEnvVar),
		fmt.Sprintf("  sweep runs           %-10d %s", b.MaxSweepRuns, runtime.MaxSweepRunsEnvVar),
	}
}

// declaresANameIn reports whether the session declares any name the expression
// uses, in the document root or in the namespace a prompt expression is
// evaluated in.
func (s *Session) declaresANameIn(expr string) bool {
	var scopes []*symbols.Scope
	for _, doc := range s.sessionDocs() {
		if doc.Scope != nil {
			scopes = append(scopes, doc.Scope)
		}
	}
	if len(scopes) == 0 {
		return false
	}
	if prompt := s.promptScope(); prompt != nil && prompt != scopes[0] {
		scopes = append(scopes, prompt)
	}
	src := source.New("literal", []byte(expr))
	lx := lexer.New(src)
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Kind != lexer.Identifier && tok.Kind != lexer.UnrestrictedName {
			continue
		}
		for _, scope := range scopes {
			if sym, ok := scope.LookupLocal(src.Text(tok.Span)); ok && sym != nil {
				return true
			}
		}
	}
	return false
}

// isLiteralAnswerError reports whether err is what an expression of literals
// alone evaluates to, rather than a failure the declarations of a session could
// answer. An index outside a written sequence, a body called with arguments it
// declares no parameters for, or an operand of the wrong kind is the answer
// whatever is declared; an unresolved name or unit is not.
func isLiteralAnswerError(err error) bool {
	for _, answer := range []error{
		runtime.ErrIndexOutOfRange,
		runtime.ErrBodyArity,
		runtime.ErrTypeMismatch,
		runtime.ErrDivisionByZero,
		runtime.ErrMultiplicityViolation,
		runtime.ErrStepLimitExceeded,
		runtime.ErrElementLimitExceeded,
	} {
		if errors.Is(err, answer) {
			return true
		}
	}
	return false
}

// isSymbolReference reports whether expr names a symbol — a single identifier,
// a quoted name or a qualified name like Demo::Vehicle::mass — rather than a
// compound expression. The notation itself decides: a name is what the parser
// reads the whole of expr as.
func isSymbolReference(expr string) bool {
	_, ok := plainName(expr)
	return ok
}

// doFeatures shows what an object holds for each feature of its type.
func (s *Session) doFeatures(name string, listing featureListing) ([]string, bool, error) {
	inst, label, rerr := s.resolveObject(name)
	if rerr != nil {
		out := []string{errPrefix + rerr.Error()}
		// The declaration may be gone with the objects materialized from it, which
		// neither an unresolved reference nor a missing object explains.
		if note := s.lost.lostNote(); note != "" {
			out = append(out, note)
		}
		return out, false, nil
	}

	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, false, fmt.Errorf("runtime init: %w", err)
	}

	if listing.json {
		return s.featuresJSON(ctx, inst, name, listing)
	}

	lines := []string{
		fmt.Sprintf("Instance: %s (ID: %d)", label, inst.ID),
		"Features:",
	}
	w := &featureValueWalk{
		ctx:      ctx,
		onPath:   map[*symbols.Symbol]bool{inst.Type: true},
		listing:  map[int64]bool{inst.ID: true},
		maxDepth: listing.depth,
		budget:   listing.budget,
		hint:     listing.truncationHint(name),
	}
	out := w.lines(inst, "  ", 0)
	// A feature value the listing rendered as an error is one the session could not answer
	// about, which a non-interactive run exits on.
	s.noteMaterializationFailure(w.errs...)
	return append(lines, out...), false, nil
}

const (
	// maxFeatureValueDepth bounds how deep %features expands nested objects.
	maxFeatureValueDepth = 8
	// maxFeatureValueLines bounds the listing as a whole, since nesting multiplies and
	// a feature value is materialized by reading it: breadth costs objects, not just output.
	maxFeatureValueLines = 200
)

// featureValueWalk expands an object graph for %features under four bounds: onPath holds
// the types being expanded above the current one (a part containing its own
// kind materializes a fresh instance per descent, so instance identity cannot
// detect the cycle), listing the objects, maxDepth, and a line budget shared across the listing.
type featureValueWalk struct {
	ctx    *runtime.Context
	onPath map[*symbols.Symbol]bool
	// listing holds the objects being expanded above the current one: one that
	// holds itself (`mRefs = self`) is named on its row, not expanded again.
	listing  map[int64]bool
	maxDepth int
	budget   int
	// hint is the truncation line's advice on how to see the rest; cut records
	// that the line was written, so unwinding the walk does not repeat it.
	hint string
	cut  bool
	// errs are the feature values the listing could not materialize, rendered as errors in
	// its lines and reported as findings about the model by the caller.
	errs []error
}

func (w *featureValueWalk) lines(inst *runtime.Instance, indent string, depth int) []string {
	return w.rows(inst, indent, depth, true)
}

// rows lists an object's values and, when asked, its behaviors; a behavior's
// occurrence is listed without its own, since its row already reports what runs.
func (w *featureValueWalk) rows(inst *runtime.Instance, indent string, depth int, withBehaviors bool) []string {
	// A destroyed object's features hold nothing to list; its lifetime says so once.
	if life, ok := w.ctx.OccurrenceLife(inst.ID); ok && life.Destroyed {
		return w.emit(nil, fmt.Sprintf("%s(%s)", indent, life))
	}
	features := w.ctx.FeaturesOfObject(inst)
	connectors := w.connectors(inst, indent)
	if len(features) == 0 && len(connectors) == 0 && withBehaviors {
		return w.emit(nil, indent+"(no features)")
	}

	// Connector lines already spent their share of the budget, so a truncated
	// listing still shows them rather than dropping what it charged for.
	truncated := func(lines []string, pad string) []string {
		return w.truncate(append(lines, connectors...), indent+pad)
	}

	var lines []string
	var behaviors []runtime.ObjectFeature
	for i, of := range features {
		if w.budget <= 0 {
			return truncated(lines, "")
		}
		feat := of.Feature
		// A state or action holds no value either; what it has is a run, or none,
		// which is listed under its own heading after the values.
		if isBehaviorFeature(feat) {
			if withBehaviors {
				behaviors = append(behaviors, of)
			}
			continue
		}
		// A constraint or requirement the part carries has no value; what it has
		// is a verdict about this instance, which is the useful thing to show.
		if verdict, ok := featureVerdict(w.ctx, feat, inst); ok {
			lines = w.emit(lines, fmt.Sprintf("%s%s: %s", indent, of.Name, verdict))
			continue
		}
		if held, elided := w.elided(feat, depth); elided {
			lines = w.emit(lines, fmt.Sprintf("%s%s : %s (not expanded: %s)", indent, of.Name, held, w.elisionReason(depth)))
			continue
		}
		fv, err := inst.GetFeatureValue(w.ctx, of.Name)
		if err != nil {
			w.errs = append(w.errs, err)
			lines = w.emit(lines, fmt.Sprintf("%s%s: <error: %v>", indent, of.Name, err))
			continue
		}
		lines = w.emit(lines, fmt.Sprintf("%s%s = %s", indent, of.Name, formatFeatureValue(w.ctx, fv)))
		// The object's remaining features keep a line each; a nested expansion
		// spends only what is left beyond them.
		reserved := len(features) - i - 1
		for _, nested := range nestedInstances(w.ctx, fv) {
			if w.listing[nested.ID] {
				continue
			}
			if w.budget <= reserved {
				lines = w.truncate(lines, indent+"  ")
				break
			}
			w.budget -= reserved
			w.onPath[nested.Type] = true
			w.listing[nested.ID] = true
			lines = append(lines, w.lines(nested, indent+"  ", depth+1)...)
			delete(w.listing, nested.ID)
			delete(w.onPath, nested.Type)
			w.budget += reserved
		}
	}
	if w.budget <= 0 && len(behaviors) > 0 {
		return truncated(lines, "")
	}
	return append(append(lines, w.behaviorLines(inst, behaviors, indent, depth)...), connectors...)
}

// isBehaviorFeature reports whether a feature is a state or action usage — a
// named transition among them — a behavior of the object rather than a value it holds.
func isBehaviorFeature(feat *runtime.EffectiveFeature) bool {
	// An abstract one (Part::performedActions) is a collection held, not a run.
	if feat.Symbol == nil || symbols.IsAbstract(feat.Symbol) {
		return false
	}
	switch feat.Symbol.Kind {
	case symbols.SymbolStateUsage, symbols.SymbolActionUsage:
		return true
	default:
		return false
	}
}

// behaviorLines lists an object's behaviors under their own heading with what each
// is doing, and the values a running one's occurrence holds beneath its row.
func (w *featureValueWalk) behaviorLines(inst *runtime.Instance, behaviors []runtime.ObjectFeature, indent string, depth int) []string {
	if len(behaviors) == 0 {
		return nil
	}
	heading, rowIndent := indent+"Behaviors:", indent+"  "
	if depth == 0 {
		heading, rowIndent = "Behaviors:", indent
	}
	lines := w.emit(nil, heading)
	for _, of := range behaviors {
		if w.budget <= 0 {
			return w.truncate(lines, rowIndent)
		}
		row := fmt.Sprintf("%s%s: %s", rowIndent, of.Name, behaviorStatus(w.ctx, inst, of.Feature))
		if _, runs := w.ctx.BehaviorNamed(inst, of.Name); !runs {
			lines = w.emit(lines, row)
			continue
		}
		if _, elided := w.elided(of.Feature, depth); elided {
			lines = w.emit(lines, fmt.Sprintf("%s (not expanded: %s)", row, w.elisionReason(depth)))
			continue
		}
		lines = w.emit(lines, row)
		fv, err := inst.GetFeatureValue(w.ctx, of.Name)
		if err != nil {
			w.errs = append(w.errs, err)
			lines = w.emit(lines, fmt.Sprintf("%s  <error: %v>", rowIndent, err))
			continue
		}
		for _, occurrence := range nestedInstances(w.ctx, fv) {
			if w.budget <= 0 {
				return w.truncate(lines, rowIndent+"  ")
			}
			w.onPath[occurrence.Type] = true
			lines = append(lines, w.rows(occurrence, rowIndent+"  ", depth+1, false)...)
			delete(w.onPath, occurrence.Type)
		}
	}
	return lines
}

// behaviorStatus describes what an object is doing with a behavior its type
// declares. An exhibited machine's active state is rendered as %current renders
// it, so the two agree; a transition is the step between states it declares. A
// behavior a redefinition renamed reports the one execution under both names.
func behaviorStatus(ctx *runtime.Context, inst *runtime.Instance, feat *runtime.EffectiveFeature) string {
	if trans, ok := feat.Symbol.Decl.(*ast.TransitionMember); ok {
		return transitionStatus(trans)
	}
	kind := "state"
	if feat.Symbol.Kind == symbols.SymbolActionUsage {
		kind = "action"
	}
	behavior, ok := ctx.BehaviorNamed(inst, feat.Name)
	if !ok {
		return kind + ", not running"
	}
	switch {
	case behavior.State != nil:
		return fmt.Sprintf("%s, %s", behavior.Kind, machineStatus(behavior.State))
	case behavior.Action != nil:
		return fmt.Sprintf("%s, %s", behavior.Kind, executionStatus(behavior.Action.State()))
	default:
		return kind + ", not running"
	}
}

// transitionStatus renders a transition as the step it declares, `first source
// then target`; a target transition written inside its source state has no
// source of its own to name.
func transitionStatus(trans *ast.TransitionMember) string {
	if trans.Source == nil {
		return "transition, then " + stateNameAsWritten(trans.Target)
	}
	return fmt.Sprintf("transition, %s → %s", stateNameAsWritten(trans.Source), stateNameAsWritten(trans.Target))
}

// stateNameAsWritten renders a transition end as the model spells it: `modes.closed`
// reaches through the exhibited machine, `Modes::closed` into its namespace, and a
// name that is not a basic one keeps its quotes, so the row can be typed back.
func stateNameAsWritten(qn *ast.QualifiedName) string {
	if qn == nil {
		return "<?>"
	}
	var sb strings.Builder
	if qn.Global {
		sb.WriteString("$::")
	}
	for i, p := range qn.Parts {
		switch {
		case i == 0:
		case p.Chained:
			sb.WriteString(".")
		default:
			sb.WriteString("::")
		}
		sb.WriteString(lexer.NameText(p.Text))
	}
	return sb.String()
}

// machineStatus is where a state machine run stands: not started, completed, or
// the configuration it is in.
func machineStatus(exec *runtime.StateExecutor) string {
	switch exec.State() {
	case runtime.StateReady:
		return "not started"
	case runtime.StateCompleted:
		return "completed"
	default:
		return "current state " + currentStateName(exec)
	}
}

// executionStatus spells an execution state as a listing reads it.
func executionStatus(state runtime.ExecutionState) string {
	if state == runtime.StateReady {
		return "not started"
	}
	return strings.ToLower(state.String())
}

// connectors lists the connectors the object owns that no feature names: an
// anonymous `connect a.p to b.q` relates the features at its ends whether or not
// it is named, and its ends are the only way to see what it relates.
func (w *featureValueWalk) connectors(inst *runtime.Instance, indent string) []string {
	conns, err := inst.OwnedConnectors(w.ctx)
	if err != nil {
		w.errs = append(w.errs, err)
		return w.emit(nil, fmt.Sprintf("%s(anonymous connector): <error: %v>", indent, err))
	}
	var lines []string
	for _, conn := range conns {
		lines = w.emit(lines, fmt.Sprintf("%s(anonymous %s) = %s", indent, connectorKeyword(conn),
			formatValue(w.ctx, runtime.Value{Kind: runtime.ValInstance, Instance: conn.ID})))
		for _, end := range conn.Ends {
			if w.budget <= 0 {
				return w.truncate(lines, indent+"  ")
			}
			lines = w.emit(lines, fmt.Sprintf("%s  %s = %s", indent, endLabel(end), formatValue(w.ctx, end.Value)))
		}
	}
	return lines
}

// connectorKeyword names the kind of connector an object materializes, for a
// connector that has no name to show.
func connectorKeyword(conn *runtime.Instance) string {
	// `connect a.p to b.q` states its ends after the keyword rather than naming a
	// kind, so it is labelled by the kind it declares.
	if usage, ok := conn.Type.Decl.(*ast.Usage); ok && usage.Keyword != "" && usage.Keyword != "connect" {
		return usage.Keyword
	}
	return "connector"
}

// endLabel names one end of a connector, by position when the model gives the
// end no name of its own.
func endLabel(end runtime.ConnectorEnd) string {
	if end.Name != "" {
		return end.Name
	}
	return "end"
}

func (w *featureValueWalk) emit(lines []string, line string) []string {
	w.budget--
	return append(lines, line)
}

// truncate ends the listing, saying so once and how to see the rest.
func (w *featureValueWalk) truncate(lines []string, indent string) []string {
	if w.cut {
		return lines
	}
	w.cut = true
	return append(lines, indent+"… (listing truncated; "+w.hint+")")
}

// elided reports whether expanding a feature would revisit a type already on
// the path or exceed the depth bound, naming the type it holds. Asked before
// the feature value is read, since reading it materializes the object.
func (w *featureValueWalk) elided(feat *runtime.EffectiveFeature, depth int) (string, bool) {
	held := w.ctx.CompositeTypeOf(feat)
	if held == nil {
		// A variation is materialized from the variant it selects, so the depth
		// bound applies to it too.
		if w.ctx.IsVariationFeature(feat) && depth >= w.maxDepth {
			return feat.Name, true
		}
		return "", false
	}
	if depth >= w.maxDepth || w.onPath[held] {
		return held.Name, true
	}
	return "", false
}

func (w *featureValueWalk) elisionReason(depth int) string {
	if depth >= w.maxDepth {
		return fmt.Sprintf("depth %d", w.maxDepth)
	}
	return "contains its own kind"
}

// nestedInstances returns the instances a feature value holds, whether it carries one
// value or a collection of them. An object that holds no value has nothing to
// expand: it reads as unset, not as an object with no features.
func nestedInstances(ctx *runtime.Context, fv *runtime.FeatureValue) []*runtime.Instance {
	values := []runtime.Value{fv.Value}
	switch fv.Values.Kind {
	case runtime.ValSequence:
		values = fv.Values.Sequence().Elements()
	case runtime.ValSet:
		values = fv.Values.Set().Elements()
	}

	var out []*runtime.Instance
	for _, val := range values {
		id, ok := val.Object()
		if !ok || ctx.HoldsNoValue(val) {
			continue
		}
		if nested, ok := ctx.Instance(id); ok {
			out = append(out, nested)
		}
	}
	return out
}

// featureVerdict evaluates a constraint or requirement feature against the
// instance that carries it and renders the outcome for a feature value listing.
// Reports false for a feature that holds a value rather than a verdict, which a
// multi-valued one does: it collects checks rather than being one.
func featureVerdict(ctx *runtime.Context, feat *runtime.EffectiveFeature, inst *runtime.Instance) (string, bool) {
	if feat.Symbol == nil || !feat.Scalar() {
		return "", false
	}
	var (
		kind   string
		passed bool
		err    error
	)
	switch feat.Symbol.Kind {
	case symbols.SymbolConstraintUsage:
		kind = "constraint"
		passed, err = ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), inst)
	case symbols.SymbolRequirementUsage:
		kind = "requirement"
		passed, err = ctx.EvaluateRequirementOn(feat.Symbol, feat.DeclScope(), inst)
	default:
		return "", false
	}
	switch {
	case unevaluable(err):
		return fmt.Sprintf("<%s: not evaluated: %v>", kind, err), true
	case err != nil || !passed:
		return fmt.Sprintf("<%s: violated>", kind), true
	default:
		return fmt.Sprintf("<%s: satisfied>", kind), true
	}
}

// doInstances lists the objects the session holds, named and displaced alike.
func (s *Session) doInstances() ([]string, bool, error) {
	if s.heldObjects() == 0 {
		if note := s.lost.goneNote(); note != "" {
			return []string{note}, false, nil
		}
		return []string{"(no instances created)"}, false, nil
	}

	names := make([]string, 0, len(s.instances))
	for name := range s.instances {
		names = append(names, name)
	}
	slices.Sort(names)
	lines := []string{"Instances:"}
	for _, name := range names {
		lines = append(lines, fmt.Sprintf("  %s (ID: %d%s)", s.declaredName(name), s.instances[name].ID, s.destroyedNote(s.instances[name])))
	}
	// An object a later %instantiate unnamed is still held, and is listed as
	// the id that reaches it.
	for _, u := range s.unnamed {
		if s.rtCtx == nil {
			break
		}
		if held, ok := s.rtCtx.Instance(u.obj.ID); !ok || held != u.obj {
			continue
		}
		lines = append(lines, fmt.Sprintf("  #%d (ID: %d, displaced from %s%s)", u.obj.ID, u.obj.ID, s.declaredName(u.fqn), s.destroyedNote(u.obj)))
	}
	// Some of what the session materialized may be gone even though the rest
	// survived, which the list would otherwise not say.
	if note := s.lost.partlyGoneNote(); note != "" {
		lines = append(lines, note)
	}
	return lines, false, nil
}

// destroyedNote is the note an instance listing adds to an object `destroy` ended.
func (s *Session) destroyedNote(inst *runtime.Instance) string {
	if s.rtCtx == nil {
		return ""
	}
	if life, ok := s.rtCtx.OccurrenceLife(inst.ID); ok && life.Destroyed {
		return ", destroyed"
	}
	return ""
}

// formatFeatureValue renders what a feature value reads as: what it holds, the empty
// sequence when an optional feature holds nothing, and unset for a required one.
func formatFeatureValue(ctx *runtime.Context, fv *runtime.FeatureValue) string {
	val, err := fv.ReadValue(fv.Feature.Name)
	if err != nil {
		return runtime.UnsetText
	}
	return formatValue(ctx, val)
}

// formatValue renders a value as every surface spells it. ctx may be nil, which
// only costs the unset reading of an object that holds no value.
func formatValue(ctx *runtime.Context, val runtime.Value) string {
	if ctx != nil && ctx.HoldsNoValue(val) {
		return runtime.UnsetText
	}
	if val.Kind == runtime.ValInstance {
		return fmt.Sprintf("Instance(ID: %d)", val.Instance)
	}
	switch val.Kind {
	case runtime.ValSequence:
		if val.Sequence() == nil {
			return "[]"
		}
		parts := make([]string, len(val.Sequence().Elements()))
		for i, element := range val.Sequence().Elements() {
			parts[i] = formatValue(ctx, element)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case runtime.ValSet:
		if val.Set() == nil {
			return "Set{}"
		}
		parts := make([]string, len(val.Set().Elements()))
		for i, element := range val.Set().Elements() {
			parts[i] = formatValue(ctx, element)
		}
		return "Set{" + strings.Join(parts, ", ") + "}"
	case runtime.ValArray:
		return val.Array().Format(func(element runtime.Value) string { return formatValue(ctx, element) })
	}
	return runtime.FormatValue(val)
}

// doCalc invokes a calculation with the arguments the command line states.
// argText is the raw argument list: a sequence of expressions, which the
// notation separates with commas and this prompt also accepts separated by
// whitespace alone. Named arguments (`v0 = ...`) are not supported here — the
// notation writes them inside an invocation's parentheses, which is a different
// production than an argument list at a prompt.
//
// A calc usage named without arguments binds its inputs from its own members,
// so it is evaluated as a usage and every output feature it computes is listed
// from that one run (SysML 7.17).
func (s *Session) doCalc(calcName, argText string) ([]string, bool, error) {
	return errorLines(s.evalCalc(calcName, argText))
}

// evalCalc carries out %calc, reporting what stopped an evaluation as an error
// rather than as a line of output, so a caller outside the prompt — the command
// line — can tell an evaluated calculation from one that could not be run.
func (s *Session) evalCalc(calcName, argText string) ([]string, []NamedValue, error) {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return nil, nil, errors.New("no declarations loaded")
	}

	sym, _, lerr := s.lookupSymbolOfKinds(calcName, symbols.SymbolCalcDef, symbols.SymbolCalcUsage)
	if lerr != nil {
		return nil, nil, lerr
	}

	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, nil, err
	}

	if strings.TrimSpace(argText) == "" {
		if lines, values, handled, err := s.calcUsageOutputs(ctx, sym, calcName); handled {
			return lines, values, err
		}
	}

	exprs, err := s.argExprs(argText)
	if err != nil {
		return nil, nil, err
	}

	// Arguments are evaluated where the prompt evaluates any expression, so a
	// quantity argument names the units the session's imports bring in.
	scope := s.promptScope()
	argValues := make([]runtime.Value, len(exprs))
	argTexts := make([]string, len(exprs))
	for i, arg := range exprs {
		val, err := ctx.EvalWithScope(arg.expr, scope)
		if err != nil {
			return nil, nil, fmt.Errorf("evaluation of argument %q failed: %w", arg.text, err)
		}
		argValues[i] = val
		argTexts[i] = arg.text
	}

	result, err := ctx.InvokeCalc(sym, argValues, scope)
	if err != nil {
		// A name of another kind is a wrong argument, so it is reported as
		// itself rather than as a calculation that failed.
		if errors.Is(err, runtime.ErrNotACalc) {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("calc invocation failed: %w", err)
	}

	return []string{
		fmt.Sprintf("✓ %s(%s)", calcName, strings.Join(argTexts, ", ")),
		fmt.Sprintf("  = %s", formatValue(ctx, result)),
	}, []NamedValue{{Name: "result", Value: formatValue(ctx, result)}}, nil
}

// calcUsageOutputs lists the outputs of a calc usage evaluated from its own
// member values. It reports handled=false when the name is not a calc usage, or
// is one that computes no output features, so those keep being invoked as
// calculations with an empty argument list.
func (s *Session) calcUsageOutputs(ctx *runtime.Context, sym *symbols.Symbol, calcName string) ([]string, []NamedValue, bool, error) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageCalc {
		return nil, nil, false, nil
	}
	outputs, err := ctx.CalcUsageOutputs(sym, sym.OwnerScope, nil)
	if err != nil {
		return nil, nil, true, fmt.Errorf("calc usage evaluation failed: %w", err)
	}
	if len(outputs) == 0 {
		return nil, nil, false, nil
	}
	lines := make([]string, 0, len(outputs)+1)
	lines = append(lines, fmt.Sprintf("✓ %s", calcName))
	values := make([]NamedValue, 0, len(outputs))
	for _, out := range outputs {
		lines = append(lines, fmt.Sprintf("  %s = %s", out.Name, formatValue(ctx, out.Value)))
		values = append(values, NamedValue{Name: out.Name, Value: formatValue(ctx, out.Value)})
	}
	return lines, values, true, nil
}

// splitCalcArgs splits `%calc`'s tail into the calc's name and its argument
// list, accepting the invocation form `Fall(a, b)` as well as a bare name
// followed by arguments.
func splitCalcArgs(tail string) (name, argText string) {
	cut := indexOutsideName(tail, " \t(")
	if cut < 0 {
		return tail, ""
	}
	name, rest := tail[:cut], strings.TrimSpace(tail[cut:])
	if closesAtEnd(rest) {
		return name, rest[1 : len(rest)-1]
	}
	return name, rest
}

// indexOutsideName is the first index in text of one of chars that is not
// inside a quoted name, so a name's own space does not end it.
func indexOutsideName(text, chars string) int {
	inName, escaped := false, false
	for i, r := range text {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '\'':
			inName = !inName
		case !inName && strings.ContainsRune(chars, r):
			return i
		}
	}
	return -1
}

// closesAtEnd reports whether text is one parenthesized group: its opening
// parenthesis is closed by its last character, as an invocation's argument list
// is and a sequence of parenthesized arguments is not.
func closesAtEnd(text string) bool {
	if !strings.HasPrefix(text, "(") || !strings.HasSuffix(text, ")") {
		return false
	}
	depth := 0
	for i, r := range text {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i == len(text)-1
			}
		}
	}
	return false
}

// argExpr is one parsed argument: its expression and the text it was written as.
type argExpr struct {
	expr ast.Node
	text string
}

// parseExprList parses an argument list as expressions, so an argument that
// contains spaces — a quantity, a parenthesized subexpression, a nested
// invocation — survives as the one expression it is.
func parseExprList(text string) ([]argExpr, error) {
	var out []argExpr
	for _, arg := range splitArgs(text) {
		if isNamedArgument(arg) {
			return nil, fmt.Errorf("named arguments are not supported here; pass arguments positionally")
		}
		expr, err := parseWholeExpr(arg)
		if err != nil {
			return nil, err
		}
		out = append(out, argExpr{expr: expr, text: arg})
	}
	return out, nil
}

// splitArgs cuts an argument list into one text per argument. A comma separates
// arguments, and so does whitespace — except where the fragment after it
// continues the expression before it, as a quantity's unit does.
func splitArgs(text string) []string {
	var args []string
	for _, group := range splitTopLevel(text) {
		var buf string
		for _, frag := range group {
			switch {
			case buf == "":
				buf = frag
			case continuesExpr(buf, frag):
				buf += " " + frag
			default:
				args = append(args, buf)
				buf = frag
			}
		}
		if buf != "" {
			args = append(args, buf)
		}
	}
	return args
}

// continuesExpr reports whether frag continues the expression buf holds rather
// than starting the next argument: a unit or index bracket does, as does a
// fragment that is no expression on its own or that follows an unfinished one
// (`5 - 3` is one argument, `5 -3` is two).
func continuesExpr(buf, frag string) bool {
	if strings.HasPrefix(frag, "[") || strings.HasPrefix(frag, "#") {
		return true
	}
	if _, err := parseWholeExpr(frag); err != nil {
		return true
	}
	_, err := parseWholeExpr(buf)
	return err != nil
}

// splitTopLevel splits text at commas outside any bracket or string, each group
// as the whitespace-separated fragments it is written in.
func splitTopLevel(text string) [][]string {
	var groups [][]string
	var frags []string
	var frag strings.Builder
	depth, q := 0, quoteTracker{}

	flushFrag := func() {
		if frag.Len() > 0 {
			frags = append(frags, frag.String())
			frag.Reset()
		}
	}
	for _, r := range text {
		switch {
		case q.inside(r):
			// A string or quoted name is one fragment, space and comma included.
		case r == '(' || r == '[':
			depth++
		case r == ')' || r == ']':
			depth--
		case depth == 0 && r == ',':
			flushFrag()
			groups = append(groups, frags)
			frags = nil
			continue
		case depth == 0 && (r == ' ' || r == '\t'):
			flushFrag()
			continue
		}
		frag.WriteRune(r)
	}
	flushFrag()
	return append(groups, frags)
}

// isNamedArgument reports whether text binds a name (`v0 = 1.0`), which is a
// production of an invocation rather than of an argument list at the prompt. The
// binding is the argument's own: an `=` nested in a call, in a bracket or in a
// string belongs to that expression.
func isNamedArgument(text string) bool {
	depth, q := 0, quoteTracker{}
	for i, r := range text {
		switch {
		case q.inside(r):
		case r == '(' || r == '[':
			depth++
		case r == ')' || r == ']':
			depth--
		case depth == 0 && r == '=' && i > 0 && i+1 < len(text):
			if text[i+1] == '=' || strings.ContainsRune("=<>!+-*/", rune(text[i-1])) {
				continue
			}
			return isIdentifier(strings.TrimSpace(text[:i]))
		}
	}
	return false
}

// isIdentifier reports whether text is one bare name, the only thing a named
// argument's left side can be.
func isIdentifier(text string) bool {
	if text == "" {
		return false
	}
	for i, r := range text {
		if r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r)) {
			continue
		}
		return false
	}
	return true
}

// parseWholeExpr parses text as one complete expression, reporting text the
// expression parser does not consume in full.
func parseWholeExpr(text string) (ast.Node, error) {
	p := parser.New(source.New("arg", []byte(text)))
	expr := p.ParseExpression()
	if expr == nil {
		return nil, fmt.Errorf("failed to parse argument %q", text)
	}
	if _, bad := expr.(*ast.ErrorNode); bad || len(p.Diagnostics) > 0 || p.Offset() != len(text) {
		return nil, fmt.Errorf("failed to parse argument %q", text)
	}
	return expr, nil
}

// doConstraint evaluates a constraint definition.
func (s *Session) doConstraint(name string) ([]string, bool, error) {
	return s.withTrace(s.checkConstraint(name)).Lines, false, nil
}

// promptScope is the namespace a prompt expression is evaluated in: the last
// namespace the session declared, whose imports are then visible to it exactly
// as they are to a member written there (KerML 8.2.3.5.3). A session that
// declared no namespace evaluates at the document root. Both session documents
// are read, in buffer order, so a namespace loaded from a .kerml file counts.
func (s *Session) promptScope() *symbols.Scope {
	docs := s.sessionDocs()
	if len(docs) == 0 {
		return nil
	}
	type entry struct {
		member ast.Node
		scope  *symbols.Scope
	}
	var members []entry
	for _, doc := range docs {
		if doc.AST == nil || doc.Scope == nil {
			continue
		}
		for _, m := range doc.AST.Members {
			members = append(members, entry{m, doc.Scope})
		}
	}
	sort.SliceStable(members, func(i, j int) bool {
		return members[i].member.Span().Offset < members[j].member.Span().Offset
	})
	for i := len(members) - 1; i >= 0; i-- {
		member := members[i].member
		if mem, ok := member.(*ast.Membership); ok {
			member = mem.Member
		}
		var ident ast.Identification
		switch n := member.(type) {
		case *ast.Package:
			ident = n.Ident
		case *ast.Namespace:
			ident = n.Ident
		default:
			continue
		}
		name := ident.Name
		if name == "" {
			name = ident.ShortName
		}
		if sym, ok := members[i].scope.LookupLocal(name); ok && sym != nil && sym.Scope != nil {
			return sym.Scope
		}
	}
	// No namespace to work in: the root holding the last declaration, so a
	// top-level member loaded from a .kerml file is still in reach.
	if len(members) > 0 {
		return members[len(members)-1].scope
	}
	return docs[0].Scope
}

// declaringScope returns the scope an element's conditions were written in,
// which is what their names — a member of the enclosing package, a measurement
// unit an import brought in — resolve against. The document root reaches only
// what the root itself declares, so it is a fallback for a symbol carrying no
// declaring scope rather than the scope to evaluate in.
func declaringScope(sym *symbols.Symbol, root *symbols.Scope) *symbols.Scope {
	if sym != nil && sym.OwnerScope != nil {
		return sym.OwnerScope
	}
	return root
}

// verdictDetail explains a failed verdict: a condition that evaluated to false
// is the model's answer, not a malfunction, so it is not an error line. what
// names the kind of condition, e.g. "Assertion" or "Required condition".
func verdictDetail(what string, err error) string {
	var violation *runtime.ViolationError
	if errors.As(err, &violation) {
		return fmt.Sprintf("%s evaluated to false: %s", what, violation.Condition)
	}
	return what + " evaluated to false"
}

// onInstance renders the " (on <owner> ID: n)" suffix that marks a result as
// being about one object rather than about declared defaults. The owner is
// spelled as the notation writes it, so it can be typed back.
func onInstance(inst *runtime.Instance, owner string) string {
	if inst == nil {
		return ""
	}
	return fmt.Sprintf(" (on %s ID: %d)", owner, inst.ID)
}

// objectMention names an object by id and, unless the id is all the label
// says, the name or path it was reached by.
func objectMention(inst *runtime.Instance, label string) string {
	if isObjectID(label) {
		return fmt.Sprintf("object #%d", inst.ID)
	}
	return fmt.Sprintf("object #%d of %q", inst.ID, label)
}

// doRequirement evaluates a requirement definition.
func (s *Session) doRequirement(name string) ([]string, bool, error) {
	return s.withTrace(s.checkRequirement(name)).Lines, false, nil
}

// doSatisfy evaluates satisfaction assertions: every one the model states, or,
// given a name, the ones the named element states.
func (s *Session) doSatisfy(args []string) ([]string, bool, error) {
	var name string
	if len(args) > 0 {
		name = args[0]
	}
	var out []string
	for _, v := range s.satisfyVerdicts(name) {
		out = append(out, v.Lines...)
	}
	return out, false, nil
}

// satisfyVerdict renders the verdict of one satisfaction assertion, evaluated
// against an object of its subject: the one the session already created for that
// subject, so a `%instantiate` before it is what the verdict is about.
func (s *Session) satisfyVerdict(ctx *runtime.Context, a *runtime.SatisfyAssertion) Verdict {
	subject, owner := s.subjectInstance(a)
	if subject == nil && a.Subject != nil {
		// No object of the subject exists yet, so the verdict is about a fresh
		// one, created here rather than inside the evaluation so it can be named.
		if inst, serr := ctx.SatisfySubject(a); serr == nil {
			subject, owner = inst, s.keepSubject(a, inst)
		}
	}
	result, err := ctx.CheckSatisfactionOn(a, subject)
	subject, owner = s.reportedSubject(result, subject, owner)
	// A `satisfy requirement r by p` declares its requirement rather than
	// referencing one, so the assertion names it.
	req := a.AssertedRequirement()
	if unevaluable(err) {
		return s.withVerifications(unevaluableVerdict(satisfyText(a), satisfyText(a), err, subject, owner), ctx, req)
	}
	if err != nil || !result.Holds {
		return s.withVerifications(Verdict{Subject: satisfyText(a), Status: VerdictFails, Lines: []string{
			fmt.Sprintf("✗ %s fails%s", satisfyText(a), onInstance(subject, owner)),
			"  " + verdictDetail("Required condition", err),
		}}, ctx, req)
	}
	return s.withVerifications(Verdict{Subject: satisfyText(a), Status: VerdictHolds, Lines: []string{
		fmt.Sprintf("✓ %s holds%s", satisfyText(a), onInstance(subject, owner)),
	}}, ctx, req)
}

// subjectInstance returns the object the session has already created for an
// assertion's subject, with the label it is reported under, or nil for none. A
// chained subject is the object reached from the one created for the feature
// the chain starts from.
func (s *Session) subjectInstance(a *runtime.SatisfyAssertion) (*runtime.Instance, string) {
	name := s.subjectName(a)
	if a.SubjectChain != nil {
		if inst, label := s.objectNamed(name); inst != nil {
			return inst, label
		}
		return nil, ""
	}
	if inst, ok := s.instances[name]; ok {
		return inst, s.declaredName(name)
	}
	return nil, ""
}

// keepSubject holds the object created for an assertion's subject like
// %instantiate would, so a repeated %satisfy is about the same object rather
// than another copy of it, and returns the label it is reported under. For a
// chained subject the object held is the one the chain starts from, which owns
// the rest.
func (s *Session) keepSubject(a *runtime.SatisfyAssertion, inst *runtime.Instance) string {
	name := s.subjectName(a)
	if a.SubjectChain == nil {
		s.instances[name] = inst
		return s.declaredName(name)
	}
	root := inst
	for {
		owner, _ := root.Owner()
		if owner == nil {
			break
		}
		root = owner
	}
	if rootName := s.rootName(a); rootName != "" && s.instanceName(root) == "" {
		s.instances[rootName] = root
	}
	if held, label := s.objectNamed(name); held == inst {
		return label
	}
	return s.declaredName(name)
}

// subjectName is the name an assertion's subject is known by: its
// fully-qualified name, or the reference as written when it resolves to nothing.
// A chained subject is known by the name of the feature the chain starts from
// and the features walked from it, as objectAt reaches it.
func (s *Session) subjectName(a *runtime.SatisfyAssertion) string {
	if a.SubjectChain != nil {
		if root := s.rootName(a); root != "" {
			return strings.Join(append([]string{root}, a.SubjectPath...), "::")
		}
		return a.SubjectRef
	}
	if idx := s.symbolIndex(); idx != nil && a.Subject != nil {
		if fqn := idx.GetFQN(a.Subject); fqn != "" {
			return fqn
		}
	}
	return a.SubjectRef
}

// rootName is the fully-qualified name of the feature a chained subject starts
// from, "" when it resolves to nothing.
func (s *Session) rootName(a *runtime.SatisfyAssertion) string {
	if idx := s.symbolIndex(); idx != nil && a.SubjectRoot != nil {
		return idx.GetFQN(a.SubjectRoot)
	}
	return ""
}

// performingObject resolves the object a debugging session's behavior is
// performed by: its connections route what the behavior sends. No argument
// performs the behavior outside any object.
// It also returns the name that object is held under — its id when no name
// reaches it — so a submission that drops it can end the session.
func (s *Session) performingObject(args []string) (*runtime.Instance, string, error) {
	if len(args) == 0 {
		return nil, "", nil
	}
	return s.resolveObject(args[0])
}

// --- Action Debugging Commands ---

// doAction starts an action executor debugging session.
func (s *Session) doAction(name string, performer []string) ([]string, bool, error) {
	lines, err := s.startAction(name, performer)
	if err != nil {
		if errors.Is(err, errRuntimeInit) {
			return nil, false, err
		}
		return []string{errPrefix + err.Error()}, false, nil
	}
	lines = append(lines, "", "Use %step to advance, %tokens to inspect, %continue to run to completion")
	return lines, false, nil
}

// startAction creates the action executor a debugging session runs, reporting
// what prevented it as an error so a caller outside the prompt can act on it.
func (s *Session) startAction(name string, performer []string) ([]string, error) {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRuntimeInit, err)
	}

	sym, fqn, lerr := s.lookupSymbolOfKinds(name, symbols.SymbolActionUsage, symbols.SymbolActionDef)
	if lerr != nil {
		return nil, lerr
	}

	if sym.Kind != symbols.SymbolActionUsage && sym.Kind != symbols.SymbolActionDef {
		return nil, fmt.Errorf("%q is not an action", name)
	}

	self, selfFQN, perr := s.performingObject(performer)
	if perr != nil {
		return nil, perr
	}

	// Create executor
	exec, err := ctx.CreateActionExecutorFor(sym, self)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}
	exec.SetTrace(s.trace)

	// Store session, letting go of the one it replaces
	s.actionExec.release()
	s.actionExec = &actionSession{
		name:     name,
		fqn:      qualifiedOr(fqn, name),
		selfFQN:  selfFQN,
		symbol:   sym,
		executor: exec,
		rtCtx:    ctx,
	}
	s.endedAction = nil

	// Display initial state
	tokens := exec.Tokens()
	return []string{
		fmt.Sprintf("✓ Started action executor for %q", name),
		fmt.Sprintf("  State: %s", exec.State()),
		fmt.Sprintf("  Tokens: %d", len(tokens)),
	}, nil
}

// doStep advances the action executor one step, or the state machine one event
// when the session debugs a machine instead.
func (s *Session) doStep() ([]string, bool, error) {
	if s.actionExec == nil {
		if s.stateExec != nil {
			return s.stepState()
		}
		return []string{s.noActionSessionMsg()}, false, nil
	}

	exec := s.actionExec.executor

	// Check if already completed
	if exec.State() == runtime.StateCompleted {
		return []string{"✓ Action already completed"}, false, nil
	}

	// Step
	noted := exec.NoteCount()
	err := exec.Step()
	if errors.Is(err, runtime.ErrNothingDue) {
		out := []string{"Nothing to step: the action waits on the clock, which %step does not move"}
		out = append(out, actionWaitLines(exec, s.actionExec.rtCtx)...)
		return append(out, s.noteSummary(exec.Notes(), noted)...), false, nil
	}
	if err != nil {
		out := []string{fmt.Sprintf("error: step failed: %v", err)}
		return append(out, s.noteSummary(exec.Notes(), noted)...), false, nil
	}

	// Display state
	tokens := exec.Tokens()
	out := []string{
		"✓ Step complete",
		fmt.Sprintf("  State: %s", exec.State()),
		fmt.Sprintf("  Tokens: %d", len(tokens)),
	}
	out = append(out, s.noteSummary(exec.Notes(), noted)...)
	if node := exec.PausedAt(); node != "" {
		out = append(out, fmt.Sprintf("  ⏸ Paused at breakpoint %q", node))
	}

	if exec.State() == runtime.StateCompleted {
		out = append(out, "", "✓ Action completed")
		out = append(out, renderResults(s.actionExec.contextOf(), exec.Results())...)
	}

	return out, false, nil
}

// doContinue runs the action to completion.
func (s *Session) doContinue() ([]string, bool, error) {
	return errorLines(s.continueAction())
}

// continueAction runs the active action to completion, or to the first
// breakpoint hit, reporting a failed run as an error.
func (s *Session) continueAction() ([]string, []NamedValue, error) {
	if s.actionExec == nil {
		return nil, nil, s.noActionSessionErr()
	}

	exec := s.actionExec.executor

	// Check if already completed
	if exec.State() == runtime.StateCompleted {
		return []string{"✓ Action already completed"}, namedValues(s.actionExec.contextOf(), exec.Results()), nil
	}

	// Run to completion, or to the first breakpoint hit
	noted := exec.NoteCount()
	if err := exec.RunToCompletion(); err != nil {
		return s.noteSummary(exec.Notes(), noted), nil, fmt.Errorf("execution failed: %w", err)
	}

	if node := exec.PausedAt(); node != "" {
		out := []string{
			fmt.Sprintf("⏸ Paused at breakpoint %q", node),
			fmt.Sprintf("  State: %s", exec.State()),
			fmt.Sprintf("  Tokens: %d", len(exec.Tokens())),
		}
		out = append(out, s.noteSummary(exec.Notes(), noted)...)
		return append(out, "", "Use %tokens to inspect, %step or %continue to resume"), nil, nil
	}

	// Display results
	out := []string{
		"✓ Action completed",
		fmt.Sprintf("  Final state: %s", exec.State()),
	}
	out = append(out, s.noteSummary(exec.Notes(), noted)...)
	out = append(out, renderResults(s.actionExec.contextOf(), exec.Results())...)

	return out, namedValues(s.actionExec.contextOf(), exec.Results()), nil
}

// renderResults lists an action's output values, in name order so a report of
// the same run always reads the same way.
func renderResults(ctx *runtime.Context, results map[string]runtime.Value) []string {
	values := namedValues(ctx, results)
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values)+1)
	out = append(out, "  Results:")
	for _, v := range values {
		out = append(out, fmt.Sprintf("    %s = %s", v.Name, v.Value))
	}
	return out
}

// doTokens displays active tokens.
func (s *Session) doTokens() ([]string, bool, error) {
	if s.actionExec == nil {
		return []string{s.noActionSessionMsg()}, false, nil
	}

	exec := s.actionExec.executor
	tokens := exec.Tokens()

	if len(tokens) == 0 {
		return []string{"No active tokens"}, false, nil
	}

	out := []string{fmt.Sprintf("Active tokens (%d):", len(tokens))}
	for _, tok := range tokens {
		line := fmt.Sprintf("  Token %d @ %s", tok.ID, nodeLabel(tok.Location))
		if awaiting := exec.Awaiting(tok); len(awaiting) > 0 {
			line += fmt.Sprintf(" (arrived from %s; awaiting %s)", nodeLabel(tok.Via.Source), sourceLabels(awaiting))
		}
		out = append(out, line)
	}

	// A token carries no values of its own: it reads and writes the features of
	// the performance it is in, so those are shown once, a node's under its path.
	if values := namedValues(s.actionExec.contextOf(), exec.Results()); len(values) > 0 {
		out = append(out, "  Values:")
		for _, v := range values {
			out = append(out, fmt.Sprintf("    %s = %s", v.Name, v.Value))
		}
	}

	return out, false, nil
}

// nodeLabel names a node, or describes one that declares no name by kind.
func nodeLabel(node ast.Node) string {
	if name := runtime.ActionNodeName(node); name != "" {
		return name
	}
	return anonymousNodeLabel(node)
}

// sourceLabels lists the nodes the successions leave, comma-separated.
func sourceLabels(edges []lower.ActionEdge) string {
	labels := make([]string, len(edges))
	for i, edge := range edges {
		labels[i] = nodeLabel(edge.Source)
	}
	return strings.Join(labels, ", ")
}

// anonymousNodeLabel describes a node that declares no name, by kind.
func anonymousNodeLabel(node ast.Node) string {
	switch node.(type) {
	case *ast.InitialNode:
		return "<initial>"
	case *ast.FinalNode:
		return "<final>"
	case *ast.ForkNode:
		return "<fork>"
	case *ast.JoinNode:
		return "<join>"
	case *ast.MergeNode:
		return "<merge>"
	case *ast.DecisionNode:
		return "<decision>"
	case *ast.ActionExecutionNode:
		return "<action>"
	case nil:
		return "<none>"
	default:
		return "<anonymous>"
	}
}

// doBreak sets a breakpoint at a named node of the running action.
func (s *Session) doBreak(nodeName string) ([]string, bool, error) {
	if s.actionExec == nil {
		return []string{s.noActionSessionMsg()}, false, nil
	}

	exec := s.actionExec.executor
	names := exec.NodeNames()
	if !slices.Contains(names, nodeName) {
		out := []string{fmt.Sprintf("error: action %q has no node named %q", s.actionExec.name, nodeName)}
		if len(names) > 0 {
			out = append(out, "  nodes: "+strings.Join(names, ", "))
		}
		return out, false, nil
	}
	exec.SetBreakpoint(nodeName)

	return []string{
		fmt.Sprintf("✓ Breakpoint set at node %q", nodeName),
		"  %continue runs until a token reaches it",
	}, false, nil
}

// doStop stops the current debugging session.
func (s *Session) doStop() ([]string, bool, error) {
	if s.actionExec == nil && s.stateExec == nil {
		return []string{s.noDebugSessionMsg()}, false, nil
	}

	sessionName := ""
	if s.actionExec != nil {
		sessionName = s.actionExec.name
		s.actionExec.release()
		s.actionExec = nil
	} else if s.stateExec != nil {
		sessionName = s.stateExec.name
		s.stateExec.release()
		s.stateExec = nil
	}

	return []string{fmt.Sprintf("✓ Stopped debugging session for %q", sessionName)}, false, nil
}

// --- State Machine Debugging Commands ---

// doStateMachine starts a state machine executor debugging session.
func (s *Session) doStateMachine(name string, performer []string) ([]string, bool, error) {
	lines, err := s.startStateMachine(name, performer)
	if err != nil {
		if errors.Is(err, errRuntimeInit) {
			return nil, false, err
		}
		s.noteIfMaterializationFailure(err)
		return []string{errPrefix + err.Error()}, false, nil
	}
	lines = append(lines, "", "Use %events to see queue, %current for state, %advance <time> to step")
	return lines, false, nil
}

// startStateMachine creates the state executor a debugging session runs,
// reporting what prevented it as an error.
func (s *Session) startStateMachine(name string, performer []string) ([]string, error) {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRuntimeInit, err)
	}

	// A reference to an object the session holds denotes that object, whose
	// exhibited machine is already running: the debugger drives that machine
	// rather than a detached run of the shared usage. A materialized state
	// machine exhibits none, so a reference to it runs its machine afresh —
	// including as a machine another object performs.
	inst, label, rerr := s.resolveObject(name)
	var (
		sym *symbols.Symbol
		fqn string
	)
	switch {
	case inst != nil:
		if _, exhibits := inst.ExhibitedState(); exhibits || !isMachineSymbol(inst.Type) {
			return s.debugExhibitedMachine(ctx, name, label, inst, performer)
		}
		sym, fqn = inst.Type, symbols.FQNOf(inst.Type)
	case looksLikeObjectPath(name):
		return nil, rerr
	default:
		var lerr error
		sym, fqn, lerr = s.lookupSymbolOfKinds(name, symbols.SymbolStateDef, symbols.SymbolStateUsage)
		if lerr != nil {
			return nil, lerr
		}
		if !isMachineSymbol(sym) {
			var noInstance *NotInstantiatedError
			if errors.As(rerr, &noInstance) {
				return nil, fmt.Errorf("%q is not a state machine, and %w", name, rerr)
			}
			return nil, fmt.Errorf("%q is not a state machine", name)
		}
	}

	// A machine named alone runs on the one held object exhibiting it. Zero or
	// several exhibitors is refused: a detached run would write to no object.
	if len(performer) == 0 {
		switch exhibitors := s.exhibitorsOf(ctx, sym); len(exhibitors) {
		case 0:
			if types := s.exhibitingTypes(ctx, sym); len(types) > 0 {
				return nil, s.exhibitorsError(name, types, nil)
			}
		case 1:
			ex := exhibitors[0]
			if len(ex.machines) > 1 {
				return nil, ambiguousMachine(name, ex.inst, ex.name, ex.machines)
			}
			return s.attachExhibitedMachine(ctx, name, ex.name, ex.inst, ex.machines[0]), nil
		default:
			return nil, s.exhibitorsError(name, nil, exhibitors)
		}
	}

	self, selfFQN, perr := s.performingObject(performer)
	if perr != nil {
		return nil, perr
	}

	// The machine the object exhibits is already running on it: a second
	// performance would run entry and do behaviors against the same slots again.
	if self != nil {
		switch exhibited := self.ExhibitedStatesOf(sym); len(exhibited) {
		case 0:
		case 1:
			lines := s.attachExhibitedMachine(ctx, name, selfFQN, self, exhibited[0])
			notice := fmt.Sprintf("note: %s already exhibits %q, so this session attaches to that running machine rather than starting a second performance of it (as `%%state %s` would)",
				objectMention(self, selfFQN), name, performer[0])
			return append([]string{lines[0], notice}, lines[1:]...), nil
		default:
			return nil, ambiguousMachine(name, self, selfFQN, exhibited)
		}
	}

	// Create executor
	exec, err := ctx.CreateStateExecutorFor(sym, self)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}
	exec.SetTrace(s.trace)

	// Store session, letting go of the one it replaces
	s.stateExec.release()
	s.stateExec = &stateSession{
		name:     name,
		fqn:      qualifiedOr(fqn, name),
		selfFQN:  selfFQN,
		symbol:   sym,
		executor: exec,
		rtCtx:    ctx,
	}
	s.endedState = nil

	lines := []string{fmt.Sprintf("✓ Started state machine executor for %q", name)}
	if self != nil {
		lines = append(lines, fmt.Sprintf("  Performed by %s, which exhibits no running machine of this kind", objectMention(self, selfFQN)))
	}
	return append(lines,
		fmt.Sprintf("  Current state: %s", currentStateName(exec)),
		timeLabel+semantics.FormatReal(exec.CurrentTime()),
		fmt.Sprintf("  Events: %d", exec.EventQueue().Len()),
	), nil
}

// isMachineSymbol reports whether sym declares a state machine.
func isMachineSymbol(sym *symbols.Symbol) bool {
	return sym != nil && (sym.Kind == symbols.SymbolStateDef || sym.Kind == symbols.SymbolStateUsage)
}

// debugExhibitedMachine binds the debugging session to the machine an object
// already exhibits, so %current, %events and %advance drive that object's
// machine and %features shows what it wrote. label is the object as the REPL
// reports it, which the session is then held under.
func (s *Session) debugExhibitedMachine(
	ctx *runtime.Context,
	name, label string,
	inst *runtime.Instance,
	performer []string,
) ([]string, error) {
	if len(performer) > 0 {
		return nil, fmt.Errorf("%q is an object, which performs its exhibited machine itself", label)
	}
	behavior, ok := inst.ExhibitedState()
	if !ok {
		return nil, fmt.Errorf("object %q exhibits no state machine", label)
	}
	return s.attachExhibitedMachine(ctx, name, label, inst, behavior), nil
}

// attachExhibitedMachine binds the session to a machine an object exhibits. The
// session is keyed by the object's label, so a restart of the machine rebinds it.
func (s *Session) attachExhibitedMachine(
	ctx *runtime.Context,
	name, label string,
	inst *runtime.Instance,
	behavior *runtime.ObjectBehavior,
) []string {
	behavior.State.SetTrace(s.trace)

	s.stateExec.release()
	s.stateExec = &stateSession{
		name:      name,
		fqn:       label,
		selfFQN:   label,
		symbol:    behavior.Symbol,
		executor:  behavior.State,
		machine:   behavior.Name,
		machineAt: exhibitedPosition(behavior),
		rtCtx:     ctx,
	}
	s.endedState = nil

	return []string{
		fmt.Sprintf("✓ Debugging state machine %q exhibited by %s", behavior.Name, objectMention(inst, label)),
		fmt.Sprintf("  Current state: %s", currentStateName(behavior.State)),
		timeLabel + semantics.FormatReal(behavior.State.CurrentTime()),
		fmt.Sprintf("  Events: %d", behavior.State.EventQueue().Len()),
	}
}

// ExhibitorsError reports a machine `%state <machine>` alone cannot attach to:
// no held object exhibits it, or several do, so no one running performance is meant.
type ExhibitorsError struct {
	Machine string          // the machine asked for, as the prompt prints names
	Types   []string        // the types declaring an exhibit of it, in declaration order, as the prompt prints names
	Objects []RelatedObject // the held objects exhibiting it, in walk order; none when no object does
}

func (e *ExhibitorsError) Error() string {
	if len(e.Objects) == 0 {
		types := make([]string, len(e.Types))
		for i, t := range e.Types {
			types[i] = fmt.Sprintf("%q", t)
		}
		return fmt.Sprintf("no object of this session exhibits %q, which runs only on an object of %s: use %%instantiate to create one, then %%state <object> or %%state %s <object>",
			e.Machine, strings.Join(types, " or "), e.Machine)
	}
	labels := make([]string, len(e.Objects))
	for i, o := range e.Objects {
		labels[i] = fmt.Sprintf("#%d", o.ID)
		if o.Label != "" {
			labels[i] = fmt.Sprintf("#%d of %q", o.ID, o.Label)
		}
	}
	return fmt.Sprintf("%d objects of this session exhibit %q (%s), so naming the machine alone attaches to none of them: use %%state <object> or %%state %s <object> to name one",
		len(e.Objects), e.Machine, strings.Join(labels, ", "), e.Machine)
}

// exhibitor is a held object running a machine, with the machines of it that run
// under one declaration.
type exhibitor struct {
	carrier
	machines []*runtime.ObjectBehavior
}

// exhibitorsOf finds the held objects exhibiting sym's machine, in carrier order.
// Nothing is materialized: an object not yet held runs no machine to attach to.
func (s *Session) exhibitorsOf(ctx *runtime.Context, sym *symbols.Symbol) []exhibitor {
	var found []exhibitor
	s.walkHeldObjects(ctx, func(cur carrier) bool {
		if machines := cur.inst.ExhibitedStatesOf(sym); len(machines) > 0 {
			found = append(found, exhibitor{carrier: cur, machines: machines})
		}
		return true
	})
	return found
}

// exhibitingTypes finds the types of the session's documents declaring an exhibit
// of sym's machine (the usage itself, or one typed by or naming it), in declaration order.
func (s *Session) exhibitingTypes(ctx *runtime.Context, sym *symbols.Symbol) []*symbols.Symbol {
	var types []*symbols.Symbol
	for _, e := range s.exhibitEntries(ctx) {
		// One mention per type: the entries of a type are contiguous.
		if len(types) > 0 && types[len(types)-1] == e.owner {
			continue
		}
		if ctx.ExhibitsState(e.member, sym) {
			types = append(types, e.owner)
		}
	}
	return types
}

// exhibitEntry is a named member declaring an exhibited state, with the type owning it.
type exhibitEntry struct {
	owner, member *symbols.Symbol
}

// exhibitIndex is the documents' exhibitEntry list in declaration order, valid
// for the runtime context it was built alongside.
type exhibitIndex struct {
	ctx     *runtime.Context
	entries []exhibitEntry
}

// exhibitEntries returns the session's exhibited-state declarations, indexed once
// per runtime context: a submission rebuilds the context, and the scopes with it.
func (s *Session) exhibitEntries(ctx *runtime.Context) []exhibitEntry {
	if s.exhibits != nil && s.exhibits.ctx == ctx {
		return s.exhibits.entries
	}
	idx := &exhibitIndex{ctx: ctx}
	var collect func(scope *symbols.Scope)
	collect = func(scope *symbols.Scope) {
		if scope == nil {
			return
		}
		if owner := scope.Owner(); owner != nil {
			scope.ForEachMember(func(member *symbols.Symbol) bool {
				if member.Name != "" && member.Decl != nil {
					if b, ok := lower.ClassifierBehaviorOf(member.Decl); ok && b.Kind == lower.ExhibitedState {
						idx.entries = append(idx.entries, exhibitEntry{owner: owner, member: member})
					}
				}
				return true
			})
		}
		for _, child := range scope.Children() {
			collect(child)
		}
	}
	for _, scope := range s.docScopes() {
		collect(scope)
	}
	s.exhibits = idx
	return idx.entries
}

// exhibitorsError reports sym's machine as one `%state <machine>` alone cannot
// attach to, naming the types declaring an exhibit of it and the objects exhibiting it.
func (s *Session) exhibitorsError(name string, types []*symbols.Symbol, exhibitors []exhibitor) error {
	e := &ExhibitorsError{Machine: name}
	for _, typ := range types {
		e.Types = append(e.Types, declarationNotation(typ))
	}
	for _, ex := range exhibitors {
		ref := RelatedObject{ID: ex.inst.ID}
		if !isObjectID(ex.name) {
			ref.Label = ex.name
		}
		e.Objects = append(e.Objects, ref)
	}
	return e
}

// AmbiguousMachineError reports a state definition an object exhibits as the
// body of several usages, so naming the definition names no one machine.
type AmbiguousMachineError struct {
	Object  string // the object, as objectMention prints it
	Machine string // the definition asked for, as typed
	// Usages are the exhibited usages running it, in declaration order, as the
	// prompt prints names; "" for one without a name.
	Usages []string
}

func (e *AmbiguousMachineError) Error() string {
	names := make([]string, len(e.Usages))
	for i, u := range e.Usages {
		names[i] = u
		if u == "" {
			names[i] = "an unnamed one"
		}
	}
	return fmt.Sprintf("%s exhibits %q as %d machines, so naming the definition attaches to none of them: name the exhibited usage instead — %s",
		e.Object, e.Machine, len(e.Usages), strings.Join(names, " or "))
}

// ambiguousMachine reports a definition inst exhibits as the body of several
// machines, naming the usages that would address one.
func ambiguousMachine(machine string, inst *runtime.Instance, label string, exhibited []*runtime.ObjectBehavior) error {
	e := &AmbiguousMachineError{Object: objectMention(inst, label), Machine: machine}
	for _, b := range exhibited {
		usage := ""
		if member := b.Member(); member != nil && member.Name != "" {
			usage = declarationNotation(member)
		}
		e.Usages = append(e.Usages, usage)
	}
	return e
}

// stepState takes the machine's next step: a change condition that has become
// true, the event due next, or one round of the do behaviors active now.
func (s *Session) stepState() ([]string, bool, error) {
	exec := s.stateExec.executor
	// A machine parked on a change condition becomes runnable when data written
	// outside it makes the condition true, so resume it and let the poll decide.
	if exec.HasPendingWork() || exec.WatchesChangeCondition() {
		exec.Resume()
	}
	if exec.State() != runtime.StateRunning {
		return []string{fmt.Sprintf("✓ State machine %s (%s)", exec.State(), currentStateName(exec))}, false, nil
	}

	noted := exec.NoteCount()
	step, err := s.stateStep(exec)
	if err != nil {
		out := []string{errPrefix + err.Error()}
		return append(out, s.noteSummary(exec.Notes(), noted)...), false, nil
	}
	out := []string{
		"✓ " + step,
		fmt.Sprintf("  Current state: %s", currentStateName(exec)),
		timeLabel + semantics.FormatReal(exec.CurrentTime()),
		fmt.Sprintf("  Events: %d", exec.EventQueue().Len()),
	}
	return append(out, s.noteSummary(exec.Notes(), noted)...), false, nil
}

// stateStep performs one step and reports what it was.
func (s *Session) stateStep(exec *runtime.StateExecutor) (string, error) {
	fired, err := exec.PollChangeEvents()
	if err != nil {
		return "", fmt.Errorf("change condition failed: %w", err)
	}
	if fired {
		return "Change event dispatched", nil
	}
	if exec.EventQueue().Len() > 0 || exec.HasPendingSignal() {
		if err := exec.ProcessNextEvent(); err != nil {
			return "", fmt.Errorf("event processing failed: %w", err)
		}
		if note := droppedSignalNote(exec); note != "" {
			return "Event dispatched, but " + note, nil
		}
		return "Event dispatched", nil
	}
	if exec.HasPendingDoWork() {
		ran, err := exec.RunDoRound()
		if err != nil {
			return "", fmt.Errorf("do behavior failed: %w", err)
		}
		return fmt.Sprintf("Ran %d do action(s)", ran), nil
	}
	// Nothing progressed, so a machine resumed for this step parks again.
	exec.Suspend()
	if reason := exec.SuspendReason(); reason != "" {
		return "Nothing to do: " + reason, nil
	}
	return "Nothing to do: the machine is quiescent", nil
}

// doInvoke invokes an operation on an object, with the object as its performer.
func (s *Session) doInvoke(name, operation string, args []string) ([]string, bool, error) {
	lines, err := s.invokeOperation(name, operation, args)
	if err != nil {
		if errors.Is(err, errRuntimeInit) {
			return nil, false, err
		}
		s.noteIfMaterializationFailure(err)
		return []string{errPrefix + err.Error()}, false, nil
	}
	return lines, false, nil
}

// invokeOperation binds the arguments written as `name=<expression>` and runs the
// operation the object's type owns, performed by that object; the arguments are
// parsed before the object is reached.
func (s *Session) invokeOperation(name, operation string, args []string) ([]string, error) {
	parsed, err := parseArguments(args)
	if err != nil {
		return nil, err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRuntimeInit, err)
	}
	inst, label, rerr := s.resolveObject(name)
	if rerr != nil {
		return nil, rerr
	}
	bound, err := s.evalArguments(ctx, parsed)
	if err != nil {
		return nil, err
	}
	results, err := ctx.InvokeOperation(inst, operation, bound)
	if err != nil {
		return nil, err
	}
	out := []string{fmt.Sprintf("✓ Invoked %s on %s", operation, objectMention(inst, label))}
	if values := namedValues(ctx, results); len(values) > 0 {
		out = append(out, "", "Results:")
		for _, v := range values {
			out = append(out, fmt.Sprintf("  %s = %s", v.Name, v.Value))
		}
	}
	return out, nil
}

// argument is one `name=<expression>` argument of %invoke or %send, parsed and
// not yet evaluated.
type argument struct {
	param string
	node  ast.Node
}

// parseArguments takes apart and parses `name=<expression>` arguments.
func parseArguments(args []string) ([]argument, error) {
	parsed := make([]argument, 0, len(args))
	for _, arg := range args {
		param, expr, ok := strings.Cut(arg, "=")
		param, expr = strings.TrimSpace(param), strings.TrimSpace(expr)
		if !ok || param == "" || expr == "" {
			return nil, fmt.Errorf("argument %q is not written as <parameter>=<expression>", arg)
		}
		node, diags := parseExprAlone(expr)
		if len(diags) > 0 {
			return nil, exprError(expr, diags[0].Message, diags[0].Span, len(exprPrefix))
		}
		if node == nil {
			return nil, fmt.Errorf("argument %s: could not parse %q", param, expr)
		}
		parsed = append(parsed, argument{param: param, node: node})
	}
	return parsed, nil
}

// evalArguments evaluates parsed arguments where the prompt evaluates any
// expression, binding each to its parameter.
func (s *Session) evalArguments(ctx *runtime.Context, parsed []argument) (map[string]runtime.Value, error) {
	if len(parsed) == 0 {
		return nil, nil
	}
	scope := s.promptScope()
	bound := make(map[string]runtime.Value, len(parsed))
	for _, arg := range parsed {
		value, err := ctx.EvalWithScope(arg.node, scope)
		if err != nil {
			return nil, fmt.Errorf("argument %s: %w", arg.param, err)
		}
		bound[arg.param] = value
	}
	return bound, nil
}

// doEvents displays the event queue.
func (s *Session) doEvents() ([]string, bool, error) {
	if s.stateExec == nil {
		return []string{s.noStateSessionMsg()}, false, nil
	}

	exec := s.stateExec.executor
	queue := exec.EventQueue()
	signals, err := s.signalsInFlight(exec)
	if err != nil {
		return nil, false, err
	}
	deferred := exec.DeferredEvents()

	if queue.Len() == 0 && len(signals) == 0 && len(deferred) == 0 {
		return []string{"Event queue empty"}, false, nil
	}

	// Note: EventQueue doesn't expose events directly, so just show count
	var out []string
	if queue.Len() > 0 {
		out = append(out, fmt.Sprintf("Event queue: %d events", queue.Len()))
	}
	if len(signals) > 0 {
		out = append(out, fmt.Sprintf("Signals in flight: %d", len(signals)))
		for _, msg := range signals {
			out = append(out, "  "+signalText(msg))
		}
	}
	if len(deferred) > 0 {
		out = append(out, fmt.Sprintf("Deferred by the active state, held until it leaves: %d", len(deferred)))
		for _, event := range deferred {
			out = append(out, "  "+eventText(event))
		}
	}
	return append(out, "Use %advance <time> to process next event"), false, nil
}

// signalsInFlight lists the messages on the bus the debugged machine would
// deliver on its next step.
func (s *Session) signalsInFlight(exec *runtime.StateExecutor) ([]runtime.Message, error) {
	var out []runtime.Message
	for _, msg := range s.stateExec.rtCtx.PendingMessages() {
		accepted, err := exec.AcceptsMessage(msg)
		if err != nil {
			return nil, fmt.Errorf("state machine %q cannot accept %s: %w", s.stateExec.name, signalText(msg), err)
		}
		if accepted {
			out = append(out, msg)
		}
	}
	return out, nil
}

// doCurrent shows current state and configuration.
func (s *Session) doCurrent() ([]string, bool, error) {
	if s.stateExec == nil {
		return []string{s.noStateSessionMsg()}, false, nil
	}

	exec := s.stateExec.executor
	stateStack := exec.StateStack()
	stateData := exec.StateData()

	out := []string{
		fmt.Sprintf("Current state: %s", currentStateName(exec)),
		"Time: " + semantics.FormatReal(exec.CurrentTime()),
		"Last event at: " + semantics.FormatReal(exec.LastEventAt()),
		fmt.Sprintf("Execution state: %s", exec.State()),
	}

	if reason := exec.SuspendReason(); reason != "" {
		out = append(out, fmt.Sprintf("Cannot progress: %s", reason))
	}

	if len(stateStack) > 1 {
		out = append(out, "", "State stack (active configuration):")
		for i, stateNode := range stateStack {
			if stateNode.Name != "" {
				out = append(out, fmt.Sprintf("  %d. %s", i, stateNode.Name))
			}
		}
	}

	if values := namedValues(s.stateExec.contextOf(), stateData); len(values) > 0 {
		out = append(out, "", "State data:")
		for _, v := range values {
			out = append(out, fmt.Sprintf("  %s = %s", v.Name, v.Value))
		}
	}

	return out, false, nil
}

// currentStateName renders the machine's active configuration: the active
// state's name, or one name per orthogonal region.
func currentStateName(exec *runtime.StateExecutor) string {
	active := exec.ActiveStates()
	if len(active) == 0 {
		return "<none>"
	}
	names := make([]string, 0, len(active))
	for _, state := range active {
		if state.Name != "" {
			names = append(names, state.Name)
		} else {
			names = append(names, "<anonymous>")
		}
	}
	return strings.Join(names, " | ")
}

// parseDuration reads the argument of %advance: a number of time units, with
// an optional trailing "s".
func parseDuration(arg string) (float64, error) {
	text := strings.TrimSuffix(strings.TrimSpace(arg), "s")
	d, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid time %q (expected a number of time units, e.g. 30 or 30s)", arg)
	}
	if math.IsNaN(d) || math.IsInf(d, 0) {
		return 0, fmt.Errorf("invalid time %q (expected a finite number of time units, e.g. 30 or 30s)", arg)
	}
	if d < 0 {
		return 0, fmt.Errorf("invalid time %q (must not be negative)", arg)
	}
	return d, nil
}

// doAdvance advances simulation time by the given duration, processing every
// event scheduled at or before the deadline.
func (s *Session) doAdvance(timeStr string) ([]string, bool, error) {

	duration, err := parseDuration(timeStr)
	if err != nil {
		return []string{errPrefix + err.Error()}, false, nil
	}
	lines, err := s.advanceBy(duration)
	if err != nil {
		return append([]string{errPrefix + err.Error()}, lines...), false, nil
	}
	return lines, false, nil
}

// advanceBy advances the debuggers' runtime clock by duration and reports what
// ran; a failed step is an error, a budget stop part of the report.
func (s *Session) advanceBy(duration float64) ([]string, error) {
	if s.actionExec == nil && s.stateExec == nil {
		return nil, errors.New(noSessionText("debugging session", s.mostRecentlyEnded(), ""))
	}
	var state *runtime.StateExecutor
	if s.stateExec != nil {
		state = s.stateExec.executor
	}
	var action *runtime.ActionExecutor
	if s.actionExec != nil {
		action = s.actionExec.executor
	}

	// The two debuggers share one context unless a submission rebuilt it between
	// starting them; each context's clock is advanced by the duration, and the
	// header reports the state machine's, the action's own clock a line of its own.
	contexts := distinctContexts(s.stateExec.contextOf(), s.actionExec.contextOf())
	moved, out, err := s.advanceContexts(contexts, duration)
	if err != nil {
		return out, err
	}

	// Sessions that took no step and have nowhere to go say why; ones whose work
	// is only due past the deadline still report the drain and what is left.
	if moved.idle() && !s.debuggersHavePendingWork(contexts) {
		out := []string{"No pending work - simulation time is now " + semantics.FormatReal(moved.report.To)}
		out = append(out, s.separateActionClockLines(contexts, moved)...)
		if state != nil {
			if reason := state.SuspendReason(); reason != "" {
				out = append(out, "  "+reason)
			}
		}
		if action != nil && action.State() == runtime.StateWaiting {
			out = append(out, actionWaitLines(action, s.actionExec.rtCtx)...)
		}
		return out, nil
	}

	out = []string{advancedHeader(moved.report)}
	if state != nil {
		out = append(out, stateStatusLines(state)...)
	}
	if action != nil {
		out = append(out, actionStatusLines(action)...)
	}
	out = append(out, s.separateActionClockLines(contexts, moved)...)
	out = append(out, s.advanceReportLines(moved, contexts)...)

	if state != nil && state.State() == runtime.StateCompleted {
		out = append(out, "", stateCompletedText)
	}
	if action != nil && action.State() == runtime.StateCompleted {
		out = append(out, "", "✓ Action completed")
		out = append(out, renderResults(s.actionExec.contextOf(), action.Results())...)
	}
	return out, nil
}

const stateCompletedText = "✓ State machine completed (a transition reached `done`)"

// distinctContexts returns the contexts given, dropping none and repeats.
func distinctContexts(contexts ...*runtime.Context) []*runtime.Context {
	var distinct []*runtime.Context
	for _, ctx := range contexts {
		if ctx != nil && !slices.Contains(distinct, ctx) {
			distinct = append(distinct, ctx)
		}
	}
	return distinct
}

// advanceOutcome is what advancing the clock did: the drain summed over the contexts
// moved (its instants the first's), each context's own report, and the budget that cut it short.
type advanceOutcome struct {
	report    runtime.AdvanceReport
	byContext []runtime.AdvanceReport
	stopped   error
}

// idle reports a drain that ran nothing.
func (o advanceOutcome) idle() bool {
	return o.report.Events == 0 && o.report.DoSteps == 0 && o.report.Steps == 0
}

// advanceContexts advances each context's clock by duration; a failed step is
// the error, with the choice summary of what ran before it.
func (s *Session) advanceContexts(contexts []*runtime.Context, duration float64) (advanceOutcome, []string, error) {
	var moved advanceOutcome
	for i, ctx := range contexts {
		report, err := ctx.Advance(duration)
		moved.byContext = append(moved.byContext, report)
		if i == 0 {
			moved.report.From, moved.report.To = report.From, report.To
		}
		moved.report.Events += report.Events
		moved.report.DoSteps += report.DoSteps
		moved.report.Steps += report.Steps
		moved.report.Dropped = append(moved.report.Dropped, report.Dropped...)
		moved.report.Notes = append(moved.report.Notes, report.Notes...)
		if err != nil {
			if !isBudgetStop(err) {
				return moved, s.noteSummary(moved.report.Notes, 0), fmt.Errorf("advance failed: %w", err)
			}
			moved.stopped = err
			break
		}
	}
	return moved, nil, nil
}

// separateActionClockLines say where the action debugger's own clock went when
// it runs in a context other than the state debugger's, whose clock the header reports.
func (s *Session) separateActionClockLines(contexts []*runtime.Context, moved advanceOutcome) []string {
	if len(contexts) < 2 {
		return nil
	}
	own := "  The action runs in a context of its own, whose clock "
	if len(moved.byContext) < 2 {
		return []string{own + "stays at " + semantics.FormatReal(s.actionExec.rtCtx.Clock().Now()) + ": the advance stopped before reaching it"}
	}
	report := moved.byContext[1]
	return []string{own + "advanced from " + semantics.FormatReal(report.From) + " to " + semantics.FormatReal(report.To)}
}

// advancedHeader is the first line of an advance that ran something.
func advancedHeader(report runtime.AdvanceReport) string {
	return fmt.Sprintf("✓ Advanced to %s (%d event(s) processed)", semantics.FormatReal(report.To), report.Events)
}

// stateStatusLines report where a machine stands after an advance.
func stateStatusLines(state *runtime.StateExecutor) []string {
	return []string{
		fmt.Sprintf("  Current state: %s", currentStateName(state)),
		"  Last event at: " + semantics.FormatReal(state.LastEventAt()),
		fmt.Sprintf("  Remaining events: %d", state.EventQueue().Len()),
	}
}

// actionStatusLines report where an action stands after an advance.
func actionStatusLines(action *runtime.ActionExecutor) []string {
	out := []string{
		fmt.Sprintf("  Action state: %s", action.State()),
		fmt.Sprintf("  Tokens: %d", len(action.Tokens())),
	}
	if node := action.PausedAt(); node != "" {
		out = append(out, fmt.Sprintf("  ⏸ Paused at breakpoint %q", node))
	}
	return out
}

// advanceReportLines report what the drain ran: choices, do and action steps,
// dropped signals, the budget that stopped it and what still waits on the clock.
func (s *Session) advanceReportLines(moved advanceOutcome, contexts []*runtime.Context) []string {
	out := s.noteSummary(moved.report.Notes, 0)
	if moved.report.DoSteps > 0 {
		out = append(out, fmt.Sprintf("  Do behavior actions run: %d", moved.report.DoSteps))
	}
	if moved.report.Steps > 0 {
		out = append(out, fmt.Sprintf("  Action steps taken: %d", moved.report.Steps))
	}
	for _, d := range moved.report.Dropped {
		if note := droppedDispatchNote(d); note != "" {
			out = append(out, "  "+note)
		}
	}
	out = append(out, s.budgetStopLines(moved.stopped, moved.report)...)
	for _, ctx := range contexts {
		out = append(out, clockWaitLines(ctx)...)
	}
	return out
}

// isBudgetStop reports an advance a budget cut short, which is a stop to
// report rather than a failed run.
func isBudgetStop(err error) bool {
	return errors.Is(err, runtime.ErrStateEventLimitExceeded) ||
		errors.Is(err, runtime.ErrDoStepLimitExceeded) ||
		errors.Is(err, runtime.ErrActionStepLimitExceeded)
}

// budgetStopLines say which budget cut a drain short, so it does not read as a
// machine that settled, and how to raise it.
func (s *Session) budgetStopLines(stopped error, report runtime.AdvanceReport) []string {
	switch {
	case errors.Is(stopped, runtime.ErrStateEventLimitExceeded):
		out := []string{fmt.Sprintf("  Stopped at the event budget (%d events; raise %s to allow more)",
			s.budgets.MaxStateEvents, runtime.MaxStateEventsEnvVar)}
		// A drain that never advanced time is all at one instant — often a cycle
		// of untriggered (completion) transitions, which no budget can drain.
		if report.To == report.From {
			out = append(out, fmt.Sprintf("  All %d event(s) were processed at simulation time %s without advancing it; if the machine cycles through untriggered (completion) transitions, which re-fire immediately, no budget is large enough",
				report.Events, semantics.FormatReal(report.From)))
		}
		return out
	case errors.Is(stopped, runtime.ErrDoStepLimitExceeded):
		return []string{fmt.Sprintf("  Stopped at the do action budget (%d steps; raise %s to allow more)",
			s.budgets.MaxDoSteps, runtime.MaxDoStepsEnvVar)}
	case errors.Is(stopped, runtime.ErrActionStepLimitExceeded):
		return []string{"  Stopped at the action step budget: " + stopped.Error()}
	}
	return nil
}

// debuggersHavePendingWork reports work the debugged executors still have, now
// or once the clock reaches a wait on it.
func (s *Session) debuggersHavePendingWork(contexts []*runtime.Context) bool {
	if s.stateExec != nil && s.stateExec.executor.HasPendingWork() {
		return true
	}
	if s.actionExec != nil && s.actionExec.executor.State() == runtime.StateRunning {
		return true
	}
	for _, ctx := range contexts {
		if len(ctx.Clock().Waits()) > 0 {
			return true
		}
	}
	return false
}

// clockWaitLines list what is left waiting on a context's clock, earliest first.
func clockWaitLines(ctx *runtime.Context) []string {
	waits := ctx.Clock().Waits()
	if len(waits) == 0 {
		return nil
	}
	out := []string{"  Waiting on the clock:"}
	for _, w := range waits {
		out = append(out, fmt.Sprintf("    t=%s: %s, %s", semantics.FormatReal(w.Due), w.Holder, w.What))
	}
	return out
}

// actionWaitLines say what a parked action's tokens wait for, and that %advance
// moves the clock for those waiting on it.
func actionWaitLines(exec *runtime.ActionExecutor, ctx *runtime.Context) []string {
	out := tokenWaitLines(exec)
	if next, ok := exec.NextWait(); ok {
		now := ctx.Clock().Now()
		out = append(out, fmt.Sprintf("  Use %%advance %s to move the clock from t=%s to the earliest wait",
			semantics.FormatReal(next-now), semantics.FormatReal(now)))
	}
	return out
}

// tokenWaitLines list what each parked token of an action waits for.
func tokenWaitLines(exec *runtime.ActionExecutor) []string {
	var out []string
	for _, token := range exec.Tokens() {
		if token.Wait != nil {
			out = append(out, fmt.Sprintf("  Token %d: %s", token.ID, token.Wait))
		}
	}
	return out
}
