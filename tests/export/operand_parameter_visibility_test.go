package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// An operator's operand is a private parameter and an invocation's argument a public
// one, as the pilot writes them (its OperandEList states the first; KerMLExpressions.xtext
// ArgumentMember leaves the second); a classification operator's type argument is public too.
func TestOperandParametersArePrivateAndArgumentsPublic(t *testing.T) {
	const model = `package Ops {
    private import ScalarValues::*;
    part def P;
    part def Q :> P;
    calc def F { in x : Real; in y : Real; return : Real = x + y; }
    part p : P;
    attribute sum : Real = 1.0 + 2.0;
    attribute called : Real = F(1.0, 2.0);
    attribute tested : Boolean = p istype Q;
    attribute all : Boolean = (1..3)->forAll { in i : Positive; i > 0 };
    attribute piped : Real = 1.0->F(2.0);
    attribute summed : Real = (1.0, 2.0)->reduce '+';
    calc def H { in body; return : Real = 1.0; }
    attribute applied : Real = H({ in x : Real; x });
}
`
	turtle, err := convert.Convert("ops.sysml", []byte(model), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	memberships := func(subject string) []string {
		var out []string
		for _, block := range strings.Split(graph, "\n\n") {
			if strings.Contains(block, "a sysml:ParameterMembership") && strings.Contains(block, subject) {
				out = append(out, block)
			}
		}
		return out
	}
	assertVisibility := func(name, subject, want string) {
		t.Helper()
		found := memberships(subject)
		if len(found) == 0 {
			t.Fatalf("%s: no ParameterMembership mentions %s", name, subject)
		}
		for _, block := range found {
			if got := strings.Contains(block, `sysml:visibility "private"`); got != (want == "private") {
				t.Errorf("%s: ParameterMembership visibility private=%v, want %s:\n%s", name, got, want, block)
			}
		}
	}
	assertVisibility("operator operand", "Ops__sum_", "private")
	assertVisibility("invocation argument", "Ops__called_", "public")
	assertVisibility("`->` receiver and body", "Ops__all_pvalue_pin", "private")
	countPrivate := func(name, subject string, wantPrivate, wantTotal int) {
		t.Helper()
		private := 0
		for _, block := range memberships(subject) {
			if strings.Contains(block, `sysml:visibility "private"`) {
				private++
			}
		}
		if total := len(memberships(subject)); private != wantPrivate || total != wantTotal {
			t.Fatalf("%s: %d of %d ParameterMemberships private, want %d of %d", name, private, total, wantPrivate, wantTotal)
		}
	}
	// `1.0->F(2.0)`: the receiver is an operand, the listed argument an argument.
	countPrivate("`->` receiver and argument", "Ops__piped_pvalue_pin", 1, 2)
	// `xs->reduce '+'`: the bare operand is written without parentheses, private like the receiver.
	countPrivate("`->` receiver and bare operand", "Ops__summed_pvalue_pin", 2, 2)
	// `H({ … })`: a body inside an argument list is an argument, public.
	countPrivate("body in an argument list", "Ops__applied_pvalue_pin", 0, 1)
	back, err := convert.Convert("ops.ttl", turtle, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, want := range []string{"->reduce '+'", "->forAll {", "H({"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("notation from the graph lacks %q:\n%s", want, back)
		}
	}
	// The operator's operand is private, its type argument public.
	countPrivate("istype operand and type argument", "Ops__tested_", 1, 2)
}
