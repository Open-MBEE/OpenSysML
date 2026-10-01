// Package notation registers the REPL's %print and %save: the session's model
// written as SysML notation, or converted to the format a file name selects.
package notation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

func init() { replext.RegisterNotation(writer{}) }

// errPrefix opens the line a command writes when it fails.
const errPrefix = "error: "

// formatAdvice is the remedy for a save path whose format cannot be told. The
// prompt has no format flag, so it names the file name remedy first and the
// command line's flag alongside it, in the words the command line uses.
const formatAdvice = convert.ExtensionAdvice + ", or pass -convert on the command line"

type writer struct{}

// Print writes the buffer through the writer a `.sysml` save writes with, so
// the prompt shows what a save would hold. RDF is another format, and none of
// it is reported here.
func (writer) Print(origin string, src []byte) []string {
	out, syntax, err := convert.ConvertTolerant(origin, src, convert.FormatSysML, convert.FormatSysML)
	if err != nil {
		return []string{errPrefix + err.Error()}
	}
	return append(printWarnings(syntax), notationLines(out)...)
}

// PrintElement prints one element and its body: the source its declaration
// spans, with the notes and comments written above it.
func (writer) PrintElement(file *source.SourceFile, span source.Span, shown string) []string {
	out, syntax, err := convert.SysMLElement(file, span)
	if err != nil {
		if errors.Is(err, convert.ErrNoNotation) {
			return []string{fmt.Sprintf("no notation to print for %s: its declaration spans no source", shown)}
		}
		return []string{errPrefix + err.Error()}
	}
	lines := append(printWarnings(syntax), notationLines(out)...)
	if len(lines) == 0 {
		return []string{fmt.Sprintf("no notation to print for %s: its declaration spans no source", shown)}
	}
	return lines
}

// Save writes the model to path. The format follows the file extension:
// `.sysml`/`.kerml` writes the notation, `.ttl` writes RDF Turtle, `.json` the
// API element form.
//
// A session that does not fully parse is still saved as notation, with its
// syntax errors reported as warnings: that save writes the user's own text back
// through the formatter, so it is exactly as valid as what they typed, and
// refusing it would leave the only copy inside a REPL they are about to close.
// A `.ttl` save of the same session is refused, because a graph built from a
// tree the parser recovered would be quietly missing declarations.
func (writer) Save(origin string, src []byte, path string) ([]string, error) {
	format, err := convert.FormatOfPath(path)
	if err != nil {
		return []string{"error: " + convert.Advise(err, formatAdvice).Error()}, nil
	}
	var lines []string
	// Reported before the conversion, so a refused .ttl save carries it too.
	if convert.IsExperimental(convert.FormatSysML, format) {
		lines = append(lines, "note: "+convert.ExperimentalNotice)
	}
	// Diagnostics are positions in the session buffer, not in the file about to
	// be written, so they are labelled as such.
	out, syntax, err := convert.ConvertTolerant(origin, src, convert.FormatSysML, format)
	if err != nil {
		return append(lines, "error: "+err.Error()), nil
	}
	if syntax != nil {
		lines = append(lines, strings.Split("warning: "+syntax.Error(), "\n")...)
		lines = append(lines, "warning: the file is saved as typed; fix these and save again")
	}
	// WriteFile's errors already name the path, so they are not prefixed again.
	replaced, err := export.WriteFile(path, out)
	if err != nil {
		return nil, err
	}
	saved := fmt.Sprintf("saved %d bytes of %s to %s", len(out), format, path)
	if replaced {
		saved += " (replaced the existing file)"
	}
	return append(lines, saved), nil
}

// printWarnings reports the syntax errors of a printed buffer, in the wording a
// save reports them with: the notation is printed as typed either way.
func printWarnings(syntax *convert.SyntaxError) []string {
	if syntax == nil {
		return nil
	}
	lines := strings.Split("warning: "+syntax.Error(), "\n")
	return append(lines, "warning: the model is printed as typed; fix these and print again")
}

// notationLines splits written notation into prompt lines, dropping the trailing
// blank line the writer ends a document with.
func notationLines(out []byte) []string {
	text := strings.TrimRight(string(out), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
