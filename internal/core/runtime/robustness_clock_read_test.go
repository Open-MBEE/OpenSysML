package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessClockRead exercises the failure modes of reading the
// shared clock through an occurrence's `localClock.currentTime`: a write to the
// clock, and a read through a part that holds no object.
func TestRuntimeRobustnessClockRead(t *testing.T) {
	t.Run("current_time_is_not_assigned", testClockReadCurrentTimeNotAssigned)
	t.Run("own_current_time_is_not_assigned", testClockReadOwnCurrentTimeNotAssigned)
	t.Run("clock_of_a_part_holding_no_object", testClockReadOfPartHoldingNoObject)
	t.Run("current_time_reads_the_run_clock", testClockReadFollowsTheRunClock)
	t.Run("time_instant_is_no_point_on_a_named_scale", testClockReadInstantAgainstNamedScale)
}

// stationObserving is a station whose exhibited machine performs Observe on
// entry, with the given body of Observe.
func stationObserving(body string) string {
	return `package test {
		private import ScalarValues::*;
		part def Telescope { attribute azimuth : Real default = 0.0; }
		part def Station {
			part tel : Telescope[0..1];
			attribute now : Real default = -1.0;
			action def Observe {
				first start then a;
				action a { ` + body + ` }
				first a then done;
			}
			exhibit state run {
				entry; then observing;
				state observing { entry action observe : Observe; }
			}
		}
	}`
}

// testClockReadCurrentTimeNotAssigned: the clock advances with the run, so an
// assignment to its currentTime is refused with a typed error, not stored.
func testClockReadCurrentTimeNotAssigned(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, stationObserving("assign localClock.currentTime := 5.0;"), "test::Station")
	if !errors.Is(err, ErrClockNotAssignable) || !strings.Contains(err.Error(), "assignment to localClock.currentTime") {
		t.Fatalf("error = %v, want ErrClockNotAssignable over localClock.currentTime", err)
	}
}

// testClockReadOwnCurrentTimeNotAssigned: a clock's own body naming currentTime
// bare is refused the same way, not stored as data of the body.
func testClockReadOwnCurrentTimeNotAssigned(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		private import Clocks::*;
		part def Chrono :> Clock {
			attribute seen : Real default = -1.0;
			action def Tick {
				first start then a;
				action a { assign currentTime := 5.0; assign seen := currentTime; }
				first a then done;
			}
			exhibit state run {
				entry; then ticking;
				state ticking { entry action tick : Tick; }
			}
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Chrono")
	if !errors.Is(err, ErrClockNotAssignable) || !strings.Contains(err.Error(), "assignment to currentTime") {
		t.Fatalf("error = %v, want ErrClockNotAssignable over a bare currentTime", err)
	}
}

// testClockReadOfPartHoldingNoObject: a part holding nothing has no clock to
// read, so the read yields no value and the assignment of it is refused.
func testClockReadOfPartHoldingNoObject(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, stationObserving("assign now := tel.localClock.currentTime;"), "test::Station")
	if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "write now of object") {
		t.Fatalf("error = %v, want ErrMultiplicityViolation writing an empty clock read to now", err)
	}
}

// testClockReadFollowsTheRunClock: the run starts at instant zero, so a read of
// the clock at the first step holds zero, not the wall clock.
func testClockReadFollowsTheRunClock(t *testing.T) {
	ctx, inst, err := instantiateWithLibraries(t, stationObserving("assign now := localClock.currentTime;"), "test::Station")
	if err != nil {
		t.Fatal(err)
	}
	fv, err := inst.GetFeatureValue(ctx, "now")
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatValue(fv.Value); got != "0.0" {
		t.Fatalf("now = %v, want the clock's instant zero", got)
	}
}

// testClockReadInstantAgainstNamedScale: the Time clock's instant is on the
// clock's own scale, so comparing it with a point on UTC, which the library
// relates to no other reference, is refused rather than compared by magnitude.
func testClockReadInstantAgainstNamedScale(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		private import Time::*;
		part def Station {
			attribute epoch : TimeInstantValue = 0 [UTC];
			attribute late : Boolean default = false;
			action def Observe {
				first start then a;
				action a { assign late := Time::universalClock.currentTime > epoch; }
				first a then done;
			}
			exhibit state run {
				entry; then observing;
				state observing { entry action observe : Observe; }
			}
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Station")
	if !errors.Is(err, ErrUnevaluableLibraryFunction) || !strings.Contains(err.Error(), "Time::UTC") {
		t.Fatalf("error = %v, want ErrUnevaluableLibraryFunction naming Time::UTC", err)
	}
}
