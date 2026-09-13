package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every input line a witness lists reads back from the line that spells it, its
// value kept as written for the run to evaluate.
func TestParseInputReadsEverySpelling(t *testing.T) {
	cases := []struct {
		line    string
		feature string
		written string
	}{
		{"input n = 5", "n", "5"},
		{"input n = -5", "n", "-5"},
		{"input flag = false", "flag", "false"},
		{"input mode = Mode::Fast", "mode", "Mode::Fast"},
		{"input temp = 2.5 [K]", "temp", "2.5 [K]"},
		{"input 'odd name' = 1 + 2", "odd name", "1 + 2"},
	}
	for _, c := range cases {
		got, err := ParseInput(c.line)
		if err != nil {
			t.Errorf("%q: %v", c.line, err)
			continue
		}
		if got.Feature != c.feature || got.Written != c.written || got.Value.Kind != ValInvalid {
			t.Errorf("%q: %+v", c.line, got)
		}
		if got.String() != c.line {
			t.Errorf("%q read back as %q", c.line, got)
		}
	}
	for _, text := range []string{"n = 5", "input n", "input = 5", "input n =", "input two words = 1", "input 'open = 1"} {
		_, err := ParseInput(text)
		var typed *InputParseError
		if !errors.As(err, &typed) || !errors.Is(err, ErrInvalidInput) || typed.Text != text {
			t.Errorf("%q: error %T %v, want an InputParseError naming the line", text, err, err)
		}
	}
	if in := InputOf("n", intOf(7)); in.Written != "7" || in.Value.Kind != ValConst || in.String() != "input n = 7" {
		t.Errorf("InputOf = %+v", in)
	}
}

// A witness header lists its inputs before its moves and ends at a blank line,
// what follows being the trace; a header of inputs alone, of moves alone or of
// neither reads back, and an input after a move is refused naming its line.
// ParseChoices reads the moves of a header spelling no input.
func TestParseWitnessReadsInputsBeforeChoices(t *testing.T) {
	text := "input n = 5\ninput mode = Mode::Fast\nstep 1: 2@b first of 1@a, 2@b\nstep 2: decision d -> 1->x\n\ninput late = 1\n"
	w, err := ParseWitness(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Inputs) != 2 || w.Inputs[1].Feature != "mode" || FormatChoices(w.Choices) != "step 1: 2@b first of 1@a, 2@b; step 2: decision d -> 1->x" {
		t.Fatalf("read %+v", w)
	}
	if w.Trace != "input late = 1\n" || w.String() != text {
		t.Errorf("spelt back as\n%s", w)
	}
	again, err := ParseWitness(w.String())
	if err != nil || again.String() != w.String() {
		t.Errorf("does not round-trip: %v\n%s", err, again)
	}
	inputsOnly, err := ParseWitness("input n = 5\nno choice points\n")
	if err != nil || len(inputsOnly.Inputs) != 1 || len(inputsOnly.Choices) != 0 || inputsOnly.Empty() {
		t.Errorf("inputs alone read as %+v, %v", inputsOnly, err)
	}
	if inputsOnly.String() != "input n = 5\nno choice points\n\n" {
		t.Errorf("inputs alone spelt as %q", inputsOnly.String())
	}
	movesOnly, err := ParseWitness("step 1: 2@b first of 1@a, 2@b\n")
	if err != nil || len(movesOnly.Inputs) != 0 || len(movesOnly.Choices) != 1 {
		t.Errorf("moves alone read as %+v, %v", movesOnly, err)
	}
	if none, err := ParseWitness("no choice points\n"); err != nil || !none.Empty() {
		t.Errorf("no choice points read as %+v, %v", none, err)
	}
	_, err = ParseWitness("step 1: 2@b first of 1@a, 2@b\ninput n = 5\n")
	var typed *InputParseError
	if !errors.As(err, &typed) || typed.Line != 2 || !strings.Contains(err.Error(), "before the moves") {
		t.Errorf("an input after a move: %v", err)
	}
	if _, err := ParseWitness("input n = 5\ninput = 6\n"); !errors.As(err, &typed) || typed.Line != 2 {
		t.Errorf("an unreadable input line is not named: %v", err)
	}
	if _, err := ParseChoices("input n = 5\nstep 1: 2@b first of 1@a, 2@b\n"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("ParseChoices over inputs: %v", err)
	}
	choices, err := ParseChoices("step 1: 2@b first of 1@a, 2@b\n")
	if err != nil || len(choices) != 1 {
		t.Errorf("ParseChoices without inputs: %v, %v", choices, err)
	}
}

// inputModel reads an input parameter, an enumeration parameter and an attribute
// with a default, and records what it saw.
const inputModel = `package test {
	enum def Mode { Fast; Slow; }
	action gate {
		in n : Integer;
		in mode : Mode;
		attribute limit : Integer = 3;
		attribute over : Boolean = false;
		attribute fast : Boolean = false;
		first start;
		action check { assign over := n > limit; assign fast := mode == Mode::Fast; }
		done;
		succession first start then check;
		succession first check then done;
	}
}`

// A witness's inputs are fixed before the run's first move, through the path a
// caller's inputs take: a parameter gets its value, an attribute's default is
// overridden, and an enumeration value keeps its literal.
func TestReplayFixesWitnessInputs(t *testing.T) {
	m := parseExploreModel(t, inputModel)
	sym := m.action(t, "gate")
	literals := m.idx.LookupQualified("test::Mode::Fast")
	if len(literals) != 1 {
		t.Fatalf("Mode::Fast indexed %d times", len(literals))
	}
	fast := literals[0]
	run := func(t *testing.T, policy SchedulePolicy) (map[string]Value, error) {
		t.Helper()
		ctx, _ := m.fresh()
		mustSchedule(t, ctx, policy)
		out, err := ctx.ExecuteAction(sym)
		if err != nil {
			return nil, err
		}
		return out, ctx.Unfollowed()
	}
	t.Run("in memory", func(t *testing.T) {
		out, err := run(t, ReplayOf(Witness{Inputs: []InputTaken{InputOf("n", intOf(5)), InputOf("mode", NewEnumLiteral(fast))}}))
		if err != nil {
			t.Fatal(err)
		}
		if FormatValue(out["over"]) != "true" || FormatValue(out["fast"]) != "true" || FormatValue(out["n"]) != "5" {
			t.Errorf("held %v", out)
		}
	})
	t.Run("from a file", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "witness.txt")
		text := "input n = 5\ninput limit = 10\ninput mode = Mode::Slow\nno choice points\n\n[step 1] trace ignored\n"
		if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		policy := mustPolicy(t, "replay:"+file)
		w, ok := policy.Witness()
		if !ok || len(w.Inputs) != 3 || len(w.Choices) != 0 {
			t.Fatalf("Witness() = %+v, %v", w, ok)
		}
		out, err := run(t, policy)
		if err != nil {
			t.Fatal(err)
		}
		if FormatValue(out["over"]) != "false" || FormatValue(out["fast"]) != "false" || FormatValue(out["limit"]) != "10" {
			t.Errorf("held %v", out)
		}
		if out["mode"].Kind != ValEnumLiteral || out["mode"].LiteralText() != "Mode::Slow" {
			t.Errorf("mode held as %s", FormatValue(out["mode"]))
		}
		twice, err := run(t, policy)
		if err != nil || FormatValue(twice["limit"]) != "10" {
			t.Errorf("a second run under the file: %v, %v", twice, err)
		}
	})
	t.Run("caller's inputs stay", func(t *testing.T) {
		ctx, _ := m.fresh()
		mustSchedule(t, ctx, ReplayOf(Witness{Inputs: []InputTaken{InputOf("n", intOf(1))}}))
		out, err := ctx.ExecuteActionWithInputs(sym, map[string]Value{"mode": NewEnumLiteral(fast)})
		if err != nil {
			t.Fatal(err)
		}
		if FormatValue(out["over"]) != "false" || FormatValue(out["fast"]) != "true" {
			t.Errorf("held %v", out)
		}
	})
	t.Run("without inputs as before", func(t *testing.T) {
		ctx, _ := m.fresh()
		mustSchedule(t, ctx, ReplayPolicy(nil))
		_, err := ctx.ExecuteActionWithInputs(sym, map[string]Value{"n": intOf(4), "mode": NewEnumLiteral(fast)})
		if err != nil {
			t.Fatal(err)
		}
	})
	refused := func(t *testing.T, w Witness, feature, reason string) {
		t.Helper()
		_, err := run(t, ReplayOf(w))
		var typed *WitnessInputError
		if !errors.As(err, &typed) || !errors.Is(err, ErrWitnessInput) || typed.Feature != feature {
			t.Fatalf("%v: error %T %v, want a WitnessInputError naming %s", w.Inputs, err, err, feature)
		}
		if !strings.Contains(err.Error(), reason) {
			t.Errorf("%v: %v does not say %q", w.Inputs, err, reason)
		}
	}
	t.Run("no such feature", func(t *testing.T) {
		refused(t, Witness{Inputs: []InputTaken{InputOf("n", intOf(1)), InputOf("absent", intOf(1))}}, "absent", "declares no such feature")
	})
	t.Run("unreadable value", func(t *testing.T) {
		refused(t, Witness{Inputs: []InputTaken{{Feature: "n", Written: "5 +"}}}, "n", "not an expression")
		refused(t, Witness{Inputs: []InputTaken{{Feature: "n", Written: "nowhere"}}}, "n", "does not evaluate")
	})
	t.Run("no performance took them", func(t *testing.T) {
		ctx, _ := m.fresh()
		mustSchedule(t, ctx, ReplayOf(Witness{Inputs: []InputTaken{InputOf("n", intOf(1))}}))
		ctx.scheduling()
		err := ctx.Unfollowed()
		var typed *WitnessInputError
		if !errors.As(err, &typed) || typed.Feature != "n" {
			t.Errorf("Unfollowed() = %v, want a WitnessInputError naming n", err)
		}
	})
}
