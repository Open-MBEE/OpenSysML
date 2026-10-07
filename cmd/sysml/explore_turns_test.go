package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var seenLine = regexp.MustCompile(`seen = \d+`)

// lostUpdateModel has two actions due at one instant, each reading a shared
// counter in one step and writing it from what it read in the next.
const lostUpdateModel = `package Race {
    private import ScalarValues::*;

    part def Counter { attribute count : Integer = 0; }
    part counter : Counter;

    action left {
        attribute seen : Integer = 0;
        first start;
        then action wait accept after 1 [SI::s];
        then action read assign seen := counter.count;
        then action write assign counter.count := seen + 1;
        then done;
    }

    action right {
        attribute seen : Integer = 0;
        first start;
        then action wait accept after 1 [SI::s];
        then action read assign seen := counter.count;
        then action write assign counter.count := seen + 1;
        then done;
    }
}
`

// TestExploreInterleavesExecutorsMoveByMove checks that executors due at one
// instant interleave between their moves: both reads may run before either write,
// so explore, check and a seed reach the lost update, while declared and reverse
// keep running each executor through its turn.
func TestExploreInterleavesExecutorsMoveByMove(t *testing.T) {
	binary := buildCLI(t)
	invocation := []string{"-instantiate", "Race::counter", "-action", "Race::left", "-action", "Race::right", "-advance", "1"}
	lost, leftFirst, rightFirst := "Race::left.seen = 0; Race::right.seen = 0", "Race::left.seen = 0; Race::right.seen = 1", "Race::left.seen = 1; Race::right.seen = 0"

	explored := check(t, binary, lostUpdateModel, append([]string{"-schedule", "explore"}, invocation...)...)
	wantReport(t, explored, 0, "✓ explored Race::left, Race::right: 3 outcomes", lost+" |", leftFirst+" |", rightFirst+" |", "complete (")

	checked := check(t, binary, lostUpdateModel, append([]string{"-engine", "check", "-check-diverge", "Race::left.seen"}, invocation...)...)
	wantReport(t, checked, 1, "divergent: Race::left.seen ends as 0 or 1",
		"outcome: "+lost, "outcome: "+leftFirst, "outcome: "+rightFirst, "witness of 6 choices replayed")

	results := func(policy string) string {
		got := check(t, binary, lostUpdateModel, append([]string{"-schedule", policy}, invocation...)...)
		wantReport(t, got, 0, "Action completed")
		return strings.Join(seenLine.FindAllString(got.output(), -1), ", ")
	}
	if got := results("declared"); got != "seen = 0, seen = 1" {
		t.Errorf("declared runs left through its turn first, want seen = 0, seen = 1, got %s", got)
	}
	if got := results("reverse"); got != "seen = 1, seen = 0" {
		t.Errorf("reverse runs right through its turn first, want seen = 1, seen = 0, got %s", got)
	}
	seeded := map[string]bool{}
	for seed := 1; seed <= 12; seed++ {
		seeded[results(fmt.Sprintf("seed:%d", seed))] = true
	}
	for _, want := range []string{"seen = 0, seen = 0", "seen = 0, seen = 1", "seen = 1, seen = 0"} {
		if !seeded[want] {
			t.Errorf("no seed of 1..12 reaches %s: %v", want, seeded)
		}
	}
}

// heldEntryModel has a machine whose timer transition holds work's entry cascade
// while the Go it sends is still to be dispatched, and an action due at the same
// instant that reads what the cascade writes.
const heldEntryModel = `package Held {
    private import ScalarValues::*;
    item def Go;
    part def P {
        attribute order : Integer = 0;
        attribute seen : Integer = 0;
        state m parallel {
            state left {
                entry; then prep;
                state prep;
                state work {
                    ref :>> runToCompletionScope = self;
                    state inner { entry action { assign order := order * 10 + 1; } }
                    entry action { send new Go() to m; } then inner;
                }
                transition first prep accept after 1 [SI::s] then work;
            }
            state right {
                entry; then a;
                state a;
                state b { entry action { assign order := order * 10 + 2; } }
                transition first a accept Go then b;
            }
        }
        action peek {
            first start;
            then action wait accept after 1 [SI::s];
            then action read assign seen := order;
            then done;
        }
    }
    part p : P;
}
`

// TestExploreRunsAHeldEntryAsAMove checks that resuming a held entry counts as the
// machine's move, so the Go queued behind it is still dispatched at the instant.
func TestExploreRunsAHeldEntryAsAMove(t *testing.T) {
	binary := buildCLI(t)
	invocation := []string{"-instantiate", "Held::p", "-state", "Held::P::m Held::p", "-action", "Held::P::peek Held::p", "-advance", "1"}

	explored := check(t, binary, heldEntryModel, append([]string{"-schedule", "explore"}, invocation...)...)
	wantReport(t, explored, 0, "✓ explored Held::P::peek, Held::P::m: 10 outcomes", "this.order = 12; this.seen = 1", "complete (")
	if strings.Contains(explored.output(), `finalState = "inner+a"`) {
		t.Errorf("an explored run left Go undispatched at the instant it was sent:\n%s", explored.output())
	}
	for seed := 1; seed <= 8; seed++ {
		got := check(t, binary, heldEntryModel, append([]string{"-schedule", fmt.Sprintf("seed:%d", seed)}, invocation...)...)
		wantReport(t, got, 0, "Current state: inner | b")
	}
}
