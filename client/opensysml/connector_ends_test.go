package opensysml_test

import (
	"context"
	"slices"
	"testing"
)

const connectorEndsSource = `
package Ends {
    port def P;
    part def Unit { port p : P; }
    part def Vehicle {
        port q : P;
        port r : P;
        part x : Unit;
        part y : Unit;
        connection plain connect q to r;
        connection named connect (a ::> x.p, b ::> y.p);
        binding bound bind x.p = y.p;
        bind q = r;
    }
}
`

// A connector's ends are its child symbols, named or not, each referencing the
// feature it attaches, a chain as one entry per chaining feature.
func TestLookupSymbolListsConnectorEnds(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, connectorEndsSource)
	cases := []struct {
		connector string
		ends      []string
		targets   [][]string
	}{
		{"Ends::Vehicle::plain", []string{"Ends__Vehicle__plain_pend0", "Ends__Vehicle__plain_pend1"},
			[][]string{{"Ends::Vehicle::q"}, {"Ends::Vehicle::r"}}},
		{"Ends::Vehicle::named", []string{"Ends::Vehicle::named::a", "Ends::Vehicle::named::b"},
			[][]string{{"Ends::Vehicle::x", "Ends::Unit::p"}, {"Ends::Vehicle::y", "Ends::Unit::p"}}},
		{"Ends::Vehicle::bound", []string{"Ends__Vehicle__bound_pend0", "Ends__Vehicle__bound_pend1"},
			[][]string{{"Ends::Vehicle::x", "Ends::Unit::p"}, {"Ends::Vehicle::y", "Ends::Unit::p"}}},
		{"Ends::Vehicle::@7", []string{"Ends__Vehicle___407_pend0", "Ends__Vehicle___407_pend1"},
			[][]string{{"Ends::Vehicle::q"}, {"Ends::Vehicle::r"}}},
	}
	for _, tc := range cases {
		connector, err := client.LookupSymbol(context.Background(), model, tc.connector)
		if err != nil {
			t.Fatalf("LookupSymbol(%s): %v", tc.connector, err)
		}
		if !slices.Equal(connector.ChildIDs, tc.ends) {
			t.Errorf("%s children = %v, want %v", tc.connector, connector.ChildIDs, tc.ends)
			continue
		}
		for i, id := range connector.ChildIDs {
			end, err := client.LookupSymbol(context.Background(), model, id)
			if err != nil {
				t.Fatalf("LookupSymbol(%s): %v", id, err)
			}
			if end.Kind != "connectorEnd" {
				t.Errorf("%s kind = %q, want connectorEnd", id, end.Kind)
			}
			var targets []string
			for _, spec := range end.Specializations {
				if spec.Kind == "references" {
					targets = append(targets, spec.TargetID)
				}
			}
			if !slices.Equal(targets, tc.targets[i]) {
				t.Errorf("%s references %v, want %v", id, targets, tc.targets[i])
			}
		}
	}
}
