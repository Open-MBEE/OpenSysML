package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
)

// A choice point renders one canonical line per kind, and its diagnostic is
// informational: the model is not wrong for admitting several orders.
func TestChoicePointRendering(t *testing.T) {
	cases := []struct {
		choice ChoicePoint
		want   string
	}{
		{ChoicePoint{Kind: ChoiceTokenOrder, Step: 2, Alternatives: []string{"2@left", "3@right"}, Taken: 1},
			"choice step 2: tokens 2@left, 3@right (unordered; took 3@right first)"},
		{ChoicePoint{Kind: ChoiceDecisionBranch, Step: 3, Where: "decision pick", Alternatives: []string{"1->a", "2->b"}, Taken: 0},
			"choice step 3: decision pick branches 1->a, 2->b hold (unordered; took 1->a)"},
		{ChoicePoint{Kind: ChoiceWriteOrder, Step: 4, Alternatives: []string{"x := 1 by token 2", "x := 2 by token 3"}, Taken: 0},
			"choice step 4: writes x := 1 by token 2, x := 2 by token 3 (unordered; x := 1 by token 2 stood)"},
		{ChoicePoint{Kind: ChoiceTransition, Where: "state idle on accept Go", Alternatives: []string{"1->low", "2->high"}, Taken: 0},
			"choice state idle on accept Go: transitions 1->low, 2->high (unordered; took 1->low)"},
	}
	for _, c := range cases {
		if got := c.choice.String(); got != c.want {
			t.Errorf("String() = %q, want %q", got, c.want)
		}
		d := c.choice.Diagnostic()
		if d.Severity != passes.SeverityInfo {
			t.Errorf("%s: severity = %v, want info", c.want, d.Severity)
		}
		if d.Code != ChoiceDiagnosticCode || d.Source != "runtime" {
			t.Errorf("%s: code/source = %q/%q", c.want, d.Code, d.Source)
		}
		if d.Message != "choice point: "+strings.TrimPrefix(c.want, "choice ") {
			t.Errorf("%s: message = %q", c.want, d.Message)
		}
	}
}

// Each run starts its own list of choice points; the ones of a run ending in an
// error are still reported, since they explain how the run got there.
func TestChoicesResetPerRun(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		action route {
			attribute level : Integer = 75;
			attribute handler : Integer = 0;
			first start;
			then decide select;
				if level > 50 then warn;
				if level > 70 then alarm;
			action warn { assign handler := 1; }
			then done;
			action alarm { assign handler := 2; }
			then done;
		}
		action plain {
			attribute n : Integer = 0;
			first start;
			then action one { assign n := 1; }
			then done;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	route := findSymbolByName(idx.DocumentRoot("<test>"), "route", ast.DefAction)
	plain := findSymbolByName(idx.DocumentRoot("<test>"), "plain", ast.DefAction)
	if route == nil || plain == nil {
		t.Fatal("actions not found")
	}

	if _, err := ctx.ExecuteAction(route); err != nil {
		t.Fatalf("route: %v", err)
	}
	choices := ctx.Choices()
	if len(choices) != 1 || choices[0].Kind != ChoiceDecisionBranch {
		t.Fatalf("route choices = %v, want one decision branch", choices)
	}
	if choices[0].File != "<test>" || choices[0].Span.Len == 0 {
		t.Errorf("decision choice is not located: file %q span %+v", choices[0].File, choices[0].Span)
	}

	if _, err := ctx.ExecuteAction(plain); err != nil {
		t.Fatalf("plain: %v", err)
	}
	if got := ctx.Choices(); len(got) != 0 {
		t.Fatalf("plain inherited choices from the earlier run: %v", got)
	}
}

// Two transitions out of one state enabled by one event are a choice point
// naming the state, the event and both transitions; the first still fires.
func TestTransitionChoiceNamesStateAndEvent(t *testing.T) {
	src := `package test {
		state Dispatcher {
			attribute level : Integer = 8;
			entry; then idle;
			state idle;
			state low;
			state high;
			transition first idle accept Go if level > 5 then low;
			transition first idle accept Go if level > 7 then high;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Dispatcher", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, []string{"Go"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "idle,low" {
		t.Fatalf("visited %v, want idle then low: the first enabled transition still fires", visited)
	}
	choices := ctx.Choices()
	if len(choices) != 1 || choices[0].Kind != ChoiceTransition {
		t.Fatalf("choices = %v, want one transition choice", choices)
	}
	want := "choice state idle on accept Go: transitions 1->low, 2->high (unordered; took 1->low)"
	if got := choices[0].String(); got != want {
		t.Errorf("choice = %q, want %q", got, want)
	}
}

// A transition on a substate and one on its enclosing state enabled by the same
// event are ordered by UML (the substate's fires), so they are no choice point.
func TestAncestorPriorityIsNotAChoice(t *testing.T) {
	src := `package test {
		state Monitor {
			attribute temp : Integer = 30;
			entry; then running;
			state running {
				entry; then fine;
				state fine;
				state warm;
				transition first fine accept Tick if temp > 20 then warm;
			}
			state halted;
			transition first running accept Tick if temp > 10 then halted;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Monitor", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, []string{"Tick"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "running,fine,warm" {
		t.Fatalf("visited %v, want the substate's transition to fire", visited)
	}
	if got := ctx.Choices(); len(got) != 0 {
		t.Fatalf("ancestor priority reported as a choice: %v", got)
	}
}

// Two tokens writing one feature in one step is a choice point naming both
// writes and the one that stood, whether or not the values differ: which
// performance wrote last is the executor's order either way.
func TestWriteConflictChoice(t *testing.T) {
	run := func(t *testing.T, leftValue string) []ChoicePoint {
		src := `package test {
			private import ScalarValues::*;
			action race {
				attribute x : Integer = 0;
				first start;
				fork split;
				action left { assign x := ` + leftValue + `; }
				action right { assign x := 2; }
				join sync;
				done;
				succession first start then split;
				succession first split then left;
				succession first split then right;
				succession first left then sync;
				succession first right then sync;
				succession first sync then done;
			}
		}`
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "race", ast.DefAction)
		if sym == nil {
			t.Fatal("action not found")
		}
		if _, err := ctx.ExecuteAction(sym); err != nil {
			t.Fatalf("execute: %v", err)
		}
		var writes []ChoicePoint
		for _, c := range ctx.Choices() {
			if c.Kind == ChoiceWriteOrder {
				writes = append(writes, c)
			}
		}
		return writes
	}

	conflicting := run(t, "1")
	if len(conflicting) != 1 {
		t.Fatalf("write choices = %v, want one", conflicting)
	}
	want := "choice step 3: writes x := 1 by token 2, x := 2 by token 3 (unordered; x := 1 by token 2 stood)"
	if got := conflicting[0].String(); got != want {
		t.Errorf("choice = %q, want %q", got, want)
	}
	agreeing := run(t, "2")
	if len(agreeing) != 1 {
		t.Fatalf("write choices = %v, want one", agreeing)
	}
	want = "choice step 3: writes x := 2 by token 2, x := 2 by token 3 (unordered; x := 2 by token 2 stood)"
	if got := agreeing[0].String(); got != want {
		t.Errorf("choice = %q, want %q", got, want)
	}
}
