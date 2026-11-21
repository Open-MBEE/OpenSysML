package runtime

import (
	"context"
	"errors"
	"testing"
)

func TestMatchTraceCancellationDuringStartupIsInconclusive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := parseExploreModel(t, "package test {}")
	rows := []TraceRow{{Observe: map[string]Value{"screen": NewStringValue("blank")}}}
	starter := func(*Context) (*Invocation, error) {
		cancel()
		return nil, errors.New("startup stopped")
	}

	report, err := MatchTrace(ctx, model.fresh, starter, rows, CheckBudget{})
	if err != nil || report.Status != TraceInconclusive || report.Reason != context.Canceled.Error() {
		t.Fatalf("got report %+v, err %v; want inconclusive cancellation", report, err)
	}
}
