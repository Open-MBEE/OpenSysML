package migrate_test

import (
	"os"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtrace"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// migratedRun loads a fixture's migrated notation into a runtime context.
type migratedRun struct {
	idx *symbols.Index
	ctx *runtime.Context
}

func newMigratedRun(t *testing.T, fixture string) *migratedRun {
	t.Helper()
	data, err := os.ReadFile("testdata/xmi/" + fixture + ".xmi")
	if err != nil {
		t.Fatal(err)
	}
	r, err := migrate.Migrate(fixture+".xmi", data)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	p := parser.New(source.New(fixture+".sysml", r.Notation))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse the migrated notation: %v", p.Diagnostics[0])
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(fixture+".sysml", root)
	idx.ExpandWildcardImports()
	res := resolve.New(idx)
	ctx := runtime.NewContext(runtime.NewModel(passes.NewTypedModel(res), res), 10_000_000)
	return &migratedRun{idx: idx, ctx: ctx}
}

func (m *migratedRun) sym(t *testing.T, fqn string) *symbols.Symbol {
	t.Helper()
	syms := m.idx.LookupQualified(fqn)
	if len(syms) != 1 {
		t.Fatalf("%s names %d symbols, want one", fqn, len(syms))
	}
	return syms[0]
}

// perform runs the occurrence def as an interaction under a trace and answers
// the names of the transfers in the order performed and the run's sequence rendering.
func (m *migratedRun) perform(t *testing.T, fqn string) ([]string, []string, map[string]runtime.PerformedTransfer) {
	t.Helper()
	trace := runtime.NewTraceRecorder()
	m.ctx.SetTrace(trace)
	run, err := m.ctx.ExecuteOccurrence(m.sym(t, fqn))
	if err != nil {
		t.Fatalf("ExecuteOccurrence %s: %v", fqn, err)
	}
	var names []string
	byName := map[string]runtime.PerformedTransfer{}
	for _, tr := range run.Transfers {
		if tr.From == nil || tr.To == nil || tr.From == tr.To {
			t.Errorf("transfer %s runs from %v to %v, want two lifeline objects", tr.Name, tr.From, tr.To)
		}
		names = append(names, tr.Name)
		byName[tr.Name] = tr
	}
	rendering, err := runtrace.Render(runtrace.KindSequence, trace, runtrace.Options{})
	if err != nil {
		t.Fatalf("render the run: %v", err)
	}
	var edges []string
	for _, e := range rendering.Edges {
		edges = append(edges, e.Label)
	}
	return names, edges, byName
}

// A migrated interaction executes: its messages transfer between the lifeline
// objects in the modelled order, and the run renders as the same sequence. The
// rig's Spinup interaction has a synchronous call whose receiving event performs
// the operation, its reply carrying the call's result, and signal messages.
func TestMigratedInteractionsExecuteInModelledOrder(t *testing.T) {
	t.Run("block-owned interaction", func(t *testing.T) {
		names, edges, transfers := newMigratedRun(t, "rig_interactions").perform(t, "Model::Rig::Spinup")
		want := []string{"spin", "spun", "go", "brake", "callSpin", "parEnd", "parOp1", "new"}
		if !slices.Equal(names, want) {
			t.Errorf("transfers = %v, want %v", names, want)
		}
		if transfers["spin"].Performed == nil {
			t.Error("the call message spin did not perform Motor::spin on the receiving event")
		}
		if v := transfers["spun"].Message.Value; v == nil || runtime.FormatValue(*v) != "30.0" {
			t.Errorf("the reply spun carries %v, want the call's result 30.0", v)
		}
		if transfers["go"].Message.SignalType != "Go" || runtime.FormatValue(transfers["go"].Message.Payload["n"]) != "3" {
			t.Errorf("the signal message go is %s with payload %v, want a Go with n = 3", transfers["go"].Message.SignalType, transfers["go"].Message.Payload)
		}
		if len(edges) != len(want) {
			t.Errorf("the run renders %d messages %v, want %d", len(edges), edges, len(want))
		}
	})
	t.Run("typed usage of the block-owned interaction", func(t *testing.T) {
		// `occurrence spinup : Spinup { ref part :>> c = ctrl; … }` inherits the def's
		// lifelines, events and messages; the usage alone binds nothing, so each
		// lifeline stands for an object of its type.
		names, _, transfers := newMigratedRun(t, "rig_interactions").perform(t, "Model::Rig::spinup")
		want := []string{"spin", "spun", "go", "brake", "callSpin", "parEnd", "parOp1", "new"}
		if !slices.Equal(names, want) {
			t.Errorf("transfers = %v, want %v", names, want)
		}
		if transfers["spin"].Performed == nil {
			t.Error("the call message spin did not perform Motor::spin on the receiving event")
		}
	})
	t.Run("interaction using another and asserting an invariant", func(t *testing.T) {
		// Run uses 'Spin Up' as `occurrence warm : 'Spin Up' { attribute :>> rpm = 30.0; }`
		// before m.at30 asserts `speed == 30.0`, then sends slow: the nested use's
		// message transfers first, between Run's own lifeline objects.
		names, edges, transfers := newMigratedRun(t, "interaction_use").perform(t, "Model::Rig::Run")
		want := []string{"spin", "slow"}
		if !slices.Equal(names, want) {
			t.Errorf("transfers = %v, want %v", names, want)
		}
		if transfers["spin"].To != transfers["slow"].To || transfers["spin"].From != transfers["slow"].From {
			t.Errorf("the nested use's spin runs between %v and %v, want Run's own lifeline objects %v and %v", transfers["spin"].From, transfers["spin"].To, transfers["slow"].From, transfers["slow"].To)
		}
		if len(edges) != len(want) {
			t.Errorf("the run renders %d messages %v, want %d", len(edges), edges, len(want))
		}
	})
	t.Run("use-case-owned interaction", func(t *testing.T) {
		names, edges, _ := newMigratedRun(t, "usecase_sequence").perform(t, "Model::Acquire Images::Acquire Images")
		want := []string{"move", "pumpDown", "new", "record"}
		if !slices.Equal(names, want) {
			t.Errorf("transfers = %v, want %v", names, want)
		}
		if len(edges) != len(want) {
			t.Errorf("the run renders %d messages %v, want %d", len(edges), edges, len(want))
		}
	})
}
