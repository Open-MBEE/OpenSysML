// Package all links every REPL feature replext lets a build leave out, so a
// REPL built with it serves every command the full `sysml` does.
package all

import (
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/graphviz"
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/instancegraph"
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/notation"
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/positional"
)
