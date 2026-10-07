package main

import (
	"flag"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/usage"
)

const kernelCommand = "sysml-jupyter-kernel"

// options is what the command line resolves to.
type options struct {
	connectionFile  string
	install         bool
	user            bool
	prefix          string
	name            string
	displayName     string
	printKernelspec bool
	verbose         bool
	showVersion     bool
	showMan         bool
}

// registerFlags declares the command's flags on fs, so a run, the help and the
// man page all read one declaration of each.
func registerFlags(fs *flag.FlagSet) *options {
	var o options
	fs.StringVar(&o.connectionFile, "connection-file", "", "Serve the connection Jupyter describes in this file (Jupyter passes it)")
	fs.BoolVar(&o.install, "install", false, "Write the kernelspec that starts this binary where Jupyter finds kernels, and exit")
	fs.BoolVar(&o.user, "user", false, "With -install, install for this user alone (the default without -prefix)")
	fs.StringVar(&o.prefix, "prefix", "", "With -install, install under this prefix (PREFIX/share/jupyter/kernels), as into a virtual environment")
	fs.StringVar(&o.name, "name", defaultKernelName, "With -install, the kernelspec's directory name, which notebooks bind to")
	fs.StringVar(&o.displayName, "display-name", defaultDisplayName, "With -install, the name front ends list the kernel under")
	fs.BoolVar(&o.printKernelspec, "print-kernelspec", false, "Write the kernel.json -install would write to stdout, and exit")
	fs.BoolVar(&o.verbose, "verbose", false, "Log the protocol traffic that is dropped or fails to stderr")
	fs.BoolVar(&o.showVersion, "version", false, "Show version and exit")
	fs.BoolVar(&o.showMan, "man", false, "Write this command's manual page, in roff, to stdout and exit")
	return &o
}

// docFlags is a flag set holding the command's flags, for rendering the help or
// the man page without a command line to parse.
func docFlags() *flag.FlagSet {
	fs := flag.NewFlagSet(kernelCommand, flag.ContinueOnError)
	registerFlags(fs)
	return fs
}

// doc describes the kernel for both the terminal help and the man page.
func doc() usage.Doc {
	return usage.Doc{
		Command:    kernelCommand,
		ManSection: 1,
		Summary:    "serve SysML v2 to Jupyter notebooks",
		Synopsis:   []string{"-connection-file FILE [-verbose]", "-install [-user | -prefix DIR] [-name NAME] [-display-name NAME]", "-print-kernelspec", "-version | -man"},
		Description: []string{
			kernelCommand + " is a Jupyter kernel over the OpenSysML REPL session: a " +
				"notebook cell is read as the prompt reads lines, so declarations " +
				"accumulate into one session model, a bare expression is evaluated, " +
				"and every % command of the sysml REPL runs as it does there. Views " +
				"rendered as Mermaid, Markdown, DOT or tables are shown in that form " +
				"by the front end; everything else prints as the prompt prints it.",
			"Jupyter starts the kernel itself, passing -connection-file; -install " +
				"writes the kernelspec that tells it how. Interrupting a cell stops " +
				"the run it drives at its next step.",
		},
		Sections: []usage.Section{{
			Title: "Installing",
			Examples: []usage.Example{
				usage.Ex(kernelCommand+" -install", "Register the kernel for this user"),
				usage.Ex(kernelCommand+" -install -prefix \"$VIRTUAL_ENV\"", "Register it in a virtual environment"),
				usage.Ex(kernelCommand+" -print-kernelspec", "Show the kernel.json a package would ship"),
				usage.Ex("jupyter kernelspec list", "See that Jupyter finds it"),
			},
			Paragraphs: []string{
				"The kernelspec names this binary by its absolute path, so moving the " +
					"binary needs another -install. The pip and conda package " +
					"jupyter-opensysml-kernel installs a kernelspec of its own, holding " +
					"the binary, so it needs no -install.",
			},
		}, {
			Title: "Exit status",
			Lead:  []string{"The status reports how the kernel ended:"},
			Items: []usage.Item{
				usage.Entry("0", "The front end shut it down, or it reported what was asked of it."),
				usage.Entry("1", "It could not serve: a channel could not be bound."),
				usage.Entry("2", "The command line or the connection file could not be acted on."),
			},
		}, {
			Title:      "Environment",
			ManOnly:    true,
			Items:      append(append(usage.BudgetEnvironment(), usage.JobsEnvironment()...), usage.ToolEnvironment()...),
			Paragraphs: []string{usage.LegacyPrefixNote, usage.BudgetScopeNote},
		}, {
			Title:   "Reporting bugs",
			ManOnly: true,
			Paragraphs: []string{
				"Report bugs at https://github.com/Open-MBEE/OpenSysML/issues.",
			},
		}},
		SeeAlso: []string{"sysml(1)", "https://github.com/Open-MBEE/OpenSysML"},
	}
}
