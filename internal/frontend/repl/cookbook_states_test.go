package repl

import (
	"path/filepath"
	"testing"
)

// TestCookbookStateAndEventRecipes drives the manual's cookbook model as the
// cookbook does and checks the recipes print the rows it quotes.
func TestCookbookStateAndEventRecipes(t *testing.T) {
	s := NewSession()
	source := filepath.Join("..", "..", "..", "docs", "manual", "examples", "cookbook.sysml")
	if _, err := s.LoadFile(source); err != nil {
		t.Fatalf("load cookbook: %v", err)
	}
	for _, cmd := range []string{
		"%trace on",
		"%instantiate dome",
		"%instantiate spareDome",
		"%state control dome",
		"%send Open",
		"%advance 1",
		"%send Slew(azimuth = 120.0)",
		"%advance 1",
		"%send Open to spareDome",
		"%advance 0.5",
		"%send Close to spareDome",
		"%advance 0.5",
	} {
		run(t, s, cmd)
	}
	wants(t, run(t, s, "%run-query DomeStates root=#1"),
		"✓ Query Cookbook::DomeStates returned 2 rows",
		"Columns: machine, statePath, region, enclosing",
		"Row 1: #1.control in open.slewing",
		`statePath = "open.slewing"`,
		`region = "pointing"`,
		`enclosing = "open"`,
		"Row 2: #1.control in open.opened",
		`region = "shutter"`)
	wants(t, run(t, s, "%run-query Opened"),
		"✓ Query Cookbook::Opened returned 1 row",
		"Row 1: Cookbook::dome (#1)",
		`qualifiedName = "Cookbook::dome"`)
	wants(t, run(t, s, "%run-query Accepted root=#1"),
		"✓ Query Cookbook::Accepted returned 1 row",
		"Row 1: t=1 Cookbook::dome.control: accept Slew",
		"time = 1.0 [s]",
		`event = "Slew"`,
		`payload = "azimuth = 120.0"`)
	wants(t, run(t, s, "%run-query Accepted root=spareDome"),
		"✓ Query Cookbook::Accepted returned 1 row",
		"Row 1: t=2 Cookbook::spareDome.control: accept Open",
		"time = 2.0 [s]",
		`event = "Open"`,
		"payload = (none)")
}
