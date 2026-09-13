package repl

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/Open-MBEE/OpenSysML/internal/core/project"
)

// LoadPaths loads model files into the session. Each path names a file, a
// directory to walk for .sysml/.kerml files, a glob pattern, or standard input
// as a lone "-", whose contents are reported as <stdin>. Every file is
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
	srcs := make([]SourceFile, 0, len(files))
	names := make([]string, 0, len(files))
	for _, file := range files {
		name, data, err := project.ReadFile(file)
		if err != nil {
			return LoadReport{}, readError(name, err)
		}
		names = append(names, name)
		srcs = append(srcs, SourceFile{Name: name, Text: string(data)})
	}
	var loaded []string
	if len(files) > 1 {
		loaded = append(loaded, fmt.Sprintf("loaded %d files:", len(files)))
		for _, name := range names {
			loaded = append(loaded, "  "+name)
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

// expandHomes expands a leading ~ in every path.
func expandHomes(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, expandHome(p))
	}
	return out
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
