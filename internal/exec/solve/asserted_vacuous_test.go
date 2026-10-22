package solve

import (
	"errors"
	"testing"
)

// TestAssertedDeniedVacuousSatisfactionIsFalse: a part asserting `not satisfy` of a
// requirement that states no condition denies a claim that is true, so the query
// about its objects asserts false; a part asserting only the plain satisfaction
// states nothing that constrains them.
func TestAssertedDeniedVacuousSatisfactionIsFalse(t *testing.T) {
	ctx, idx := fixture(t, "<test>", `
		package test {
			requirement def Documented { doc /* prose only */ }
			requirement documented : Documented;
			part lander;
			part denied {
				assert not satisfy documented by lander;
			}
			part affirmed {
				assert satisfy documented by lander;
			}
		}
	`)
	denied := symbolNamed(t, idx, "test::denied")
	q, err := Asserted(ctx, denied, denied.OwnerScope)
	if err != nil {
		t.Fatalf("translate denied: %v", err)
	}
	if len(q.Assertions) != 1 || writeTerm(q.Assertions[0].Term) != "(not true)" {
		t.Fatalf("the denial of a vacuous satisfaction is not false:\n%s", Script(q))
	}
	affirmed := symbolNamed(t, idx, "test::affirmed")
	if _, err := Asserted(ctx, affirmed, affirmed.OwnerScope); !errors.Is(err, ErrNoConditions) {
		t.Fatalf("translate affirmed: %v, want ErrNoConditions", err)
	}
}
