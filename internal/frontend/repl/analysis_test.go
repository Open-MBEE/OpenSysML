package repl

import "testing"

// analysisModel declares analysis cases %analysis runs: a usage binding its own
// subject, a definition whose subject and inputs a run supplies, a case with a
// then-sequenced body, and a usage that is a feature of a part.
const analysisModel = `package An {
	private import ScalarValues::*;
	part def Ship { attribute cost : Real = 5.0; attribute other : Real = 7.0; }
	calc def Sum { in a : Real; in b : Real; return : Real = a + b; }
	analysis def Priced {
		subject s : Ship;
		in tax : Real;
		in limit : Real default = 100.0;
		objective { require constraint { total <= limit } }
		out total : Real = Sum(s.cost, s.other) * (1.0 + tax);
	}
	analysis def Stepped {
		subject s : Ship;
		action base { out v : Real = s.cost; }
		then action doubled { in v : Real = base.v; out w : Real = v * 2.0; }
		return : Real = doubled.w;
	}
	part ship : Ship;
	analysis shipCost : Priced { subject s = ship; in tax = 0.0; }
	analysis dear : Priced { subject s = ship; in tax = 0.5; in limit = 10.0; }
	analysis plain { out x : Real = 1.0 + 2.0; }
	part def Holder { analysis inner : Priced { subject s = h; in tax = 0.0; } part h : Ship; }
}`

func analysisSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(analysisModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	return s
}

// An analysis usage runs from its own bindings, reporting each output it
// declares and what its objective decided.
func TestAnalysisUsageRuns(t *testing.T) {
	s := analysisSession(t)
	wants(t, run(t, s, "%analysis An::shipCost"), "✓ An::shipCost", "total = 12.0", "objective obj: satisfied")
	wants(t, run(t, s, "%analysis An::plain"), "✓ An::plain", "x = 3.0")
	wants(t, run(t, s, "%analysis An::dear"), "✗ An::dear", "total = 18.0", "objective obj: not satisfied: total <= limit")
}

// A definition is run on an object named as its subject, with positional and
// named arguments for its inputs; an input with a default may be left out.
func TestAnalysisDefinitionOnAnObject(t *testing.T) {
	s := analysisSession(t)
	run(t, s, "%instantiate An::ship")
	wants(t, run(t, s, "%analysis An::Priced(0.5) An::ship"), `✓ An::Priced(0.5) on object #1 of "An::ship"`, "total = 18.0")
	wants(t, run(t, s, "%analysis An::Priced(tax = 0.25) #1"), "✓ An::Priced(tax = 0.25) on object #1", "total = 15.0")
	wants(t, run(t, s, "%analysis An::Priced(0.5, limit = 10.0) An::ship"), "✗ An::Priced(0.5, limit = 10.0)", "not satisfied")
	wants(t, run(t, s, "%analysis An::Stepped An::ship"), "✓ An::Stepped", "result = 10.0")
}

// A usage owned by a part def is a feature of the object the session holds for
// it, so it is run on that object as a constraint is checked on its carrier.
func TestAnalysisUsageOfAnObject(t *testing.T) {
	s := analysisSession(t)
	run(t, s, "%instantiate An::Holder")
	wants(t, run(t, s, "%analysis An::Holder::inner"), "✓ An::Holder::inner", "total = 12.0")
}

// What stops a run is reported as an error naming the case, never as a value.
func TestAnalysisErrors(t *testing.T) {
	s := analysisSession(t)
	wants(t, run(t, s, "%analysis"), "usage: %analysis <name>[(<args>)] [<object>]")
	wants(t, run(t, s, "%analysis An::Priced(0.5)"), "error:", "analysis An::Priced: s subject is unbound")
	wants(t, run(t, s, "%analysis An::Priced An::ship"), "error:", `no instance of "An::ship"`)
	wants(t, run(t, s, "%analysis An::Sum"), "error: not an analysis case: An::Sum is a calc def")
	wants(t, run(t, s, "%analysis An::nosuch"), "unresolved reference: An::nosuch")
	wants(t, run(t, s, "%analysis An::Priced(0.5"), `error: argument list "(0.5" is not closed`)
	wants(t, run(t, s, "%analysis An::Priced 1.0 2.0"), "error:", "does not name one object")
	run(t, s, "%instantiate An::ship")
	wants(t, run(t, s, "%analysis An::Priced(nosuch = 1.0) An::ship"), "error:", `no input parameter "nosuch"`)
	// %calc keeps refusing a case, pointing at the command that runs one.
	wants(t, run(t, s, "%calc An::shipCost"), "error: not a calc: An::shipCost is an analysis usage", "run it as an analysis case")
}

// RunAnalysis reports the run as a verdict whose status is what the objective
// decided, with each output and verdict as a named value for a report.
func TestRunAnalysisVerdict(t *testing.T) {
	s := analysisSession(t)

	v := s.RunAnalysis("An::shipCost")
	if v.Status != VerdictHolds || v.Subject != "An::shipCost" {
		t.Fatalf("verdict = %+v", v)
	}
	if len(v.Values) != 2 || v.Values[0] != (NamedValue{Name: "total", Value: "12.0"}) ||
		v.Values[1] != (NamedValue{Name: "objective obj", Value: "satisfied"}) {
		t.Errorf("values = %+v", v.Values)
	}

	if v := s.RunAnalysis("An::dear"); v.Status != VerdictFails {
		t.Errorf("a not-satisfied objective is a failed verdict, got %+v", v)
	}
	if v := s.RunAnalysis("An::Priced(0.5)"); v.Status != VerdictUnresolved {
		t.Errorf("a case that could not be run is unresolved, got %+v", v)
	}
	if v := s.RunAnalysis("An::Priced(0.5"); v.Status != VerdictUnresolved {
		t.Errorf("an unclosed argument list is unresolved, got %+v", v)
	}
}

// An objective the run cannot evaluate leaves the verdict undecided, which is
// unresolved: the run decided nothing about the model.
func TestAnalysisUndecidedObjective(t *testing.T) {
	s := NewSession()
	s.Submit(`package U {
		private import ScalarValues::*;
		part def Ship { attribute cost : Real = 5.0; attribute limit : Real; }
		analysis def Budget {
			subject s : Ship;
			objective { require constraint { total <= s.limit } }
			out total : Real = s.cost;
		}
		part ship : Ship;
		analysis budget : Budget { subject s = ship; }
	}`)
	wants(t, run(t, s, "%analysis U::budget"), "? U::budget", "total = 5.0", "objective obj: undecided")
	if v := s.RunAnalysis("U::budget"); v.Status != VerdictUnresolved {
		t.Errorf("an undecided objective is unresolved, got %+v", v)
	}
}
