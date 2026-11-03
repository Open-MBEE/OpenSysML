package repl

import (
	"strings"
	"testing"
)

// displacedObjectSource owns a verification case, a constraint and a satisfy in a
// definition one usage is declared of, so a second %instantiate of the usage
// leaves two objects of the definition in the session.
const displacedObjectSource = `package test {
	private import ScalarValues::Real;
	requirement def Tightness { subject b : Bolt; require constraint { b.torque < 10.0 } }
	requirement tight : Tightness;
	part def Bolt {
		attribute torque : Real default = 99.0;
		constraint tightEnough { torque < 10.0 }
		verification torqueTest {
			objective { verify tight; }
			return verdict = VerificationCases::PassIf(tight(this));
		}
		satisfy tight;
	}
	part b : Bolt;
}`

// TestRequirementSweepsTheObjectADisplacedNameLeft checks that %requirement runs a
// definition-owned case on each object of the definition the session holds — the one
// the name denotes and the one a second %instantiate displaced from it, labelled as
// %instances lists them — while %constraint on a definition-owned constraint is a
// question between the two until one of them is named.
func TestRequirementSweepsTheObjectADisplacedNameLeft(t *testing.T) {
	s := submitted(t, displacedObjectSource)
	first := objectIDIn(t, run(t, s, "%instantiate test::b"))
	second := objectIDIn(t, run(t, s, "%instantiate test::b"))
	if first == second {
		t.Fatalf("a second %%instantiate reused id #%s", first)
	}
	wants(t, run(t, s, "%instances"), "test::b (ID: "+second+")", "#"+first+" (ID: "+first+", displaced from test::b)")

	got := run(t, s, "%requirement test::tight")
	wants(t, got,
		"✗ Verification test::Bolt::torqueTest on #"+first+" (displaced from test::b) verdict: fail",
		"✗ Verification test::Bolt::torqueTest on test::b (object #"+second+") verdict: fail")
	if n := strings.Count(got, "Verification test::Bolt::torqueTest"); n != 2 {
		t.Errorf("%%requirement ran the case %d time(s), want once per object:\n%s", n, got)
	}
	// %satisfy already sweeps both objects; %requirement agrees with it.
	wants(t, run(t, s, "%satisfy"),
		"✗ satisfy tight fails (on #"+first+" ID: "+first+")",
		"✗ satisfy tight fails (on test::b ID: "+second+")")

	wants(t, run(t, s, "%constraint test::Bolt::tightEnough"), "error:",
		"carried by more than one object of this session (test::b, #"+first+")")
	wants(t, run(t, s, "%constraint test::Bolt::tightEnough #"+first), "failed (on #"+first+" ID: "+first+")")
	wants(t, run(t, s, "%constraint test::Bolt::tightEnough test::b"), "failed (on test::b ID: "+second+")")
	named := run(t, s, "%requirement test::tight #"+first)
	wants(t, named, "✗ Verification test::Bolt::torqueTest on #"+first+" (displaced from test::b) verdict: fail")
	if strings.Contains(named, "object #"+second) {
		t.Errorf("%%requirement on #%s reports the named object's verdict too:\n%s", first, named)
	}
}
