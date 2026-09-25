package repl

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// LoadPaths loads model files into the session. Each path names a file, a
// directory to walk for .sysml/.kerml files, a glob pattern, or standard input
// as a lone "-", whose contents are reported as <stdin>. A root namespace the
// files import but none of them or the library declares is looked for in the
// model files beside and below them, and each file declaring it is loaded too,
// its own imports followed the same way. Every file is
// accepted before the buffer is analyzed, so the order files are loaded in does
// not affect name resolution: a file may reference a declaration another file
// loaded after it makes. Diagnostics name the file they belong to and count
// lines from its start.
func (s *Session) LoadPaths(paths []string) ([]string, error) {
	defer s.enter()()
	return s.loadPaths(paths)
}

func (s *Session) loadPaths(paths []string) ([]string, error) {
	rep, err := s.loadPathsReport(paths)
	if err != nil {
		return nil, err
	}
	out := append(rep.Loaded, rep.Found...)
	return append(out, rep.Declared...), nil
}

// LoadReport is what a load produced, in parts so a caller can keep what the
// analysis found off the stream it prints results on.
type LoadReport struct {
	Loaded   []string // the files read, when more than one was
	Found    []string // diagnostics and the notes that belong with them
	Declared []string // what the load declared, empty if the analysis errored
	Errors   bool     // whether the analysis found an error
}

// LoadPathsReport loads model files as LoadPaths does, reporting what the
// analysis found apart from what the load declared.
func (s *Session) LoadPathsReport(paths []string) (LoadReport, error) {
	defer s.enter()()
	return s.loadPathsReport(paths)
}

func (s *Session) loadPathsReport(paths []string) (LoadReport, error) {
	files, err := ExpandPaths(paths)
	if err != nil {
		return LoadReport{}, err
	}
	srcs, err := s.readSources(s.withDependencies(files))
	if err != nil {
		return LoadReport{}, err
	}
	var loaded []string
	if len(srcs) > 1 {
		loaded = append(loaded, fmt.Sprintf("loaded %d files:", len(srcs)))
		for _, src := range srcs {
			loaded = append(loaded, "  "+src.Name)
		}
	}
	found, declared := renderSplit(s.submitFiles(srcs), s.verbosity)
	return LoadReport{Loaded: loaded, Found: found, Declared: declared, Errors: s.hasAnalysisErrors()}, nil
}

// ExpandPaths turns the paths a caller was given — files, directories to walk
// for .sysml/.kerml files, or glob patterns — into the model files to load, in a
// deterministic order and without duplicates.
func ExpandPaths(paths []string) ([]string, error) {
	files, err := project.Expand(expandHomes(paths))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errors.New("no model files to load")
	}
	return files, nil
}

// withDependencies appends to files the model files beside and below them that
// declare a root namespace they import and neither they nor the library declare.
func (s *Session) withDependencies(files []string) []string {
	return append(files, project.Dependencies(files, s.ws.IsLibraryRoot)...)
}

// expandHomes expands a leading ~ in every path.
func expandHomes(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, expandHome(p))
	}
	return out
}

// readSources reads every path into the file a load submits, under the name it
// is reported by; the error is a *ReadError or a *ReservedNameError.
func (s *Session) readSources(paths []string) ([]SourceFile, error) {
	files := make([]SourceFile, 0, len(paths))
	for _, path := range paths {
		name, data, err := project.ReadFile(path)
		if err != nil {
			return nil, readError(name, err)
		}
		if name == docName {
			return nil, &ReservedNameError{Name: name}
		}
		files = append(files, SourceFile{Name: name, Text: string(data)})
	}
	return files, nil
}

// ReservedNameError is a file a load refused because its name is the one the
// session keeps its typed text under, which a loaded file cannot share.
type ReservedNameError struct {
	Name string
}

func (e *ReservedNameError) Error() string {
	return fmt.Sprintf("cannot load %s: the name is reserved for the text typed at the prompt", e.Name)
}

// ReadError is a file a load could not read, under the name it is reported by.
type ReadError struct {
	Path string
	Err  error
}

// Error names the path once: the read error repeats it and so does every
// caller that wraps this.
func (e *ReadError) Error() string { return fmt.Sprintf("cannot read %s: %v", e.Path, e.Err) }

func (e *ReadError) Unwrap() error { return e.Err }

// readError reports a file that could not be read.
func readError(path string, err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}
	return &ReadError{Path: path, Err: err}
}
