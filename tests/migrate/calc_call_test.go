package migrate_test

import (
	"strings"
	"testing"
)

// A call of a calc declares its result pin once, taking the call's value: the
// pin is not declared bare and then again with the value, which would leave the
// action with two members named result (KerML 8.3.2.4.5).
func TestCalcCallDeclaresItsResultPinOnce(t *testing.T) {
	r := migrateFixtureFile(t, "calc_context")
	notation := string(r.Notation)
	start := strings.Index(notation, "action rate {")
	if start < 0 {
		t.Fatalf("no rate action:\n%s", notation)
	}
	body := notation[start : start+strings.Index(notation[start:], "\n        }")]
	if n := strings.Count(body, "out result"); n != 1 {
		t.Errorf("rate declares its result pin %d times, want once:\n%s", n, body)
	}
	if !strings.Contains(body, "out result[1] = Tank::Rate(Run::context.tank, factor);") {
		t.Errorf("rate's result pin does not take the call's value:\n%s", body)
	}
}
