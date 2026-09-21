package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessBindingEndNames covers how a name at a binding end of a
// performed action resolves: parameter before performer feature, never a cycle.
func TestRuntimeRobustnessBindingEndNames(t *testing.T) {
	t.Run("parameter_masks_the_performers_feature", testBindingEndParameterMasksFeature)
	t.Run("pin_valued_by_itself_reads_what_it_masks", testBindingEndPinReadsWhatItMasks)
	t.Run("name_resolving_to_nothing_is_refused", testBindingEndUnresolvedName)
	t.Run("required_pin_given_none_is_refused", testBindingEndRequiredPinGivenNone)
}

// bindingEndHost is a part whose performed action has a parameter named as the
// part's own attribute; the pin of a nested action is bound to that name.
func bindingEndHost(pin, binding string) string {
	return `
	package test {
		private import ScalarValues::*;
		private import SequenceFunctions::*;
		part def Host {
			attribute level : Integer = 7;
			attribute seen : Integer = -1;
			perform action relaying {
				in level : Integer[0..1];
				first start;
				action noting {
					` + pin + `
					assign seen := size(n);
				}
				` + binding + `
				done;
				succession first start then noting;
				succession first noting then done;
			}
		}
	}`
}

// testBindingEndParameterMasksFeature: `bind noting.n = level` written in the
// action's body names the action's parameter, given nothing, not the part's 7.
func testBindingEndParameterMasksFeature(t *testing.T) {
	ctx, inst, err := instantiateWithLibraries(t, bindingEndHost("in n : Integer[0..1];", "bind noting.n = level;"), "test::Host")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if got := readInt(t, ctx, inst, "seen"); got != 0 {
		t.Fatalf("seen = %v, want 0: the parameter admitting none masks the part's level", got)
	}
}

// testBindingEndPinReadsWhatItMasks: `inout n = n` reads the n around the usage,
// not the pin itself nor the parameter it redefines.
func testBindingEndPinReadsWhatItMasks(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		action def Noting {
			inout n : Integer;
			out twice : Integer;
			first start;
			action doubling { assign twice := 2 * n; }
			done;
			succession first start then doubling;
			succession first doubling then done;
		}
		part def Host {
			attribute n : Integer = 7;
			attribute seen : Integer = -1;
			perform action relaying {
				first start;
				action noting : Noting { inout n = n; }
				action recording { assign seen := noting.twice; }
				done;
				succession first start then noting;
				succession first noting then recording;
				succession first recording then done;
			}
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Host")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if got := readInt(t, ctx, inst, "seen"); got != 14 {
		t.Fatalf("seen = %v, want 14: the pin reads the part's n", got)
	}
}

// testBindingEndUnresolvedName: a binding end naming nothing is ErrBindingEnd.
func testBindingEndUnresolvedName(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, bindingEndHost("in n : Integer[0..1];", "bind noting.n = nowhere;"), "test::Host")
	if !errors.Is(err, ErrBindingEnd) || !strings.Contains(err.Error(), "nowhere") {
		t.Fatalf("error = %v, want ErrBindingEnd naming nowhere", err)
	}
}

// testBindingEndRequiredPinGivenNone: a required pin bound to an empty parameter
// is refused, not filled from the part.
func testBindingEndRequiredPinGivenNone(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, bindingEndHost("in n : Integer;", "bind noting.n = level;"), "test::Host")
	if err == nil {
		t.Fatal("a required pin bound to an empty parameter was accepted")
	}
	if !errors.Is(err, ErrUnboundParameter) && !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("error = %v, want a typed refusal of the empty required pin", err)
	}
}
