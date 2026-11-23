package progress

import (
	"context"
	"slices"
	"testing"
)

func TestAnalyzePendingCycleWithResponseExit(t *testing.T) {
	g := New()
	picks := []int{1}
	g.Observe(Step{To: "start"}, true, false, picks)
	picks[0] = 0
	accepted := Step{From: "start", To: "pending", Move: "accept"}
	retry := Step{From: "pending", To: "pending", Move: "retry"}
	g.Observe(accepted, true, true, nil)
	g.Observe(Step{From: "pending", To: "responded", Move: "respond"}, true, false, nil)
	g.Observe(retry, false, false, nil)
	g.MarkTerminal("responded")

	r, err := g.Analyze(context.Background())
	if err != nil || !r.Activated || !r.Cleared || r.Root != "start" ||
		!slices.Equal(r.StartupPicks, []int{1}) || !slices.Equal(r.Prefix, []Step{accepted}) ||
		!slices.Equal(r.Cycle, []Step{retry}) {
		t.Fatalf("want the accepted request's retry witness even with a response available; got %+v, %v", r, err)
	}
}
