package repl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// formatAdvice is the remedy for a save path whose format cannot be told. The
// prompt has no format flag, so it names the file name remedy first and the
// command line's flag alongside it, in the words the command line uses.
const formatAdvice = convert.ExtensionAdvice + ", or pass -convert on the command line"

// doSave writes the session's model to path. The format follows the file
// extension: `.sysml`/`.kerml` writes the notation, `.ttl` writes RDF Turtle,
// `.json` the API element form.
//
// A session that does not fully parse is still saved as notation, with its
// syntax errors reported as warnings: that save writes the user's own text back
// through the formatter, so it is exactly as valid as what they typed, and
// refusing it would leave the only copy inside a REPL they are about to close.
// A `.ttl` save of the same session is refused, because a graph built from a
// tree the parser recovered would be quietly missing declarations.
func (s *Session) doSave(path string) ([]string, bool, error) {
	// The text as typed, not the analyzed buffer: work the parser could not read
	// is masked out of that buffer and is exactly what this save exists for.
	src := s.text()
	if strings.TrimSpace(src) == "" {
		return []string{"nothing to save: the session is empty"}, false, nil
	}
	path = expandHome(path)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		// Not a format complaint: the extension, if any, is beside the point.
		return []string{fmt.Sprintf("error: %s is a directory: name the file to write inside it", path)}, false, nil
	}
	format, err := convert.FormatOfPath(path)
	if err != nil {
		return []string{"error: " + convert.Advise(err, formatAdvice).Error()}, false, nil
	}
	var lines []string
	// Reported before the conversion, so a refused .ttl save carries it too.
	if convert.IsExperimental(convert.FormatSysML, format) {
		lines = append(lines, "note: "+convert.ExperimentalNotice)
	}
	// Diagnostics are positions in the session buffer, not in the file about to
	// be written, so they are labelled as such.
	out, syntax, err := convert.ConvertTolerant(sessionOrigin, []byte(src), convert.FormatSysML, format)
	if err != nil {
		return append(lines, "error: "+err.Error()), false, nil
	}
	if syntax != nil {
		lines = append(lines, strings.Split("warning: "+syntax.Error(), "\n")...)
		lines = append(lines, "warning: the file is saved as typed; fix these and save again")
	}
	// WriteFile's errors already name the path, so they are not prefixed again.
	replaced, err := export.WriteFile(path, out)
	if err != nil {
		return nil, false, err
	}
	saved := fmt.Sprintf("saved %d bytes of %s to %s", len(out), format, path)
	if replaced {
		saved += " (replaced the existing file)"
	}
	return append(lines, saved), false, nil
}

// expandHome expands a leading `~` or `~/` to the user's home directory, which
// the prompt is expected to understand even though no shell has been through
// the line.
func expandHome(path string) string {
	// Either separator: `~/` is what a user types even where the separator is
	// a backslash.
	if path != "~" && !(len(path) > 1 && path[0] == '~' && os.IsPathSeparator(path[1])) {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}
