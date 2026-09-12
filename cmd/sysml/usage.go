package main

import (
	"flag"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/interop/flexo"
	"github.com/Open-MBEE/OpenSysML/internal/usage"
)

// doc describes the command for both the terminal help and the man page, so a
// mode documented for one is documented for the other.
func doc() usage.Doc {
	return usage.Doc{
		Command:    "sysml",
		ManSection: 1,
		Summary:    "run, check, convert and render SysML v2 and KerML models",
		Synopsis:   []string{"[options] [file...]"},
		Description: []string{
			"sysml loads the models it is given — a file, a directory to walk or a " +
				"glob — as a single model, so a declaration in one file resolves " +
				"against the others whichever order they were named in. With no " +
				"expression or check to carry out it opens an interactive prompt; " +
				"otherwise it does what was asked and exits on the verdict, which " +
				"is what lets a run gate a build.",
		},
		Sections: []usage.Section{{
			Title: "Examples",
			Examples: []usage.Example{
				usage.Ex("sysml", "Start interactive REPL"),
				usage.Ex(`sysml -e "5 + 3"`, "Evaluate and exit"),
				usage.Ex(`sysml -e "expr" file.sysml`, "Load file, evaluate, and exit"),
				usage.Ex("sysml file.sysml", "Load file and start REPL"),
				usage.Ex("sysml -debug file.sysml", "Load file, reporting every diagnostic"),
				usage.Ex("sysml -trace file.sysml", "Load file, reporting each execution step"),
			},
		}, {
			Title: "Checking a model",
			Examples: []usage.Example{
				usage.Ex("sysml -constraint MassBudget model.sysml", "Evaluate one constraint and exit"),
				usage.Ex("sysml -requirement PowerMargin model.sysml", "Evaluate one requirement and exit"),
				usage.Ex("sysml -satisfy model.sysml", "Evaluate every satisfaction assertion"),
				usage.Ex("sysml -satisfy=Ctx model.sysml", "...only the ones Ctx states"),
				usage.Ex("sysml -instantiate p -constraint C model.sysml", "Check C against an object of p"),
				usage.Ex("sysml -validate model.sysml", "Report diagnostics only"),
				usage.Ex("sysml -validate -strict model.sysml", "...asking whether it is conforming SysML v2"),
				usage.Ex(`sysml -calc "Fall(3, 4)" model.sysml`, "Invoke a calculation"),
				usage.Ex("sysml -analysis shipCost model.sysml", "Run an analysis case"),
				usage.Ex(`sysml -analysis "CostAnalysis ship" model.sysml`, "...on an object as its subject"),
				usage.Ex("sysml -analysis speedCheck model.sysml", "Run a verification case for its verdict"),
				usage.Ex(`sysml -run-query "Heavy root=scope" model.sysml`, "Execute a document query"),
				usage.Ex("sysml -action Drive model.sysml", "Run an action to completion"),
				usage.Ex("sysml -state Mission -advance 10 model.sysml", "Run a state machine for 10 time units"),
				usage.Ex("sysml -action Ping -state Sm -advance 5 m.sysml", "...an action and a machine on one clock"),
				usage.Ex("sysml -schedule explore -action Drive m.sysml", "Run every linearization; table the outcomes"),
				usage.Ex("sysml -satisfy -json model.sysml", "Report the verdicts as JSON"),
			},
			Paragraphs: []string{"Each check flag may be repeated."},
		}, {
			Title: "Analysis engines",
			Examples: []usage.Example{
				usage.Ex("sysml -engines", "List the engines, their authority and status"),
				usage.Ex("sysml -engine explore -action Drive m.sysml", "The same as -schedule explore"),
				usage.Ex("sysml -engine all -requirement R m.sysml", "Every engine that covers the question"),
				usage.Ex("sysml -engine run -json -constraint C m.sysml", "One engine; the plan in the report"),
			},
			Paragraphs: []string{
				"Every check is a question put to an analysis engine: run executes the " +
					"model once under -schedule, explore runs every linearization, sweep " +
					"runs the rows of -sweep, and solve puts condition sets to an SMT " +
					"solver. -engine auto (the default) picks the engine of highest " +
					"authority that covers the question and falls back to the next when " +
					"it refuses or covers nothing; -engine <name> puts the question to " +
					"that engine alone, and its refusal is the answer; -engine all puts " +
					"it to every engine that covers it, one after another in name order, " +
					"and composes their answers: a witnessed violation stands over any " +
					"universal claim, and a contradiction is reported as a disagreement " +
					"decided in the interpreter's favor.",
				"Every verdict is followed by its standing: the claim, the strength of " +
					"the evidence — observed (one run), witnessed (a replayed execution), " +
					"bounded (every case within a budget), proved (every case) — and what " +
					"earned it, as `holds (observed: 1 run under reverse)`. A budget the " +
					"engine reached lowers the strength and is named. Under -json each " +
					"check carries the plan and one results[] entry per engine that " +
					"answered, with its engine, claim, strength, bounds and witness.",
			},
		}, {
			Title: "Sweeping and sampling a parameter",
			Examples: []usage.Example{
				usage.Ex(`sysml -calc Twice -sweep "n=1..8:2" m.sysml`, "One run per range value"),
				usage.Ex(`sysml -analysis C -sweep "m=1..3" -sweep "v=1..2" m.sysml`, "Every combination of the two"),
				usage.Ex(`sysml -analysis C -sweep "m=1..9" -samples 20 -seed 7 m.sysml`, "20 values drawn uniformly"),
				usage.Ex(`sysml -analysis C -sweep "m=1..3" -json m.sysml`, "Report the rows as JSON"),
			},
			Paragraphs: []string{
				"A sweep is orchestration over the ordinary run: each row is one " +
					"-analysis or -calc invocation with the swept parameter bound to " +
					"that row's value and every other argument as given, so nothing " +
					"about how a case executes changes. Name a single -analysis or " +
					"-calc; a parameter the case does not declare, one the invocation " +
					"already binds, a step of zero or one pointing away from the end " +
					"of the range is an error rather than a silent row.",
				"An endpoint and a step carry the syntax and the units an argument " +
					`carries ("mass=0.0 [SI::kg]..9.0 [SI::kg]:3.0 [SI::kg]"), and ` +
					"the end of the range is included where the step lands on it.",
				"The values are produced in the swept parameter's declared type, not " +
					"the literals': a Real parameter swept over 1..4:1 is bound to 1.0, " +
					"2.0, 3.0, 4.0 and an Integer one swept over 1.0..3.0:1.0 to 1, 2, 3, " +
					"while an endpoint or step no Integer parameter can take (:0.5) is " +
					"refused before any run, as is a range over a Boolean, String, " +
					"enumeration or non-scalar parameter. A parameter declaring no type " +
					"takes the range as written, and the table says so.",
				"A range between whole numbers steps by one where no :<step> is written; " +
					"one with a fractional endpoint needs one. A range read as reals takes " +
					"an Integer endpoint or step only where a Real holds it without rounding, " +
					"and steps only where the reals tell its rows apart. Rows come out in the order the " +
					"ranges were given, the first varying slowest, and a run that " +
					"failed is a row carrying its error rather than the end of the " +
					"table.",
				"-samples draws that many values uniformly from each range instead of " +
					"stepping through it — Integers inclusively for a parameter taking " +
					"Integers, reals in [<from>, <to>) for one taking reals — and needs " +
					"-seed: the same seed draws the same table on every platform. " +
					"Uniform is the only distribution " +
					"on offer — the bundled library states no probability " +
					"distributions — so a range must be written <from>..<to> rather " +
					"than as a named distribution.",
			},
		}, {
			Title: "Conversion",
			Examples: []usage.Example{
				usage.Ex("sysml model.sysml -convert ttl", "SysML notation to RDF Turtle, on stdout"),
				usage.Ex("sysml model.ttl -convert sysml", "RDF Turtle to SysML notation"),
				usage.Ex("sysml model.sysml -convert ttl -o m.ttl", "Write the conversion to a file"),
				usage.Ex("sysml in.txt -convert ttl -from sysml", "Name the input format explicitly"),
			},
			Paragraphs: []string{
				"The input format is taken from the file extension (.sysml, .kerml, " +
					".ttl) unless -from names it. Converting to the format it is " +
					"already in rewrites the input: notation is reformatted, Turtle " +
					"is normalized.",
				// Printed rather than restated, so the help cannot drift from what a
				// conversion reports.
				export.ExperimentalNotice,
				export.MigrationNotice,
				"Every run that converts RDF or migrates a v1 model says so on stderr. " +
					"Saving to .sysml or .kerml is stable.",
			},
		}, {
			Title: "Native compilation",
			Examples: []usage.Example{
				usage.Ex("sysml model.sysml -compile Pkg::Fib -o fib", "Compile a calc def to a C executable"),
				usage.Ex("sysml model.sysml -compile Pkg::Fib -target go -o fib", "...via Go"),
				usage.Ex("sysml model.sysml -compile Pkg::Fib -source -o fib.c", "Write the generated source only"),
			},
			Paragraphs: []string{
				"The executable takes the calc's parameters as arguments and prints " +
					"its result; it computes what sysml -calc computes, or fails with " +
					"the same error. Only the scalar subset compiles (Integer, Real, " +
					"Boolean; see docs/project/native-compilation.md).",
			},
		}, {
			Title: "Syncing against a repository",
			Lead:  []string{"A dry run unless -sync-apply:"},
			Examples: []usage.Example{
				usage.Ex("sysml model.sysml -sync-diff repo.ttl", "Show the change set and exit"),
				usage.Ex("sysml model.sysml -sync-diff repo.ttl -sync-base last-seen.ttl", ""),
				usage.Ex("sysml model.sysml -sync-diff repo.ttl -sync-confirm-deletes", ""),
				usage.Ex("sysml model.sysml -sync-diff repo.ttl -sync-mint-ids -sync-annotate out.sysml", ""),
				usage.Ex("sysml model.sysml -sync-diff http://localhost:8083", "Against the live API; no writes"),
				usage.Ex("sysml model.sysml -sync-apply http://localhost:8083", "Write the change set as a commit"),
			},
			Paragraphs: []string{
				"The diff correlates elements by their effective id — an @ElementId " +
					"annotation, or the encoded qualified name — so a rename, move or " +
					"retype is an update, never a delete plus a create. " +
					"Repository-only elements are reported as deletes but applying " +
					"them needs -sync-confirm-deletes; conflicts — a declared id the " +
					"branch no longer has, or a repository change since the last-seen " +
					"commit — exit 1 and are never resolved silently.",
				"The last-seen commit is tool state in <model>.sync.json (or " +
					"-sync-state), never written into the notation. -sync-apply " +
					"refuses a change set the dry run would have flagged, sends each " +
					"update under its retained id, and records the resulting commit; " +
					"the token comes from " + flexo.EnvToken + ".",
			},
		}, {
			Title: "Rendering a view",
			Examples: []usage.Example{
				usage.Ex("sysml model.sysml -render Views::vehicleView", "ASCII text at a terminal"),
				usage.Ex("sysml model.sysml -render Views::vehicleView -render-form markdown", ""),
				usage.Ex("sysml model.sysml -render Views::vehicleView -o view.mmd", ""),
				usage.Ex("sysml types.sysml model.sysml -render Views::vehicleView", "several files, loaded as one model"),
				usage.Ex("sysml model.sysml -render-all rendered", ""),
			},
			Paragraphs: []string{
				"The rendering is the one the view's render member states, and a " +
					"containment tree where it states none. It is tool-defined " +
					"output: SysML v2 specifies the notation, not how a tool draws " +
					"it. Notices — an empty view, an element the rendering cannot " +
					"represent — go on stderr. Every file named is loaded as one " +
					"model, so a view may expose elements a sibling file declares.",
			},
		}, {
			Title: "Rendering a document",
			Examples: []usage.Example{
				usage.Ex("sysml model.sysml -render-document Reports::MassReport", "Markdown on stdout"),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -o report.md", ""),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -doc-form pdf -o report.pdf", ""),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -doc-form html -o report.html", ""),
				usage.Ex("sysml model.sysml -render-documents rendered", "every document, linked"),
				usage.Ex("sysml model.sysml -render-documents site -doc-form html -html-css theme.css", ""),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -doc-form html -html-theme report -o report.html", "a bundled theme"),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -doc-form html -html-mermaid cdn -o report.html", "diagrams drawn in the browser"),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -doc-form pdf "+
					"-pdf-engine pandoc -doc-title-page -doc-toc -doc-number-sections -o report.pdf", ""),
				usage.Ex("sysml -html-default-css -o sysml-document.css", "the default stylesheet"),
				usage.Ex("sysml -html-default-css -html-theme modern -o modern.css", "a theme's full sheet"),
			},
			Paragraphs: []string{
				"A document is a part def specializing DocumentQueries::Document. Its " +
					"queries are bound in the model and run against it, and the " +
					"result is written as CommonMark-compatible Markdown.",
				"-doc-form html writes semantic HTML instead, carrying each element's " +
					"identity and kind, styled by a stylesheet in a cascade layer your " +
					"own CSS overrides without !important.",
				"-doc-form pdf converts that Markdown with an external converter named " +
					"by -pdf-engine — weasyprint (default), pandoc or prince — run as " +
					"a subprocess, never linked in; diagrams are pre-rendered to SVG " +
					"with mermaid-cli (mmdc). None of these tools is needed until PDF " +
					"output is asked for; scripts/download-doc-pdf-toolchain.sh " +
					"provisions pinned copies.",
				"HTML output needs nothing external and loads nothing by default: -html-theme " +
					"picks one of the bundled looks (default, modern, print, report), -html-css adds " +
					"your own stylesheets, -html-no-default-css drops the default one, " +
					"-html-fragment writes the document element alone to embed in a " +
					"page of yours, and -html-default-css writes the default sheet out " +
					"— or, with -html-theme, a theme's whole sheet — to start from. Diagrams are written as Mermaid source; -html-mermaid " +
					"cdn has the page load a pinned Mermaid release from jsDelivr so a " +
					"browser draws them, or names a URL of your own to load it from.",
			},
		}, {
			Title: "Flag order",
			Paragraphs: []string{
				"Flags may be written before or after the model they apply to. A file " +
					"named like a flag is read as a file after --, which ends the " +
					"flags: sysml -trace -- -m.sysml",
			},
		}, {
			Title: "Reading from standard input",
			Examples: []usage.Example{
				usage.Ex("cat model.sysml | sysml -validate -", "A lone - names standard input"),
				usage.Ex("cat model.sysml | sysml - -convert ttl -from sysml", ""),
			},
			Paragraphs: []string{
				"What was read from standard input is called <stdin> in diagnostics, " +
					"and a file really named \"-\" is read by naming it ./- instead.",
			},
		}, {
			Title: "Profiling a run",
			Examples: []usage.Example{
				usage.Ex("sysml -validate -memstats model.sysml", "Report what the run cost, on stderr"),
				usage.Ex("sysml -validate -memprofile heap.out model.sysml", "Write a heap profile for go tool pprof"),
				usage.Ex("sysml -validate -cpuprofile cpu.out model.sysml", "Write a CPU profile for go tool pprof"),
			},
		}, {
			Title: "Exit status",
			Lead:  []string{"Every run that is not a prompt exits:"},
			Items: []usage.Item{
				usage.Entry("0", "It did what was asked."),
				usage.Entry("1", "The model answered false for a check."),
				usage.Entry("2", "What was asked could not be carried out at all — an unreadable "+
					"file, a model that did not analyse cleanly, an unresolved name, a "+
					"failed conversion."),
			},
		}, {
			Title: "Output streams",
			Paragraphs: []string{
				"What was asked for is reported on stdout and what went wrong on " +
					"stderr, prefixed \"sysml: \" unless it locates a finding in the " +
					"source.",
			},
		}, {
			Title:      "Environment",
			ManOnly:    true,
			Items:      append(append(append(usage.BudgetEnvironment(), usage.JobsEnvironment()...), usage.ToolEnvironment()...), solverEnvironment()...),
			Paragraphs: []string{usage.LegacyPrefixNote, usage.BudgetScopeNote},
		}, {
			Title:   "Files",
			ManOnly: true,
			Items: []usage.Item{
				usage.Entry("$XDG_STATE_HOME/sysml/history", "Prompt history, when XDG_STATE_HOME is set and writable."),
				usage.Entry("~/.sysml_history", "Prompt history otherwise. When neither can be written the history is kept for the session only."),
			},
		}, {
			Title:   "Reporting bugs",
			ManOnly: true,
			Paragraphs: []string{
				"Report bugs at https://github.com/Open-MBEE/OpenSysML/issues.",
			},
		}},
		SeeAlso: []string{"sysml-lsp(1)", "sysml-grpc(1)", "https://github.com/Open-MBEE/OpenSysML"},
	}
}

// solverEnvironment describes the variables of the experimental solving
// extension, which only the prompt's solver commands read.
func solverEnvironment() []usage.Item {
	return []usage.Item{
		usage.Entry("OPENSYSML_SMT", "Executable the %check, %explain, %solve, %configure and %optimize commands drive as their SMT solver, speaking SMT-LIB2 on standard input (experimental). Unset looks for z3, then cvc5, on PATH."),
		usage.Entry("OPENSYSML_SMT_TIMEOUT", "How long one solver query may take, as a Go duration. Default 10s, after which the verdict is unknown."),
		usage.Entry("OPENSYSML_SMT_CORE_BUDGET", "How long %explain may spend reducing an unsat core to a minimal one, as a Go duration. Default 30s."),
		usage.Entry("OPENSYSML_SMT_MAX_CONFIGURATIONS", "How many variant selections %configure ... all may report before saying the enumeration was cut short. Default 32."),
	}
}

// registerFlags declares the command's flags on fs, so the help, the man page
// and a run all read one declaration of each.
func registerFlags(fs *flag.FlagSet) {
	fs.BoolVar(&showHelp, "help", false, "Show this help and exit")
	fs.BoolVar(&showHelp, "h", false, "Show this help (shorthand)")
	fs.BoolVar(&showMan, "man", false, "Write this command's manual page, in roff, to stdout and exit")
	fs.Var(&evalExprs, "eval", "Evaluate expression and exit (can be specified multiple times)")
	fs.Var(&evalExprs, "e", "Evaluate expression and exit (shorthand)")
	fs.BoolVar(&showVersion, "version", false, "Show version information")
	fs.BoolVar(&showVersion, "v", false, "Show version (shorthand)")
	fs.BoolVar(&debugMode, "debug", false, "Report every diagnostic over the whole session buffer, with the pass that produced it")
	fs.BoolVar(&quietMode, "quiet", false, "Report errors only, suppressing warnings")
	fs.BoolVar(&strictMode, "strict", false, "Judge the model as conforming SysML v2: notation no pinned production admits is an error, not a warning")
	fs.BoolVar(&traceMode, "trace", false, "Report each execution step: expression evaluation, calc invocation, action tokens, state transitions")
	fs.Var(&schedule, "schedule", "Scheduling policy every run resolves its choice points under (concurrent tokens, overlapping guards, competing transitions): declared, reverse (default), seed:<n> for a reproducible pseudo-random order, or explore[:runs=N,depth=D] to run every linearization within the budget and table the distinct outcomes")
	fs.BoolVar(&listEngines, "engines", false, "List the analysis engines this build knows — name, authority, the questions each answers and whether its process is found — and exit")
	fs.Var(&engine, "engine", "Analysis engine every check is put to: auto (default) picks the strongest engine covering the question, all puts it to every covering engine in name order and composes their answers, or an engine by name (run, explore, check, sweep, solve, or tool:<name> from OPENSYSML_TOOLS), whose refusal is then the answer; -engine explore is -schedule explore, and -engine check searches every schedule of each -action for a violation, deadlock, failure or divergence")
	fs.Var(&jobsFlag, "jobs", "Runs of one check that may go concurrently — the linearizations of an exploration, the engines -engine all consults — each on a worker of its own over the shared model; the result is the same at any count. Default OPENSYSML_JOBS, else the number of CPUs")
	fs.StringVar(&convertFormat, "convert", "", "Convert the model to this format instead of running it: sysml, kerml, ttl, turtle or rdf (RDF is experimental)")
	fs.StringVar(&queryText, "query", "", "Evaluate OSLC Query text against the model instead of running the REPL")
	fs.StringVar(&outputPath, "output", "", "Write conversion output to this file (default: stdout)")
	fs.StringVar(&outputPath, "o", "", "Write conversion output to this file (shorthand)")
	fs.StringVar(&fromFormat, "from", "", "Input format for -convert: sysml, kerml, ttl, turtle, rdf, or xmi/uml/mdzip for a SysML v1 model to migrate (experimental; default: from the input's extension)")
	fs.StringVar(&migrationReport, "migration-report", "", "With -convert from xmi: write the element-by-element migration report to this file (JSON when it ends in .json, text otherwise)")
	fs.StringVar(&renderView, "render", "", "Render this view of the model (every file named, loaded as one) instead of running it, in the form its render member states")
	fs.StringVar(&renderAllDir, "render-all", "", "Render every declared view into this directory")
	fs.StringVar(&renderDoc, "render-document", "", "Compile this document definition, run its queries and write the rendered Markdown")
	fs.StringVar(&renderDocsDir, "render-documents", "", "Render every document definition as linked Markdown into this directory")
	fs.StringVar(&renderForm, "render-form", "", "Form -render or -render-all writes: text, mermaid or markdown (default: destination-dependent for -render, each kind's machine form for -render-all)")
	fs.StringVar(&docForm, "doc-form", "", "Form -render-document and -render-documents write: markdown (default), html or pdf, which drives an external converter")
	fs.StringVar(&pdfEngine, "pdf-engine", "", "Converter -doc-form pdf drives: weasyprint (default), pandoc or prince")
	fs.BoolVar(&pdfTitlePage, "pdf-title-page", false, "Put the document title on a page of its own (-doc-form pdf)")
	fs.BoolVar(&pdfTOC, "pdf-toc", false, "Write a table of contents ahead of the content (-doc-form pdf)")
	fs.BoolVar(&pdfNumbering, "pdf-number-sections", false, "Number the section headings hierarchically (-doc-form pdf)")
	fs.BoolVar(&pdfTitlePage, "doc-title-page", false, "Put the document title on a page of its own (-doc-form html or pdf)")
	fs.BoolVar(&pdfTOC, "doc-toc", false, "Write a table of contents ahead of the content (-doc-form html or pdf)")
	fs.BoolVar(&pdfNumbering, "doc-number-sections", false, "Number the section headings hierarchically (-doc-form html or pdf)")
	fs.Var(&htmlCSS, "html-css", "Style the HTML with this stylesheet: a file is inlined, a URL is linked (repeatable, applied in order after the default sheet)")
	fs.StringVar(&htmlTheme, "html-theme", "", "Style the HTML page with a bundled theme layered over the default stylesheet: default, modern, print or report (default: the default stylesheet alone)")
	fs.BoolVar(&htmlNoCSS, "html-no-default-css", false, "Leave the default stylesheet out, so only -html-css sheets style the document")
	fs.BoolVar(&htmlShowCSS, "html-default-css", false, "Write the default document stylesheet and exit, as a starting point for your own")
	fs.BoolVar(&htmlFragment, "html-fragment", false, "Write the document element alone, without the page shell or a stylesheet, to embed in a page of your own")
	fs.StringVar(&htmlMermaid, "html-mermaid", "", "Have the HTML page load Mermaid to draw its diagrams: cdn loads a pinned release from jsDelivr, a URL loads the script it names (default: diagrams stay Mermaid source)")
	fs.StringVar(&syncDiffWith, "sync-diff", "", "Show the change set between the model and this repository — a graph file (.ttl) or a SysML v2 API endpoint URL — keyed by effective element id, instead of running it; never writes")
	fs.StringVar(&syncApplyTo, "sync-apply", "", "Apply the change set to the model's project branch at this SysML v2 API endpoint URL, then record the commit in the sync state (token from "+flexo.EnvToken+")")
	fs.StringVar(&syncBase, "sync-base", "", "Repository graph at the last-seen commit; with it, repository changes since then surface as conflicts")
	fs.StringVar(&syncState, "sync-state", "", "Sync state file recording project, branch and last-seen commit (default: <model>.sync.json beside the model)")
	fs.BoolVar(&syncConfirmDeletes, "sync-confirm-deletes", false, "Confirm repository-side deletes; without it the diff reports them but applying is refused")
	fs.BoolVar(&syncMintIDs, "sync-mint-ids", false, "Mint a UUID for each unannotated element being created, so the repository can address it stably")
	fs.StringVar(&syncAnnotate, "sync-annotate", "", "Write the model to this file with each minted id declared as an @ElementId annotation (needs -sync-mint-ids)")
	fs.Var(&deprecatedFlag{instead: "-to has been replaced by -convert, as `sysml model.sysml -convert ttl`"}, "to", "Replaced by -convert, which names the output format")
	fs.Var(&modelChecks.instantiate, "instantiate", "Create an object of this definition before the checks, so a verdict is about it (repeatable)")
	fs.Var(&modelChecks.constraints, "constraint", "Evaluate this constraint and exit (repeatable)")
	fs.Var(&modelChecks.requirements, "requirement", "Evaluate this requirement and exit, reporting beside its verdict the verdict of every verification case verifying it (repeatable)")
	fs.Var(&modelChecks.satisfy, "satisfy", "Evaluate every satisfaction assertion, or with -satisfy=<name> those the named element states, reporting beside each verdict the verdict of every verification case verifying the requirement (repeatable)")
	fs.BoolVar(&modelChecks.validate, "validate", false, "Analyse the model and report its diagnostics, exiting nonzero on an error")
	fs.Var(&modelChecks.calcs, "calc", "Invoke this calculation and report what it computed, as -calc \"Fall(3, 4)\" (repeatable)")
	fs.Var(&modelChecks.analyses, "analysis", "Run this analysis or verification case and report its outputs and the verdict of its objective, as -analysis \"Pkg::Case(3.0) Pkg::part\" with arguments for its inputs and an object as its subject; a verification case also reports the verdict its body produced (repeatable)")
	fs.Var(&modelChecks.sweeps, "sweep", "Run the named -analysis or -calc once per value of this range, as -sweep \"speed=0.0 [SI::'m/s']..10.0 [SI::'m/s']:2.0 [SI::'m/s']\"; the values are produced in the parameter's declared type; several ranges run their cartesian product (repeatable)")
	fs.Var(&modelChecks.samples, "samples", "Draw this many values uniformly from each -sweep range instead of stepping through them, Integers or reals as the parameter is typed; needs -seed")
	fs.Var(&modelChecks.seed, "seed", "Seed -samples draws from, so the same seed draws the same table")
	fs.Var(&modelChecks.queries, "run-query", "Execute this document query and report its rows, as -run-query \"HeavySubsystems root=telescope\" (repeatable)")
	fs.Var(&modelChecks.actions, "action", "Run this action to completion, as -action \"Drive rover1\" to run it on an object (repeatable)")
	fs.Var(&modelChecks.states, "state", "Run this state machine, as -state \"Mission rover1\" to run it on an object (repeatable)")
	fs.Var(&modelChecks.advance, "advance", "Simulated time units to run the -action and -state behaviors for, on one shared clock (default: a state machine takes only its initial transition; an action runs to completion)")
	fs.Var(&modelChecks.checker.diverge, "check-diverge", "With -engine check or -engine all: report this feature divergent when schedules leave it with different final values, as -check-diverge x or -check-diverge this.level for the performing object's; default every attribute of the action and of its performing object, the action's own when it has none (repeatable)")
	fs.Var(&modelChecks.checker.properties, "check-property", "With -engine check or -engine all: evaluate this constraint or requirement at every stable state of the action, on the performing object when there is one, and report a schedule at which it is false (repeatable)")
	fs.StringVar(&modelChecks.checker.witness, "check-witness", "", "With -engine check or -engine all: write a witness file to this directory for each violation and each divergent value — the schedule's choices, a blank line, then the run's trace — which -schedule replay:<file> and %replay follow")
	modelChecks.checker.depth.flag, modelChecks.checker.states.flag = "check-depth", "check-states"
	fs.Var(&modelChecks.checker.depth, "check-depth", "With -engine check or -engine all: the most moves one schedule may make before the search backtracks, named as the depth bound hit (default 10000)")
	fs.Var(&modelChecks.checker.states, "check-states", "With -engine check or -engine all: the most distinct states the search may visit, named as the states bound hit; the same figure -engine all gives an exploration as its runs (default 1000000)")
	fs.Var(&modelChecks.checker.timeout, "check-timeout", "With -engine check or -engine all: the time the check's plan may run for, as 30s or 2m; a search it stops is reported incomplete with the states and depth reached, not as a verdict")
	fs.BoolVar(&modelChecks.jsonOut, "json", false, "Report checks as one JSON document rather than as lines")
	fs.StringVar(&compileCalc, "compile", "", "Compile this calc def to a native executable named by -o, as -compile Pkg::Fib")
	fs.StringVar(&compileTarget, "target", "c", "Backend -compile generates code for: c (default) or go")
	fs.BoolVar(&compileSource, "source", false, "With -compile, write the generated source to -o instead of building it")
	fs.StringVar(&cpuProfilePath, "cpuprofile", "", "Write a CPU profile of the run to this file, for go tool pprof")
	fs.StringVar(&memProfilePath, "memprofile", "", "Write a heap profile of the run to this file, for go tool pprof")
	fs.BoolVar(&memStats, "memstats", false, "Report on stderr what the run cost: wall time, memory allocated, memory taken from the OS")
}

// docFlags is a flag set holding the command's flags, for rendering the help or
// the man page without a command line to parse.
func docFlags() *flag.FlagSet {
	fs := flag.NewFlagSet("sysml", flag.ContinueOnError)
	registerFlags(fs)
	return fs
}
