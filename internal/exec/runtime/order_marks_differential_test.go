package runtime

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/tests/fixtures"
)

// orderMarkApproximations pins the order fixtures whose marks are known not to agree
// with exploration, each with the names marked and the names varying as they stand,
// so a movement either way is seen; the reason says what approximates.
var orderMarkApproximations = map[string]struct {
	marked, varying []string
	reason          string
}{
	"action_order_explicit_argument_binds_pin": {
		marked:  []string{"dep.result", "dep.x", "derived", "fixed", "lit.result", "x", "y"},
		varying: []string{"dep.result", "dep.x", "derived", "x", "y"},
		reason:  "a performed action's footprint is dynamic, so its outputs stand on every mark, a literal argument's included",
	},
	"action_order_send_payload_from_racing_writer": {
		marked:  []string{"receiver.msg", "total", "x"},
		varying: []string{"msg", "total", "x"},
		reason:  "an accept's payload is reported twice, as the node's pin and under its bare name: the pin is marked, the bare name is not, and the outcome spells the pin's object alike",
	},
}

// Over every order fixture, the outputs the default run marks order-dependent are
// exactly those exploration finds at more than one value: a mark where every
// linearization agrees is a false positive, a bare value that varies a false negative.
// The pinned approximations are held where they stand.
func TestOrderMarksAgreeWithExploredOutcomes(t *testing.T) {
	conformanceDir := filepath.Join("testdata", "conformance")
	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatal(err)
	}
	cases := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".expected.json") || !strings.HasPrefix(name, "action_order_") && !strings.HasPrefix(name, "state_order_") {
			continue
		}
		caseName := strings.TrimSuffix(name, ".expected.json")
		expected := loadExpectedOutcome(t, conformanceDir, caseName)
		if len(expected.Outcomes) == 0 || len(expected.Performers) > 0 {
			continue
		}
		cases++
		t.Run(caseName, func(t *testing.T) {
			marked, varying := markedAndVaryingOutputs(t, conformanceDir, caseName, expected)
			if pinned, ok := orderMarkApproximations[caseName]; ok {
				if !slices.Equal(marked, pinned.marked) || !slices.Equal(varying, pinned.varying) {
					t.Errorf("default run marks %v order-dependent and exploration varies %v; pinned as %v and %v (%s)",
						marked, varying, pinned.marked, pinned.varying, pinned.reason)
				}
				return
			}
			if !slices.Equal(marked, varying) {
				t.Errorf("default run marks %v order-dependent; exploration varies %v", marked, varying)
			}
		})
	}
	if cases == 0 {
		t.Fatal("no order fixture with an admissible set found")
	}
}

// markedAndVaryingOutputs runs the case once under its pinned policy and reports the outputs
// marked order-dependent, then explores it and reports the outputs with more than one value.
func markedAndVaryingOutputs(t *testing.T, conformanceDir, caseName string, expected ExpectedOutcome) (marked, varying []string) {
	t.Helper()
	sysmlPath := fixtures.ModelPath(conformanceDir, caseName)
	data, err := os.ReadFile(sysmlPath)
	if err != nil {
		t.Fatal(err)
	}
	src := source.New(sysmlPath, data)
	p := parser.New(src)
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse %s: %v", sysmlPath, p.Diagnostics)
	}
	idx, sources := indexCaseDocuments(t, conformanceDir, src, file, expected)
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	model.SetSourceText(source.TextOf(sources, nil))
	fresh := func() *Context {
		ctx := NewContext(typedModel(model, resolver), 10000)
		applyCaseDraws(t, ctx, expected)
		return ctx
	}
	run := conformanceRun(t, idx, sysmlPath, expected)

	ctx := fresh()
	if err := ctx.SetSchedule(casePolicy(t, expected, DefaultSchedulePolicy)); err != nil {
		t.Fatal(err)
	}
	outcome, err := run(ctx)
	if err != nil {
		t.Fatalf("default run: %v", err)
	}
	names := make([]string, 0, len(outcome.Outputs))
	for name := range outcome.Outputs {
		names = append(names, name)
	}
	for name := range ctx.OrderDependentAmong(names) {
		marked = append(marked, name)
	}
	sort.Strings(marked)

	policy, err := ExplorePolicy(expected.ExploreBudget.budget())
	if err != nil {
		t.Fatal(err)
	}
	exploration, err := Explore(context.Background(), policy, func() (*Context, error) { return fresh(), nil }, run)
	if err != nil {
		t.Fatalf("explore: %v", err)
	}
	if !exploration.Complete() {
		t.Fatalf("exploration %s", exploration.Status())
	}
	texts := make(map[string]map[string]bool)
	for _, explored := range exploration.Outcomes {
		for _, out := range explored.Outcome.outputsSpelled(func(name string) string { return name }, false) {
			if texts[out.Name] == nil {
				texts[out.Name] = make(map[string]bool)
			}
			texts[out.Name][out.Text] = true
		}
	}
	for name, values := range texts {
		if len(values) > 1 {
			varying = append(varying, name)
		}
	}
	sort.Strings(varying)
	return marked, varying
}
