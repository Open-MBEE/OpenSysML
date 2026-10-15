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

	{Name: "workspace_info", Group: GroupLSP, Summary: "Describe the workspace root and the documents it holds, with their diagnostic counts."},

	{Name: "apply_edits", Group: GroupRuntime, RPCs: []string{"ApplyEdits"},
		Summary: "Apply workspace edits to a parsed model and answer the edited documents."},
	{Name: "compare_results", Group: GroupRuntime, RPCs: []string{"CompareResults"}, Flags: []string{"compare-results"},
		Summary: "Run the configurations a SysML v1 tool's results index and table the tool's distributions beside the runs'."},
	{Name: "convert", Group: GroupRuntime, RPCs: []string{"Convert"}, Flags: []string{"convert", "from", "id"},
		Summary: "Convert a model between the textual notation, JSON, RDF and XMI."},
	{Name: "diagnostics", Group: GroupRuntime, RPCs: []string{"GetDiagnostics"}, Flags: []string{"validate"},
		Summary: "Report every diagnostic of a parsed model, as sysml -validate does."},
	{Name: "evaluate", Group: GroupRuntime, RPCs: []string{"Evaluate"}, Flags: []string{"eval"},
		Summary: "Evaluate an expression against a parsed model."},
	{Name: "evaluate_calc", Group: GroupRuntime, RPCs: []string{"EvaluateCalc"}, Flags: []string{"calc"},
		Summary: "Evaluate a calculation definition or usage."},
	{Name: "execute_action", Group: GroupRuntime, RPCs: []string{"ExecuteAction"}, Flags: []string{"action", "schedule"},
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
	{Name: "list_engines", Group: GroupRuntime, RPCs: []string{"ListEngines"}, Flags: []string{"engines"},
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
	{Name: "render_model_view", Group: GroupRuntime, RPCs: []string{"RenderView"}, Flags: []string{"render", "render-ports", "render-show"},
		Summary: "Render a view as its nodes, edges and compartments."},
	{Name: "run_analysis", Group: GroupRuntime, RPCs: []string{"RunAnalysis"}, Flags: []string{"analysis", "engine"},
		Summary: "Run an analysis case and report its result."},
	{Name: "run_document_query", Group: GroupRuntime, RPCs: []string{"RunDocumentQuery"}, Flags: []string{"run-query"},
		Summary: "Run a document query definition and report its rows."},
	{Name: "run_sweep", Group: GroupRuntime, RPCs: []string{"RunSweep"}, Flags: []string{"sweep"},
		Summary: "Run an analysis case over ranges of its parameters."},
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

	{Name: "session_close", Group: GroupSession, Summary: "Close an open REPL session."},
	{Name: "session_list", Group: GroupSession, Summary: "List the open REPL sessions."},
	{Name: "session_open", Group: GroupSession, Summary: "Open a REPL session over a workspace and answer its id."},
}

// RPCsWithoutTools names the service methods that must not be tools, each
// with the reason. Every method of the service is either here or in some
// entry's RPCs; the service has no streaming or stateful method today, so the
// list is empty, and a method added without a tool fails the gate until it is
// listed here with a reason.
var RPCsWithoutTools = map[string]string{}

// Unserved names the sysml CLI operations and operation arguments that no
// tool performs yet, each with the request or response the service lacks for
// it. A flag listed here is an operation the CLI has and the service does not;
// adding the field or RPC and the tool moves the flag into an entry.
var Unserved = map[string]string{
	"disable-lint":        "DiagnosticsRequest has no lint selection",
	"enable-lint":         "DiagnosticsRequest has no lint selection",
	"self-check":          "no RPC runs the SysMLValidation reflective check",
	"self-check-package":  "no RPC runs the SysMLValidation reflective check",
	"tool-dry-run":        "RunAnalysisRequest has no dry-run field",
	"record-run":          "no RPC records a run into the model (AnalysisRecords)",
	"record-into":         "no RPC records a run into the model (AnalysisRecords)",
	"advance":             "ExecuteActionRequest has no final-time field",
	"seed":                "ExecuteActionRequest has no seed field",
	"draws":               "ExecuteActionRequest has no draws field",
	"clock-step":          "ExecuteActionRequest has no clock-step field",
	"runs":                "no RPC runs an action repeatedly and tables the observations",
	"observe":             "no RPC runs an action repeatedly and tables the observations",
	"samples":             "RunSweepRequest has no samples field",
	"probe":               "ListEnginesRequest has no probe field",
	"check-property":      "no RPC checks every schedule of a behavior",
	"check-diverge":       "no RPC checks every schedule of a behavior",
	"check-input":         "no RPC checks every schedule of a behavior",
	"check-assume":        "no RPC checks every schedule of a behavior",
	"check-witness":       "no RPC checks every schedule of a behavior",
	"check-depth":         "no RPC checks every schedule of a behavior",
	"check-states":        "no RPC checks every schedule of a behavior",
	"check-unroll":        "no RPC checks every schedule of a behavior",
	"check-timeout":       "no RPC checks every schedule of a behavior",
	"portable":            "MigrateRequest has no portable field",
	"render-all":          "no RPC renders every declared view",
	"render-form":         "RenderViewRequest answers data, not a drawn form",
	"render-palette":      "RenderViewRequest answers data, not a drawn form",
	"render-link":         "RenderViewRequest answers data, not a drawn form",
	"render-unplaced":     "RenderViewRequest answers data, not a drawn form",
	"render-style":        "RenderViewRequest answers data, not a drawn form",
	"render-lines":        "RenderViewRequest answers data, not a drawn form",
	"render-overlay":      "RenderViewRequest answers data, not a drawn form",
	"list":                "no RPC lists the documents and views of a model",
	"list-kind":           "no RPC lists the documents and views of a model",
	"list-form":           "no RPC lists the documents and views of a model",
	"render-documents":    "no RPC renders every document definition",
	"diagram-form":        "RenderDocumentRequest has no diagram-form field",
	"doc-title-page":      "RenderDocumentRequest has no title-page field",
	"doc-toc":             "RenderDocumentRequest has no table-of-contents field",
	"doc-number-sections": "RenderDocumentRequest has no section-numbering field",
	"doc-number-figures":  "RenderDocumentRequest has no figure-numbering field",
	"pdf-engine":          "RenderDocumentRequest refuses the pdf form",
	"html-default-css":    "RenderDocumentRequest has no HTML styling fields",
	"html-no-default-css": "RenderDocumentRequest has no HTML styling fields",
	"html-css":            "RenderDocumentRequest has no HTML styling fields",
	"html-theme":          "RenderDocumentRequest has no HTML styling fields",
	"html-fragment":       "RenderDocumentRequest has no HTML styling fields",
	"html-math":           "RenderDocumentRequest has no HTML styling fields",
	"html-mermaid":        "RenderDocumentRequest has no HTML styling fields",
	"trace":               "ExecuteActionRequest has no trace field",
	"render-run":          "no RPC renders a recorded run as a timeline or sequence",
}

// CLIOnly names the sysml CLI flags that are not operations of the service
// and must not become tools, each with the reason: they drive the process,
// its files and its output, not a model.
var CLIOnly = map[string]string{
	"no-record-cache":      "a server start option (sysml-mcp -no-record-cache)",
	"json":                 "output formatting; every tool answers protojson",
	"output":               "writes a file on the host; a tool answers the content",
	"jobs":                 "host concurrency",
	"compile":              "drives a native toolchain on the host",
	"target":               "drives a native toolchain on the host",
	"source":               "drives a native toolchain on the host",
	"sync-diff":            "talks to a remote SysML v2 API repository from the host",
	"sync-apply":           "talks to a remote SysML v2 API repository from the host",
	"sync-base":            "talks to a remote SysML v2 API repository from the host",
	"sync-state":           "talks to a remote SysML v2 API repository from the host",
	"sync-annotate":        "talks to a remote SysML v2 API repository from the host",
	"sync-confirm-deletes": "talks to a remote SysML v2 API repository from the host",
	"sync-mint-ids":        "talks to a remote SysML v2 API repository from the host",
	"debug":                "process logging (sysml-mcp -log)",
	"quiet":                "process logging (sysml-mcp -log)",
	"cpuprofile":           "process profiling",
	"memprofile":           "process profiling",
	"memstats":             "process profiling",
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

// Lookup finds the entry of a tool.
func Lookup(name string) (Entry, bool) {
	for _, e := range Tools {
		if e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
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
