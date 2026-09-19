package usage

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// textWidth is the column the terminal help wraps prose at, narrow enough to
// survive a copied transcript in an 80-column window.
const textWidth = 78

// WriteText writes the terminal help to w: the synopsis, the sections wanted
// first, the flags by group, then every other section not reserved for the man page.
func (d Doc) WriteText(w io.Writer, fs *flag.FlagSet) {
	// PrintDefaults writes to the flag set's own stream, restored after so it
	// does not decide where a later error is reported.
	previous := fs.Output()
	fs.SetOutput(w)
	defer fs.SetOutput(previous)

	d.writeSynopsis(w)
	for _, para := range d.Description {
		fmt.Fprintf(w, "\n%s\n", Wrap(para, textWidth))
	}
	for _, sec := range d.Sections {
		if sec.BeforeOptions && !sec.ManOnly {
			writeSection(w, sec)
		}
	}
	if len(d.Options) == 0 {
		fmt.Fprintf(w, "\nOptions:\n")
		fs.PrintDefaults()
	}
	for _, g := range d.Options {
		fmt.Fprintf(w, "\n%s:\n", g.Title)
		for _, o := range g.Options {
			writeOption(w, fs, o)
		}
	}

	for _, sec := range d.Sections {
		if sec.ManOnly || sec.BeforeOptions {
			continue
		}
		writeSection(w, sec)
	}
}

// WriteHint writes what a misuse is answered with: the synopsis and where the
// help is, rather than the help itself, which would bury the error.
func (d Doc) WriteHint(w io.Writer) {
	d.writeSynopsis(w)
	fmt.Fprintf(w, "Run '%s -help' for the options.\n", d.Command)
}

func (d Doc) writeSynopsis(w io.Writer) {
	for i, form := range d.Synopsis {
		label := "Usage:"
		if i > 0 {
			label = "      "
		}
		fmt.Fprintf(w, "%s %s %s\n", label, d.Command, form)
	}
}

func writeSection(w io.Writer, sec Section) {
	fmt.Fprintf(w, "\n%s:\n", sec.Title)
	for _, para := range sec.Lead {
		fmt.Fprintf(w, "%s\n", Wrap(para, textWidth))
	}
	writeExamples(w, sec.Examples)
	writeItems(w, sec.Items)
	for i, para := range sec.Paragraphs {
		if i > 0 || len(sec.Examples) > 0 || len(sec.Items) > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s\n", Wrap(para, textWidth))
	}
}

// optionColumn is where an option's description starts, beside a spelling that
// fits and under one that does not.
const optionColumn = 30

// writeOption writes the flag's spellings and, wrapped in the column beside
// them, its description with the default the flag package would show.
func writeOption(w io.Writer, fs *flag.FlagSet, o Option) {
	f := fs.Lookup(o.Name)
	text := help(f)
	if !isZeroDefault(f.DefValue) {
		text += fmt.Sprintf(" (default %s)", f.DefValue)
	}
	spelling := "  " + o.spelling()
	margin := strings.Repeat(" ", optionColumn)
	body := indent(Wrap(text, textWidth-optionColumn), margin)
	if len(spelling) < optionColumn-1 {
		// The body opens with the margin; the spelling takes that many of its spaces.
		fmt.Fprintf(w, "%s%s\n", spelling, body[len(spelling):])
	} else {
		fmt.Fprintf(w, "%s\n%s\n", spelling, body)
	}
}

// writeExamples writes one command per line with the comments aligned in a
// column, so a block of them reads as a table.
func writeExamples(w io.Writer, examples []Example) {
	width := commandWidth(examples, textWidth-2)
	for _, ex := range examples {
		folded := foldCommand(ex.Command, textWidth-2)
		if len(folded) > 1 {
			if ex.Comment != "" {
				fmt.Fprintf(w, "  # %s\n", ex.Comment)
			}
			for _, line := range folded {
				fmt.Fprintf(w, "  %s\n", line)
			}
			continue
		}
		if ex.Comment == "" {
			fmt.Fprintf(w, "  %s\n", ex.Command)
			continue
		}
		fmt.Fprintf(w, "  %-*s  # %s\n", width, ex.Command, ex.Comment)
	}
}

// commandWidth is the width the commented example commands are padded to,
// disregarding the ones too wide for a line, which are folded instead.
func commandWidth(examples []Example, limit int) int {
	width := 0
	for _, ex := range examples {
		if ex.Comment != "" && len(ex.Command) > width && len(ex.Command) <= limit {
			width = len(ex.Command)
		}
	}
	return width
}

// foldCommand breaks a command too wide for width at its argument boundaries,
// continuing each line the way a shell reads it.
func foldCommand(command string, width int) []string {
	if len(command) <= width {
		return []string{command}
	}
	const continuation = " \\"
	var lines []string
	line := ""
	for _, word := range strings.Fields(command) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word)+len(continuation) <= width:
			line += " " + word
		default:
			lines = append(lines, line+continuation)
			line = "    " + word
		}
	}
	return append(lines, line)
}

// writeItems writes a labelled list, in a column while the labels are short
// enough for one and with the description indented under its label otherwise,
// as the flag defaults are.
func writeItems(w io.Writer, items []Item) {
	width := 0
	for _, item := range items {
		if len(item.Label) > width {
			width = len(item.Label)
		}
	}
	if width > 20 {
		for _, item := range items {
			fmt.Fprintf(w, "  %s\n", item.Label)
			fmt.Fprintf(w, "%s\n", indent(Wrap(item.Text, textWidth-8), "        "))
		}
		return
	}
	body := textWidth - width - 4
	for _, item := range items {
		text := indent(Wrap(item.Text, body), strings.Repeat(" ", width+4))
		fmt.Fprintf(w, "  %-*s%s\n", width+2, item.Label, strings.TrimLeft(text, " "))
	}
}

// indent prefixes every line of text with prefix.
func indent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// Wrap breaks text into lines of at most width characters, so a sentence
// printed rather than restated still reads as a paragraph.
func Wrap(text string, width int) string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	return strings.Join(append(lines, line), "\n")
}
