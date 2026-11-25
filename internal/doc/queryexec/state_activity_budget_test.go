package queryexec

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// stateActivityBudgetFixture declares many states over a session holding many
// objects: a per-state walk of the session would exhaust the visit budget.
func stateActivityBudgetFixture() string {
	const states, objects = 400, 300
	var b strings.Builder
	b.WriteString("private import ScalarValues::*;\nstate def Machine {\n")
	for i := 0; i < states; i++ {
		fmt.Fprintf(&b, "\tstate s%d;\n", i)
	}
	b.WriteString("}\npart plain;\n")
	for i := 0; i < objects; i++ {
		fmt.Fprintf(&b, "part p%d;\n", i)
	}
	b.WriteString(`
calc def ActivityQuery :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, relationships = true), type = "StateUsage", exact = true),
		properties = ("name", "isActive"), tolerate = true
	)
}
calc def InvokedTwice :> Query {
	in root : Element;
	in allA : Element[0..*] = ActivityQuery(root = root);
	in allB : Element[0..*] = ActivityQuery(root = root);
	WhereName(source = Union(source = allA, other = allB), operator = "startsWith", value = "")
}
`)
	return b.String()
}

// One query reads the activity of every state from one walk of the session,
// however many objects it holds and however many rows ask for the property.
func TestQueryStateActivityStaysWithinTheVisitBudget(t *testing.T) {
	fixture := loadExecutionFixture(t, stateActivityBudgetFixture())
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	var roots []Root
	for i := 0; i < 300; i++ {
		object, err := ctx.Instantiate(fixture.symbol(t, fmt.Sprintf("p%d", i)))
		if err != nil {
			t.Fatalf("Instantiate p%d: %v", i, err)
		}
		roots = append(roots, Root{Label: fmt.Sprintf("p%d", i), Object: object})
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx, Roots: roots,
	}
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "Machine"))}}
	result, err := Execute(fixture.program(t, "ActivityQuery"), context, bindings, Options{})
	if err != nil {
		t.Fatalf("execute Activity: %v", err)
	}
	if got := len(result.Rows()); got != 400 {
		t.Fatalf("activity rows = %d, want 400", got)
	}
	// Two invoked document queries share the index built by the first read.
	result, err = Execute(fixture.program(t, "InvokedTwice"), context, bindings, Options{})
	if err != nil {
		t.Fatalf("execute InvokedTwice: %v", err)
	}
	if got := len(result.Rows()); got != 400 {
		t.Fatalf("invoked rows = %d, want 400", got)
	}
}
