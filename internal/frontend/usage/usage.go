// Package usage describes a command's help as data, so the help a binary
// prints and the man page shipped for it are rendered from one source and
// cannot drift.
package usage

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
	// Sections are the blocks printed after the options, in order.
	Sections []Section
	// SeeAlso names related pages as "name(section)", plus any URLs.
	SeeAlso []string
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

// Section is the manual section the page belongs in, 1 where none is stated.
func (d Doc) Section() int {
	if d.ManSection == 0 {
		return 1
	}
	return d.ManSection
}
