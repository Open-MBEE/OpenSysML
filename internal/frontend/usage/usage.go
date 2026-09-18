// Package usage describes a command's help as data, so the help a binary
// prints and the man page shipped for it are rendered from one source and
// cannot drift.
package usage

import (
	"flag"
	"fmt"
	"sort"
	"strings"
)

// Doc is everything the help and the man page say about one command, apart from
// the flags themselves: those are read from the command's own flag set, which is
// the only place a flag is declared.
type Doc struct {
	// Command is the program name, as invoked and as the man page is titled.
	Command string
	// ManSection is the manual section the page belongs in.
	ManSection int
	// Summary is the one-line description of the command, for the NAME section.
	Summary string
	// Synopsis holds the invocation forms, without the program name.
	Synopsis []string
	// Description holds the paragraphs a reader needs before the flags.
	Description []string
	// Options groups the flags by the task they serve, in the order printed.
	// Empty, the flags are listed as the flag set declares them.
	Options []OptionGroup
	// Sections are the blocks printed after the options, in order.
	Sections []Section
	// SeeAlso names related pages as "name(section)", plus any URLs.
	SeeAlso []string
}

// OptionGroup is one titled block of the option list.
type OptionGroup struct {
	// Title heads the block: "Checking a model", "Deprecated".
	Title   string
	Options []Option
}

// Option is one entry of the option list: a flag with its alternative spellings.
type Option struct {
	// Name is the flag as declared, without its dash.
	Name string
	// Aliases are the other spellings the flag set declares for the same
	// setting, listed with the name rather than as entries of their own.
	Aliases []string
	// Arg is the placeholder the flag's argument is shown as, "<name>"; one
	// written "[=<name>]" is optional and attaches to the flag. Empty for a
	// boolean flag.
	Arg string
}

// Section is one titled block of the help: worked examples, prose, a labelled
// list, or a combination, rendered in that order.
type Section struct {
	// Title heads the block: "Examples", "Conversion".
	Title string
	// Lead is the prose introducing what the block lists.
	Lead []string
	// Examples are commands with the comment explaining each.
	Examples []Example
	// Items are labelled descriptions: environment variables, exit statuses.
	Items []Item
	// Paragraphs is the prose closing the block.
	Paragraphs []string
	// BeforeOptions prints the block ahead of the option list in the terminal
	// help, for the examples a reader wants before any flag.
	BeforeOptions bool
	// ManOnly keeps the block out of the terminal help, for reference material
	// that belongs in a page a reader consults rather than in a flag summary.
	ManOnly bool
}

// Example is one invocation and what it does.
type Example struct {
	Command string
	Comment string
}

// Item is one labelled entry of a list: a variable, a status, a file.
type Item struct {
	Label string
	Text  string
}

// Ex is an example invocation and the comment explaining it.
func Ex(command, comment string) Example {
	return Example{Command: command, Comment: comment}
}

// Entry is one labelled description of a list.
func Entry(label, text string) Item {
	return Item{Label: label, Text: text}
}

// Opt is an option list entry for the flag name, with the argument placeholder
// arg ("" for a boolean flag) and any alternative spellings.
func Opt(name, arg string, aliases ...string) Option {
	return Option{Name: name, Arg: arg, Aliases: aliases}
}

// Section is the manual section the page belongs in, 1 where none is stated.
func (d Doc) Section() int {
	if d.ManSection == 0 {
		return 1
	}
	return d.ManSection
}

// CheckOptions reports a flag the groups leave out or name twice, or an option
// the flag set does not declare, so a flag cannot go undocumented.
func (d Doc) CheckOptions(fs *flag.FlagSet) error {
	if len(d.Options) == 0 {
		return nil
	}
	seen := map[string]bool{}
	for _, g := range d.Options {
		for _, o := range g.Options {
			for _, name := range append([]string{o.Name}, o.Aliases...) {
				if fs.Lookup(name) == nil {
					return fmt.Errorf("option -%s is documented but not declared", name)
				}
				if seen[name] {
					return fmt.Errorf("option -%s is documented twice", name)
				}
				seen[name] = true
			}
		}
	}
	var err error
	fs.VisitAll(func(f *flag.Flag) {
		if err == nil && !seen[f.Name] {
			err = fmt.Errorf("flag -%s is declared but in no option group", f.Name)
		}
	})
	return err
}

// names lists the option's spellings, shorthand first: -e, -eval.
func (o Option) names() []string {
	names := append([]string{o.Name}, o.Aliases...)
	sort.SliceStable(names, func(i, j int) bool { return len(names[i]) < len(names[j]) })
	return names
}

// spelling is the option as the terminal help writes it: "-e, -eval <expr>",
// "-satisfy[=<name>]".
func (o Option) spelling() string {
	text := "-" + strings.Join(o.names(), ", -")
	switch {
	case o.Arg == "":
	case strings.HasPrefix(o.Arg, "["):
		text += o.Arg
	default:
		text += " " + o.Arg
	}
	return text
}

// help is the flag's usage string with its backquoted placeholder unquoted.
func help(f *flag.Flag) string {
	_, text := flag.UnquoteUsage(f)
	return text
}
