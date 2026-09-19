package runtime

import "testing"

// A transition between two regions of one composite state exits its source only:
// KerML StateTransitionPerformance orders `guard then transitionLinkSource.exit`,
// so the composite state and the regions holding neither endpoint are untouched.
func TestCrossRegionTransitionExitsSourceOnly(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute crossed : Integer = 0;
		attribute sourceExits : Integer = 0;
		attribute otherExits : Integer = 0;
		attribute otherEntries : Integer = 0;
		attribute runningExits : Integer = 0;

		entry; then init;
		state init;
		state running parallel {
			exit { assign runningExits := runningExits + 1; }

			state left {
				entry; then ls;
				state ls;
				state lidle { exit { assign sourceExits := sourceExits + 1; } }
				succession first ls then lidle;
				transition first lidle if crossed == 0 then rtarget;
			}
			state right {
				entry; then rs;
				state rs;
				state ridle;
				state rtarget { entry { assign crossed := crossed + 1; } }
				succession first rs then ridle;
			}
			state other {
				entry; then os;
				state os;
				state oidle {
					entry { assign otherEntries := otherEntries + 1; }
					exit { assign otherExits := otherExits + 1; }
				}
				succession first os then oidle;
			}
		}

		succession first init then running;
	}
}`)

	assertRegionConfig(t, exec, map[string]string{"right": "rtarget", "other": "oidle"})
	if got := countVisits(exec.stateVisits, "running"); got != 1 {
		t.Errorf("running entered %d times, want 1 (the composite state is not re-entered)", got)
	}
	for name, want := range map[string]int64{
		"crossed":      1,
		"sourceExits":  1,
		"otherEntries": 1,
		"otherExits":   0,
		"runningExits": 0,
	} {
		if got := intValue(t, exec.stateData, name); got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
}

// A cross-region transition whose target is nested inside the target region's
// composite state moves that inner region: the composite state stays active and
// the state it was running exits, leaving one active state per region.
func TestCrossRegionTransitionIntoNestedTargetExitsTheAbandonedState(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute crossed : Integer = 0;
		attribute innerExits : Integer = 0;
		attribute wrapperEntries : Integer = 0;

		entry; then init;
		state init;
		state running parallel {
			state left {
				entry; then ls;
				state ls;
				state lmid;
				state lidle;
				succession first ls then lmid;
				succession first lmid then lidle;
				transition first lidle if crossed == 0 then rtarget;
			}
			state right {
				entry; then rs;
				state rs;
				state wrapper parallel {
					entry { assign wrapperEntries := wrapperEntries + 1; }

					state inner {
						entry; then is;
						state is;
						state ridle { exit { assign innerExits := innerExits + 1; } }
						state rtarget { entry { assign crossed := crossed + 1; } }
						succession first is then ridle;
					}
				}
				succession first rs then wrapper;
			}
		}

		succession first init then running;
	}
}`)

	assertRegionConfig(t, exec, map[string]string{"right": "wrapper", "inner": "rtarget"})
	for _, name := range []string{"wrapper", "running"} {
		if got := countVisits(exec.stateVisits, name); got != 1 {
			t.Errorf("%s entered %d times, want 1 (it is neither exited nor re-entered)", name, got)
		}
	}
	for name, want := range map[string]int64{"crossed": 1, "innerExits": 1, "wrapperEntries": 1} {
		if got := intValue(t, exec.stateData, name); got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
}

// A region whose cross-region target is nested inside a composite state it is not
// running records that composite state as its active one, so the target is not
// claimed by two regions at once and is exited once when the machine leaves.
func TestCrossRegionTransitionIntoInactiveCompositeRecordsTheEnteredState(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute crossed : Integer = 0;
		attribute rtargetExits : Integer = 0;

		entry; then init;
		state init;
		state running parallel {
			state left {
				entry; then ls;
				state ls;
				state lidle;
				succession first ls then lidle;
				transition first lidle if crossed == 0 then rtarget;
			}
			state right {
				entry; then rs;
				state rs;
				state ridle;
				state wrapper parallel {
					state inner {
						entry; then is;
						state is;
						state istart;
						state rtarget {
							entry { assign crossed := crossed + 1; }
							exit { assign rtargetExits := rtargetExits + 1; }
						}
						succession first is then istart;
					}
				}
				succession first rs then ridle;
			}
		}
		state done;

		succession first init then running;
		transition first rtarget if crossed == 1 then done;
	}
}`)

	if got := exec.getCurrentState(); got == nil || got.Name != "done" {
		t.Fatalf("current state = %v, want done", got)
	}
	if got := intValue(t, exec.stateData, "rtargetExits"); got != 1 {
		t.Errorf("rtargetExits = %d, want 1 (the target is exited once, not through two regions)", got)
	}
}

// A source active in a region nested deeper than its target's region leaves its own
// region set up to the level the two regions share: the source state and the
// composite state holding its region exit, the target region's old state exits, and
// the composite state owning both regions stays active.
func TestCrossRegionTransitionFromDeeperRegionExitsUpToTheSharedLevel(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute lidleExits : Integer = 0;
		attribute deepExits : Integer = 0;
		attribute wrapperExits : Integer = 0;

		entry; then init;
		state init;
		state running parallel {
			state left {
				entry; then ls;
				state ls;
				state lidle { exit { assign lidleExits := lidleExits + 1; } }
				state lstate;
				succession first ls then lidle;
			}
			state right {
				entry; then rs;
				state rs;
				state wrapper parallel {
					exit { assign wrapperExits := wrapperExits + 1; }

					state inner {
						entry; then is;
						state is;
						state ideep { exit { assign deepExits := deepExits + 1; } }
						succession first is then ideep;
						transition first ideep if lidleExits == 0 then lstate;
					}
				}
				succession first rs then wrapper;
			}
		}

		succession first init then running;
	}
}`)

	assertRegionConfig(t, exec, map[string]string{"left": "lstate"})
	if got := countVisits(exec.stateVisits, "running"); got != 1 {
		t.Errorf("running entered %d times, want 1 (the composite state owning both regions stays active)", got)
	}
	for _, name := range []string{"lidleExits", "deepExits", "wrapperExits"} {
		if got := intValue(t, exec.stateData, name); got != 1 {
			t.Errorf("%s = %d, want 1", name, got)
		}
	}
}

// The state owning the source's region may be a substate rather than a state of a
// region: the concurrent region holding the target is still found, so it exits the
// state it was running instead of keeping it beside the target.
func TestCrossRegionTransitionFromARegionOwnedByASubstate(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute lidleExits : Integer = 0;
		attribute midExits : Integer = 0;

		entry; then init;
		state init;
		state running parallel {
			state left {
				entry; then ls;
				state ls;
				state lidle { exit { assign lidleExits := lidleExits + 1; } }
				state lstate;
				succession first ls then lidle;
			}
			state right {
				entry; then rs;
				state rs;
				state wrapper {
					state mid parallel {
						exit { assign midExits := midExits + 1; }

						state inner {
							entry; then is;
							state is;
							state ideep;
							succession first is then ideep;
							transition first ideep if lidleExits == 0 then lstate;
						}
					}
				}
				succession first rs then mid;
			}
		}

		succession first init then running;
	}
}`)

	assertRegionConfig(t, exec, map[string]string{"left": "lstate"})
	for _, name := range []string{"lidleExits", "midExits"} {
		if got := intValue(t, exec.stateData, name); got != 1 {
			t.Errorf("%s = %d, want 1", name, got)
		}
	}
}

// Exiting a state exits its subperformances, so a transition out of a nested
// non-orthogonal state still exits every state up to the endpoints' least common
// ancestor.
func TestNestedTransitionExitsUpToTheLCA(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute innerExits : Integer = 0;
		attribute outerExits : Integer = 0;

		entry; then init;
		state init;
		state outer {
			exit { assign outerExits := outerExits + 1; }

			state inner { exit { assign innerExits := innerExits + 1; } }
		}
		state done;

		succession first init then inner;
		transition first inner then done;
	}
}`)

	if len(exec.activeConfig.regionStates) != 0 {
		t.Errorf("regions still active: %v", regionConfig(exec))
	}
	if got := exec.getCurrentState(); got == nil || got.Name != "done" {
		t.Fatalf("current state = %v, want done", got)
	}
	for _, name := range []string{"innerExits", "outerExits"} {
		if got := intValue(t, exec.stateData, name); got != 1 {
			t.Errorf("%s = %d, want 1 (the nested state and its parent are both exited)", name, got)
		}
	}
}

// A transition out of a region whose target lies outside the composite state
// still leaves the whole region set, sibling regions included.
func TestTransitionOutOfCompositeStateExitsEveryRegion(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute leftExits : Integer = 0;
		attribute rightExits : Integer = 0;
		attribute runningExits : Integer = 0;

		entry; then init;
		state init;
		state running parallel {
			exit { assign runningExits := runningExits + 1; }

			state left {
				entry; then ls;
				state ls;
				state lidle { exit { assign leftExits := leftExits + 1; } }
				succession first ls then lidle;
				transition first lidle then done;
			}
			state right {
				entry; then rs;
				state rs;
				state ridle { exit { assign rightExits := rightExits + 1; } }
				succession first rs then ridle;
			}
		}
		state done;

		succession first init then running;
	}
}`)

	if len(exec.activeConfig.regionStates) != 0 {
		t.Errorf("regions still active: %v", regionConfig(exec))
	}
	if got := exec.getCurrentState(); got == nil || got.Name != "done" {
		t.Fatalf("current state = %v, want done", got)
	}
	for _, name := range []string{"leftExits", "rightExits", "runningExits"} {
		if got := intValue(t, exec.stateData, name); got != 1 {
			t.Errorf("%s = %d, want 1", name, got)
		}
	}
}

// A region's active state can be nested below the state the region declares, so
// leaving the composite state must exit each state on the way out exactly once.
func TestTransitionOutOfCompositeStateExitsNestedStatesOnce(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute midExits : Integer = 0;
		attribute wrapperExits : Integer = 0;
		attribute deepExits : Integer = 0;

		entry; then init;
		state init;
		state running parallel {
			state left {
				entry; then ls;
				state ls;
				state lidle;
				succession first ls then lidle;
			}
			state right {
				entry; then rs;
				state rs;
				state wrapper {
					exit { assign wrapperExits := wrapperExits + 1; }

					state mid parallel {
						exit { assign midExits := midExits + 1; }

						state inner {
							entry; then is;
							state is;
							state ideep { exit { assign deepExits := deepExits + 1; } }
							succession first is then ideep;
							transition first ideep if midExits == 0 then done;
						}
					}
				}
				succession first rs then mid;
			}
		}
		state done;

		succession first init then running;
	}
}`)

	if len(exec.activeConfig.regionStates) != 0 {
		t.Errorf("regions still active: %v", regionConfig(exec))
	}
	if got := exec.getCurrentState(); got == nil || got.Name != "done" {
		t.Fatalf("current state = %v, want done", got)
	}
	for _, name := range []string{"midExits", "wrapperExits", "deepExits"} {
		if got := intValue(t, exec.stateData, name); got != 1 {
			t.Errorf("%s = %d, want 1 (exited exactly once)", name, got)
		}
	}
}
