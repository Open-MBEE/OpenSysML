// Package sessiontools is the session group of sysml-mcp tools: named REPL
// sessions that keep a model, its instances, a running action or state machine
// and the solver's state across calls, as the sysml prompt does. Every tool
// takes an optional session name and answers with the data the prompt formats
// beside the prompt's own text.
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

// SessionName names a session; empty is the default session.
type SessionName struct {
	Session string `json:"session,omitempty" jsonschema:"session name; omit for the default session"`
}

func (n SessionName) sessionName() string { return n.Session }

// SessionOpenInput names the session to open and, optionally, the directory
// its unresolved root namespaces are looked for under.
type SessionOpenInput struct {
	SessionName
	Root string `json:"root,omitempty" jsonschema:"directory under the workspace root, relative to it or absolute, whose model files load when a submission imports or qualifies a root namespace nothing in the session declares; omit to resolve against the session and the library only"`
}

// sessionNamer is a tool input naming the session it addresses.
type sessionNamer interface{ sessionName() string }

// handler adapts a function over a session to a tool handler: the session named
// is used one call at a time, and the call's context interrupts a long run.
func handler[In sessionNamer, Out any](m *Manager, fn func(context.Context, *repl.Session, In) (Out, error)) func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		var out Out
		err := m.Use(ctx, in.sessionName(), func(s *repl.Session) error {
			var err error
			out, err = fn(ctx, s, in)
			return err
		})
		// A call the client abandoned interrupted the session's command, so what
		// it reports is the run cut short, not an answer.
		if ctxErr := ctx.Err(); ctxErr != nil && err == nil {
			err = fmt.Errorf("the call was cancelled and the session's command interrupted: %w", ctxErr)
		}
		return nil, out, err
	}
}

// failed is an error carrying the lines the prompt printed before or with it,
// so a client reads what the session reported, not only that it failed.
func failed(err error, lines []string) error {
	if err == nil || len(lines) == 0 {
		return err
	}
	return fmt.Errorf("%w\n%s", err, strings.Join(lines, "\n"))
}

// text joins the prompt's lines as one block.
func text(lines []string) string { return strings.Join(lines, "\n") }

// SessionOpened reports the session a call opened or closed.
type SessionOpened struct {
	Session string `json:"session" jsonschema:"the session's name"`
}

// SessionListInput takes no arguments.
type SessionListInput struct{}

// SessionList is the open sessions.
type SessionList struct {
	Sessions []string `json:"sessions" jsonschema:"names of the open sessions, sorted"`
}

// Diagnostic is one finding about the session's model, placed in the file it
// was loaded from or in the typed input.
type Diagnostic struct {
	Severity string `json:"severity" jsonschema:"error, warning or info"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty" jsonschema:"the loaded file the finding is in; empty for text submitted directly"`
	Line     int    `json:"line" jsonschema:"1-based line within the file or submission"`
	Column   int    `json:"column" jsonschema:"1-based column"`
	Pass     string `json:"pass,omitempty" jsonschema:"the analysis pass that produced the finding"`
	Code     string `json:"code,omitempty" jsonschema:"the finding's code"`
}

func diagnostics(ds []repl.Diagnostic) []Diagnostic {
	out := make([]Diagnostic, 0, len(ds))
	for _, d := range ds {
		out = append(out, Diagnostic{Severity: d.Severity, Message: d.Message, File: d.File, Line: d.Line, Column: d.Column, Pass: d.Pass, Code: d.Code})
	}
	return out
}

// LoadInput names the files to load.
type LoadInput struct {
	SessionName
	Paths []string `json:"paths" jsonschema:"files, directories (walked for .sysml/.kerml/.ipynb) or glob patterns, relative to the workspace root or absolute; a path outside the root is refused"`
}

// LoadResult is what a load read and what the analysis found.
type LoadResult struct {
	Files       []string     `json:"files" jsonschema:"the model files the paths named, in load order; files they import from beside them load too"`
	Declared    []string     `json:"declared" jsonschema:"what the load declared, as the prompt summarizes it; empty when the analysis found an error"`
	Errors      bool         `json:"errors" jsonschema:"whether the session's model now has an error"`
	Diagnostics []Diagnostic `json:"diagnostics" jsonschema:"every finding over the session's model after the load, located in its file"`
	Text        string       `json:"text" jsonschema:"what %load printed"`
}

// SubmitInput is declaration text for the session.
type SubmitInput struct {
	SessionName
	Text string `json:"text" jsonschema:"SysML v2 (or KerML) declarations, as typed at the prompt; a redeclaration of a name replaces the earlier one"`
}

// SubmitResult is what a submission declared and what the analysis found.
type SubmitResult struct {
	Accepted    bool         `json:"accepted" jsonschema:"whether the text was taken into the model without an error of its own"`
	Declared    []string     `json:"declared" jsonschema:"the names this submission introduced"`
	Notices     []string     `json:"notices" jsonschema:"side effects, such as a debugging session the submission ended"`
	Diagnostics []Diagnostic `json:"diagnostics" jsonschema:"the findings about this submission, located in it"`
	Text        string       `json:"text" jsonschema:"what the prompt printed for the submission"`
}

// EvalInput is an expression to evaluate.
type EvalInput struct {
	SessionName
	Expression string `json:"expression" jsonschema:"a SysML expression: a literal, a feature or qualified name, or a compound expression over the session's declarations"`
}

// EvalResult is the value an expression evaluated to.
type EvalResult struct {
	Expression string `json:"expression"`
	Value      any    `json:"value" jsonschema:"the value in the runtime API's JSON form (opensysml.v1.Value): a typed scalar, sequence, set, instance reference or function"`
	Text       string `json:"text" jsonschema:"the value as the prompt prints it"`
	Output     string `json:"output" jsonschema:"what %eval printed"`
}

// PrintInput names what to print.
type PrintInput struct {
	SessionName
	Name string `json:"name,omitempty" jsonschema:"qualified name of one element to print with its body; omit for the whole model"`
}

// Notation is a model, or part of one, as SysML notation.
type Notation struct {
	Text string `json:"text" jsonschema:"the notation, as %print writes it; what is printed can be submitted back"`
}

// SaveInput names the file to write the model to.
type SaveInput struct {
	SessionName
	Path  string `json:"path" jsonschema:"file to write, relative to the workspace root or absolute but under it: .sysml notation, or .ttl/.json RDF (experimental)"`
	Apply bool   `json:"apply,omitempty" jsonschema:"write the file; without it the bytes the file would hold (notation, Turtle or JSON by extension) are returned as content and nothing is touched"`
}

// SaveResult is what a save wrote, or would write.
type SaveResult struct {
	Path    string `json:"path" jsonschema:"the file, resolved"`
	Applied bool   `json:"applied" jsonschema:"whether the file was written; false for an empty session, which has nothing to save"`
	Content string `json:"content,omitempty" jsonschema:"without apply: the file's bytes, in the format the extension names"`
	Text    string `json:"text" jsonschema:"what %save printed: notes, warnings, and what was or would be saved"`
}

// ClearResult is what a reset took with it.
type ClearResult struct {
	Notices []string `json:"notices" jsonschema:"what the reset discarded: objects held, debugging sessions"`
}

// CommandInput is one prompt command line.
type CommandInput struct {
	SessionName
	Line string `json:"line" jsonschema:"one meta command line as typed at the sysml prompt, starting with %, e.g. %list, %help, %instantiate Demo::car, %features car all"`
}

// CommandResult is what a command printed.
type CommandResult struct {
	Output string `json:"output" jsonschema:"the lines the command printed"`
	Quit   bool   `json:"quit,omitempty" jsonschema:"the line was %quit, which ends a terminal and is only noted here"`
}

// Register adds the session tools to s over m.
func Register(s *mcp.Server, m *Manager) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_open",
		Description: "Open a named REPL session: an empty model that declarations, instances and runs accumulate into across calls. The default session opens itself on first use, so this is only needed for a second, independent session, or for one with a root: a directory under the workspace root whose model files load on demand, as %load would load them, when a submission imports or qualifies a root namespace that neither the session nor the library declares.",
		Annotations: toolkit.Mutating("Open session", false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in SessionOpenInput) (*mcp.CallToolResult, SessionOpened, error) {
		name, err := m.OpenIn(in.Session, in.Root)
		return nil, SessionOpened{Session: name}, err
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_close",
		Description: "Close a REPL session, discarding its model, instances and any run in progress.",
		Annotations: toolkit.Mutating("Close session", false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in SessionName) (*mcp.CallToolResult, SessionOpened, error) {
		name := in.Session
		if name == "" {
			name = DefaultSession
		}
		return nil, SessionOpened{Session: name}, m.Close(in.Session)
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_list",
		Description: "List the open REPL sessions.",
		Annotations: toolkit.ReadOnly("List sessions"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ SessionListInput) (*mcp.CallToolResult, SessionList, error) {
		return nil, SessionList{Sessions: m.Names()}, nil
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_load",
		Description: "Load model files into a session, as %load does: files, directories or globs, resolved against the workspace root and refused outside it. Every file is accepted before the model is analyzed, so load order does not matter, and files beside the paths that declare a namespace they import load too. Returns what loaded and the diagnostics.",
		Annotations: toolkit.Mutating("Load files", true),
	}, handler(m, func(_ context.Context, sess *repl.Session, in LoadInput) (LoadResult, error) {
		if len(in.Paths) == 0 {
			return LoadResult{}, errors.New("paths is required: at least one file, directory or glob")
		}
		paths := make([]string, 0, len(in.Paths))
		for _, p := range in.Paths {
			resolved, err := m.Resolve(p)
			if err != nil {
				return LoadResult{}, err
			}
			paths = append(paths, resolved)
		}
		files, err := sess.ExpandPaths(paths)
		if err != nil {
			return LoadResult{}, err
		}
		report, err := sess.LoadPathsReport(paths)
		if err != nil {
			return LoadResult{}, err
		}
		lines := append(append(append([]string{}, report.Loaded...), report.Found...), report.Declared...)
		return LoadResult{
			Files:       files,
			Declared:    orEmpty(report.Declared),
			Errors:      report.Errors,
			Diagnostics: diagnostics(sess.LocatedDiagnostics()),
			Text:        text(lines),
		}, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_submit",
		Description: "Submit SysML declarations to a session, as typing them at the prompt does: the text joins the model, the whole model is re-analyzed, and what it declared and the diagnostics about it come back. Submitting a name again replaces its earlier declaration. For an expression use session_eval; for a % command use repl_command.",
		Annotations: toolkit.Mutating("Submit declarations", false),
	}, handler(m, func(_ context.Context, sess *repl.Session, in SubmitInput) (SubmitResult, error) {
		if strings.TrimSpace(in.Text) == "" {
			return SubmitResult{}, errors.New("text is required")
		}
		res := sess.Submit(in.Text)
		verbosity := sess.Verbosity()
		out := SubmitResult{
			Accepted:    !res.Rejected(),
			Declared:    orEmpty(res.Declared),
			Notices:     orEmpty(res.Notices),
			Diagnostics: diagnostics(sess.Locate(res, res.OwnDiagnostics(verbosity))),
			Text:        text(repl.RenderResult(res, verbosity)),
		}
		if res.Refused != nil {
			return out, failed(res.Refused, nil)
		}
		return out, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_eval",
		Description: "Evaluate an expression in a session, as %eval does: a literal, a declared feature or qualified name, an enumeration literal, a unit, or a compound expression over the session's declarations. Returns the value as data and as the prompt prints it.",
		Annotations: toolkit.ReadOnly("Evaluate expression"),
	}, handler(m, func(_ context.Context, sess *repl.Session, in EvalInput) (EvalResult, error) {
		if strings.TrimSpace(in.Expression) == "" {
			return EvalResult{}, errors.New("expression is required")
		}
		ev, err := sess.Evaluate(in.Expression)
		if err != nil {
			return EvalResult{}, err
		}
		value, err := valueJSON(ev)
		if err != nil {
			return EvalResult{}, err
		}
		return EvalResult{Expression: in.Expression, Value: value, Text: ev.Text, Output: text(ev.Lines)}, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_print",
		Description: "Print a session's model as SysML notation, or one named element with its body, as %print does. Comments are kept, and what is printed can be submitted back.",
		Annotations: toolkit.ReadOnly("Print notation"),
	}, handler(m, func(_ context.Context, sess *repl.Session, in PrintInput) (Notation, error) {
		lines, err := sess.Print(in.Name)
		if err != nil {
			return Notation{}, failed(err, lines)
		}
		return Notation{Text: text(lines)}, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_save",
		Description: "Write a session's model to a file under the workspace root, as %save does: .sysml notation with comments preserved, or .ttl/.json RDF (experimental). A path outside the root is refused. Without apply nothing is written and the file's content, in the format the extension names, is returned as content.",
		Annotations: toolkit.Mutating("Save model", true),
	}, handler(m, func(_ context.Context, sess *repl.Session, in SaveInput) (SaveResult, error) {
		path, err := m.Resolve(in.Path)
		if err != nil {
			return SaveResult{}, err
		}
		if !in.Apply {
			content, lines, err := sess.SavePreview(path)
			if err != nil {
				return SaveResult{Path: path}, failed(err, lines)
			}
			return SaveResult{Path: path, Content: string(content), Text: text(lines)}, nil
		}
		lines, written, err := sess.Save(path)
		if err != nil {
			return SaveResult{Path: path}, failed(err, lines)
		}
		return SaveResult{Path: path, Applied: written, Text: text(lines)}, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_clear",
		Description: "Reset a session, as %clear does: every declaration, object and debugging session goes; the session stays open and empty.",
		Annotations: toolkit.Mutating("Clear session", true),
	}, handler(m, func(_ context.Context, sess *repl.Session, _ SessionName) (ClearResult, error) {
		return ClearResult{Notices: orEmpty(sess.Clear())}, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name: "repl_command",
		Description: "Run one sysml prompt command (a % line) in a session and return what it printed. Prefer the typed tools, which return the same data structured: session_load/session_submit/session_eval/session_print/session_save/session_clear, session_run, debug_*, send, advance, features, instances, check/explain/solve/configure/optimize, sweep, runs, render_run. " +
			"This tool reaches everything else the prompt can do — %instantiate, %invoke, %constraint, %requirement, %satisfy, %validate, %calc, %analysis, %record, %import, %run-query, %render, %viz, %render-document, %trace, %schedule, %seed, %engine, %jobs, %budget, %verbosity, %strict, %lint, %search, %views, %documents, %list, the %check-* settings and the repository commands. " +
			"Discover the commands with `%help` (every command with its arguments) and the settings with the command alone (`%schedule`, `%engine`). A command that reports an error fails the call with its output.",
		Annotations: toolkit.Mutating("Run prompt command", false),
	}, handler(m, func(_ context.Context, sess *repl.Session, in CommandInput) (CommandResult, error) {
		line := strings.TrimSpace(in.Line)
		if line == "" {
			return CommandResult{}, errors.New("line is required: a % command, see %help")
		}
		if !repl.IsMeta(line) {
			return CommandResult{}, fmt.Errorf("%q is no %% command; declarations go to session_submit and expressions to session_eval", line)
		}
		lines, quit, err := sess.RunMeta(line)
		if fields := repl.MetaArgs(line); len(fields) > 0 && !repl.KnownMeta(fields[0]) {
			return CommandResult{}, failed(fmt.Errorf("%q is no command this build serves", fields[0]), lines)
		}
		if err != nil {
			var usage *repl.UsageError
			if errors.As(err, &usage) {
				return CommandResult{}, failed(err, usage.Lines)
			}
			return CommandResult{}, failed(err, lines)
		}
		out := CommandResult{Output: text(lines), Quit: quit}
		if quit {
			out.Output = text(append(lines, "a session is closed with session_close"))
		}
		for _, l := range lines {
			if strings.HasPrefix(l, "error: ") {
				return out, failed(errors.New(strings.TrimPrefix(l, "error: ")), lines)
			}
		}
		return out, nil
	}))
	registerRuns(s, m)
	registerDebug(s, m)
	registerSolve(s, m)
}

// orEmpty is lines as a JSON array that is never null.
func orEmpty(lines []string) []string {
	if lines == nil {
		return []string{}
	}
	return lines
}
