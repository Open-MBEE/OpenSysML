//go:build !sysml_prod && !sysml_noreplext

package main

import (
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/instancegraph"
)

// The REPL extension no other group owns: %features ... json.
func init() {
	replextFeature.link(nil)
}
