package repl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/modelform"
)

// Graphs exports the lowered graph of the named action or state machine, and
// of every behavior it performs, as the canonical graphs:1 JSON an external
// engine is sent. A name that is no behavior is modelform.ErrGraphsSubject.
func (s *Session) Graphs(name string) ([]byte, error) {
	defer s.enter()()
	return s.graphs(name)
}

func (s *Session) graphs(name string) ([]byte, error) {
	sym, fqn, err := s.lookupSymbol(name)
	if err != nil {
		return nil, err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRuntimeInit, err)
	}
	graphs, err := modelform.GraphsOf(ctx.Model(), sym)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", notationName(fqn), err)
	}
	return modelform.MarshalGraphs(graphs)
}

// doGraphs prints the graphs the way every other name-taking command reports:
// a name the session cannot find, or one that is no behavior, is a line
// rather than a failure of the command.
func (s *Session) doGraphs(name string) ([]string, bool, error) {
	raw, err := s.graphs(name)
	if err != nil {
		if errors.Is(err, errRuntimeInit) {
			return nil, false, err
		}
		return []string{"error: " + err.Error()}, false, nil
	}
	return strings.Split(strings.TrimRight(string(raw), "\n"), "\n"), false, nil
}
