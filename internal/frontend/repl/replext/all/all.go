// Package all links every REPL feature replext lets a build leave out, so a
// REPL built with it serves every command the full `sysml` does.
package all

import (
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/graphviz"      // registers the Graphviz REPL commands
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/instancegraph" // registers the instance-graph REPL commands
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/notation"      // registers the notation REPL commands
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/positional"    // registers the positional REPL commands
)
