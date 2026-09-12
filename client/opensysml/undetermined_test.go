package opensysml_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const undeterminedSource = `package U {
	private import ScalarValues::*;
	part def D;
	attribute u;
	part gear[1..*] : D;
	attribute fixed = (u > 3) and false;
}`

// A model-level result the model leaves open arrives as an Undetermined over every
// transport, with its reason and count, and distinct from an Unset or a Null.
func TestUndeterminedResultsCrossEveryTransport(t *testing.T) {
	address := startService(t)
	for name, client := range map[string]opensysml.Client{
		"in-process":    newClient(t),
		"connect-proto": dialClient(t, address),
		"connect-json":  dialClient(t, address, opensysml.WithJSONBody()),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			info, err := client.ServerInfo(ctx)
			if err != nil {
				t.Fatalf("ServerInfo: %v", err)
			}
			if !info.Has(opensysml.CapabilityUndeterminedValue) {
				t.Errorf("capabilities %v do not name %s", info.Capabilities, opensysml.CapabilityUndeterminedValue)
			}
			model := parse(t, client, undeterminedSource)

			got, err := client.Evaluate(ctx, model, "U::u + 5")
			if err != nil {
				t.Fatalf("Evaluate(u + 5): %v", err)
			}
			want := opensysml.Undetermined{Reason: "U::u has no value in the model", CountLower: "1", CountUpper: "1"}
			if got != want {
				t.Errorf("u + 5 = %#v, want %#v", got, want)
			}
			if fmt.Sprint(got) != "<undetermined>" {
				t.Errorf("u + 5 renders as %q, want <undetermined>", fmt.Sprint(got))
			}

			open, err := client.Evaluate(ctx, model, "U::gear")
			if err != nil {
				t.Fatalf("Evaluate(gear): %v", err)
			}
			u, ok := open.(opensysml.Undetermined)
			if !ok || u.CountLower != "1" || u.CountUpper != "*" {
				t.Errorf("gear = %#v, want an Undetermined counting 1..*", open)
			}

			folded, err := client.Evaluate(ctx, model, "U::fixed")
			if err != nil || folded != opensysml.Bool(false) {
				t.Errorf("fixed = %#v (%v), want Bool(false)", folded, err)
			}
		})
	}
}

// Undetermined is reported, never accepted: a caller sending one is told so.
func TestExecuteActionRefusesAnUndeterminedInput(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, behaviorSource)
	_, err := client.ExecuteAction(context.Background(), model, "Test::addFive",
		map[string]opensysml.Value{"result": opensysml.Undetermined{Reason: "x"}})
	if !errors.Is(err, opensysml.CodeInvalidArgument) {
		t.Errorf("err = %v, want CodeInvalidArgument", err)
	}
}

func TestUndeterminedIsNeitherUnsetNorNull(t *testing.T) {
	u := opensysml.Undetermined{Reason: "u has no value in the model"}
	if opensysml.Equal(u, opensysml.Unset{}) || opensysml.Equal(u, opensysml.Null("")) || opensysml.Equal(u, opensysml.Sequence{}) {
		t.Error("an Undetermined equals an Unset, a Null or an empty sequence")
	}
	if !opensysml.Equal(u, opensysml.Undetermined{Reason: "u has no value in the model"}) {
		t.Error("an Undetermined does not equal its own value")
	}
}
