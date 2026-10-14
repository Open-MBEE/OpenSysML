package main

import (
	"flag"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
)

// Drift gate: every sysml flag that is an operation or one of its arguments
// is classified exactly once in the sysml-mcp tool catalog, as the tool that
// performs it, as unserved by the service, or as CLI-only. The General and
// Deprecated option groups are the process's own and are not operations;
// an alias is classified under its long spelling.
func TestEverySysmlFlagIsClassifiedForMCP(t *testing.T) {
	process := map[string]bool{}
	for _, group := range doc().Options {
		for _, opt := range group.Options {
			// An alias (-e for -eval) is classified under its long spelling.
			for _, alias := range opt.Aliases {
				process[alias] = true
			}
			if group.Title == "General" || group.Title == "Deprecated" {
				process[opt.Name] = true
			}
		}
	}
	if len(process) == 0 {
		t.Fatal("the General and Deprecated option groups were not found")
	}
	var classified int
	docFlags().VisitAll(func(f *flag.Flag) {
		if process[f.Name] {
			return
		}
		classified++
		if _, err := catalog.ClassifyFlag(f.Name); err != nil {
			t.Error(err)
		}
	})
	if classified == 0 {
		t.Fatal("no sysml flag was classified")
	}
	// The catalog may not name a flag the CLI no longer has.
	for _, e := range catalog.Tools {
		for _, name := range e.Flags {
			if docFlags().Lookup(name) == nil {
				t.Errorf("catalog tool %s lists sysml -%s, which the CLI does not have", e.Name, name)
			}
		}
	}
	for _, table := range []struct {
		name  string
		flags map[string]string
	}{{"Unserved", catalog.Unserved}, {"CLIOnly", catalog.CLIOnly}} {
		for name := range table.flags {
			if docFlags().Lookup(name) == nil {
				t.Errorf("catalog.%s lists sysml -%s, which the CLI does not have", table.name, name)
			}
		}
	}
}
