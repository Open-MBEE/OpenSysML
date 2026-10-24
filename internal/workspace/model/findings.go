package model

import (
	"fmt"

	checkws "github.com/Open-MBEE/OpenSysML/internal/check/workspace"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// WorkspaceFindings reports what is wrong with the workspace as a whole rather
// than with one document: today, a library package more than one document
// declares. Each finding is attached to the document it is reported in, which
// may be a library document. The findings are computed once per generation.
func (w *Workspace) WorkspaceFindings() []checkws.Finding {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.findingsGen == w.generation && w.findingsKnown {
		return w.findings
	}
	w.findings = checkws.DuplicateLibraries(w.index)
	libraryPath := libs.LibraryPath()
	for i := range w.findings {
		// A workspace document standing in for a bundled file carries the
		// library's mark in the index, but its text is the workspace's own.
		f := &w.findings[i]
		f.Library = w.docs[f.Document] == nil
		if f.Library && libraryPath != "" {
			// Library files are named relative to the directory they were loaded from.
			f.Diagnostic.Message += fmt.Sprintf("; the paths are relative to %s, %s", libs.LibraryPathEnvVar, libraryPath)
		}
	}
	w.findingsGen, w.findingsKnown = w.generation, true
	return w.findings
}

// FindingsOn reports the workspace findings attached to the named document.
func (w *Workspace) FindingsOn(name string) []checkws.Finding {
	var out []checkws.Finding
	for _, f := range w.WorkspaceFindings() {
		if f.Document == name {
			out = append(out, f)
		}
	}
	return out
}
