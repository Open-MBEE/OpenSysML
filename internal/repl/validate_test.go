package repl

import (
	"strings"
	"testing"
)

const validateFixture = "../exec/runtime/testdata/conformance/instance_validate_nested_tree.sysml"

// TestValidateObjectReportsEveryAssertion checks that %validate walks the
// object tree and reports each assertion on the object it is about, then sums up.
func TestValidateObjectReportsEveryAssertion(t *testing.T) {
	s := loadFixture(t, validateFixture)
	wants(t, run(t, s, "%instantiate test::car"), "✓ Created instance of test::car")

	out := run(t, s, "%validate car")
	wantsInOrder(t, out,
		"✓ assert constraint massOk holds (on test::car ID: 1)",
		"✓ requirement lightEnough holds (on test::car ID: 1)",
		"✗ assert constraint fails (on test::car.engine ID: 2)",
		"Assertion evaluated to false: power < 200.0",
		"✓ satisfy strongEngine by car.engine holds (on test::car.engine ID: 2)",
		"✓ assert constraint ratePositive holds (on test::car.engine.injector ID: 3)",
		"✗ assert constraint pressureOk fails (on test::car.wheels[1] ID: 4)",
		"Assertion evaluated to false: pressure >= 30.0",
		"✗ assert constraint pressureOk fails (on test::car.wheels[2] ID: 5)",
		"✗ assert constraint pressureOk fails (on test::car.wheels[3] ID: 6)",
		"✗ test::car is not valid: 4 of 8 assertions fail",
	)

	verdicts := s.ValidateObject("car")
	if len(verdicts) != 9 {
		t.Fatalf("got %d verdicts, want 8 assertions and a summary:\n%s", len(verdicts), out)
	}
	if got := WorstStatus(verdicts); got != VerdictFails {
		t.Errorf("worst status = %v, want fails", got)
	}
	if last := verdicts[len(verdicts)-1]; last.Subject != "test::car" || last.Status != VerdictFails {
		t.Errorf("summary = %+v, want car failing", last)
	}
}

// TestValidateNestedObject checks that a nested object is validated as its own
// root — its subtree alone counted, a satisfaction whose chained subject it is
// still found through its holder.
func TestValidateNestedObject(t *testing.T) {
	s := loadFixture(t, validateFixture)
	run(t, s, "%instantiate test::car")

	out := run(t, s, "%validate car.engine")
	wantsInOrder(t, out,
		"✗ assert constraint fails (on test::car.engine ID: 2)",
		"✓ satisfy strongEngine by car.engine holds (on test::car.engine ID: 2)",
		"✓ assert constraint ratePositive holds (on test::car.engine.injector ID: 3)",
		"✗ test::car.engine is not valid: 1 of 3 assertions fails",
	)
	if strings.Contains(out, "massOk") {
		t.Errorf("the car's own assertion leaked into its engine's validation:\n%s", out)
	}

	injector := run(t, s, "%validate car.engine.injector")
	wants(t, injector,
		"✓ assert constraint ratePositive holds (on test::car.engine.injector ID: 3)",
		"✓ test::car.engine.injector is valid: its 1 assertion holds",
	)
	if got := WorstStatus(s.ValidateObject("#1.engine.injector")); got != VerdictHolds {
		t.Errorf("status = %v, want holds", got)
	}
}

// TestValidateUnknownObject checks the answers to a reference that denotes no
// object and to a missing argument.
func TestValidateUnknownObject(t *testing.T) {
	s := loadFixture(t, validateFixture)
	wants(t, run(t, s, "%validate"), "usage: %validate <object>")
	wants(t, run(t, s, "%validate car"), "error:")

	verdicts := s.ValidateObject("nosuch")
	if len(verdicts) != 1 || verdicts[0].Status != VerdictUnresolved {
		t.Fatalf("verdicts = %+v, want one unresolved", verdicts)
	}
}

// TestValidateUndecidedAssertion checks that an assertion that cannot be
// evaluated is reported as undecided and keeps the object from being valid.
func TestValidateUndecidedAssertion(t *testing.T) {
	s := loadFixture(t, "../exec/runtime/testdata/conformance/instance_validate_undecided.sysml")
	run(t, s, "%instantiate test::Tank")

	out := run(t, s, "%validate #1")
	wants(t, out, "? ", "is not shown valid", "undecided")
	if got := WorstStatus(s.ValidateObject("#1")); got != VerdictUnresolved {
		t.Errorf("status = %v, want unresolved", got)
	}
}

// TestValidateObjectStatingNoAssertion checks that an object no assertion is
// about is not shown valid: nothing was decided, and the standing says so too.
func TestValidateObjectStatingNoAssertion(t *testing.T) {
	s := loadFixture(t, "../exec/runtime/testdata/conformance/instance_validate_no_assertion.sysml")
	run(t, s, "%instantiate test::crate")

	out := run(t, s, "%validate crate")
	wants(t, out, "? test::crate states no assertion to validate")
	if strings.Contains(out, "standing: holds") {
		t.Errorf("an object nothing was decided about is not standing:\n%s", out)
	}
	verdicts := s.ValidateObject("crate")
	if len(verdicts) != 1 || verdicts[0].Status != VerdictUnresolved {
		t.Errorf("verdicts = %+v, want the summary alone, unresolved", verdicts)
	}
}
