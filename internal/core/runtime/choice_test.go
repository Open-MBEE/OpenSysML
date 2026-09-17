package runtime

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
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
		{ChoicePoint{Kind: ChoiceRegionOrder, Where: "on accept Go", Alternatives: []string{"a1(exit)", "b1(exit)"}, Taken: 1},
			"choice on accept Go: next a1(exit), b1(exit) (unordered; took b1(exit) first)"},
		{ChoicePoint{Kind: ChoiceRegionOrder, Where: "do round at t=0.0", Alternatives: []string{"a1", "b1"}, Taken: 1},
			"choice do round at t=0.0: states a1, b1 react (unordered; took b1 first)"},
		{ChoicePoint{Kind: ChoiceEntryOrder, Where: "entering work", Alternatives: []string{"a1(entry)", "b1(entry)"}, Taken: 0},
			"choice entering work: next a1(entry), b1(entry) (unordered; took a1(entry) first)"},
		{ChoicePoint{Kind: ChoiceExitOrder, Where: "exiting work", Alternatives: []string{"a1(exit)", "b1(exit)"}, Taken: 1},
			"choice exiting work: next a1(exit), b1(exit) (unordered; took b1(exit) first)"},
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

// Reporting a choice never changes the run: a guard after the first holding one
// that cannot be evaluated is no alternative, not an error the run never had;
// before any guard holds it still fails the run as it always did.
func TestLaterGuardErrorIsNotAChoiceNorAFailure(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		action route {
			attribute level : Integer = 75;
			attribute handler : Integer = 0;
			first start;
			then decide select;
				if level > 50 then warn;
				if 1 / (level - 75) > 0 then alarm;
			action warn { assign handler := 1; }
			then done;
			action alarm { assign handler := 2; }
			then done;
		}
		action broken {
			attribute level : Integer = 75;
			first start;
			then decide select;
				if 1 / (level - 75) > 0 then alarm;
				if level > 50 then warn;
			action warn;
			then done;
			action alarm;
			then done;
		}
		state Dispatcher {
			attribute level : Integer = 8;
			entry; then idle;
			state idle;
			state low;
			state high;
			transition first idle accept Go if level > 5 then low;
			transition first idle accept Go if 1 / (level - 8) > 0 then high;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	root := idx.DocumentRoot("<test>")
	route := findSymbolByName(root, "route", ast.DefAction)
	broken := findSymbolByName(root, "broken", ast.DefAction)
	dispatcher := findSymbolByName(root, "Dispatcher", ast.DefState)
	if route == nil || broken == nil || dispatcher == nil {
		t.Fatal("behaviors not found")
	}

	values, err := ctx.ExecuteAction(route)
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	if got := FormatTraceValue(values["handler"]); got != "1" {
		t.Fatalf("handler = %s, want the first holding guard's branch", got)
	}
	if got := ctx.Choices(); len(got) != 0 {
		t.Fatalf("an unevaluable guard was reported as a choice: %v", got)
	}
	want := "unevaluable guard step 2: decision select branch 2->alarm: division by zero (not selected)"
	if got := ctx.UnevaluableGuards(); len(got) != 1 || got[0].String() != want {
		t.Fatalf("unevaluable guards = %v, want [%s]", got, want)
	}
	diag := ctx.UnevaluableGuards()[0].Diagnostic()
	if diag.Severity != passes.SeverityInfo || diag.Code != UnevaluableGuardCode || diag.Source != "runtime" {
		t.Errorf("diagnostic = %+v, want an informational %s from the runtime", diag, UnevaluableGuardCode)
	}
	if !strings.HasPrefix(diag.Message, "guard not evaluable: ") {
		t.Errorf("message = %q, want it to say the guard is not evaluable", diag.Message)
	}
	if file, span := ctx.UnevaluableGuards()[0].Location(); file != "<test>" || span.Len == 0 {
		t.Errorf("location = %q %v, want the guard's span in the test file", file, span)
	}

	if _, err := ctx.ExecuteAction(broken); err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("broken: err = %v, want the first guard's evaluation error", err)
	}
	if got := ctx.UnevaluableGuards(); len(got) != 0 {
		t.Fatalf("the first guard's failure was noted rather than raised: %v", got)
	}

	_, visited, err := ctx.ExecuteStateWithEvents(dispatcher, []string{"Go"})
	if err != nil {
		t.Fatalf("dispatcher: %v", err)
	}
	if strings.Join(visited, ",") != "idle,low" {
		t.Fatalf("visited %v, want idle then low", visited)
	}
	if got := ctx.Choices(); len(got) != 0 {
		t.Fatalf("an unevaluable transition guard was reported as a choice: %v", got)
	}
	got := ctx.UnevaluableGuards()
	if len(got) != 1 || got[0].Where != "state idle on accept Go" || got[0].Alternative != "2->high" ||
		!strings.Contains(got[0].Reason, "division by zero") || got[0].Step != 0 {
		t.Fatalf("unevaluable guards = %+v, want the second transition out of idle on Go", got)
	}
	if !strings.HasPrefix(got[0].Describe(), "state idle on accept Go: transition 2->high: ") {
		t.Errorf("description = %q, want the state, event and transition first", got[0].Describe())
	}
}

// A choice's later guard is read the same way: once a branch holds, one that
// cannot be evaluated is noted and not taken; a first one still fails the run.
func TestLaterChoiceGuardErrorIsNotAChoiceNorAFailure(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		state Router {
			attribute level : Integer = 0;
			attribute route : Integer = 0;
			entry; then idle;
			state idle;
			choice pick;
			state low { entry { assign route := 1; } }
			state high { entry { assign route := 2; } }
			transition first idle accept Go do assign level := 8 then pick;
			transition first pick if level > 5 then low;
			transition first pick if 1 / (level - 8) > 0 then high;
		}
		state Broken {
			attribute level : Integer = 0;
			entry; then idle;
			state idle;
			choice pick;
			state low;
			state high;
			transition first idle accept Go do assign level := 8 then pick;
			transition first pick if 1 / (level - 8) > 0 then high;
			transition first pick if level > 5 then low;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	root := idx.DocumentRoot("<test>")
	router := findSymbolByName(root, "Router", ast.DefState)
	broken := findSymbolByName(root, "Broken", ast.DefState)
	if router == nil || broken == nil {
		t.Fatal("machines not found")
	}

	values, visited, err := ctx.ExecuteStateWithEvents(router, []string{"Go"})
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	if strings.Join(visited, ",") != "idle,low" || FormatTraceValue(values["route"]) != "1" {
		t.Fatalf("visited %v with route %s, want idle then low by the first holding branch", visited, FormatTraceValue(values["route"]))
	}
	if got := ctx.Choices(); len(got) != 0 {
		t.Fatalf("an unevaluable choice guard was reported as a choice: %v", got)
	}
	got := ctx.UnevaluableGuards()
	if len(got) != 1 || got[0].Where != "choice pick" || got[0].Alternative != "2->high" ||
		!strings.Contains(got[0].Reason, "division by zero") || got[0].Step != 0 {
		t.Fatalf("unevaluable guards = %+v, want the second branch out of pick", got)
	}
	if file, span := got[0].Location(); file != "<test>" || span.Len == 0 {
		t.Errorf("location = %q %v, want the guard's span in the test file", file, span)
	}

	if _, _, err := ctx.ExecuteStateWithEvents(broken, []string{"Go"}); err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("broken: err = %v, want the first guard's evaluation error", err)
	}
	if got := ctx.UnevaluableGuards(); len(got) != 0 {
		t.Fatalf("the first guard's failure was noted rather than raised: %v", got)
	}
}

// A guard read only to report a choice is previewed: what evaluating it costs
// and does is undone, so the run spends and traces exactly what first-match did.
func TestLaterGuardIsProbedWithoutCost(t *testing.T) {
	route := func(second string) string {
		return `package test {
			private import ScalarValues::*;
			calc def cost { in n : Integer; return : Integer = if n > 0 ? cost(n - 1) + 1 else 0; }
			action route {
				attribute level : Integer = 75;
				attribute handler : Integer = 0;
				first start;
				then decide select;
					if level > 50 then warn;
					if ` + second + ` then alarm;
				action warn { assign handler := 1; }
				then done;
				action alarm { assign handler := 2; }
				then done;
			}
		}`
	}
	spent := func(t *testing.T, second string) int64 {
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, route(second)))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "route", ast.DefAction)
		if sym == nil {
			t.Fatal("action not found")
		}
		values, err := ctx.ExecuteAction(sym)
		if err != nil {
			t.Fatalf("route with guard %s: %v", second, err)
		}
		if got := FormatTraceValue(values["handler"]); got != "1" {
			t.Fatalf("handler = %s with guard %s, want the first holding guard's branch", got, second)
		}
		if len(ctx.Choices()) != 1 || len(ctx.UnevaluableGuards()) != 0 {
			t.Fatalf("notes with guard %s = %v, want the one branch choice", second, ctx.Notes())
		}
		return ctx.run.steps
	}
	cheap := spent(t, "level > 70")
	dear := spent(t, "cost(40) > 0")
	if cheap != dear {
		t.Errorf("steps spent = %d with a cheap later guard, %d with a dear one; want the same", cheap, dear)
	}
}

// The first transition read is the run's, not a preview: its failure fails the
// run as it always has.
func TestFirstTransitionFailureStillFailsTheRun(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		state Dispatcher {
			attribute level : Integer = 8;
			entry; then idle;
			state idle;
			state low;
			state high;
			transition first idle accept Go if 1 / (level - 8) > 0 then high;
			transition first idle accept Go if level > 5 then low;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Dispatcher", ast.DefState)
	if sym == nil {
		t.Fatal("state not found")
	}
	_, _, err := ctx.ExecuteStateWithEvents(sym, []string{"Go"})
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("err = %v, want the first transition's evaluation error", err)
	}
	if got := ctx.UnevaluableGuards(); len(got) != 0 {
		t.Fatalf("the first transition's failure was noted rather than raised: %v", got)
	}
}

// Writes to one object through different features are writes to one destination:
// a step writing `cell.mark` and `twin.mark` of the same cell is one conflict.
func TestWriteConflictOnOneObjectThroughTwoChains(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Cell {
			attribute mark : Integer = 0;
		}
		part def Rig {
			part cell : Cell;
			ref part twin : Cell = cell;
			perform action marking {
				first start;
				fork split;
				action viaCell { assign cell.mark := 1; }
				action viaTwin { assign twin.mark := 2; }
				join sync;
				done;
				succession first start then split;
				succession first split then viaCell;
				succession first split then viaTwin;
				succession first viaCell then sync;
				succession first viaTwin then sync;
				succession first sync then done;
			}
		}
	}`
	ctx, _, err := instantiateWithLibraries(t, src, "test::Rig")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	var writes []string
	for _, c := range ctx.Choices() {
		if c.Kind == ChoiceWriteOrder {
			writes = append(writes, c.String())
		}
	}
	if len(writes) != 1 || !strings.Contains(writes[0], "writes mark of object #") ||
		!strings.Contains(writes[0], ":= 1 by token 2,") || !strings.Contains(writes[0], ":= 2 by token 3") {
		t.Fatalf("write choices = %q, want one conflict on the cell's mark", writes)
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

// Leaves in sibling regions select the same transition out of the composite
// state enclosing them, which fires once: so does the choice among the
// transitions out of it.
func TestSharedAncestorChoiceIsReportedOnce(t *testing.T) {
	src := `package test {
		state Machine {
			attribute level : Integer = 8;
			entry; then work;
			state work parallel {
				state a { entry; then a1; state a1; }
				state b { entry; then b1; state b1; }
			}
			state low;
			state high;
			transition first work accept Go if level > 5 then low;
			transition first work accept Go if level > 7 then high;
			transition first work accept Go if 1 / (level - 8) > 0 then high;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, []string{"Go"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "work,a1,b1,low" {
		t.Fatalf("visited %v, want both regions entered, then the first transition out of work", visited)
	}
	want := []string{
		"choice entering work: next a1(entry), b1(entry) (unordered; took a1(entry) first)",
		"choice state work on accept Go: transitions 1->low, 2->high (unordered; took 1->low)",
		"choice exiting work: next a1(exit), b1(exit) (unordered; took a1(exit) first)",
	}
	if got := choiceStrings(ctx.Choices()); !slices.Equal(got, want) {
		t.Fatalf("choices = %v, want exactly %v", got, want)
	}
	if got := ctx.UnevaluableGuards(); len(got) != 1 || got[0].Alternative != "3->high" {
		t.Fatalf("unevaluable guards = %v, want the third transition out of work, once", got)
	}
}

// choiceStrings spells the choices as %trace does.
func choiceStrings(choices []ChoicePoint) []string {
	out := make([]string, len(choices))
	for i, c := range choices {
		out[i] = c.String()
	}
	return out
}

// noteStrings spells the notes as %trace does.
func noteStrings(notes []RunNote) []string {
	out := make([]string, len(notes))
	for i, n := range notes {
		out[i] = n.String()
	}
	return out
}

// A transition out of a composite state loses to one a nested state takes on the
// same event, so the alternatives found out of the composite state were never
// the run's to choose among: nothing about them is reported.
func TestAncestorChoiceSuppressedByNestedTransitionIsNotReported(t *testing.T) {
	src := `package test {
		state Machine {
			attribute level : Integer = 8;
			entry; then work;
			state work parallel {
				state a {
					entry; then a1;
					state a1;
					state a2;
					transition first a1 accept Go then a2;
				}
				state b { entry; then b1; state b1; }
			}
			state low;
			state high;
			transition first work accept Go if level > 5 then low;
			transition first work accept Go if level > 7 then high;
			transition first work accept Go if 1 / (level - 8) > 0 then high;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, []string{"Go"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "work,a1,b1,a2" {
		t.Fatalf("visited %v, want the nested transition to fire and work to stay active", visited)
	}
	want := []string{"choice entering work: next a1(entry), b1(entry) (unordered; took a1(entry) first)"}
	if got := noteStrings(ctx.Notes()); !slices.Equal(got, want) {
		t.Fatalf("the outranked transitions out of work were reported: %v", got)
	}
}

// A step that fails after one token already went still made an ordering choice:
// the failing token could have gone first, and the diagnostics of a failed run
// must say what the run did before it failed.
func TestTokenOrderIsReportedWhenALaterTokenFails(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		action race {
			attribute x : Integer = 0;
			attribute n : Integer = 0;
			first start;
			fork split;
			action safe { assign x := 1; }
			action failing { assign x := 1 / n; }
			join sync;
			done;
			succession first start then split;
			succession first split then failing;
			succession first split then safe;
			succession first safe then sync;
			succession first failing then sync;
			succession first sync then done;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "race", ast.DefAction)
	if sym == nil {
		t.Fatal("action not found")
	}
	_, err := ctx.ExecuteAction(sym)
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("err = %v, want the failing token's error", err)
	}
	var tokenOrders []ChoicePoint
	for _, c := range ctx.Choices() {
		if c.Kind == ChoiceTokenOrder {
			tokenOrders = append(tokenOrders, c)
		}
	}
	if len(tokenOrders) != 1 {
		t.Fatalf("token-order choices = %v, want the one step both tokens took part in", tokenOrders)
	}
	got := tokenOrders[0]
	if len(got.Alternatives) != 2 || !strings.HasSuffix(got.Alternatives[0], "@failing") ||
		!strings.HasSuffix(got.Alternatives[1], "@safe") || got.Taken != 1 {
		t.Fatalf("choice = %s, want failing and safe as the alternatives, safe taken first", got)
	}
}

// One message two parked accepts both answer to goes to whichever is stepped
// first: the recipient is the executor's choice, and the accept left waiting is
// the alternative even though it did nothing in the step.
func TestSharedMessageAcceptIsAChoice(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		action listen {
			first start;
			fork split;
			action left accept a : Integer;
			action right accept b : Integer;
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
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "listen", ast.DefAction)
	if sym == nil {
		t.Fatal("action not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	for i := 0; i < 10 && exec.State() != StateWaiting; i++ {
		if err := exec.Step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	if exec.State() != StateWaiting {
		t.Fatalf("state = %v, want both accepts parked", exec.State())
	}
	if got := ctx.Choices(); len(got) != 0 {
		t.Fatalf("choices before any message = %v, want none: two parked accepts have nothing to take", got)
	}
	at := func(node string) (Token, bool) {
		for _, tok := range exec.Tokens() {
			if nodeIdentifier(tok.Location) == node {
				return tok, true
			}
		}
		return Token{}, false
	}
	left, _ := at("left")
	right, _ := at("right")
	if left.Wait == nil || right.Wait == nil {
		t.Fatalf("tokens = %v, want both accepts parked", exec.Tokens())
	}
	one := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	ctx.PostMessage(Message{SignalType: "Integer", Value: &one})
	if err := exec.Step(); err != nil {
		t.Fatalf("step with the message in flight: %v", err)
	}
	if still, ok := at("left"); !ok || still.Wait == nil {
		t.Fatalf("tokens = %v, want left to keep waiting", exec.Tokens())
	}
	if _, ok := at("right"); ok {
		t.Fatalf("tokens = %v, want right, stepped first, to take the message and move on", exec.Tokens())
	}
	got := ctx.Choices()
	if len(got) != 1 || got[0].Kind != ChoiceTokenOrder {
		t.Fatalf("choices = %v, want the one recipient choice", got)
	}
	want := fmt.Sprintf("choice step %d: tokens %d@left, %d@right (unordered; took %d@right first)",
		got[0].Step, left.ID, right.ID, right.ID)
	if got[0].String() != want {
		t.Fatalf("choice = %s, want %s", got[0], want)
	}
	if pending := ctx.PendingMessages(); len(pending) != 0 {
		t.Fatalf("pending messages = %+v, want the one message taken", pending)
	}
	if err := exec.Step(); err != nil {
		t.Fatalf("step with nothing in flight: %v", err)
	}
	if got := ctx.Choices(); len(got) != 1 {
		t.Fatalf("choices = %v, want no choice while left waits alone", got)
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

// Three tokens writing one feature in one step are one choice point listing
// every token's write and the one that stood, not two pairwise ones.
func TestThreeWritersAreOneChoice(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		action race {
			attribute x : Integer = 0;
			first start;
			fork split;
			action a { assign x := 1; }
			action b { assign x := 2; }
			action c { assign x := 3; }
			join sync;
			done;
			succession first start then split;
			succession first split then a;
			succession first split then b;
			succession first split then c;
			succession first a then sync;
			succession first b then sync;
			succession first c then sync;
			succession first sync then done;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "race", ast.DefAction)
	if sym == nil {
		t.Fatal("action not found")
	}
	values, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := FormatTraceValue(values["x"]); got != "1" {
		t.Fatalf("x = %s, want the last token stepped (the lowest) to stand", got)
	}
	writes := writeChoices(ctx)
	want := "choice step 3: writes x := 1 by token 2, x := 2 by token 3, x := 3 by token 4 (unordered; x := 1 by token 2 stood)"
	if len(writes) != 1 || writes[0].String() != want {
		t.Fatalf("write choices = %v, want exactly [%s]", writes, want)
	}
}

// A token writing one feature twice in its step contributes its last write only:
// had it gone last, that is what would stand.
func TestRepeatedWritesByOneTokenListItsLast(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		action race {
			attribute x : Integer = 0;
			first start;
			fork split;
			action left { assign x := 1; assign x := 3; }
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
	values, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := FormatTraceValue(values["x"]); got != "3" {
		t.Fatalf("x = %s, want left's last write to stand", got)
	}
	writes := writeChoices(ctx)
	want := "choice step 3: writes x := 3 by token 2, x := 2 by token 3 (unordered; x := 3 by token 2 stood)"
	if len(writes) != 1 || writes[0].String() != want {
		t.Fatalf("write choices = %v, want exactly [%s]", writes, want)
	}
}

// A feature and the one redefining it are one destination, so writes under either
// name within one step conflict, direct from the performer or through a chain.
func TestAliasWritesAreOneDestination(t *testing.T) {
	flow := func(viaMark, viaLabel string) string {
		return `
			first start;
			fork split;
			action viaMark { ` + viaMark + ` }
			action viaLabel { ` + viaLabel + ` }
			join sync;
			done;
			succession first start then split;
			succession first split then viaMark;
			succession first split then viaLabel;
			succession first viaMark then sync;
			succession first viaLabel then sync;
			succession first sync then done;`
	}
	cases := map[string]struct{ src, fqn string }{
		"performer": {`package test {
			private import ScalarValues::*;
			part def Cell { attribute mark : Integer = 0; }
			part def Twin :> Cell {
				attribute :>> mark;
				attribute label :>> mark;
				perform action marking {` + flow("assign mark := 1;", "assign label := 2;") + `}
			}
		}`, "test::Twin"},
		"chain": {`package test {
			private import ScalarValues::*;
			part def Cell { attribute mark : Integer = 0; }
			part def Twin :> Cell { attribute label :>> mark; }
			part def Rig {
				part cell : Twin;
				perform action marking {` + flow("assign cell.mark := 1;", "assign cell.label := 2;") + `}
			}
		}`, "test::Rig"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, _, err := instantiateWithLibraries(t, c.src, c.fqn)
			if err != nil {
				t.Fatalf("instantiate: %v", err)
			}
			writes := writeChoices(ctx)
			if len(writes) != 1 || len(writes[0].Alternatives) != 2 {
				t.Fatalf("write choices = %v, want one conflict on the cell's mark", writes)
			}
			got := writes[0].String()
			if !strings.Contains(got, " of object #") || !strings.Contains(got, ":= 1 by token 2,") ||
				!strings.Contains(got, ":= 2 by token 3") || !strings.Contains(got, ":= 1 by token 2 stood") {
				t.Fatalf("choice = %q, want both names' writes as one destination, the mark's write standing", got)
			}
		})
	}
}

// writeChoices is the write-order choice points of the last run.
func writeChoices(ctx *Context) []ChoicePoint {
	var writes []ChoicePoint
	for _, c := range ctx.Choices() {
		if c.Kind == ChoiceWriteOrder {
			writes = append(writes, c)
		}
	}
	return writes
}

// An object a probed later guard makes is undone with its identity, so the run's
// objects are numbered as when no later guard is probed at all.
func TestProbedGuardLeavesObjectIdentitiesUntouched(t *testing.T) {
	route := func(second string) string {
		return `package test {
			private import ScalarValues::*;
			item def Cell { attribute v : Integer; }
			action route {
				attribute level : Integer = 75;
				attribute made : Integer = 0;
				first start;
				then decide select;
					if level > 50 then warn;
					if ` + second + ` then alarm;
				action warn { assign made := new Cell(2).v; }
				then done;
				action alarm { assign made := new Cell(3).v; }
				then done;
			}
		}`
	}
	objects := func(t *testing.T, second string) string {
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, route(second)))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "route", ast.DefAction)
		if sym == nil {
			t.Fatal("action not found")
		}
		values, err := ctx.ExecuteAction(sym)
		if err != nil {
			t.Fatalf("route with guard %s: %v", second, err)
		}
		if got := FormatTraceValue(values["made"]); got != "2" {
			t.Fatalf("made = %s with guard %s, want the first holding guard's branch", got, second)
		}
		if len(ctx.Choices()) != 1 || len(ctx.UnevaluableGuards()) != 0 {
			t.Fatalf("notes with guard %s = %v, want the one branch choice", second, ctx.Notes())
		}
		return fmt.Sprint(ctx.InstanceIDs())
	}
	plain := objects(t, "level > 70")
	probed := objects(t, "new Cell(1).v > 0")
	if plain != probed {
		t.Errorf("objects %s after a probed guard made one, %s otherwise; want the same identities", probed, plain)
	}
}

// A selected transition whose guard another region's reaction falsified before
// its turn does not fire, so nothing about selecting it is reported; the
// region order that let the other reaction go first is the run's choice, drawn
// per unit until the effect falsifies the guard and the blocked firing drops out.
func TestNotesOfATransitionBlockedBeforeFiringAreDropped(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		state Machine {
			attribute level : Integer = 8;
			entry; then work;
			state work parallel {
				state a {
					entry; then a1;
					state a1;
					state a2;
					transition first a1 accept Go do assign level := 0 then a2;
				}
				state b {
					entry; then b1;
					state b1;
					state b2;
					state b3;
					transition first b1 accept Go if level > 5 then b2;
					transition first b1 accept Go if level > 7 then b3;
				}
			}
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, []string{"Go"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "work,a1,b1,a2" {
		t.Fatalf("visited %v, want region a to fire and region b, its guards blocked by a's effect, to stay", visited)
	}
	want := []string{
		"choice entering work: next a1(entry), b1(entry) (unordered; took a1(entry) first)",
		"choice on accept Go: next a1(exit), b1(exit) (unordered; took a1(exit) first)",
		"choice on accept Go: next a1->a2(effect), b1(exit) (unordered; took a1->a2(effect) first)",
	}
	if got := noteStrings(ctx.Notes()); !slices.Equal(got, want) {
		t.Fatalf("notes %v, want the entry and region-order choices alone: %v", got, want)
	}
}

// A region-order choice names the occurrence dispatched, not the trigger of the
// region drawn first: two regions may spell one message differently (its type
// and a supertype), and the report must read the same whichever a seed draws.
func TestRegionOrderChoiceNamesTheOccurrenceNotTheTakenTrigger(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		attribute def Base;
		attribute def Go :> Base;
		state Machine {
			entry; then start;
			state start;
			transition first start do send new Go() then work;
			state work parallel {
				state a {
					entry; then a1;
					state a1;
					state a2;
					transition first a1 accept Go then a2;
				}
				state b {
					entry; then b1;
					state b1;
					state b2;
					transition first b1 accept Base then b2;
				}
			}
		}
	}`
	under := func(spelling string) ChoicePoint {
		t.Helper()
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
		if sym == nil {
			t.Fatal("state machine not found")
		}
		policy, err := ParseSchedulePolicy(spelling)
		if err != nil {
			t.Fatal(err)
		}
		if err := ctx.SetSchedule(policy); err != nil {
			t.Fatal(err)
		}
		if _, _, err := ctx.ExecuteStateWithEvents(sym, nil); err != nil {
			t.Fatalf("%s: execute: %v", spelling, err)
		}
		return firstRegionOrderChoice(t, spelling, ctx.Notes(), "a1(exit), b1(exit)")
	}
	const want = "on accept Go"
	if choice := under("reverse"); choice.Taken != 0 || choice.Where != want {
		t.Fatalf("reverse: %s, want a1 first %q", choice.String(), want)
	}
	for seed := 1; seed <= 32; seed++ {
		choice := under(fmt.Sprintf("seed:%d", seed))
		if choice.Where != want {
			t.Fatalf("seed:%d: choice %s, want it named %q whichever region it took first", seed, choice.String(), want)
		}
		if choice.Taken == 1 {
			return
		}
	}
	t.Fatal("no seed up to 32 took b1 first; the case does not exercise the alternate draw")
}

// firstRegionOrderChoice returns the first draw among the firings of a
// dispatch, checking the run's notes are the entry order and those draws alone
// and every draw of the dispatch is named alike.
func firstRegionOrderChoice(t *testing.T, label string, notes []RunNote, alternatives string) ChoicePoint {
	t.Helper()
	var firings []ChoicePoint
	for _, note := range notes {
		choice, ok := note.(ChoicePoint)
		switch {
		case ok && choice.Kind == ChoiceEntryOrder:
		case ok && choice.Kind == ChoiceRegionOrder:
			firings = append(firings, choice)
		default:
			t.Fatalf("%s: note %v, want the entry and region-order choices alone", label, note)
		}
	}
	if len(firings) == 0 || strings.Join(firings[0].Alternatives, ", ") != alternatives {
		t.Fatalf("%s: notes %v, want a region-order choice among %s first", label, notes, alternatives)
	}
	for _, choice := range firings[1:] {
		if choice.Where != firings[0].Where {
			t.Fatalf("%s: the draws of one dispatch are named %q and %q", label, firings[0].Where, choice.Where)
		}
	}
	return firings[0]
}

// A message sent from an event feature is named after that feature, as the
// accept subsetting it is written: two event features of one type dispatched to
// the same regions make choices a reader can tell apart.
func TestRegionOrderChoiceNamesTheEventFeatureSent(t *testing.T) {
	const src = `package test {
		item def Ping;
		state Machine {
			item alert : Ping;
			item alarm : Ping;
			entry; then start;
			state start;
			transition first start do send %s then work;
			state work parallel {
				state a {
					entry; then a1;
					state a1;
					state a2;
					transition first a1 accept :> %[1]s then a2;
				}
				state b {
					entry; then b1;
					state b1;
					state b2;
					transition first b1 accept :> %[1]s then b2;
				}
			}
		}
	}`
	for _, feature := range []string{"alert", "alarm"} {
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, fmt.Sprintf(src, feature)))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
		if sym == nil {
			t.Fatal("state machine not found")
		}
		if _, _, err := ctx.ExecuteStateWithEvents(sym, nil); err != nil {
			t.Fatalf("%s: execute: %v", feature, err)
		}
		want := "on accept :> " + feature
		if choice := firstRegionOrderChoice(t, feature, ctx.Notes(), "a1(exit), b1(exit)"); choice.Where != want {
			t.Fatalf("%s: choice %v, want a region-order choice %q", feature, choice, want)
		}
	}
}

// A transition that fires and fails in its effect was the run's choice all the
// same: the choice explains how the run got to the failure.
func TestNotesOfATransitionFailingInItsEffectAreKept(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		state Dispatcher {
			attribute level : Integer = 8;
			entry; then idle;
			state idle;
			state low;
			state high;
			transition first idle accept Go if level > 5 do assign level := 1 / (level - 8) then low;
			transition first idle accept Go if level > 7 then high;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Dispatcher", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, _, err := ctx.ExecuteStateWithEvents(sym, []string{"Go"})
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("err = %v, want the effect's evaluation error", err)
	}
	want := "choice state idle on accept Go: transitions 1->low, 2->high (unordered; took 1->low)"
	if got := ctx.Choices(); len(got) != 1 || got[0].String() != want {
		t.Fatalf("choices = %v, want exactly [%s]", got, want)
	}
}

// Change-triggered transitions out of one state enabled by one poll are a choice
// point as event-triggered ones are; the first declared fires.
func TestChangeTransitionChoice(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		state Monitor {
			attribute temp : Integer = 0;
			entry; then start;
			state start;
			state watching;
			state cool;
			state hot;
			transition first start do assign temp := 30 then watching;
			transition first watching accept when temp > 20 then cool;
			transition first watching accept when temp > 25 then hot;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Monitor", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "start,watching,cool" {
		t.Fatalf("visited %v, want the first declared change transition to fire", visited)
	}
	want := "choice state watching on change: transitions 1->cool, 2->hot (unordered; took 1->cool)"
	if got := ctx.Choices(); len(got) != 1 || got[0].String() != want {
		t.Fatalf("choices = %v, want exactly [%s]", got, want)
	}
}

// A change guard read once an earlier transition is enabled is a probe: one that
// cannot be evaluated is not enabled and not an error, and stays armed.
func TestLaterChangeGuardErrorIsNotAChoiceNorAFailure(t *testing.T) {
	monitor := func(first, second string) string {
		return `package test {
			private import ScalarValues::*;
			state Monitor {
				attribute temp : Integer = 0;
				entry; then start;
				state start;
				state watching;
				state cool;
				state hot;
				transition first start do assign temp := 30 then watching;
				transition first watching accept when temp > 20 if ` + first + ` then cool;
				transition first watching accept when temp > 25 if ` + second + ` then hot;
			}
		}`
	}
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, monitor("temp > 0", "1 / (temp - 30) > 0")))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Monitor", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "start,watching,cool" {
		t.Fatalf("visited %v, want the first enabled change transition to fire", visited)
	}
	if got := ctx.Choices(); len(got) != 0 {
		t.Fatalf("an unevaluable guard was reported as a choice: %v", got)
	}
	want := "unevaluable guard state watching on change: transition 2->hot: eval change guard: eval guard of transition watching -> hot: division by zero (not selected)"
	if got := ctx.UnevaluableGuards(); len(got) != 1 || got[0].String() != want {
		t.Fatalf("unevaluable guards = %v, want [%s]", got, want)
	}

	idx, _, ctx = buildRuntime(t, "<test>", parseAndBuild(t, monitor("1 / (temp - 30) > 0", "temp > 0")))
	sym = findSymbolByName(idx.DocumentRoot("<test>"), "Monitor", ast.DefState)
	if _, _, err := ctx.ExecuteStateWithEvents(sym, nil); err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("first guard: err = %v, want its evaluation error to fail the run", err)
	}
	if got := ctx.Notes(); len(got) != 0 {
		t.Fatalf("a first guard's failure was noted: %v", got)
	}
}

// A composite state's change transition loses to a nested state's on the same rise
// and parallel regions fire alongside: the rise reports which region reacts first
// and, in the one state with two enabled, which transition; the outranked
// composite state draws nothing.
func TestChangeTransitionChoiceUnderHierarchyAndRegions(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		state Machine {
			attribute temp : Integer = 0;
			entry; then start;
			state start;
			state work parallel {
				state a {
					entry; then a1;
					state a1;
					state a2;
					state a3;
					transition first a1 accept when temp > 20 then a2;
					transition first a1 accept when temp > 25 then a3;
				}
				state b {
					entry; then b1;
					state b1;
					state b2;
					transition first b1 accept when temp > 20 then b2;
				}
			}
			state halted;
			state stopped;
			transition first start do assign temp := 30 then work;
			transition first work accept when temp > 20 then halted;
			transition first work accept when temp > 25 then stopped;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "start,work,a1,b1,a2,b2" {
		t.Fatalf("visited %v, want both regions to take their nested transitions and work to stay active", visited)
	}
	want := []string{
		"choice entering work: next a1(entry), b1(entry) (unordered; took a1(entry) first)",
		"choice on change: next a1(exit), b1(exit) (unordered; took a1(exit) first)",
		"choice state a1 on change: transitions 1->a2, 2->a3 (unordered; took 1->a2)",
		"choice on change: next a2(entry), b1(exit) (unordered; took a2(entry) first)",
	}
	if got := choiceStrings(ctx.Choices()); !slices.Equal(got, want) {
		t.Fatalf("choices = %v, want exactly %v", got, want)
	}
}

// A state's completion is one occurrence: several completion transitions out of
// it are one transition choice, drawn when the completion is dispatched, and the
// ones not drawn leave the queue rather than firing on a completion of their own.
func TestCompletionTransitionChoiceIsReportedOnce(t *testing.T) {
	src := `package test {
		state Switch {
			entry; then ready;
			state ready;
			state left;
			state right;
			state settled;
			transition first ready then left;
			transition first ready then right;
			transition first left then settled;
			transition first right then settled;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Switch", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "ready,left,settled" {
		t.Fatalf("visited %v, want ready, left, settled: one completion fires one transition", visited)
	}
	want := []string{"choice state ready: transitions 1->left, 2->right (unordered; took 1->left)"}
	var got []string
	for _, choice := range ctx.Choices() {
		got = append(got, choice.String())
	}
	if !slices.Equal(got, want) {
		t.Fatalf("choices = %v, want exactly %v", got, want)
	}
}

// The guards of a state's queued completion transitions are read again when its
// completion is dispatched: one a sibling region's earlier completion has since
// disabled is no alternative, so the other fires and no choice is reported.
func TestCompletionTransitionChoiceRereadsGuards(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		state Machine {
			attribute flag : Boolean = true;
			entry; then work;
			state work parallel {
				state a {
					entry; then a1;
					state a1;
					state a2;
					transition first a1 do assign flag := false then a2;
				}
				state b {
					entry; then ready;
					state ready;
					state left;
					state right;
					transition first ready if flag then left;
					transition first ready then right;
				}
			}
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "work,a1,ready,a2,right" {
		t.Fatalf("visited %v, want a1's completion to clear flag before ready's is dispatched, and ready to move to right", visited)
	}
	if got, want := choiceStrings(ctx.Choices()), []string{"choice entering work: next a1(entry), ready(entry) (unordered; took a1(entry) first)"}; !slices.Equal(got, want) {
		t.Fatalf("choices = %v, want the entry order alone: the disabled completion transition is no alternative", got)
	}
}

// A completion transition whose guard was false when the state completed is read
// again when the completion is dispatched: one a sibling region's earlier
// completion has since enabled is an alternative, so the two are a choice.
func TestCompletionTransitionChoiceSeesLaterEnabledGuard(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		state Machine {
			attribute flag : Boolean = false;
			entry; then work;
			state work parallel {
				state a {
					entry; then a1;
					state a1;
					state a2;
					transition first a1 do assign flag := true then a2;
				}
				state b {
					entry; then ready;
					state ready;
					state left;
					state right;
					transition first ready if flag then left;
					transition first ready then right;
				}
			}
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "work,a1,ready,a2,left" {
		t.Fatalf("visited %v, want a1's completion to set flag before ready's is dispatched, and the draw to take left", visited)
	}
	want := []string{
		"choice entering work: next a1(entry), ready(entry) (unordered; took a1(entry) first)",
		"choice state ready: transitions 1->left, 2->right (unordered; took 1->left)",
	}
	if got := choiceStrings(ctx.Choices()); !slices.Equal(got, want) {
		t.Fatalf("choices = %v, want exactly %v: the guard enabled since queuing is an alternative", got, want)
	}
}

// A state whose only completion transition has a false guard still completes:
// the guard is read again when the completion is dispatched, and one enabled
// since fires.
func TestSingleCompletionTransitionFiresOnceGuardHolds(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		state Machine {
			attribute flag : Boolean = false;
			entry; then work;
			state work parallel {
				state a {
					entry; then a1;
					state a1;
					state a2;
					transition first a1 do assign flag := true then a2;
				}
				state b {
					entry; then ready;
					state ready;
					state left;
					transition first ready if flag then left;
				}
			}
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "work,a1,ready,a2,left" {
		t.Fatalf("visited %v, want ready's completion to fire once a1's effect set flag", visited)
	}
	if got, want := choiceStrings(ctx.Choices()), []string{"choice entering work: next a1(entry), ready(entry) (unordered; took a1(entry) first)"}; !slices.Equal(got, want) {
		t.Fatalf("choices = %v, want the entry order alone: one completion transition is no choice", got)
	}
}

// A completion transition into a join whose other branch has not arrived is not
// enabled, so it is no alternative to draw: the other completion transition
// fires and the queue is not drained on a join that cannot move.
func TestCompletionTransitionChoiceSkipsUnreadyJoin(t *testing.T) {
	src := `package test {
		state Machine {
			entry; then work;
			state work parallel {
				state a {
					entry; then a1;
					state a1;
					state a2;
					transition first a1 accept Go then a2;
				}
				state b {
					entry; then ready;
					state ready;
					state right;
					transition first ready then sync;
					transition first ready then right;
				}
			}
			join sync;
			state done;
			transition first a2 then sync;
			transition first sync then done;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, []string{"Go"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "work,a1,ready,right,a2" {
		t.Fatalf("visited %v, want ready to move to right while the join waits on a2, then a2 alone", visited)
	}
	if got, want := choiceStrings(ctx.Choices()), []string{"choice entering work: next a1(entry), ready(entry) (unordered; took a1(entry) first)"}; !slices.Equal(got, want) {
		t.Fatalf("choices = %v, want the entry order alone: a transition into an unready join is no alternative", got)
	}
}

// A queued completion transition whose guard can no longer be read once one
// alternative is enabled is noted as not selected, as for a triggered event,
// rather than failing the run.
func TestLaterCompletionGuardErrorIsNotedNotRaised(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		state Machine {
			attribute d : Integer = 1;
			entry; then work;
			state work parallel {
				state a {
					entry; then a1;
					state a1;
					state a2;
					transition first a1 do assign d := 0 then a2;
				}
				state b {
					entry; then ready;
					state ready;
					state left;
					state right;
					transition first ready if d < 2 then left;
					transition first ready if 1 / d > 0 then right;
				}
			}
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "work,a1,ready,a2,left" {
		t.Fatalf("visited %v, want a1's completion to zero d before ready's is dispatched, and ready to move to left", visited)
	}
	if got, want := choiceStrings(ctx.Choices()), []string{"choice entering work: next a1(entry), ready(entry) (unordered; took a1(entry) first)"}; !slices.Equal(got, want) {
		t.Fatalf("choices = %v, want the entry order alone: an unevaluable completion guard is no alternative", got)
	}
	got := ctx.UnevaluableGuards()
	if len(got) != 1 || got[0].Where != "state ready" || got[0].Alternative != "2->right" ||
		!strings.Contains(got[0].Reason, "division by zero") {
		t.Fatalf("unevaluable guards = %+v, want the second completion transition out of ready", got)
	}
}

// A completion guard that cannot be read when the state completes is not read
// then: the guards are read when the completion is dispatched, where a later
// unevaluable one is noted as not selected once an earlier one is enabled.
func TestBrokenLaterCompletionGuardDoesNotAbortEntry(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		state Machine {
			entry; then ready;
			state ready;
			state left;
			state right;
			transition first ready then left;
			transition first ready if 1 / 0 > 0 then right;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	_, visited, err := ctx.ExecuteStateWithEvents(sym, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(visited, ",") != "ready,left" {
		t.Fatalf("visited %v, want ready to complete and move to left", visited)
	}
	if choices := ctx.Choices(); len(choices) != 0 {
		t.Fatalf("choices = %v, want none: an unevaluable completion guard is no alternative", choices)
	}
	got := ctx.UnevaluableGuards()
	if len(got) != 1 || got[0].Where != "state ready" || got[0].Alternative != "2->right" ||
		!strings.Contains(got[0].Reason, "division by zero") {
		t.Fatalf("unevaluable guards = %+v, want the second completion transition out of ready", got)
	}
}
