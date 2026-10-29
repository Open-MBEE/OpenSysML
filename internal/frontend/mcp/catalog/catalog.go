// Package catalog is the one source of truth for the tools sysml-mcp serves:
// what each tool is called, which group serves it, which RPC of the sysml-grpc
// service and which sysml CLI operation it stands for, and a one-line summary.
//
// Registration (internal/frontend/mcp), the tool_catalog tool, the sysml-mcp
// manual page and the drift gates all read this table, so a tool added or
// removed anywhere else fails a test until the catalog says so too. The gates
// are TestEveryRPCIsServedByATool and TestRegisteredToolsMatchTheCatalog in
// internal/frontend/mcp and TestEverySysmlFlagIsClassifiedForMCP in cmd/sysml.
package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// Group names a set of tools that sysml-mcp serves together (-tools).
type Group string

// The groups, in the order the manual page lists them.
const (
	GroupLSP     Group = "lsp"
	GroupRuntime Group = "runtime"
	GroupSession Group = "session"
)

// Entry is one tool of the catalog.
type Entry struct {
	// Name is the MCP tool name.
	Name string
	// Group is the group that serves the tool.
	Group Group
	// RPCs are the sysml-grpc service methods the tool stands for; empty for
	// a tool with no service counterpart (the session and lsp groups, and
	// tool_catalog itself).
	RPCs []string
	// Flags are the sysml CLI flags (without the dash) whose operation the
	// tool performs. Every flag of the CLI that is an operation or one of
	// its arguments is listed exactly once across Tools, Unserved and
	// CLIOnly.
	Flags []string
	// Summary is a one-line description of what the tool answers.
	Summary string
}

// Operation spells the operation an entry maps to, for the catalog a client
// reads: its RPCs and CLI flags, or "none" for a tool that is MCP-only.
func (e Entry) Operation() string {
	var parts []string
	for _, rpc := range e.RPCs {
		parts = append(parts, "rpc "+rpc)
	}
	for _, flag := range e.Flags {
		parts = append(parts, "sysml -"+flag)
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

// Tools is the catalog, grouped and then alphabetical by name.
var Tools = []Entry{
	{Name: "tool_catalog", Summary: "List the tools this server serves, each with the RPC or sysml flag it stands for."},

	{Name: "apply_model_edit", Group: GroupLSP, Summary: "Edit a model by operations on its elements (setValue, rename, addMember, delete, move, ...) and answer the text edits; write them with apply."},
	{Name: "close_document", Group: GroupLSP, Summary: "Release a document held open by open_document; the server reads the file on disk again."},
	{Name: "code_actions", Group: GroupLSP, Summary: "List the quick fixes and refactorings available for a range of a document, each with its edits."},
	{Name: "completion", Group: GroupLSP, Summary: "Propose completions at a position: keywords, names in scope and members of the accessed type."},
	{Name: "definition", Group: GroupLSP, Summary: "Find where the name at a position is declared."},
	{Name: "document_symbols", Group: GroupLSP, Summary: "Outline a document: the elements it declares, nested as they are owned."},
	{Name: "documents", Group: GroupLSP, Summary: "List the document definitions declared across the workspace."},
	{Name: "format", Group: GroupLSP, Summary: "Format a document, or a range of it, in the canonical style; write the file with apply."},
	{Name: "hover", Group: GroupLSP, Summary: "Explain the element at a position: kind, qualified name, type, value and documentation."},
	{Name: "inlay_hints", Group: GroupLSP, Summary: "List the inferred types and values an editor would show inline for a document or range."},
	{Name: "open_document", Group: GroupLSP, Summary: "Hold a document open with the given text, analyzed instead of the file on disk."},
	{Name: "references", Group: GroupLSP, Summary: "Find every place in the workspace that refers to the element at a position."},
	{Name: "rename", Group: GroupLSP, Summary: "Rename the element at a position everywhere in the workspace; write the files with apply."},
	{Name: "render_document", Group: GroupLSP, Summary: "Render a document definition of the workspace to Markdown, its queries evaluated and its diagrams embedded."},
	{Name: "render_view", Group: GroupLSP, Summary: "Render a declared view or pseudo-view of a workspace document as Mermaid, dot, PlantUML, D2, draw.io, text or a table."},
	{Name: "signature_help", Group: GroupLSP, Summary: "Describe the invocation enclosing a position: its parameters and the active one."},
	{Name: "update_document", Group: GroupLSP, Summary: "Replace the text of a document held open by open_document and answer the diagnostics of the new text."},
	{Name: "views", Group: GroupLSP, Summary: "List the views a document declares and the pseudo-views render_view can draw of it."},
	{Name: "workspace_diagnostics", Group: GroupLSP, Summary: "List the errors, warnings and hints of one workspace document or of the whole workspace."},
	{Name: "workspace_info", Group: GroupLSP, Summary: "Describe the workspace root and the documents it holds, with their diagnostic counts."},
	{Name: "workspace_symbols", Group: GroupLSP, Summary: "Search the workspace's declared elements by name substring."},

	{Name: "apply_edits", Group: GroupRuntime, RPCs: []string{"ApplyEdits"},
		Summary: "Apply workspace edits to a parsed model and answer the edited documents."},
	{Name: "compare_results", Group: GroupRuntime, RPCs: []string{"CompareResults"}, Flags: []string{"compare-results"},
		Summary: "Run the configurations a SysML v1 tool's results index and table the tool's distributions beside the runs'."},
	{Name: "convert", Group: GroupRuntime, RPCs: []string{"Convert"}, Flags: []string{"convert", "from", "id", "include-library-stubs"},
		Summary: "Convert a model between the textual notation, JSON, RDF and XMI."},
	{Name: "diagnostics", Group: GroupRuntime, RPCs: []string{"GetDiagnostics"}, Flags: []string{"validate", "disable-lint", "enable-lint"},
		Summary: "Report every diagnostic of a parsed model, as sysml -validate does."},
	{Name: "explain_diagnostic", Group: GroupRuntime, Flags: []string{"explain"},
		Summary: "Explain a diagnostic code, or list every code with its summary."},
	{Name: "evaluate", Group: GroupRuntime, RPCs: []string{"Evaluate"}, Flags: []string{"eval"},
		Summary: "Evaluate an expression against a parsed model."},
	{Name: "evaluate_calc", Group: GroupRuntime, RPCs: []string{"EvaluateCalc"}, Flags: []string{"calc"},
		Summary: "Evaluate a calculation definition or usage."},
	{Name: "execute_action", Group: GroupRuntime, RPCs: []string{"ExecuteAction"},
		Flags:   []string{"action", "schedule", "advance", "seed", "draws", "clock-step"},
		Summary: "Execute an action with bound inputs and report its outputs."},
	{Name: "execute_state", Group: GroupRuntime, RPCs: []string{"ExecuteState"}, Flags: []string{"state"},
		Summary: "Run a state machine over a sequence of events and report the trace."},
	{Name: "export_graphs", Group: GroupRuntime, RPCs: []string{"ExportGraphs"}, Flags: []string{"graphs"},
		Summary: "Export the lowered action and state graphs of a model."},
	{Name: "import_data", Group: GroupRuntime, RPCs: []string{"ImportData"},
		Flags:   []string{"import", "import-as", "import-map", "import-format", "import-dry-run"},
		Summary: "Set the feature values a CSV, TSV, JSON or JSON Lines file assigns, one row per element."},
	{Name: "instantiate", Group: GroupRuntime, RPCs: []string{"Instantiate"}, Flags: []string{"instantiate"},
		Summary: "Instantiate a definition or usage and report the object's feature values."},
	{Name: "list_engines", Group: GroupRuntime, RPCs: []string{"ListEngines"}, Flags: []string{"engines", "probe"},
		Summary: "List the analysis engines and whether each is ready."},
	{Name: "migrate", Group: GroupRuntime, RPCs: []string{"Migrate"},
		Flags:   []string{"migrate", "migration-report", "migration-results", "layout", "image-base-url"},
		Summary: "Migrate a SysML v1 model to SysML v2 and report what moved."},
	{Name: "parse", Group: GroupRuntime, RPCs: []string{"ParseFile", "ParseSources"}, Flags: []string{"strict"},
		Summary: "Parse files or inline sources and answer the roots, diagnostics and a model hash."},
	{Name: "query", Group: GroupRuntime, RPCs: []string{"Query"}, Flags: []string{"query"},
		Summary: "Run an OSLC query against a parsed model."},
	{Name: "render_model_document", Group: GroupRuntime, RPCs: []string{"RenderDocument"}, Flags: []string{"render-document", "doc-form"},
		Summary: "Render a document definition as Markdown or HTML."},
	{Name: "record_run", Group: GroupRuntime, RPCs: []string{"RecordRun"}, Flags: []string{"record-run", "record-into"},
		Summary: "Run an analysis case and record the runs into the model as AnalysisRecords declarations."},
	{Name: "render_model_view", Group: GroupRuntime, RPCs: []string{"RenderView"}, Flags: []string{"render", "render-ports", "render-show", "render-depth", "render-compartments"},
		Summary: "Render a view as its nodes, edges and compartments."},
	{Name: "check_behavior", Group: GroupRuntime, RPCs: []string{"CheckBehavior"}, Flags: []string{"check-property", "check-diverge", "check-input", "check-assume", "check-witness", "check-depth", "check-states", "check-unroll", "check-timeout"},
		Summary: "Search every schedule of an action or state machine for violated properties, deadlocks, failures and divergent values."},
	{Name: "run_analysis", Group: GroupRuntime, RPCs: []string{"RunAnalysis"}, Flags: []string{"analysis", "engine", "tool-dry-run"},
		Summary: "Run an analysis case and report its result."},
	{Name: "run_document_query", Group: GroupRuntime, RPCs: []string{"RunDocumentQuery"}, Flags: []string{"run-query"},
		Summary: "Run a document query definition and report its rows."},
	{Name: "run_runs", Group: GroupRuntime, RPCs: []string{"RunRuns"}, Flags: []string{"runs", "observe"},
		Summary: "Run an action or a Monte Carlo case repeatedly and table what the runs observed."},
	{Name: "run_sweep", Group: GroupRuntime, RPCs: []string{"RunSweep"}, Flags: []string{"sweep", "samples"},
		Summary: "Run an analysis case over ranges of its parameters."},
	{Name: "self_check", Group: GroupRuntime, RPCs: []string{"SelfCheck"}, Flags: []string{"self-check", "self-check-package"},
		Summary: "Apply the reflective SysMLValidation constraints to every element of a model."},
	{Name: "server_info", Group: GroupRuntime, RPCs: []string{"GetServerInfo"},
		Summary: "Report the service version and the capabilities it serves."},
	{Name: "symbol", Group: GroupRuntime, RPCs: []string{"GetSymbol"},
		Summary: "Describe one element of a parsed model."},
	{Name: "validate_instance", Group: GroupRuntime, RPCs: []string{"ValidateInstance"},
		Summary: "Validate an instantiated object against its definition, as sysml -validate=<object> does."},
	{Name: "verify_constraint", Group: GroupRuntime, RPCs: []string{"VerifyConstraint"}, Flags: []string{"constraint"},
		Summary: "Verify a constraint and answer the verdict."},
	{Name: "verify_requirement", Group: GroupRuntime, RPCs: []string{"VerifyRequirement"}, Flags: []string{"requirement"},
		Summary: "Verify a requirement and answer the verdict."},
	{Name: "verify_satisfaction", Group: GroupRuntime, RPCs: []string{"VerifySatisfaction"}, Flags: []string{"satisfy"},
		Summary: "Verify a satisfy relationship and answer the verdict."},

	{Name: "advance", Group: GroupSession, Summary: "Advance the debugged state machine's clock by a duration, firing the timed transitions due."},
	{Name: "check", Group: GroupSession, Summary: "Check with the SMT solver whether an element's constraints are satisfiable."},
	{Name: "configure", Group: GroupSession, Summary: "Choose variant selections for a variation model, or enumerate every valid configuration."},
	{Name: "debug_breakpoints", Group: GroupSession, Summary: "List, set or clear the breakpoints of the debugged action."},
	{Name: "debug_continue", Group: GroupSession, Summary: "Run the debugged action or state machine on to its next breakpoint or its end."},
	{Name: "debug_current", Group: GroupSession, Summary: "Report the debugger's state: the action's tokens and breakpoints, or the machine's state, clock and event queue."},
	{Name: "debug_start", Group: GroupSession, Summary: "Start debugging an action or a state machine by qualified name."},
	{Name: "debug_step", Group: GroupSession, Summary: "Step the debugged action or state machine once."},
	{Name: "debug_stop", Group: GroupSession, Summary: "Stop debugging and discard the debugged executors."},
	{Name: "explain", Group: GroupSession, Summary: "Explain why an element's constraints are unsatisfiable, naming the conflicting ones."},
	{Name: "features", Group: GroupSession, Summary: "List a runtime instance's feature values, with its object graph."},
	{Name: "instances", Group: GroupSession, Summary: "List the runtime instances the session holds."},
	{Name: "optimize", Group: GroupSession, Summary: "Optimize an analysis case's objective with the solver and answer the optimum."},
	{Name: "render_run", Group: GroupSession, Summary: "Render a run's trace as a timeline or a sequence diagram."},
	{Name: "repl_command", Group: GroupSession, Summary: "Run one % meta command of the REPL and answer its output lines."},
	{Name: "runs", Group: GroupSession, Summary: "Run an action repeatedly, or a Monte Carlo case, and table the outcomes."},
	{Name: "send", Group: GroupSession, Summary: "Send a signal to the debugged state machine."},
	{Name: "session_clear", Group: GroupSession, Summary: "Clear a session's declarations, instances and debuggers."},
	{Name: "session_close", Group: GroupSession, Summary: "Close an open REPL session."},
	{Name: "session_eval", Group: GroupSession, Summary: "Evaluate an expression in a session and answer its value."},
	{Name: "session_list", Group: GroupSession, Summary: "List the open REPL sessions."},
	{Name: "session_load", Group: GroupSession, Summary: "Load model files under the workspace root into a session and answer the diagnostics."},
	{Name: "session_open", Group: GroupSession, Summary: "Open a REPL session, optionally over a root directory whose files load on demand, and answer its id."},
	{Name: "session_print", Group: GroupSession, Summary: "Print a session's declarations back as SysML notation."},
	{Name: "session_run", Group: GroupSession, Summary: "Run an action, state machine, calculation or analysis case in a session and answer the verdict."},
	{Name: "session_save", Group: GroupSession, Summary: "Preview a session's model as a file under the workspace root, or write it with apply."},
	{Name: "session_submit", Group: GroupSession, Summary: "Submit declarations to a session and answer what was declared and the diagnostics."},
	{Name: "solve", Group: GroupSession, Summary: "Synthesize feature values satisfying an element's constraints with the solver."},
	{Name: "sweep", Group: GroupSession, Summary: "Sweep a calculation over a range or sampled values and table the results."},
}

// RPCsWithoutTools names the service methods that must not be tools, each
// with the reason. Every method of the service is either here or in some
// entry's RPCs; the service has no streaming or stateful method today, so the
// list is empty, and a method added without a tool fails the gate until it is
// listed here with a reason.
var RPCsWithoutTools = map[string]string{}

// The reasons several unserved operations and CLI-only flags share.
const (
	notDrawn       = "RenderViewRequest answers data, not a drawn form"
	noListing      = "no RPC lists the documents and views of a model"
	noHTMLStyling  = "RenderDocumentRequest has no HTML styling fields"
	hostToolchain  = "drives a native toolchain on the host"
	hostRepository = "talks to a remote SysML v2 API repository from the host"
	hostProfiling  = "process profiling"
)

// Unserved names the sysml CLI operations and operation arguments that no
// tool performs yet, each with the request or response the service lacks for
// it. A flag listed here is an operation the CLI has and the service does not;
// adding the field or RPC and the tool moves the flag into an entry.
var Unserved = map[string]string{
	"metaclass-graph":     "no RPC answers the metaclass hierarchy vocabulary graph",
	"metamodel-table":     "no RPC answers the pinned metamodel's table as JSON",
	"portable":            "MigrateRequest has no portable field",
	"render-all":          "no RPC renders every declared view",
	"render-form":         notDrawn,
	"render-palette":      notDrawn,
	"render-link":         notDrawn,
	"render-unplaced":     notDrawn,
	"render-style":        notDrawn,
	"render-lines":        notDrawn,
	"render-overlay":      notDrawn,
	"list":                noListing,
	"list-kind":           noListing,
	"list-form":           noListing,
	"render-documents":    "no RPC renders every document definition",
	"diagram-form":        "RenderDocumentRequest has no diagram-form field",
	"doc-title-page":      "RenderDocumentRequest has no title-page field",
	"doc-toc":             "RenderDocumentRequest has no table-of-contents field",
	"doc-number-sections": "RenderDocumentRequest has no section-numbering field",
	"doc-number-figures":  "RenderDocumentRequest has no figure-numbering field",
	"pdf-engine":          "RenderDocumentRequest refuses the pdf form",
	"html-default-css":    noHTMLStyling,
	"html-no-default-css": noHTMLStyling,
	"html-css":            noHTMLStyling,
	"html-theme":          noHTMLStyling,
	"html-fragment":       noHTMLStyling,
	"html-math":           noHTMLStyling,
	"html-mermaid":        noHTMLStyling,
	"trace":               "ExecuteActionRequest has no trace field",
	"render-run":          "no RPC renders a recorded run as a timeline or sequence",
}

// CLIOnly names the sysml CLI flags that are not operations of the service
// and must not become tools, each with the reason: they drive the process,
// its files and its output, not a model.
var CLIOnly = map[string]string{
	"no-record-cache":      "a server start option (sysml-mcp -no-record-cache)",
	"library-root":         "a load option; the server reads OPENSYSML_LIBRARY_ROOTS",
	"json":                 "output formatting; every tool answers protojson",
	"format":               "output formatting; every tool answers protojson, and SARIF and JUnit are written for a CI service reading files",
	"baseline":             "reads a file on the host and shapes the exit status; a tool answers every diagnostic",
	"warnings-as-errors":   "exit-status policy of the command; a tool answers each diagnostic with its severity",
	"output":               "writes a file on the host; a tool answers the content",
	"library-references":   "writes a side file on the host; convert answers libraryReferences inline",
	"bundle":               "archives a project directory on the host",
	"unbundle":             "writes a project directory on the host",
	"jobs":                 "host concurrency",
	"compile":              hostToolchain,
	"target":               hostToolchain,
	"source":               hostToolchain,
	"sync-diff":            hostRepository,
	"sync-apply":           hostRepository,
	"sync-base":            hostRepository,
	"sync-state":           hostRepository,
	"sync-annotate":        hostRepository,
	"sync-confirm-deletes": hostRepository,
	"sync-mint-ids":        hostRepository,
	"stdlib":               hostRepository,
	"debug":                "process logging (sysml-mcp -log-level)",
	"quiet":                "process logging (sysml-mcp -log-level)",
	"cpuprofile":           hostProfiling,
	"memprofile":           hostProfiling,
	"memstats":             hostProfiling,
}

// Groups lists the groups in catalog order.
func Groups() []Group {
	return []Group{GroupLSP, GroupRuntime, GroupSession}
}

// Entries answers the tools the given groups serve, tool_catalog included,
// in catalog order.
func Entries(groups ...Group) []Entry {
	serve := map[Group]bool{}
	for _, g := range groups {
		serve[g] = true
	}
	var out []Entry
	for _, e := range Tools {
		if e.Group == "" || serve[e.Group] {
			out = append(out, e)
		}
	}
	return out
}

// Names answers the sorted names of the tools the given groups serve.
func Names(groups ...Group) []string {
	var names []string
	for _, e := range Entries(groups...) {
		names = append(names, e.Name)
	}
	sort.Strings(names)
	return names
}

// ToolFor answers the tool that serves an RPC of the service.
func ToolFor(rpc string) (Entry, bool) {
	for _, e := range Tools {
		for _, r := range e.RPCs {
			if r == rpc {
				return e, true
			}
		}
	}
	return Entry{}, false
}

// ClassifyFlag answers how the catalog classifies a sysml CLI flag: the tool
// that performs it, "unserved" with what the service lacks, "cli-only" with
// the reason, or an error when the flag is unclassified or classified twice.
func ClassifyFlag(flag string) (string, error) {
	var found []string
	for _, e := range Tools {
		for _, f := range e.Flags {
			if f == flag {
				found = append(found, "tool "+e.Name)
			}
		}
	}
	if why, ok := Unserved[flag]; ok {
		found = append(found, "unserved: "+why)
	}
	if why, ok := CLIOnly[flag]; ok {
		found = append(found, "cli-only: "+why)
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("sysml -%s is not classified in internal/frontend/mcp/catalog: list it under a tool's Flags, in Unserved with what the service lacks, or in CLIOnly with the reason", flag)
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("sysml -%s is classified %d times in internal/frontend/mcp/catalog: %s", flag, len(found), strings.Join(found, "; "))
	}
}
