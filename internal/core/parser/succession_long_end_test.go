package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// longSourceEnd is a connector end whose multiplicity holds well over a hundred
// tokens, so `then` sits far past any fixed lookahead window.
func longSourceEnd() string {
	args := make([]string, 80)
	for i := range args {
		args[i] = fmt.Sprintf("a%d", i)
	}
	return "[f(" + strings.Join(args, ", ") + ")] src"
}

// longSourceName is a transition source spelled as a qualified name of over a
// hundred tokens; a transition's source carries no multiplicity.
func longSourceName() string {
	parts := make([]string, 80)
	for i := range parts {
		parts[i] = fmt.Sprintf("n%d", i)
	}
	return strings.Join(parts, "::") + "::src"
}

func lastStateMember(t *testing.T, src string) ast.Node {
	t.Helper()
	p := New(source.New("long.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}
	def := root.Members[0].(*ast.Membership).Member.(*ast.Definition)
	last := def.Members[len(def.Members)-1]
	if m, ok := last.(*ast.Membership); ok {
		return m.Member
	}
	return last
}

// How far `then` sits from `first` does not decide what a state body's `first`
// is: the two-ended form is a succession, the guarded one a transition.
func TestStateBodyFirstClassifiesPastAnyLookaheadWindow(t *testing.T) {
	end, name := longSourceEnd(), longSourceName()
	for _, tc := range []struct {
		name, member string
		want         func(ast.Node) bool
	}{
		{
			"keyword-less succession",
			"first " + end + " then dst;",
			func(n ast.Node) bool {
				u, ok := n.(*ast.Usage)
				return ok && u.Kind == ast.UsageSuccession && len(u.ConnectorEnds) == 2
			},
		},
		{
			"keyword succession",
			"succession first " + end + " then dst;",
			func(n ast.Node) bool {
				u, ok := n.(*ast.Usage)
				return ok && u.Kind == ast.UsageSuccession && len(u.ConnectorEnds) == 2
			},
		},
		{
			"keyword-less guarded transition",
			"first " + name + " if g then dst;",
			func(n ast.Node) bool { tm, ok := n.(*ast.TransitionMember); return ok && tm.Guard != nil },
		},
		{
			"keyword guarded transition",
			"succession first " + name + " if g then dst;",
			func(n ast.Node) bool { tm, ok := n.(*ast.TransitionMember); return ok && tm.Guard != nil },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := lastStateMember(t, "state def M {\n\tstate src;\n\tstate dst;\n\t"+tc.member+"\n}")
			if !tc.want(got) {
				t.Errorf("%s parsed as %s", tc.member[:40], ast.Dump(got))
			}
		})
	}
}
