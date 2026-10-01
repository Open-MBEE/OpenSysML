//go:build !sysml_prod && !sysml_noreplext

package main

import (
	"flag"

	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/instancegraph"
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/notation"
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/positional"
)

// The REPL extensions no other group owns: %print and %save, %query and
// -query, %features ... json.
func init() {
	replextFeature.link(func(fs *flag.FlagSet) {
		fs.StringVar(&queryText, "query", "", "Evaluate this OSLC Query text against the model and exit")
	})
}
