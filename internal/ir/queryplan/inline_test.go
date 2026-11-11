package queryplan

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const inlineModel = `
part def Subsystem { attribute mass : ScalarValues::Real; }
part telescope {
	part optics : Subsystem { attribute redefines mass = 8.5; }
	part mount : Subsystem { attribute redefines mass = 15.0; }
}
`

// compileInline parses and plans text over the fixture, with a resolver and
// model built over the inline index as the surfaces build them.
func compileInline(t *testing.T, f queryFixture, text string) (*Inline, *Program, error) {
	t.Helper()
	inline, err := ParseInline(f.index, text)
	if err != nil {
		return nil, nil, err
	}
	resolver := resolve.New(inline.Index)
	program, err := Compile(inline.Index, semantics.NewModel(resolver), resolver, inline.Entry)
	if err != nil {
		var planErr *Error
		if errors.As(err, &planErr) {
			err = inline.Locate(err, planErr.Origin)
		}
	}
	return inline, program, err
}

func inlineDefinition(t *testing.T, program *Program) Definition {
	t.Helper()
	for _, definition := range program.Definitions() {
		if definition.Name() == InlineDocument {
			return definition
		}
	}
	t.Fatalf("program has no inline definition: %v", program.Definitions())
	return Definition{}
}

func TestInlineBodyCompilesAsAQueryDefinition(t *testing.T) {
	f := loadQueryFixture(t, inlineModel)
	before := f.index.Documents()
	inline, program, err := compileInline(t, f, `Project(WhereType(Descendants(Named("Fixture")), "PartUsage"), ("name", "mass"))`)
	if err != nil {
		t.Fatalf("compile inline: %v", err)
	}
	if program.Entry() != InlineDocument {
		t.Errorf("entry = %q, want %q", program.Entry(), InlineDocument)
	}
	definition := inlineDefinition(t, program)
	if len(definition.Parameters()) != 0 {
		t.Errorf("inline definition declares parameters %v, want none", definition.Parameters())
	}
	if definition.Expression().Operation() != OperationProject {
		t.Errorf("operation = %s, want %s", definition.Expression().Operation(), OperationProject)
	}
	if f.index.DocumentRoot(InlineDocument) != nil {
		t.Errorf("the model's index holds the inline document")
	}
	if after := f.index.Documents(); strings.Join(after, ",") != strings.Join(before, ",") {
		t.Errorf("the model's documents changed: %v -> %v", before, after)
	}
	if inline.Index.DocumentRoot(InlineDocument) == nil {
		t.Errorf("the inline index does not hold the inline document")
	}
}

func TestInlineBodyNamesTheModelsOwnSymbols(t *testing.T) {
	f := loadQueryFixture(t, inlineModel)
	_, program, err := compileInline(t, f, `OwnedElements(Fixture::telescope)`)
	if err != nil {
		t.Fatalf("compile inline: %v", err)
	}
	args := inlineDefinition(t, program).Expression().Arguments()
	if len(args) != 1 {
		t.Fatalf("arguments = %d, want 1", len(args))
	}
	element, ok := args[0].Value.Element()
	if !ok {
		t.Fatalf("argument is not an element: %v", args[0].Value.Operation())
	}
	if want := f.symbol(t, "telescope"); element != want {
		t.Errorf("element is %p (%s), want the model's own symbol %p", element, symbols.FQNOf(element), want)
	}
}

func TestInlineParseErrorIsPositionedInTheInlineText(t *testing.T) {
	f := loadQueryFixture(t, inlineModel)
	_, _, err := compileInline(t, f, `Project(Named("Fixture"), ("name" "mass"))`)
	var inlineErr *InlineError
	if !errors.As(err, &inlineErr) {
		t.Fatalf("got %T %v, want *InlineError", err, err)
	}
	if inlineErr.Pos.Line != 1 || inlineErr.Pos.Col < 27 || inlineErr.Pos.Col > 36 {
		t.Errorf("position = %d:%d, want line 1 within the sequence", inlineErr.Pos.Line, inlineErr.Pos.Col)
	}
	if !strings.HasPrefix(err.Error(), "inline query 1:") {
		t.Errorf("message %q does not locate the fault in the inline query", err.Error())
	}
	if inlineErr.Err != nil {
		t.Errorf("a parse error wraps %v, want nothing", inlineErr.Err)
	}
	line, col := inlineErr.Line()
	if line != `Project(Named("Fixture"), ("name" "mass"))` || col != inlineErr.Pos.Col-1 {
		t.Errorf("line = %q at %d", line, col)
	}
}

func TestInlineUnterminatedBodyIsPositionedAtItsEnd(t *testing.T) {
	f := loadQueryFixture(t, inlineModel)
	text := "Named(\n\"Fixture\""
	_, _, err := compileInline(t, f, text)
	var inlineErr *InlineError
	if !errors.As(err, &inlineErr) {
		t.Fatalf("got %T %v, want *InlineError", err, err)
	}
	if inlineErr.Span.Offset > len(text) || inlineErr.Pos.Line < 2 {
		t.Errorf("position = %+v for %q", inlineErr, text)
	}
}

func TestInlinePlanningErrorIsLocatedInTheInlineText(t *testing.T) {
	f := loadQueryFixture(t, inlineModel)
	_, _, err := compileInline(t, f, `Union(Named("Fixture"), Frobnicate(Named("Fixture")))`)
	var inlineErr *InlineError
	if !errors.As(err, &inlineErr) {
		t.Fatalf("got %T %v, want *InlineError", err, err)
	}
	planErr := planningError(t, err, ErrorUnknownInvocation)
	if planErr.Target != "Frobnicate" {
		t.Errorf("target = %q", planErr.Target)
	}
	if inlineErr.Pos.Line != 1 || inlineErr.Pos.Col != 25 {
		t.Errorf("position = %d:%d, want 1:25", inlineErr.Pos.Line, inlineErr.Pos.Col)
	}
	if !strings.Contains(err.Error(), "unknown operation Frobnicate") {
		t.Errorf("message = %q", err.Error())
	}
}

func TestInlineDeclaringMoreThanAnExpressionIsRefused(t *testing.T) {
	f := loadQueryFixture(t, inlineModel)
	for _, text := range []string{
		`Named("Fixture") } package Evil { calc def Q :> DocumentQueries::Query { Named("Fixture") }`,
		"",
		"   ",
	} {
		_, _, err := compileInline(t, f, text)
		var inlineErr *InlineError
		if !errors.As(err, &inlineErr) {
			t.Errorf("%q: got %T %v, want *InlineError", text, err, err)
		}
	}
	// A body is a definition's: it may declare a parameter, bound as a named query's is.
	_, program, err := compileInline(t, f, `in root : KerML::Root::Element default Fixture::telescope; OwnedElements(root)`)
	if err != nil {
		t.Fatalf("compile inline with a parameter: %v", err)
	}
	if params := inlineDefinition(t, program).Parameters(); len(params) != 1 || params[0].Name != "root" {
		t.Errorf("parameters = %v, want root", params)
	}
}

func TestInlineLocateLeavesOtherOriginsAlone(t *testing.T) {
	f := loadQueryFixture(t, inlineModel)
	inline, err := ParseInline(f.index, `Named("Fixture")`)
	if err != nil {
		t.Fatalf("parse inline: %v", err)
	}
	elsewhere := errors.New("elsewhere")
	if got := inline.Locate(elsewhere, symbols.Origin{Doc: "queries.sysml"}); got != elsewhere {
		t.Errorf("located %v", got)
	}
	if _, ok := inline.Position(symbols.Origin{Doc: "queries.sysml"}); ok {
		t.Errorf("positioned an origin in another document")
	}
	if inline.Text() != `Named("Fixture")` {
		t.Errorf("text = %q", inline.Text())
	}
}
