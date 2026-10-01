package repl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
)

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
	lines, err := replext.Notation().Save(sessionOrigin, []byte(src), path)
	if err != nil {
		return nil, false, err
	}
	return lines, false, nil
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
