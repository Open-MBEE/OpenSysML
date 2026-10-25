package runtime

import (
	"errors"
	"strings"
	"testing"
)

func TestRuntimeRobustnessBehaviorOccurrenceAttributes(t *testing.T) {
	// machineCopy images inst into a fresh context and returns the copy's machine.
	machineCopy := func(t *testing.T, ctx *Context, inst *Instance) *StateExecutor {
		t.Helper()
		img, err := ctx.Image(inst)
		if err != nil {
			t.Fatalf("Image: %v", err)
		}
		dst := NewContext(ctx.Model(), 10000)
		if err := img.Materialize(dst); err != nil {
			t.Fatalf("Materialize: %v", err)
		}
		copied, held := dst.Instance(inst.ID)
		if !held {
			t.Fatalf("the copy holds no object #%d", inst.ID)
		}
		return objectMachine(t, copied, "modes")
	}
	stateData := func(t *testing.T, machine *StateExecutor, where string) map[string]Value {
		t.Helper()
		data, err := machine.StateDataWithError()
		if err != nil {
			t.Fatalf("%s state data: %v", where, err)
		}
		return data
	}

	t.Run("empty_occurrence_feature_after_restore", func(t *testing.T) {
		ctx, inst, err := instantiateWithLibraries(t, `package test {
			private import ScalarValues::Integer;
			private import SequenceFunctions::size;
			state def Modes {
				entry; then idle;
				state idle;
				attribute triggerCount : Integer = size(incomingTransitionTrigger);
				attribute acceptedCount : Integer = size(accepted);
			}
			part def Machine { exhibit state modes : Modes; }
		}`, "test::Machine")
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		for where, machine := range map[string]*StateExecutor{"source": objectMachine(t, inst, "modes"), "copy": machineCopy(t, ctx, inst)} {
			data := stateData(t, machine, where)
			for _, name := range []string{"triggerCount", "acceptedCount"} {
				if !valueEqual(data[name], integerValue(0)) {
					t.Errorf("%s %s = %s, want 0: the optional feature holds nothing", where, name, FormatValue(data[name]))
				}
			}
		}
	})

	t.Run("rederives_after_a_read_feature_is_written", func(t *testing.T) {
		ctx, inst, err := instantiateWithLibraries(t, `package test {
			private import ScalarValues::Integer;
			private import SequenceFunctions::size;
			attribute def Go;
			state def Modes {
				attribute offset : Integer := 1;
				attribute total : Integer = size(entryAction) + offset;
				entry action en {}
				then idle;
				state idle;
				transition first idle accept Go do assign offset := 2 then moved;
				state moved;
			}
			part def Machine { exhibit state modes : Modes; }
		}`, "test::Machine")
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		machine := objectMachine(t, inst, "modes")
		if got := stateData(t, machine, "before")["total"]; !valueEqual(got, integerValue(2)) {
			t.Fatalf("total before = %s, want 2", FormatValue(got))
		}
		machine.SendSignal("Go", nil)
		if err := machine.RunToCompletion(); err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
		if got := stateData(t, machine, "after")["total"]; !valueEqual(got, integerValue(3)) {
			t.Fatalf("total after writing offset = %s, want 3", FormatValue(got))
		}
		if got := stateData(t, machineCopy(t, ctx, inst), "copy")["total"]; !valueEqual(got, integerValue(3)) {
			t.Fatalf("copy total = %s, want 3", FormatValue(got))
		}
	})

	t.Run("chained_read_names_the_performances_part", func(t *testing.T) {
		ctx, inst, err := instantiateWithLibraries(t, `package test {
			private import ScalarValues::Integer;
			private import ScalarValues::Boolean;
			part def Widget { attribute n : Integer = 1; }
			part def Robot {
				attribute seen : Integer;
				attribute seenAgain : Integer;
				attribute same : Boolean;
				perform action go {
					part fresh : Widget;
					first start;
					then assign seen := fresh.n;
					then assign fresh.n := 5;
					then assign seenAgain := fresh.n;
					then assign same := fresh.n == fresh.n;
					then done;
				}
			}
		}`, "test::Robot")
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		if got := readInt(t, ctx, inst, "seen"); got != 1 {
			t.Errorf("seen = %d, want 1", got)
		}
		if got := readInt(t, ctx, inst, "seenAgain"); got != 5 {
			t.Errorf("seenAgain = %d, want 5: the chain reads the part the write landed on", got)
		}
		fresh := instanceAtPath(t, ctx, inst, "go.fresh")
		if got := readInt(t, ctx, fresh, "n"); got != 5 {
			t.Errorf("go.fresh.n = %d, want 5: the chain names the performance's own part", got)
		}
		for _, obj := range ctx.instances {
			if obj.owner == nil && obj.ID != inst.ID {
				t.Errorf("object #%d (%s) has no owner: the body's part was materialized outside the performance", obj.ID, SymbolText(obj.Type))
			}
		}
	})

	t.Run("valueless_feature_is_a_typed_error", func(t *testing.T) {
		_, _, err := instantiateWithLibraries(t, `package test {
			private import ScalarValues::Integer;
			private import SequenceFunctions::size;
			state def Modes {
				entry; then idle;
				state idle;
				attribute gone : Integer;
				attribute total : Integer = size(entryAction) + gone;
			}
			part def Machine { exhibit state modes : Modes; }
		}`, "test::Machine")
		if err == nil {
			t.Fatal("a machine whose attribute reads a valueless feature was instantiated")
		}
		if !errors.Is(err, ErrNoValue) || !strings.Contains(err.Error(), "gone") {
			t.Fatalf("err = %v, want a typed no-value error naming gone", err)
		}
	})
}
