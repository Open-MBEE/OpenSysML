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
