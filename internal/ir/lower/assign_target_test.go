package lower

import (
	"strings"
	"testing"
)

// A chained assignment target is lowered whole: the object the chain starts
// from, every step walked from it and the feature written, so the runtime never
// walks the target's declaration again.
func TestAssignChainedTargetLowering(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			first start;
			action write {
				assign s.reading := 4.5;
				assign m.inner.deep := 1.0;
				assign plain := 2.0;
			}
			done;
			succession first start then write;
			succession first write then done;
		}
	`)

	body := graph.Bodies[nodeNamed(t, graph, "write")]
	if len(body) != 3 {
		t.Fatalf("write lowered to %d statements, want 3: %#v", len(body), body)
	}

	depth2, ok := body[0].(Assign)
	if !ok {
		t.Fatalf("statement 0 = %T, want Assign", body[0])
	}
	if depth2.Chain == nil {
		t.Fatal("statement 0 lowered without a chained target")
	}
	if depth2.Target != "reading" {
		t.Errorf("statement 0 writes %q, want %q", depth2.Target, "reading")
	}
	if got := FeaturePath(depth2.Chain.Base); got != "s" {
		t.Errorf("statement 0 chain base = %q, want %q", got, "s")
	}
	if len(depth2.Chain.Steps) != 0 {
		t.Errorf("statement 0 chain steps = %v, want none", depth2.Chain.Steps)
	}
	if depth2.Chain.Text != "s.reading" {
		t.Errorf("statement 0 chain text = %q, want %q", depth2.Chain.Text, "s.reading")
	}

	depth3, ok := body[1].(Assign)
	if !ok {
		t.Fatalf("statement 1 = %T, want Assign", body[1])
	}
	if depth3.Chain == nil {
		t.Fatal("statement 1 lowered without a chained target")
	}
	if depth3.Target != "deep" {
		t.Errorf("statement 1 writes %q, want %q", depth3.Target, "deep")
	}
	if got := FeaturePath(depth3.Chain.Base); got != "m" {
		t.Errorf("statement 1 chain base = %q, want %q", got, "m")
	}
	if got := strings.Join(depth3.Chain.Steps, ","); got != "inner" {
		t.Errorf("statement 1 chain steps = %q, want %q", got, "inner")
	}
	if depth3.Chain.Text != "m.inner.deep" {
		t.Errorf("statement 1 chain text = %q, want %q", depth3.Chain.Text, "m.inner.deep")
	}

	plain, ok := body[2].(Assign)
	if !ok {
		t.Fatalf("statement 2 = %T, want Assign", body[2])
	}
	if plain.Chain != nil {
		t.Errorf("statement 2 lowered a chained target %#v, want none", plain.Chain)
	}
	if plain.Target != "plain" {
		t.Errorf("statement 2 writes %q, want %q", plain.Target, "plain")
	}
}

// A namespace-qualified target names no object to write a feature of, so it is
// not lowered as a chain: `P::glob` reaches a package member, not an occurrence.
func TestAssignQualifiedTargetRemainsUnsupported(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			first start;
			action write {
				assign Other::glob := 4.5;
			}
			done;
			succession first start then write;
			succession first write then done;
		}
	`)

	body := graph.Bodies[nodeNamed(t, graph, "write")]
	if len(body) != 1 {
		t.Fatalf("write lowered to %d statements, want 1: %#v", len(body), body)
	}
	unsupported, ok := body[0].(Unsupported)
	if !ok {
		t.Fatalf("statement 0 = %T, want Unsupported", body[0])
	}
	if !strings.Contains(unsupported.Description, "qualified target") {
		t.Errorf("statement 0 description = %q, want it to name a qualified target", unsupported.Description)
	}
}
