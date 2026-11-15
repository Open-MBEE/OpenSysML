package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessSuccessionChainEnd covers a succession end chaining through
// an object to something the flow cannot perform: each is a typed error, never a hang.
func TestRuntimeRobustnessSuccessionChainEnd(t *testing.T) {
	const parts = `
		part def Vehicle {
			action close;
			event occurrence opened;
			part door;
			state locked;
			state machine {
				entry; then off;
				state off;
				accept Go then on;
				state on;
			}
			attribute n;
		}
	`
	create := func(t *testing.T, body string) (*ActionExecutor, error) {
		t.Helper()
		src := "package test { item def Go;" + parts + "action outer { part v : Vehicle; action a; " + body + " } }"
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
		if sym == nil {
			t.Fatal("action outer not found")
		}
		return ctx.CreateActionExecutor(sym)
	}
	for name, body := range map[string]string{
		"event_occurrence_target":    `first a then v.opened;`,
		"event_occurrence_source":    `first v.opened then a;`,
		"part_target":                `succession first a then v.door;`,
		"state_with_accepter_target": `first a then v.machine;`,
		"positional_then":            `first a; then v.opened;`,
		"guarded_then":               `first a; then decide; if true then v.opened; else v.close;`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := create(t, body); !errors.Is(err, lower.ErrSuccessionEndNotPerformed) {
				t.Fatalf("error = %v, want ErrSuccessionEndNotPerformed", err)
			}
		})
	}
	t.Run("attribute_target", func(t *testing.T) {
		_, err := create(t, `first a then v.n;`)
		if err == nil || !strings.Contains(err.Error(), `undefined target node "v.n"`) {
			t.Fatalf("error = %v, want the end named as undefined", err)
		}
	})
	t.Run("chain_naming_no_feature", func(t *testing.T) {
		_, err := create(t, `first a then v.missing;`)
		if err == nil || !strings.Contains(err.Error(), `undefined target node "v.missing"`) {
			t.Fatalf("error = %v, want the end named as undefined", err)
		}
	})
	t.Run("chained_done_is_no_final_node", func(t *testing.T) {
		_, err := create(t, `first a then v.done;`)
		if err == nil || !strings.Contains(err.Error(), "v.done") {
			t.Fatalf("error = %v, want the chained end refused, not read as the flow's done", err)
		}
	})
	t.Run("state_usage_is_not_performed", func(t *testing.T) {
		exec, err := create(t, `first start then a; first a then v.locked; first v.locked then done;`)
		if err == nil {
			err = exec.RunToCompletion()
		}
		if !errors.Is(err, lower.ErrSuccessionEndNotPerformed) {
			t.Fatalf("error = %v, want ErrSuccessionEndNotPerformed", err)
		}
	})
}
