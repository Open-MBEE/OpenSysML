package repl

import (
	"strings"
	"testing"
)

// performedBindingSource performs Nudge with `:>> context = other` twice over:
// held behind a succession on Host, started at once on Lone.
const performedBindingSource = `package test {
	private import ScalarValues::*;
	part def Heater { attribute level : Integer = 0; }
	action def Nudge {
		in delta : Integer[1] = 1;
		in ref context : Heater[1];
		first apply;
		action apply { assign context.level := context.level + delta; }
		first apply then done;
	}
	part def Host {
		part other : Heater;
		perform action earlier { first start; then action w accept after 5 [SI::s]; then done; }
		perform action later : Nudge { in ref :>> context = other; }
		first earlier then later;
	}
	part def Lone {
		part other : Heater;
		perform action later : Nudge { in ref :>> context = other; }
	}
	part host : Host;
	part lone : Lone;
}`

const (
	performedInputsRefused = "inputs for a performed action"
	successionStillHolds   = "succession-order-violated"
)

// Naming a performed action with the object its declaration binds a reference
// parameter to is accepted whether the performance is held or started; naming
// another object is refused either way.
func TestPerformedActionInputsMatchTheDeclaredBinding(t *testing.T) {
	s := loadSource(t, performedBindingSource)
	run(t, s, "%instantiate test::host")

	held := run(t, s, "%tool Nudge(context = host.other) host")
	wants(t, held, successionStillHolds)
	if strings.Contains(held, performedInputsRefused) {
		t.Errorf("held performance refused its declared binding:\n%s", held)
	}
	wants(t, run(t, s, "%tool Nudge(context = host) host"), performedInputsRefused)

	run(t, s, "%instantiate test::lone")
	started := run(t, s, "%tool Nudge(context = lone.other) lone")
	if strings.Contains(started, performedInputsRefused) || strings.Contains(started, successionStillHolds) {
		t.Errorf("started performance refused its declared binding:\n%s", started)
	}
	wants(t, run(t, s, "%tool Nudge(context = lone) lone"), performedInputsRefused)
}
