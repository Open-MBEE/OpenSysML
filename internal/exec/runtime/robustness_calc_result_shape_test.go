package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessCalcResultShape covers a calculation result held in the shape its
// declared multiplicity gives it: a count or uniqueness the body's value breaks is refused
// as a typed error, whichever shape the declaration states.
func TestRuntimeRobustnessCalcResultShape(t *testing.T) {
	t.Run("result_one_refuses_a_pair", testCalcResultOneRefusesAPair)
	t.Run("result_fixed_two_refuses_a_single", testCalcResultFixedTwoRefusesASingle)
	t.Run("result_many_refuses_a_duplicate", testCalcResultManyRefusesADuplicate)
	t.Run("named_result_many_refuses_a_duplicate", testCalcNamedResultManyRefusesADuplicate)
}

// testCalcResultOneRefusesAPair: an explicit [1] result holds one value, so a pair
// computed for it is a multiplicity violation, not a sequence or its first element.
func testCalcResultOneRefusesAPair(t *testing.T) {
	src := `
		package test {
			calc def Pair {
				in n : Integer;
				return : Integer[1] = (n, n + 1);
			}
		}
	`
	err := invokeCalcInSource(t, src, "Pair", 3, 10000)
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("expected a multiplicity violation, got: %v", err)
	}
}

// testCalcResultFixedTwoRefusesASingle: a [2] result holds two values, so one is refused
// rather than wrapped into a one-element sequence.
func testCalcResultFixedTwoRefusesASingle(t *testing.T) {
	src := `
		package test {
			calc def Single {
				in n : Integer;
				return : Integer[2] = n;
			}
		}
	`
	err := invokeCalcInSource(t, src, "Single", 3, 10000)
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("expected a multiplicity violation, got: %v", err)
	}
}

// testCalcResultManyRefusesADuplicate: a [0..*] result is a unique feature, so the
// duplicate in the sequence it would hold is a uniqueness violation.
func testCalcResultManyRefusesADuplicate(t *testing.T) {
	src := `
		package test {
			calc def Twice {
				in n : Integer;
				return : Integer[0..*] = (n, n);
			}
		}
	`
	err := invokeCalcInSource(t, src, "Twice", 3, 10000)
	if !errors.Is(err, ErrUniquenessViolation) {
		t.Errorf("expected a uniqueness violation, got: %v", err)
	}
}

// testCalcNamedResultManyRefusesADuplicate: a named result is judged as the anonymous one.
func testCalcNamedResultManyRefusesADuplicate(t *testing.T) {
	src := `
		package test {
			calc def TwiceNamed {
				in n : Integer;
				return r : Integer[0..*] = (n, n);
			}
		}
	`
	err := invokeCalcInSource(t, src, "TwiceNamed", 3, 10000)
	if !errors.Is(err, ErrUniquenessViolation) {
		t.Errorf("expected a uniqueness violation, got: %v", err)
	}
}
