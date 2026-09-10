package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// libStateExecutor builds an initialized executor over the named machine in src,
// with the standard library loaded so quantities reduce against real units.
func libStateExecutor(t *testing.T, name, src string) *StateExecutor {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefState)
	if sym == nil {
		t.Fatalf("state machine %s not found", name)
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("create state executor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return exec
}

// A duration carrying a unit schedules the delay the unit says, converted to the
// clock's second: `1 [min]` waits sixty times as long as `1 [s]`.
func TestTimeTriggerUnitIsConverted(t *testing.T) {
	for _, tc := range []struct {
		name     string
		duration string
		want     float64
	}{
		{"seconds", "5 [s]", 5},
		{"real seconds", "2.5 [s]", 2.5},
		{"minutes", "1 [min]", 60},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec := libStateExecutor(t, "Machine", `package test {
				private import SI::*;
				state Machine {
					entry; then start;
					state start;
					state waiting;
					accept after `+tc.duration+` then done;
					state done;
					succession first start then waiting;
				}
			}`)
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("run: %v", err)
			}
			assertCurrentState(t, exec, "done")
			if exec.CurrentTime() != tc.want {
				t.Errorf("clock = %v, want %v", exec.CurrentTime(), tc.want)
			}
		})
	}
}

// A unit smaller than the clock's second converts by its own scale, so a
// sub-second duration is neither rounded to the base unit nor rejected.
func TestTimeTriggerSubSecondUnit(t *testing.T) {
	exec := libStateExecutor(t, "Machine", `package test {
		private import SI::*;
		attribute <ms> millisecond : ISQBase::DurationUnit {
			:>> unitConversion: MeasurementReferences::ConversionByConvention {
				:>> referenceUnit = s;
				:>> conversionFactor = 1/1000;
			}
		}
		state Machine {
			entry; then start;
			state start;
			state waiting;
			accept after 500 [ms] then done;
			state done;
			succession first start then waiting;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "done")
	if exec.CurrentTime() != 0.5 {
		t.Errorf("clock = %v, want 0.5 seconds", exec.CurrentTime())
	}
}

// An instant names a time on the same clock a duration measures on, and one
// already past fires at once rather than moving the clock back.
func TestTimeTriggerAbsoluteInstantWithUnit(t *testing.T) {
	exec := libStateExecutor(t, "Machine", `package test {
		private import SI::*;
		private import Time::*;
		state Machine {
			attribute due : TimeInstantValue = 2 [min];
			entry; then start;
			state start;
			state waiting;
			accept at due then done;
			state done;
			succession first start then waiting;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "done")
	if exec.CurrentTime() != 120 {
		t.Errorf("clock = %v, want 120 seconds", exec.CurrentTime())
	}
}

// An argument the library types as no duration or no instant is refused before
// it is scheduled, by the judgement validation makes of the same text.
func TestTimeTriggerRefusesTheTypeValidationRefuses(t *testing.T) {
	for _, tc := range []struct {
		name, trigger, want string
	}{
		{"bare number after", "after 5", "`after 5` must be a ISQBase::DurationValue, found Natural"},
		{"real after", "after 2.5", "`after 2.5` must be a ISQBase::DurationValue, found Rational"},
		{"duration at", "at 2 [min]", "`at 2 [min]` must be a Time::TimeInstantValue, found a quantity in minute"},
		{"bare number at", "at 5", "`at 5` must be a Time::TimeInstantValue, found Natural"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec := libStateExecutor(t, "Machine", `package test {
				private import SI::*;
				state Machine {
					entry; then start;
					state start;
					state waiting;
					accept `+tc.trigger+` then done;
					state done;
					succession first start then waiting;
				}
			}`)
			err := exec.RunToCompletion()
			if err == nil {
				t.Fatalf("accept %s scheduled", tc.trigger)
			}
			if !errors.Is(err, ErrTimeTriggerType) {
				t.Errorf("err = %v, want it to wrap ErrTimeTriggerType", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// A feature typed by the trigger's type is admitted whatever value it holds,
// as validation admits it, and its value is what gets scheduled.
func TestTimeTriggerAdmitsATypedFeature(t *testing.T) {
	exec := libStateExecutor(t, "Machine", `package test {
		private import SI::*;
		private import ISQBase::*;
		state Machine {
			attribute delay : DurationValue = 3 [s];
			entry; then start;
			state start;
			state waiting;
			accept after delay then done;
			state done;
			succession first start then waiting;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "done")
	if exec.CurrentTime() != 3 {
		t.Errorf("clock = %v, want 3 seconds", exec.CurrentTime())
	}
}

// A duration that does not measure time is refused as the type mismatch
// validation reports, before its value is ever computed.
func TestTimeTriggerRejectsNonTimeDimension(t *testing.T) {
	exec := libStateExecutor(t, "Machine", `package test {
		private import SI::*;
		state Machine {
			entry; then start;
			state start;
			state waiting;
			accept after 5 [kg] then done;
			state done;
			succession first start then waiting;
		}
	}`)
	err := exec.RunToCompletion()
	if err == nil {
		t.Fatal("scheduling a mass as a duration succeeded")
	}
	if want := "`after 5 [kg]` must be a ISQBase::DurationValue, found a quantity in kilogram"; !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want it to contain %q", err, want)
	}
	if !errors.Is(err, ErrTimeTriggerType) {
		t.Errorf("err = %v, want it to wrap ErrTimeTriggerType", err)
	}
	if errors.Is(err, ErrIncommensurableUnits) {
		t.Errorf("err = %v, want the static refusal, not a conversion of the value", err)
	}
}

// Converting a quantity the declarations left open keeps the dimension error its
// identity, so a caller can branch on a mass scheduled where a time was due.
func TestTimeMagnitudeRejectsNonTimeDimension(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import SI::*;
		state Machine { entry; then start; state start; }
	}`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	mass, err := evalIn(t, ctx, pkg.Scope, "5 [kg]")
	if err != nil {
		t.Fatalf("eval 5 [kg]: %v", err)
	}
	_, err = ctx.timeMagnitude(mass, "time duration")
	if err == nil {
		t.Fatal("converting a mass to seconds succeeded")
	}
	if !strings.Contains(err.Error(), "5 [kg] is not a time") {
		t.Errorf("err = %v, want it to name the quantity as written", err)
	}
	if !errors.Is(err, ErrIncommensurableUnits) {
		t.Errorf("err = %v, want it to wrap ErrIncommensurableUnits", err)
	}
}
