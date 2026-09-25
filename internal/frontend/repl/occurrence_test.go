package repl

import (
	"fmt"
	"testing"
)

const occurrenceModel = `package Shop {
	part def Widget { attribute n : ScalarValues::Integer = 1; }
	part def Bench {
		part a : Widget;
		part b : Widget;
		part c : Widget;
		part spares : Widget[0..*];
	}
	part bench : Bench;
}`

// The OccurrenceFunctions answer over the objects a session holds: `===` is
// identity of occurrences, `destroy` ends one for good, and the listings say so.
func TestOccurrenceFunctionsInSession(t *testing.T) {
	s := submitted(t, occurrenceModel)
	run(t, s, "%instantiate bench")
	bench := s.instances["Shop::bench"]
	id := fmt.Sprintf("#%d", bench.ID)
	eval := func(expr string) string { return run(t, s, "%eval in "+id+" : "+expr) }

	wants(t, eval("a === a"), "= true")
	wants(t, eval("a === b"), "= false")
	wants(t, eval("a.n == b.n"), "= true")
	wants(t, eval("OccurrenceFunctions::'==='(a, b)"), "= false")
	wants(t, eval("OccurrenceFunctions::isDuring(a)"), "= true")
	wants(t, eval("OccurrenceFunctions::isDuring(a.n)"), "value is not an occurrence")
	wants(t, eval("SequenceFunctions::size(OccurrenceFunctions::addNew(spares, c))"), "= 1")
	wants(t, eval("OccurrenceFunctions::addNew(spares, b)"), "began at 1, before the call")
	wants(t, eval("OccurrenceFunctions::addNewAt(spares, b, 2)"), "insertion index 2 is outside 1..1")

	wants(t, eval("OccurrenceFunctions::destroy(a) === a"), "= true")
	wants(t, eval("OccurrenceFunctions::isDuring(a)"), "= false")
	wants(t, eval("a.n"), "occurrence was destroyed")
	wants(t, eval("OccurrenceFunctions::destroy(a)"), "occurrence was destroyed")
	wants(t, run(t, s, "%features "+id), "a = ", "(destroyed at ")
	rejects(t, run(t, s, "%instances"), "destroyed")

	wants(t, run(t, s, "%eval OccurrenceFunctions::destroy(bench) === bench"), "= true")
	wants(t, run(t, s, "%instances"), fmt.Sprintf("Shop::bench (ID: %d, destroyed)", bench.ID))
	wants(t, run(t, s, "%features "+id), "(destroyed at ")
}
