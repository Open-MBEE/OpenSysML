package repl

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/modelform"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

const graphsModel = `package Demo {
    private import ScalarValues::*;
    part def Vehicle;
    action def Step { in x : Integer; out y : Integer; }
    action pipeline {
        attribute total : Integer = 0;
        first start;
        action a : Step;
        action b : Step;
        done;
        succession first start then a;
        succession first a then b;
        succession first b then done;
        flow a.y to b.x;
    }
}`

func graphsSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	res := s.Submit(graphsModel)
	for _, d := range res.Diagnostics {
		if d.Severity == diag.SeverityError {
			t.Fatalf("model did not load: %v", res.Diagnostics)
		}
	}
	return s
}

func TestGraphsExportsTheCanonicalForm(t *testing.T) {
	s := graphsSession(t)
	raw, err := s.Graphs("Demo::pipeline")
	if err != nil {
		t.Fatal(err)
	}
	var graphs modelform.Graphs
	if err := json.Unmarshal(raw, &graphs); err != nil {
		t.Fatalf("not graphs JSON: %v\n%s", err, raw)
	}
	// The subject's graph and that of Step, the behavior its steps perform.
	if graphs.Version != modelform.GraphsVersion || graphs.Subject != "Demo::pipeline" || len(graphs.Actions) != 2 {
		t.Errorf("graphs = version %d, subject %q, %d actions", graphs.Version, graphs.Subject, len(graphs.Actions))
	}
	if !strings.Contains(string(raw), `"flows"`) {
		t.Errorf("the flow between a.y and b.x is missing:\n%s", raw)
	}

	out, _, err := s.RunMeta("%graphs Demo::pipeline")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(out, "\n")+"\n" != string(raw) {
		t.Errorf("%%graphs prints other than Graphs returns:\n%s", strings.Join(out, "\n"))
	}
}

// A state without a body and a transition are refused in words, like on the CLI.
func TestGraphsRefusesAVertexOrAnEdgeOfAMachine(t *testing.T) {
	s := NewSession()
	s.Submit("package P { state def Machine { state off; state on; transition off_on first off then on; } }")
	for name, want := range map[string]string{
		"P::Machine::off":    "no lowered graph for the state usage P::Machine::off: a state declared without a body is a vertex of its owner's graph",
		"P::Machine::off_on": "no lowered graph for the action usage P::Machine::off_on: a transition is an edge of its owner's graph",
	} {
		if _, err := s.Graphs(name); err == nil || err.Error() != want {
			t.Errorf("Graphs(%s) = %v, want the error %q", name, err, want)
		}
	}
}

func TestGraphsReportsWhatIsNoBehavior(t *testing.T) {
	s := graphsSession(t)
	for _, test := range []struct{ name, want string }{
		{"Demo::Vehicle", "no lowered graph for the part definition Demo::Vehicle: the subject must be an action or a state machine"},
		{"Vehicle", "Vehicle: no lowered graph for the part definition Demo::Vehicle: the subject must be an action or a state machine"},
		{"Demo::'Vehicle'", "Demo::'Vehicle': no lowered graph for the part definition Demo::Vehicle: the subject must be an action or a state machine"},
		{"Demo::Missing", "unresolved reference: Demo::Missing"},
	} {
		if _, err := s.Graphs(test.name); err == nil || err.Error() != test.want {
			t.Errorf("Graphs(%s) = %v, want the error %q", test.name, err, test.want)
		}
		out, _, err := s.RunMeta("%graphs " + test.name)
		if err != nil {
			t.Fatalf("%%graphs %s failed the command: %v", test.name, err)
		}
		if len(out) != 1 || out[0] != "error: "+test.want {
			t.Errorf("%%graphs %s = %q, want the one line %q", test.name, out, "error: "+test.want)
		}
	}
	out, _, _ := s.RunMeta("%graphs")
	if len(out) != 1 || out[0] != "usage: %graphs <name>" {
		t.Errorf("%%graphs alone = %q", out)
	}
}
