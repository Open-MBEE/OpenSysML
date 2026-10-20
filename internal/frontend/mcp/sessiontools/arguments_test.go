package sessiontools

import (
	"slices"
	"testing"
)

const twoDrawsModel = `package Two {
	private import ScalarValues::Real;
	private import RandomFunctions::uniform;
	calc def Pair { in a : Real; in b : Real; return r : Real = a - b; }
	action def Diff { in a : Real; in b : Real; out d : Real = a - b; }
}
`

// A seeded session_run binds its argument list from one evaluation in one run:
// the second argument's draw continues the first's stream, so the two differ,
// their difference is what one expression drawing twice under the seed gives,
// and the same seed reproduces the whole invocation.
func TestSeededRunArgumentsDrawSuccessively(t *testing.T) {
	cs := connect(t, t.TempDir())
	must(t, cs, "session_submit", args("text", twoDrawsModel), nil)
	must(t, cs, "repl_command", args("line", "%seed 7"), nil)

	var ev EvalResult
	must(t, cs, "session_eval", args("expression", "uniform(0.0, 1.0) - uniform(0.0, 1.0)"), &ev)
	if ev.Text == "0.0" {
		t.Fatalf("one expression drawing twice drew the same value: %q", ev.Text)
	}

	draws := []string{"uniform(0.0, 1.0)", "uniform(0.0, 1.0)"}
	var first, again Verdict
	must(t, cs, "session_run", args("kind", "calc", "name", "Two::Pair", "arguments", draws), &first)
	if first.Status != "holds" || len(first.Values) != 1 || first.Values[0].Value != "["+ev.Text+"]" {
		t.Errorf("seeded run = %+v, want result %s (0.0 means both arguments drew the seed's first value)", first, ev.Text)
	}
	must(t, cs, "session_run", args("kind", "calc", "name", "Two::Pair", "arguments", draws), &again)
	if len(again.Values) != 1 || again.Values[0] != first.Values[0] {
		t.Errorf("the same seeded argument list ran differently: %+v then %+v", first.Values, again.Values)
	}

	var action Verdict
	must(t, cs, "session_run", args("kind", "action", "name", "Two::Diff", "arguments", draws), &action)
	if action.Status != "holds" || !slices.Contains(action.Values, NamedValue{Name: "d", Value: "[" + ev.Text + "]"}) {
		t.Errorf("seeded action run = %+v, want d = [%s]", action, ev.Text)
	}
}
