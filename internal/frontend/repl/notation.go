package repl

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
)

// errNoNotation is the error of a notation command in a build that does not
// link the notation writer.
var errNoNotation = errors.New("this build does not link the notation writer, so the model cannot be printed or saved")

// Print returns the session's model as SysML notation, whole when name is empty
// and the named element with its body otherwise, as %print prints it. A name
// the session does not declare is the error.
func (s *Session) Print(name string) ([]string, error) {
	if replext.Notation() == nil {
		return nil, errNoNotation
	}
	defer s.enter()()
	lines, _, err := s.doPrint(name)
	if err != nil {
		return nil, err
	}
	return lines, failedLine(lines)
}

// Save writes the session's model to path as %save does: `.sysml` notation, or
// `.ttl`/`.json` RDF, and returns the lines %save prints. A path that cannot be
// written is the error.
func (s *Session) Save(path string) ([]string, bool, error) {
	if replext.Notation() == nil {
		return nil, false, errNoNotation
	}
	defer s.enter()()
	lines, written, err := s.saveModel(path)
	if err != nil {
		return nil, false, err
	}
	return lines, written, failedLine(lines)
}

// SavePreview is the file Save would write to path, in the format its
// extension names, and the lines Save would print before writing it, with
// nothing written. A save that would be refused is the error, with its lines.
// An empty session previews as it saves: no content, and a line saying so.
func (s *Session) SavePreview(path string) ([]byte, []string, error) {
	if replext.Notation() == nil {
		return nil, nil, errNoNotation
	}
	defer s.enter()()
	content, lines, _ := s.savePreview(path)
	return content, lines, failedLine(lines)
}
