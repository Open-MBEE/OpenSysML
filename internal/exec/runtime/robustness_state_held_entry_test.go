package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessStateHeldEntry: an entry parked for a message holds the state's body
// back without ending the run; exiting the state abandons the parked entry.
func TestRuntimeRobustnessStateHeldEntry(t *testing.T) {
	t.Run("composite_entry_accept_with_no_sender_enters_no_region", testCompositeEntryAcceptWithNoSenderEntersNoRegion)
	t.Run("machine_entry_accept_with_no_sender_enters_no_start", testMachineEntryAcceptWithNoSenderEntersNoStart)
	t.Run("timed_exit_abandons_parked_entry", testTimedExitAbandonsParkedEntry)
	t.Run("entry_clock_wait_is_still_refused", testEntryClockWaitIsStillRefused)
}

func testCompositeEntryAcceptWithNoSenderEntersNoRegion(t *testing.T) {
	var visits []string
	err := runWithWatchdog(t, func() (err error) {
		_, visits, err = executeStateSource(t, "M", `package P {
			attribute def Sig;
			state def M {
				entry; then s;
				state s parallel {
					entry accept a : Sig;
					state left { entry; then a1; state a1; }
					state right { entry; then b1; state b1; }
				}
			}
		}`)
		return err
	})
	if err != nil {
		t.Fatalf("error = %v, want the run to end with the entry parked", err)
	}
	if len(visits) != 1 || visits[0] != "s" {
		t.Fatalf("stateVisits = %v, want [s]: the regions wait for the entry accept", visits)
	}
}

func testMachineEntryAcceptWithNoSenderEntersNoStart(t *testing.T) {
	var visits []string
	err := runWithWatchdog(t, func() (err error) {
		_, visits, err = executeStateSource(t, "M", `package P {
			attribute def Sig;
			state def M {
				entry accept a : Sig;
				then s;
				state s;
			}
		}`)
		return err
	})
	if err != nil {
		t.Fatalf("error = %v, want the run to end with the machine's entry parked", err)
	}
	if len(visits) != 0 {
		t.Fatalf("stateVisits = %v, want none: the start waits for the entry accept", visits)
	}
}

func testTimedExitAbandonsParkedEntry(t *testing.T) {
	var visits []string
	err := runWithWatchdog(t, func() (err error) {
		exec := stateMachineWithLibraries(t, `package test {
			private import SI::*;
			attribute def Sig;
			state Machine {
				entry; then outer;
				state outer {
					entry; then s;
					state s {
						entry accept a : Sig;
					}
				}
				transition first outer accept after 1 [SI::s] then done;
				state done;
			}
		}`)
		if err := exec.initialize(); err != nil {
			return err
		}
		err = exec.RunToCompletion()
		visits = exec.stateVisits
		return err
	})
	if err != nil {
		t.Fatalf("error = %v, want the timed transition to leave the parked entry behind", err)
	}
	if strings.Join(visits, ",") != "outer,s,done" {
		t.Fatalf("stateVisits = %v, want [outer s done]", visits)
	}
}

func testEntryClockWaitIsStillRefused(t *testing.T) {
	exec := stateMachineWithLibraries(t, `package test {
		private import SI::*;
		attribute def Sig;
		state Machine {
			entry; then s;
			state s {
				entry action {
					accept after 1 [SI::s];
					then accept a : Sig;
				}
			}
		}
	}`)
	err := exec.initialize()
	if !errors.Is(err, errStateBehaviorWaits) {
		t.Fatalf("error = %v, want errStateBehaviorWaits: only a message wait parks an entry", err)
	}
}
