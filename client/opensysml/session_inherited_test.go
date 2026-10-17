package opensysml_test

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// An action a subtype inherits is performed under the subtype's qualified name
// too, on the object as its own feature.
func TestSessionPerformNamesAnInheritedActionBySubtype(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, `package Demo {
	part def Plant {
		attribute runs : ScalarValues::Integer = 0;
		action fill { assign runs := runs + 1; }
	}
	part def BigPlant :> Plant { }
	part p : BigPlant;
}`)
	session, err := opensysml.OpenSession(client, model)
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	p, err := session.Instantiate("Demo::p")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	if _, err := session.Perform(p, "Demo::BigPlant::fill", nil); err != nil {
		t.Fatalf("Perform Demo::BigPlant::fill: %v", err)
	}
	runs, err := session.Feature(p, "runs")
	if err != nil || runs.Value != opensysml.Int(1) {
		t.Errorf("runs after fill = %#v, %v; want Int(1)", runs, err)
	}
	if _, err := session.Perform(p, "Demo::BigPlant::fil", nil); !errors.Is(err, opensysml.ErrFailure) {
		t.Errorf("Perform of a name nothing declares: %v, want ErrFailure", err)
	}
}
