package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
)

// parsingModel is typedModel with the notation's parser installed as well, so a run
// reads witness inputs and tool units the way every product frontend does.
func parsingModel(sem *semantics.Model, resolver *resolve.Resolver) *Model {
	m := typedModel(sem, resolver)
	m.SetExpressionParser(lower.ExpressionReader(parser.ParseOneExpression, m.Resolver()))
	return m
}

// A model given no parser refuses, with the typed error, the two places a run reads
// notation text: a witness input read from a file and the unit a tool answers in. A
// witness made in memory carries values, not text, so it needs no parser.
func TestRuntimeWithoutExpressionParserRefusesText(t *testing.T) {
	t.Run("witness input from a file", func(t *testing.T) {
		m := parseExploreModel(t, inputModel)
		sym := m.action(t, "gate")
		file := filepath.Join(t.TempDir(), "witness.txt")
		if err := os.WriteFile(file, []byte("input n = 5\nno choice points\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		ctx := NewContext(NewModel(m.model, m.resolver), 10000)
		mustSchedule(t, ctx, mustPolicy(t, "replay:"+file))
		_, err := ctx.ExecuteAction(sym)
		if !errors.Is(err, errNoExpressionParser) {
			t.Fatalf("ExecuteAction = %v, want errNoExpressionParser", err)
		}
		if errors.Is(err, ErrWitnessInput) {
			t.Fatalf("ExecuteAction = %v, must not read as a refused witness input", err)
		}
	})
	t.Run("witness input in memory", func(t *testing.T) {
		m := parseExploreModel(t, inputModel)
		sym := m.action(t, "gate")
		fast := m.idx.LookupQualified("test::Mode::Fast")
		if len(fast) != 1 {
			t.Fatalf("Mode::Fast indexed %d times", len(fast))
		}
		ctx := NewContext(NewModel(m.model, m.resolver), 10000)
		mustSchedule(t, ctx, ReplayOf(Witness{Inputs: []InputTaken{
			InputOf("n", intOf(5)), InputOf("mode", NewEnumLiteral(fast[0]))}}))
		out, err := ctx.ExecuteAction(sym)
		if err != nil {
			t.Fatal(err)
		}
		if FormatValue(out["over"]) != "true" {
			t.Errorf("held %v", out)
		}
	})
	t.Run("tool unit", func(t *testing.T) {
		idx, sem, parsing := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, toolModel))
		pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
		if !ok || pkg.Scope == nil {
			t.Fatal("test package not indexed")
		}
		ctx := NewContext(NewModel(sem, parsing.Model().Resolver()), 10000)
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
			"a": {Value: toolReal(2), Unit: "m/s**2"},
			"v": {Value: toolReal(43.2), Unit: "km/h"},
			"x": {Value: toolReal(11000), Unit: "cm"},
		}})
		_, err := ctx.ExecuteAction(calcNamed(t, pkg.Scope, "Drive"))
		if !errors.Is(err, errNoExpressionParser) {
			t.Fatalf("ExecuteAction = %v, want errNoExpressionParser", err)
		}
		var toolErr *ToolError
		if errors.As(err, &toolErr) {
			t.Fatalf("ExecuteAction = %v, must not read as the tool's malformed output", err)
		}
		if _, err := ctx.UnitOf(pkg.Scope, "m/s**2"); !errors.Is(err, errNoExpressionParser) {
			t.Fatalf("UnitOf = %v, want errNoExpressionParser", err)
		}
	})
	t.Run("replaced parser forgets the units the previous one read", func(t *testing.T) {
		idx, _, parsing := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, toolModel))
		pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
		if !ok || pkg.Scope == nil {
			t.Fatal("test package not indexed")
		}
		if _, err := parsing.UnitOf(pkg.Scope, "m/s**2"); err != nil {
			t.Fatalf("UnitOf with the parser installed: %v", err)
		}
		parsing.Model().SetExpressionParser(nil)
		if _, err := parsing.UnitOf(pkg.Scope, "m/s**2"); !errors.Is(err, errNoExpressionParser) {
			t.Fatalf("UnitOf after removing the parser = %v, want errNoExpressionParser", err)
		}
	})
}

// TestRuntimeEvaluatesTheRecordsItsParserLowers pins the seam: the parser is handed the
// scope the text is written in, and the run evaluates the records it returns, keeping none.
func TestRuntimeEvaluatesTheRecordsItsParserLowers(t *testing.T) {
	m := parseExploreModel(t, strings.TrimSuffix(inputModel, "}")+`
	calc def Twice { in x : Integer; return : Integer = x + x; }
}`)
	sym := m.action(t, "gate")
	file := filepath.Join(t.TempDir(), "witness.txt")
	if err := os.WriteFile(file, []byte("input n = unread\ninput mode = Mode::Fast\nno choice points\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	model := typedModel(m.model, m.resolver)
	read := lower.ExpressionReader(parser.ParseOneExpression, m.resolver)
	var scopes []*symbols.Scope
	model.SetExpressionParser(func(origin, text string, scope *symbols.Scope) (*lower.ExprTable, program.ExprID, bool) {
		scopes = append(scopes, scope)
		if text == "unread" {
			text = "Twice(limit - 1)"
		}
		return read(origin, text, scope)
	})
	ctx := NewContext(model, 10000)
	mustSchedule(t, ctx, mustPolicy(t, "replay:"+file))
	out, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatal(err)
	}
	if FormatValue(out["over"]) != "true" || FormatValue(out["fast"]) != "true" {
		t.Errorf("held %v, want the values of the lowered records: over and fast", out)
	}
	if len(scopes) != 2 || scopes[0] != sym.Scope || scopes[1] != sym.Scope {
		t.Errorf("the parser was given scopes %v, want the action's twice", scopes)
	}
	if held := len(model.invocationTargets); held != 0 {
		t.Errorf("%d invocation targets held after the run, want none of the text's", held)
	}
}
