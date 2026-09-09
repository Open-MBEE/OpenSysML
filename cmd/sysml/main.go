// Command sysml is an interactive SysML v2 REPL (spec §13).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/chzyer/readline"

	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/core/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/repl"
	"github.com/Open-MBEE/OpenSysML/internal/usage"
)

// errPrefix names the tool in the messages it writes to stderr.
const errPrefix = "sysml:"

var (
	// Version information - set via ldflags during build
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	GoVersion = "unknown"
)

type rlReader struct{ rl *readline.Instance }

func (r *rlReader) ReadLine(prompt string) (string, error) {
	r.rl.SetPrompt(prompt)
	line, err := r.rl.Readline()
	if err == readline.ErrInterrupt { // Ctrl-C clears line (continue REPL)
		return "", nil
	}
	if err == io.EOF { // Ctrl-D exits REPL
		return "", io.EOF
	}
	return line, err
}

// sessionCompleter completes prompt input from the session: meta commands,
// declared and library names, and file paths after %load and %save.
type sessionCompleter struct{ sess *repl.Session }

// Do answers a tab press with the remainder of each candidate, as readline's
// AutoCompleter expects, and how many runes of the word were already typed.
func (c *sessionCompleter) Do(line []rune, pos int) ([][]rune, int) {
	if pos < 0 || pos > len(line) {
		pos = len(line)
	}
	comp := c.sess.Complete(string(line), len(string(line[:pos])))
	out := make([][]rune, 0, len(comp.Candidates))
	for _, cand := range comp.Candidates {
		out = append(out, []rune(strings.TrimPrefix(cand, comp.Prefix)))
	}
	return out, utf8.RuneCountInString(comp.Prefix)
}

// historyPath returns the file the prompt keeps its history in:
// $XDG_STATE_HOME/sysml/history when that is set, and ~/.sysml_history
// otherwise. It returns "" when neither can be written, which leaves the
// history in memory for the session rather than failing the prompt.
func historyPath() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		if path, ok := writableFile(filepath.Join(dir, "sysml"), "history"); ok {
			return path
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path, ok := writableFile(home, ".sysml_history")
	if !ok {
		return ""
	}
	return path
}

// writableFile returns the path of name in dir, creating dir and confirming the
// file can be appended to.
func writableFile(dir, name string) (string, bool) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false
	}
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", false
	}
	if f.Close() != nil {
		return "", false
	}
	return path, true
}

// CLI flags
var (
	evalExprs       stringSlice
	showHelp        bool
	showMan         bool
	showVersion     bool
	debugMode       bool
	quietMode       bool
	traceMode       bool
	schedule        schedulePolicy
	convertFormat   string
	queryText       string
	outputPath      string
	fromFormat      string
	migrationReport string
	renderView      string
	renderAllDir    string
	renderForm      string
	renderDoc       string
	renderDocsDir   string
	docForm         string
	pdfEngine       string
	pdfTitlePage    bool
	pdfTOC          bool
	pdfNumbering    bool
	htmlCSS         stringSlice
	htmlNoCSS       bool
	htmlShowCSS     bool
	htmlFragment    bool
	htmlMermaid     string
	htmlTheme       string
	strictMode      bool
	modelChecks     checks
	compileCalc     string
	compileTarget   string
	compileSource   bool

	syncDiffWith       string
	syncApplyTo        string
	syncBase           string
	syncState          string
	syncConfirmDeletes bool
	syncMintIDs        bool
	syncAnnotate       string
)

// budgets holds the run bounds the environment resolves to, read once at startup.
var budgets = runtime.DefaultBudgets()

// schedulePolicy is -schedule as written: the policy every run resolves its
// choice points under, rejected where it is parsed so a misspelling is reported
// at startup rather than run under the default.
type schedulePolicy struct {
	value runtime.SchedulePolicy
	text  string
}

func (s *schedulePolicy) String() string { return s.text }

func (s *schedulePolicy) Set(value string) error {
	policy, err := runtime.ParseSchedulePolicy(value)
	if err != nil {
		return err
	}
	s.value, s.text = policy, value
	return nil
}

// stringSlice is a custom flag type for multiple values
type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	os.Exit(runCLI())
}

// flagGiven reports whether the run named this flag, which an empty value
// cannot be told apart from otherwise.
func flagGiven(name string) bool {
	given := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			given = true
		}
	})
	return given
}

// printUsage writes the help to w: the caller chooses the stream, since help
// asked for is a result and help shown over a misuse belongs with the error.
func printUsage(w io.Writer) {
	doc().WriteText(w, flag.CommandLine)
}

// printMan writes the command's manual page, rendered from the same description
// the help is, so the shipped page cannot document a different command.
func printMan(w io.Writer) {
	doc().WriteRoff(w, flag.CommandLine, usage.DefaultManMeta())
}

// runCLI carries out what the command line asked for and returns the exit
// status, so a profile started for the run is written before the process exits.
func runCLI() int {
	// Usage shown over a misuse goes on the stream the error naming it goes on.
	flag.Usage = func() { printUsage(flag.CommandLine.Output()) }

	registerFlags(flag.CommandLine)
	if err := flag.CommandLine.Parse(permuteArgs(flag.CommandLine, os.Args[1:])); err != nil {
		// flag.CommandLine exits on error; unreachable unless that changes.
		return 2
	}

	// Help that was asked for is the result of the run: it belongs on stdout, where
	// it can be piped, and the run did what was asked.
	if showHelp {
		printUsage(os.Stdout)
		return exitHolds
	}

	// The page asked for is the result of the run, like the help.
	if showMan {
		printMan(os.Stdout)
		return exitHolds
	}

	if debugMode && quietMode {
		fmt.Fprintln(os.Stderr, "sysml: -debug and -quiet are mutually exclusive")
		return 2
	}

	stopProfiling, err := startProfiling()
	if err != nil {
		fmt.Fprintln(os.Stderr, errPrefix, err)
		return 2
	}
	defer stopProfiling()

	// Handle version flag
	if showVersion {
		fmt.Printf("sysml %s\n", Version)
		fmt.Printf("  Commit:     %s\n", Commit)
		fmt.Printf("  Build time: %s\n", BuildTime)
		fmt.Printf("  Go version: %s\n", GoVersion)
		return 0
	}

	// A mode asked for with an empty value is a misuse, not an absent flag: it
	// would otherwise silently run the REPL instead of the query.
	if flagGiven("query") && queryText == "" {
		fmt.Fprintln(os.Stderr, `sysml: -query is empty; give it OSLC Query text, as -query 'sysml:name="battery"'`)
		return 2
	}
	if flagGiven("html-theme") && htmlTheme == "" {
		fmt.Fprintln(os.Stderr, "sysml: -html-theme is empty; name one of the themes: "+strings.Join(docrender.Themes(), ", "))
		return 2
	}
	if flagGiven("html-mermaid") && htmlMermaid == "" {
		fmt.Fprintln(os.Stderr, "sysml: -html-mermaid is empty; give it cdn or the URL of a Mermaid script")
		return 2
	}

	// Get positional arguments (files to load)
	args := flag.Args()

	if renderForm != "" && renderView == "" && renderAllDir == "" {
		fmt.Fprintln(os.Stderr, "sysml: -render-form is the form -render or -render-all writes; name the view to render with -render or a directory with -render-all")
		return 2
	}

	// The default stylesheet is asked for on its own; it needs no model, and
	// writing it is the whole run, so it cannot stand in for another.
	if htmlShowCSS {
		switch {
		case len(args) > 0:
			fmt.Fprintln(os.Stderr, "sysml: -html-default-css writes the default stylesheet, which no model shapes; ask for it without model files")
			return 2
		case renderDoc != "" || renderDocsDir != "" || renderView != "" || renderAllDir != "" ||
			convertFormat != "" || flagGiven("sync-diff") || flagGiven("sync-apply") ||
			queryText != "" || len(evalExprs) > 0 || modelChecks.requested():
			fmt.Fprintln(os.Stderr, "sysml: -html-default-css writes the default stylesheet and nothing else; ask for it in its own run")
			return 2
		case docForm != "" || pdfEngine != "" || pdfTitlePage || pdfTOC || pdfNumbering || htmlPageFlagsGiven():
			fmt.Fprintln(os.Stderr, "sysml: -html-default-css writes the default stylesheet itself; the document and stylesheet options shape a rendered document, not the sheet")
			return 2
		case fromFormat != "" || strictMode || syncBase != "" || syncState != "" ||
			syncConfirmDeletes || syncMintIDs || syncAnnotate != "":
			fmt.Fprintln(os.Stderr, "sysml: -html-default-css writes the default stylesheet, which reads no input; the input and repository options do not apply")
			return 2
		}
		if err := runDefaultStylesheet(); err != nil {
			return fail(err)
		}
		return exitHolds
	}

	if renderDoc == "" && renderDocsDir == "" &&
		(docForm != "" || pdfEngine != "" || pdfTitlePage || pdfTOC || pdfNumbering || htmlFlagsGiven()) {
		fmt.Fprintln(os.Stderr, "sysml: -doc-form, the document options and the stylesheet options apply to -render-document and -render-documents; name the document to render")
		return 2
	}

	if flagGiven("sync-diff") && syncDiffWith == "" {
		fmt.Fprintln(os.Stderr, "sysml: -sync-diff is empty; name the repository graph or endpoint to diff against")
		return 2
	}
	if flagGiven("sync-apply") && syncApplyTo == "" {
		fmt.Fprintln(os.Stderr, "sysml: -sync-apply is empty; name the SysML v2 API endpoint to apply to")
		return 2
	}
	if flagGiven("compile") && compileCalc == "" {
		fmt.Fprintln(os.Stderr, "sysml: -compile is empty; name the calc def to compile, as -compile Pkg::Fib")
		return 2
	}
	if compileCalc == "" && (flagGiven("target") || compileSource) {
		fmt.Fprintln(os.Stderr, "sysml: -target and -source apply to -compile; name the calc def to compile")
		return 2
	}
	if migrationReport != "" && convertFormat == "" {
		fmt.Fprintln(os.Stderr, "sysml: -migration-report accompanies -convert of a SysML v1 model; write `sysml model.xmi -convert sysml -migration-report report.txt`")
		return 2
	}

	if compileCalc != "" {
		switch {
		case convertFormat != "" || renderView != "" || renderAllDir != "" || renderDoc != "" || renderDocsDir != "" || queryText != "" || len(evalExprs) > 0 || fromFormat != "" || syncDiffWith != "" || syncApplyTo != "":
			fmt.Fprintln(os.Stderr, "sysml: -compile builds an executable; it cannot be combined with -convert, -render, -render-all, -render-document, -render-documents, -query, -eval, -from, -sync-diff or -sync-apply")
			return 2
		case syncBase != "" || syncState != "" || syncConfirmDeletes || syncMintIDs || syncAnnotate != "":
			fmt.Fprintln(os.Stderr, "sysml: -sync-base, -sync-state, -sync-confirm-deletes, -sync-mint-ids and -sync-annotate apply to -sync-diff or -sync-apply, not to -compile")
			return 2
		case modelChecks.requested():
			return refuse(modelChecks,
				"-compile builds an executable and decides nothing about the model; check it in its own run")
		case outputPath == "":
			fmt.Fprintln(os.Stderr, "sysml: -compile needs -o to name the executable (or the source file, with -source)")
			return 2
		}
		if err := runCompile(args); err != nil {
			return fail(err)
		}
		return exitHolds
	}

	if syncDiffWith != "" && syncApplyTo != "" {
		fmt.Fprintln(os.Stderr, "sysml: -sync-diff shows a change set without writing and -sync-apply writes one; ask for one per run")
		return 2
	}
	if syncDiffWith != "" || syncApplyTo != "" {
		mode := "-sync-diff"
		if syncApplyTo != "" {
			mode = "-sync-apply"
		}
		switch {
		case convertFormat != "" || renderView != "" || renderDoc != "" || renderAllDir != "" || renderDocsDir != "" || queryText != "" || len(evalExprs) > 0:
			fmt.Fprintf(os.Stderr, "sysml: %s syncs a change set; it cannot be combined with -convert, -render, -render-all, -render-document, -render-documents, -query or -eval\n", mode)
			return 2
		case outputPath != "" || fromFormat != "" || renderForm != "" || docForm != "" || pdfEngine != "" || pdfTitlePage || pdfTOC || pdfNumbering:
			fmt.Fprintf(os.Stderr, "sysml: %s reads SysML or Turtle inputs and reports the change set; -output, -from and the render options do not apply\n", mode)
			return 2
		case modelChecks.requested():
			return refuse(modelChecks,
				mode+" syncs a change set and decides nothing else about the model; check it in its own run")
		}
		if syncApplyTo != "" {
			return runSyncApply(args)
		}
		return runSyncDiff(args)
	}
	if syncBase != "" || syncState != "" || syncConfirmDeletes || syncMintIDs || syncAnnotate != "" {
		fmt.Fprintln(os.Stderr, "sysml: -sync-base, -sync-state, -sync-confirm-deletes, -sync-mint-ids and -sync-annotate apply to -sync-diff or -sync-apply; name the repository to sync against")
		return 2
	}

	if renderDocsDir != "" {
		switch {
		case htmlFragment:
			fmt.Fprintln(os.Stderr, "sysml: -render-documents writes whole pages linking to one another; -html-fragment writes one document element to embed, so it applies to -render-document")
			return 2
		case renderDoc != "":
			fmt.Fprintln(os.Stderr, "sysml: -render-documents renders every document; -render-document renders one; ask for one per run")
			return 2
		case renderView != "" || renderAllDir != "" || convertFormat != "":
			fmt.Fprintln(os.Stderr, "sysml: -render-documents, -render, -render-all and -convert each write documents out; ask for one per run")
			return 2
		case outputPath != "":
			fmt.Fprintln(os.Stderr, "sysml: -render-documents writes into its directory and cannot be combined with -output")
			return 2
		case queryText != "" || len(evalExprs) > 0 || fromFormat != "":
			fmt.Fprintln(os.Stderr, "sysml: -render-documents cannot be combined with -query, -eval or -from")
			return 2
		case modelChecks.requested():
			return refuse(modelChecks,
				"-render-documents writes documents out and decides nothing about the model; check it in its own run")
		}
		if err := runRenderDocuments(args); err != nil {
			return fail(err)
		}
		return exitHolds
	}

	if renderAllDir != "" {
		switch {
		case renderView != "":
			fmt.Fprintln(os.Stderr, "sysml: -render-all and -render are mutually exclusive")
			return 2
		case outputPath != "":
			fmt.Fprintln(os.Stderr, "sysml: -render-all writes into its directory and cannot be combined with -output")
			return 2
		case convertFormat != "":
			fmt.Fprintln(os.Stderr, "sysml: -render-all and -convert each write documents out; ask for one per run")
			return 2
		case queryText != "" || len(evalExprs) > 0 || fromFormat != "" || renderDoc != "":
			fmt.Fprintln(os.Stderr, "sysml: -render-all cannot be combined with -query, -eval, -from or -render-document")
			return 2
		case modelChecks.requested():
			return refuse(modelChecks,
				"-render-all writes views out and decides nothing about the model; check it in its own run")
		}
		if err := runRenderAll(args); err != nil {
			return fail(err)
		}
		return exitHolds
	}

	if convertFormat != "" {
		if queryText != "" {
			fmt.Fprintln(os.Stderr, "sysml: -convert and -query are mutually exclusive")
			return 2
		}
		if modelChecks.requested() {
			return refuse(modelChecks,
				"-convert writes the model out and decides nothing about it; check it in its own run")
		}
		if renderView != "" || renderDoc != "" {
			fmt.Fprintln(os.Stderr, "sysml: -convert, -render and -render-document each write a document out; ask for one per run")
			return 2
		}
		if err := runConvert(args); err != nil {
			return fail(err)
		}
		return exitHolds
	}

	for _, path := range args {
		if f, err := export.FormatOfPath(path); err == nil && f == export.FormatXMI {
			fmt.Fprintf(os.Stderr, "sysml: %s is a SysML v1 model; migrate it first with `sysml %s -convert sysml -output model.sysml`, then load model.sysml\n", path, path)
			return 2
		}
	}

	if queryText != "" {
		if modelChecks.requested() || renderView != "" || renderDoc != "" || len(evalExprs) > 0 || outputPath != "" || fromFormat != "" {
			fmt.Fprintln(os.Stderr, "sysml: -query cannot be combined with checks, -eval, -render, -render-document, -output or -from")
			return 2
		}
		return runQuery(args, queryText)
	}

	if renderView != "" {
		if modelChecks.requested() {
			return refuse(modelChecks,
				"-render writes a view out and decides nothing about the model; check it in its own run")
		}
		if renderDoc != "" {
			fmt.Fprintln(os.Stderr, "sysml: -render and -render-document each write a document out; ask for one per run")
			return 2
		}
		if err := runRender(args); err != nil {
			return fail(err)
		}
		return exitHolds
	}

	if renderDoc != "" {
		switch {
		case modelChecks.jsonOut && !modelChecks.checksOnly():
			fmt.Fprintln(os.Stderr, "sysml: -render-document writes a document, not JSON; -json reports checks")
			return 2
		case modelChecks.requested():
			return refuse(modelChecks,
				"-render-document writes a document out and decides nothing about the model; check it in its own run")
		case len(evalExprs) > 0 || fromFormat != "":
			fmt.Fprintln(os.Stderr, "sysml: -render-document cannot be combined with -eval or -from")
			return 2
		}
		if err := runRenderDocument(args); err != nil {
			return fail(err)
		}
		return exitHolds
	}

	// Resolve the run bounds before any model runs, so a bad value is reported at
	// startup rather than mistaken for the default at execution time. Reporting the
	// version and converting a model evaluate nothing, so they are handled above.
	budgets, err = runtime.BudgetsFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, errPrefix, err)
		return 2
	}

	// Checking mode: load, check what was named, and exit on the verdict.
	if modelChecks.requested() {
		return runChecks(args, evalExprs, modelChecks)
	}

	// Non-interactive mode: evaluate the expressions against the model and exit.
	if len(evalExprs) > 0 {
		return runNonInteractive(args, evalExprs)
	}

	// No expression to evaluate: load whatever was named and take lines.
	return runInteractiveWithFiles(args)
}

// newSession returns a session in the output modes the flags asked for, under
// the run bounds resolved at startup.
func newSession() *repl.Session {
	sess := repl.NewSession()
	if err := sess.SetBudgets(budgets); err != nil {
		// Unreachable: budgets are validated in main before any session exists.
		fmt.Fprintln(os.Stderr, errPrefix, err)
		os.Exit(2)
	}
	switch {
	case debugMode:
		sess.SetVerbosity(repl.VerbosityDebug)
	case quietMode:
		sess.SetVerbosity(repl.VerbosityQuiet)
	}
	sess.SetTracing(traceMode)
	if err := sess.SetSchedule(schedule.value); err != nil {
		fmt.Fprintln(os.Stderr, errPrefix, err)
		os.Exit(2)
	}
	sess.SetConformanceMode(conformance.ModeOf(strictMode))
	sess.SetRenderWidth(terminalWidth())
	return sess
}

// runInteractiveWithFiles loads what was named and takes lines from the prompt.
// A model that did not analyse, or a feature value a command could not materialize, leaves
// the status of a run that decided nothing, except at a terminal, where the
// prompt that opens is where it gets fixed.
func runInteractiveWithFiles(files []string) int {
	sess := newSession()
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "sysml> ",
		HistoryFile:     historyPath(),
		AutoComplete:    &sessionCompleter{sess: sess},
		InterruptPrompt: "^C",
		EOFPrompt:       "bye",
	})
	if err != nil {
		return fail(err)
	}
	defer rl.Close()

	loaded, err := loadFiles(sess, files)
	if err != nil {
		return fail(err)
	}
	terminal := atTerminal()

	fmt.Println("SysML v2 REPL — %help for commands, Ctrl-D to exit")
	if err := repl.Loop(&rlReader{rl: rl}, os.Stdout, sess); err != nil {
		return fail(err)
	}
	return sessionStatus(loaded, terminal, sess.MaterializationFailures())
}

// sessionStatus is the status a prompt session leaves: at a terminal the session
// is where an unusable model gets fixed, so it decides nothing, and otherwise the
// run is undecided if the model did not analyse or a command reported a feature value it
// could not materialize.
func sessionStatus(loaded int, terminal bool, materializeFailures []error) int {
	if terminal {
		return exitHolds
	}
	if len(materializeFailures) > 0 {
		return exitUnevaluable
	}
	return loaded
}

// loadFiles submits every positional path — a file, a directory to walk or a
// glob — as a single load, so a declaration in one file resolves against the
// others whichever order they were named in. What the load declared is reported
// on stdout and what the analysis found on stderr; the status is that of a run
// that decided nothing when the model did not analyse cleanly.
func loadFiles(sess *repl.Session, files []string) (int, error) {
	if len(files) == 0 {
		return exitHolds, nil
	}
	report, err := sess.LoadPathsReport(files)
	if err != nil {
		// Returned unwrapped: the caller reports it, so the operation and the
		// path are named once.
		return exitUnevaluable, err
	}
	writeLines(os.Stdout, report.Loaded)
	writeLines(os.Stderr, report.Found)
	if report.Errors {
		fmt.Fprintf(os.Stderr, "sysml: %s did not analyse cleanly\n", namedModels(files))
		return exitUnevaluable, nil
	}
	writeLines(os.Stdout, report.Declared)
	return exitHolds, nil
}

// runNonInteractive loads the model and evaluates every expression asked for,
// reporting the values on stdout and anything that stopped it on stderr.
func runNonInteractive(files, exprs []string) int {
	sess := newSession()
	status, err := loadFiles(sess, files)
	if err != nil {
		return fail(err)
	}
	if status != exitHolds {
		return status
	}

	for _, expr := range exprs {
		output, err := sess.EvalExpr(expr)
		if err != nil {
			return fail(err)
		}
		writeLines(os.Stdout, output)
	}
	return exitHolds
}
