package repl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/notebook"
)

// loadOptions is what a load takes besides its paths: the cells to pick from
// the notebooks among them, every cell when zero.
type loadOptions struct {
	cells notebook.Selection
}

// parseLoadArgs reads the arguments of `%load`: the paths, quoted as parseArgs
// leaves them, and `--cells <selection>` or `--cells=<selection>` anywhere
// among them. A path beginning with `--` is written `./--...`.
func parseLoadArgs(args []string) ([]string, loadOptions, error) {
	var (
		paths []string
		opts  loadOptions
		cells bool
	)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			paths = append(paths, nameText(arg))
			continue
		}
		name, value, inline := strings.Cut(arg, "=")
		if name != "--cells" {
			return nil, loadOptions{}, fmt.Errorf("unknown option %s; %%load takes --cells <n,n-m|tag:<tag>>", name)
		}
		if cells {
			return nil, loadOptions{}, errors.New("--cells given twice; one selection picks the cells of every notebook named")
		}
		if !inline {
			if i+1 == len(args) {
				return nil, loadOptions{}, errors.New("--cells needs a selection: cells by position (1,3-5) or by tag (tag:<tag>)")
			}
			i++
			value = nameText(args[i])
		}
		sel, err := notebook.ParseSelection(value)
		if err != nil {
			return nil, loadOptions{}, fmt.Errorf("--cells %s: %w", value, err)
		}
		opts.cells, cells = sel, true
	}
	return paths, opts, nil
}

// cellName names a notebook's code cell as a source: the notebook's path and
// the cell's position among the code cells, which is how a diagnostic locates it.
func cellName(path string, index int) string {
	return fmt.Sprintf("%s cell %d", path, index)
}

// readNotebook reads the code cells of the notebook at path that sel picks into
// the sources a load submits, one per cell. A cell is loaded for what it
// declares: its `%` command lines and its expressions are blanked rather than
// run, in place so a diagnostic's line is the cell's. The lines report what was
// loaded and what was passed over. A notebook written for another language, or
// in another format, is refused.
func (s *Session) readNotebook(path string, sel notebook.Selection) ([]SourceFile, []string, error) {
	nb, err := notebook.Read(path)
	if err != nil {
		var format *notebook.FormatError
		var language *notebook.LanguageError
		if errors.As(err, &format) || errors.As(err, &language) {
			return nil, nil, err
		}
		return nil, nil, readError(path, err)
	}
	picked, err := sel.Apply(nb)
	if err != nil {
		return nil, nil, err
	}
	report := notebookReport{name: path, of: len(nb.Cells), selection: sel}
	var files []SourceFile
	for _, cell := range picked {
		if cell.Tagged(notebook.SkipTag) {
			report.tagged = append(report.tagged, cell.Index)
			continue
		}
		text := declarationsOf(cell.Source, &report)
		report.cells++
		// A cell of commands and expressions alone is loaded for nothing; it
		// is counted, not submitted.
		if strings.TrimSpace(text) == "" {
			continue
		}
		name := cellName(path, cell.Index)
		if err := reservedName(name); err != nil {
			return nil, nil, err
		}
		report.declarations += len(declaredNames(parser.New(source.NewWithKind(name, []byte(text), source.KindSysML)).ParseFile()))
		files = append(files, SourceFile{Name: name, Text: text, Kind: source.KindSysML, Of: wholeOf(path, sel)})
	}
	// A notebook read whole that contributes no cell still replaces what an
	// earlier reading declared, as a file that now declares nothing does.
	if len(files) == 0 && sel.IsZero() {
		files = append(files, SourceFile{Name: path, Kind: source.KindSysML, Of: path})
	}
	return files, report.lines(), nil
}

// wholeOf is the Of of a source read from path under sel: the path when the
// whole notebook was read, nothing when sel picked cells from it.
func wholeOf(path string, sel notebook.Selection) string {
	if sel.IsZero() {
		return path
	}
	return ""
}

// declarationsOf is the text of a cell as a load submits it: its declarations
// where they stand, every `%` command line and expression line blank. The lines
// passed over are counted into report.
func declarationsOf(cell string, report *notebookReport) string {
	lines := make([]string, strings.Count(cell, "\n")+1)
	for _, stmt := range Statements(cell) {
		count := strings.Count(stmt.Text, "\n") + 1
		switch {
		case stmt.Meta:
			report.metaLines += count
		case isExpression(stmt.Text):
			report.exprLines += count
		default:
			copy(lines[stmt.Line-1:], strings.Split(stmt.Text, "\n"))
		}
	}
	return strings.Join(lines, "\n")
}

// isExpression reports whether a statement Statements split off is an
// expression the prompt would answer, not a declaration it would submit.
func isExpression(text string) bool {
	_, ok := bareExpression(text)
	return ok
}

// notebookReport is what a load says of one notebook.
type notebookReport struct {
	name         string
	selection    notebook.Selection
	cells, of    int // code cells loaded, of those the notebook holds
	declarations int // top-level declarations the loaded cells hold
	metaLines    int // `%` command lines passed over
	exprLines    int // expression lines passed over
	tagged       []int
}

func (r notebookReport) lines() []string {
	head := fmt.Sprintf("loaded %s: %d of %d code cells", r.name, r.cells, r.of)
	if !r.selection.IsZero() {
		head += fmt.Sprintf(" (--cells %s)", r.selection)
	}
	out := []string{head + fmt.Sprintf(", %d %s", r.declarations, plural(r.declarations, "declaration", "declarations"))}
	if r.metaLines > 0 || r.exprLines > 0 {
		var what []string
		if r.metaLines > 0 {
			what = append(what, fmt.Sprintf("%d %% command %s", r.metaLines, plural(r.metaLines, "line", "lines")))
		}
		if r.exprLines > 0 {
			what = append(what, fmt.Sprintf("%d expression %s", r.exprLines, plural(r.exprLines, "line", "lines")))
		}
		out = append(out, "  skipped "+strings.Join(what, " and ")+": a loaded notebook declares; its commands are not run and its expressions not evaluated")
	}
	for _, index := range r.tagged {
		out = append(out, fmt.Sprintf("  skipped cell %d: tagged %s", index, notebook.SkipTag))
	}
	return out
}
