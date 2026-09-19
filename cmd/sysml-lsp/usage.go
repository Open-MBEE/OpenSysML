package main

import (
	"flag"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/usage"
)

const lspCommand = "sysml-lsp"

// options is what the command line resolves to.
type options struct {
	strict      bool
	showVersion bool
	showHelp    bool
	showMan     bool
}

// registerFlags declares the server's flags on fs, so a run, the help and the
// man page all read one declaration of each. -h/-help are flags rather than the
// flag package's own so that help asked for is a result on stdout, and -stdio
// names the only transport there is: the standard clients pass it, so it is
// accepted and documented rather than rejected.
func registerFlags(fs *flag.FlagSet) *options {
	var o options
	fs.BoolVar(&o.strict, "strict", false,
		"Serve strict conformance from the start: notation no pinned SysML v2 production admits is an error (a client may also set the strictConformance setting)")
	fs.Bool("stdio", false, "Serve over stdin/stdout (the only transport; accepted for clients that name it)")
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
	fs := flag.NewFlagSet(lspCommand, flag.ContinueOnError)
	registerFlags(fs)
	return fs
}

// doc describes the server for both the terminal help and the man page, so a
// setting documented for one is documented for the other.
func doc() usage.Doc {
	return usage.Doc{
		Command:    lspCommand,
		ManSection: 1,
		Summary:    "Language Server for SysML v2 and KerML",
		Synopsis:   []string{"[options]"},
		Description: []string{
			"sysml-lsp serves the Language Server Protocol for SysML v2 and KerML " +
				"models: diagnostics as a document is edited, hover, go to " +
				"definition, references, completion, symbols, rename and " +
				"formatting; " + protocolMessage + ".",
			"The models an editor opens are managed as one workspace, so a " +
				"declaration in one document resolves against the others, and the " +
				"standard library is the copy embedded in this binary unless " +
				"OPENSYSML_LIBRARY_PATH names another.",
		},
		Sections: []usage.Section{{
			Title: "Starting the server",
			Examples: []usage.Example{
				usage.Ex(lspCommand, "What an editor runs"),
				usage.Ex("sysml-lsp -stdio", "The same, for a client that names the transport"),
				usage.Ex("sysml-lsp -strict", "Serve strict conformance from the start"),
			},
			Paragraphs: []string{
				"A client that reads a strictConformance setting can ask for strict " +
					"conformance instead, which then applies from the moment it is set.",
			},
		}, {
			Title: "Exit status",
			Lead:  []string{"The status reports how the session ended:"},
			Items: []usage.Item{
				usage.Entry("0", "The protocol was served to its end: the client asked to shut down and exit, or closed the stream."),
				usage.Entry("1", "The session ended without one: an exit with no shutdown, or a protocol error."),
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
		SeeAlso: []string{"sysml(1)", "sysml-grpc(1)", "https://github.com/Open-MBEE/OpenSysML"},
	}
}
