//go:build !sysml_prod && !sysml_nosync

package main

import (
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/repository" // registers the repository REPL commands
)
