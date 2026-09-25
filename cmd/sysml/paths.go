package main

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// namedModels lists the models a message is about, calling standard input what
// its diagnostics call it rather than by the "-" that named it.
func namedModels(files []string) string {
	names := make([]string, 0, len(files))
	for _, file := range files {
		if project.IsStdin(file) {
			names = append(names, project.StdinName)
			continue
		}
		names = append(names, file)
	}
	return strings.Join(names, ", ")
}
