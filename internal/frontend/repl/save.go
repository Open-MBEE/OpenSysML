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
	lines, _, err := s.saveModel(path)
	return lines, false, err
}

// saveModel is doSave with whether a file was written: not for an empty
// session, a directory, or a format the save refuses.
func (s *Session) saveModel(path string) ([]string, bool, error) {
	src, path, lines, ok := s.saveSource(path)
	if !ok {
		return lines, false, nil
	}
	return replext.Notation().Save(sessionOrigin, src, path, s.fileWriter)
}

// savePreview is the file saveModel would write to path and the lines it would
// print first, written nowhere; ok is false where the save would write nothing.
func (s *Session) savePreview(path string) ([]byte, []string, bool) {
	src, path, lines, ok := s.saveSource(path)
	if !ok {
		return nil, lines, false
	}
	return replext.Notation().Preview(sessionOrigin, src, path)
}

// saveSource is what a save of the session to path converts, with the path
// expanded, or the lines saying why nothing would be written.
func (s *Session) saveSource(path string) ([]byte, string, []string, bool) {
	// The text as typed, not the analyzed buffer: work the parser could not read
	// is masked out of that buffer and is exactly what this save exists for.
	src := s.savedText()
	if strings.TrimSpace(src) == "" {
		return nil, path, []string{"nothing to save: the session is empty"}, false
	}
	path = expandHome(path)
	if err := s.guardFiles([]string{path}, false); err != nil {
		return nil, path, []string{"error: " + err.Error()}, false
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		// Not a format complaint: the extension, if any, is beside the point.
		return nil, path, []string{fmt.Sprintf("error: %s is a directory: name the file to write inside it", path)}, false
	}
	return []byte(src), path, nil, true
}

// ExpandHome expands a leading `~` or `~/` to the user's home directory, which
// the prompt is expected to understand even though no shell has been through
// the line; a host resolving paths for the prompt expands them the same way.
func ExpandHome(path string) string { return expandHome(path) }

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
