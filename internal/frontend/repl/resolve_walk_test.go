package repl

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func resolvedParses(n int) []parsed {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "package P%d { attribute a%d = %d; }\n", i, i, i)
	}
	root := parser.New(source.New("typed", []byte(sb.String()))).ParseFile()
	return []parsed{{root: root}}
}

// A submission that leaves nothing unresolved looks for no root namespace, so
// its text is not walked: the work does not grow with the text.
func TestResolvingNothingDoesNotWalkTheText(t *testing.T) {
	s, dir := workspaceSession(t)
	search := &rootSearch{find: s.workspaceIndex().Declaring, where: "under " + dir, onDemand: true}
	cost := func(n int) float64 {
		parses := resolvedParses(n)
		return testing.AllocsPerRun(20, func() {
			s.resolveReferences(Result{}, parses, search)
		})
	}
	if small, large := cost(50), cost(100); large > small {
		t.Errorf("resolving 100 resolved packages allocates %v times, 50 packages %v: the text is walked", large, small)
	}
}
