package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/usage"
)

const mcpCommand = "sysml-mcp"

// options is what the command line resolves to.
type options struct {
	root          string
	tools         string
	strict        bool
	noRecordCache bool
	cacheSize     int
	logLevel      string
	showVersion   bool
	showHelp      bool
	showMan       bool
	printConfig   hostFlag
	configCommand string
}

// registerFlags declares the server's flags on fs, so a run, the help and the
// man page all read one declaration of each. -h/-help are flags rather than the
// flag package's own so that help asked for is a result on stdout, and -stdio
// names the only transport there is: agent hosts pass it, so it is accepted and
// documented rather than rejected.
func registerFlags(fs *flag.FlagSet) *options {
	var o options
	fs.StringVar(&o.root, "root", ".",
		"Workspace folder whose model documents the server indexes and edits; default the current directory, which an agent host sets to the project")
	fs.StringVar(&o.tools, "tools", "all",
		"Tool groups to serve, comma-separated: lsp (diagnostics, navigation, editing, rendering), runtime (one-shot evaluation, execution, verification, conversion), session (persistent REPL sessions); default all")
	fs.BoolVar(&o.strict, "strict", false,
		"Serve strict conformance from the start: notation no pinned SysML v2 production admits is an error")
	fs.BoolVar(&o.noRecordCache, "no-record-cache", false,
		"Parse every document and hold it loaded, reading no interface record from the record cache and writing none; default off, or OPENSYSML_RECORD_CACHE=0")
	fs.IntVar(&o.cacheSize, "cache-size", 100, "Maximum number of parsed models the runtime tools hold; at least 1")
	fs.StringVar(&o.logLevel, "log-level", "warn", "Lowest level logged to stderr: debug, info, warn or error")
	fs.Bool("stdio", false, "Serve over stdin/stdout (the only transport; accepted for hosts that name it)")
	fs.Var(&o.printConfig, "print-config",
		"Write the configuration block declaring this server to an agent host — "+strings.Join(hostNames(), ", ")+" — to stdout and exit; alone, list the hosts")
	fs.StringVar(&o.configCommand, "print-config-command", "",
		"Command the -print-config block names instead of this binary's absolute path, such as sysml-mcp when it is on PATH")
	fs.BoolVar(&o.showVersion, "version", false, "Show version information")
	fs.BoolVar(&o.showVersion, "v", false, "Show version (shorthand)")
	fs.BoolVar(&o.showHelp, "help", false, "Show this help and exit")
	fs.BoolVar(&o.showHelp, "h", false, "Show this help (shorthand)")
	fs.BoolVar(&o.showMan, "man", false, "Write this command's manual page, in roff, to stdout and exit")
	return &o
}

// docFlags is a flag set holding the server's flags, for rendering the help or
// the man page without a command line to parse.
func docFlags() *flag.FlagSet {
	fs := flag.NewFlagSet(mcpCommand, flag.ContinueOnError)
	registerFlags(fs)
	return fs
}

// hostList names the hosts -print-config knows, each with the file it reads.
func hostList() string {
	parts := make([]string, len(hosts))
	for i, h := range hosts {
		parts[i] = h.name + " (" + h.file + ")"
	}
	return strings.Join(parts, ", ")
}

// doc describes the server for both the terminal help and the man page, so a
// setting documented for one is documented for the other.
func doc() usage.Doc {
	return usage.Doc{
		Command:    mcpCommand,
		ManSection: 1,
		Summary:    "Model Context Protocol server for SysML v2 and KerML",
		Synopsis:   []string{"[options]"},
		Description: []string{
			"sysml-mcp serves OpenSysML to coding agents over the Model Context " +
				"Protocol, as tools an agent calls: the language server's " +
				"diagnostics, navigation, editing and rendering over the model " +
				"documents under the workspace root; the runtime's one-shot " +
				"parsing, evaluation, instantiation, execution, verification, " +
				"querying and conversion; and named REPL sessions whose " +
				"declarations, instances and runs persist across calls; " +
				protocolMessage + ".",
			"The documents under the root are managed as one workspace, so a " +
				"declaration in one resolves against the others, and the standard " +
				"library is the copy embedded in this binary unless " +
				"OPENSYSML_LIBRARY_PATH names another.",
		},
		Sections: []usage.Section{{
			Title: "Starting the server",
			Examples: []usage.Example{
				usage.Ex(mcpCommand, "What an agent host runs, in the project directory"),
				usage.Ex("sysml-mcp -root models", "Serve the models under another folder"),
				usage.Ex("sysml-mcp -tools lsp,runtime", "Serve without persistent sessions"),
			},
			Paragraphs: []string{
				"An agent host declares the server in its configuration, as " +
					"{\"mcp\": {\"sysml\": {\"type\": \"local\", \"command\": [\"sysml-mcp\"]}}} " +
					"in OpenCode's opencode.json, and starts it with the project as the " +
					"working directory.",
			},
		}, {
			Title: "Configuring a host",
			Lead: []string{
				"-print-config writes the block a host reads, naming this binary by " +
					"its absolute path so the host finds it wherever it was installed; " +
					"-print-config-command names another command instead, such as " +
					"sysml-mcp when it is on PATH. The hosts, with the file each reads, are " +
					hostList() + ".",
			},
			Examples: []usage.Example{
				usage.Ex("sysml-mcp -print-config", "List the hosts"),
				usage.Ex("sysml-mcp -print-config opencode", "The mcp block of opencode.json"),
				usage.Ex("sysml-mcp -print-config codex >> ~/.codex/config.toml", "Append the [mcp_servers.sysml] table"),
				usage.Ex("sysml-mcp -print-config cursor -print-config-command sysml-mcp", "A block that finds the server on PATH"),
			},
		}, toolsSection(), {
			Title: "Exit status",
			Lead:  []string{"The status reports how the session ended:"},
			Items: []usage.Item{
				usage.Entry("0", "The protocol was served to its end: the client closed the stream, or the process was told to stop."),
				usage.Entry("1", "The server could not start, or the session ended in a protocol error."),
				usage.Entry("2", "The command line could not be acted on."),
			},
		}, {
			Title:      "Environment",
			ManOnly:    true,
			Items:      usage.BudgetEnvironment(),
			Paragraphs: []string{usage.LegacyPrefixNote, usage.BudgetScopeNote},
		}, {
			Title:   "Reporting bugs",
			ManOnly: true,
			Paragraphs: []string{
				"Report bugs at https://github.com/Open-MBEE/OpenSysML/issues.",
			},
		}},
		SeeAlso: []string{"sysml(1)", "sysml-lsp(1)", "sysml-grpc(1)", "https://github.com/Open-MBEE/OpenSysML"},
	}
}

// toolsSection lists every tool the server can serve, from the catalog that
// also drives registration and the tool_catalog tool, so the manual page
// cannot drift from the tools (`make man` regenerates it, `make man-check`
// fails when it was not).
func toolsSection() usage.Section {
	sec := usage.Section{
		Title: "Tools",
		Lead: []string{
			"The tools, by the group that serves them (-tools), each with the " +
				"sysml-grpc RPC or sysml flag it stands for. The tool_catalog tool " +
				"answers this list to the client, narrowed to the groups served.",
		},
	}
	for _, e := range catalog.Entries(catalog.Groups()...) {
		group := string(e.Group)
		if group == "" {
			group = "every group"
		}
		text := fmt.Sprintf("%s: %s", group, e.Summary)
		if op := e.Operation(); op != "none" {
			text = fmt.Sprintf("%s: %s (%s).", group, strings.TrimSuffix(e.Summary, "."), op)
		}
		sec.Items = append(sec.Items, usage.Entry(e.Name, text))
	}
	return sec
}
