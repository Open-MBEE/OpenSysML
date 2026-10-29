package repl

import (
	"fmt"

	sedit "github.com/Open-MBEE/OpenSysML/internal/check/edit"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// ApplyModelEdit applies ops to the session document named name, as the
// editor's panel applies its edits, and answers each document they rewrite.
func (s *Session) ApplyModelEdit(name string, ops []sedit.Operation) (*model.EditResult, error) {
	defer s.enter()()
	res, _, ok, err := s.ws.ApplyEdit(name, ops, nil)
	if err == nil && !ok {
		err = fmt.Errorf("%s is not a document of the session", name)
	}
	return res, err
}
