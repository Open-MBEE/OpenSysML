package repl

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/notebook"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// LoadPaths loads model files into the session. Each path names a file, a
// directory to walk for .sysml/.kerml files, a glob pattern, or standard input
// as a lone "-", whose contents are reported as <stdin>. A root namespace the
// files reference — by an import or a qualified name — but none of them or the
// library declares is looked for in the model files beside and below them, and
// each file declaring it is loaded too, its own references followed the same
// way. Every file is
// accepted before the buffer is analyzed, so the order files are loaded in does
// not affect name resolution: a file may reference a declaration another file
// loaded after it makes. Diagnostics name the file they belong to and count
// lines from its start.
func (s *Session) LoadPaths(paths []string) ([]string, error) {
	defer s.enter()()
	return s.loadPaths(paths, loadOptions{})
}

func (s *Session) loadPaths(paths []string, opts loadOptions) ([]string, error) {
	rep, err := s.loadPathsReport(paths, opts)
	if err != nil {
		return nil, err
	}
	out := append(rep.Loaded, rep.Found...)
	return append(out, rep.Declared...), nil
}

// LoadReport is what a load produced, in parts so a caller can keep what the
// analysis found off the stream it prints results on.
type LoadReport struct {
	Loaded   []string // the files read, when more than one was, and what each notebook contributed
	Found    []string // diagnostics and the notes that belong with them
	Declared []string // what the load declared, empty if the analysis errored
	Errors   bool     // whether the analysis found an error
}

// LoadPathsReport loads model files as LoadPaths does, reporting what the
// analysis found apart from what the load declared.
func (s *Session) LoadPathsReport(paths []string) (LoadReport, error) {
	defer s.enter()()
	return s.loadPathsReport(paths, loadOptions{})
}

func (s *Session) loadPathsReport(paths []string, opts loadOptions) (LoadReport, error) {
	files, err := s.expandPaths(paths)
	if err != nil {
		return LoadReport{}, err
	}
	read, err := s.readWithDependencies(files, opts)
	if err != nil {
		return LoadReport{}, err
	}
	srcs := read.files
	loaded := append(loadedFiles(srcs), read.notes...)
	archives := s.discoveryWarnings(files)
	if len(srcs) == 0 {
		return LoadReport{Loaded: loaded, Found: archives, Errors: s.hasAnalysisErrors()}, nil
	}
	found, declared := renderSplit(s.submitFiles(srcs), s.verbosity)
	found = append(found, conversionWarnings(srcs, s.verbosity)...)
	found = append(found, archives...)
	return LoadReport{Loaded: loaded, Found: found, Declared: declared, Errors: s.hasAnalysisErrors()}, nil
}

// loadedFiles lists the files a load read when it read more than one, a
// notebook counted once however many cells it contributed.
func loadedFiles(srcs []SourceFile) []string {
	var names []string
	seen := map[string]bool{}
	for _, src := range srcs {
		name := src.Name
		if src.Of != "" {
			name = src.Of
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, "  "+name)
		}
	}
	if len(names) < 2 {
		return nil
	}
	return append([]string{fmt.Sprintf("loaded %d files:", len(names))}, names...)
}

func conversionWarnings(files []SourceFile, verbosity Verbosity) []string {
	var warnings []string
	for _, file := range files {
		warnings = append(warnings, file.Warnings...)
	}
	return warningLines(warnings, verbosity)
}

// LoadWarnings reports what the projects and archives a load of paths touched fell
// short of as warnings located at their file (code project.load), less and counting the suppressed.
func (s *Session) LoadWarnings(paths []string) ([]Diagnostic, int) {
	defer s.enter()()
	return s.loadWarnings(expandHomes(paths))
}

func (s *Session) loadWarnings(paths []string) (out []Diagnostic, suppressed int) {
	for _, w := range s.discovery().Warnings(paths) {
		d := Diagnostic{Severity: "warning", Message: w.Message, File: w.Path, Pass: "project", Code: "project.load"}
		if s.suppress != nil && s.suppress(d.File, d.Code, d.Message) {
			suppressed++
			continue
		}
		out = append(out, d)
	}
	return out, suppressed
}

// warningLines renders warnings as the lines a load reports, none when quiet.
func warningLines(warnings []string, verbosity Verbosity) []string {
	if verbosity <= VerbosityQuiet {
		return nil
	}
	var lines []string
	for _, warning := range warnings {
		lines = append(lines, "warning: "+warning)
	}
	return lines
}

// discoveryWarnings renders what the projects and archives a load of paths
// touched fell short of as the warning lines a load reports, none when quiet.
func (s *Session) discoveryWarnings(paths []string) []string {
	var out []string
	diags, _ := s.loadWarnings(paths)
	for _, d := range diags {
		out = append(out, d.File+": "+d.Message)
	}
	return warningLines(out, s.verbosity)
}

// ExpandPaths turns the paths a caller was given — files, directories to walk
// for .sysml/.kerml files and .ipynb notebooks, or glob patterns — into the
// sources to load, in a deterministic order and without duplicates. A directory
// holding a project interchange file stands for the project's own files and
// those of the installed projects it uses, found under the session's library roots.
func (s *Session) ExpandPaths(paths []string) ([]string, error) {
	defer s.enter()()
	return s.expandPaths(paths)
}

func (s *Session) expandPaths(paths []string) ([]string, error) {
	files, err := s.discovery().Expand(expandHomes(paths))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errors.New("no model files to load")
	}
	if err := s.guardFiles(files, true); err != nil {
		return nil, err
	}
	return files, nil
}

// SetLibraryRoots replaces the directories installed projects are looked for
// under, besides the well-known ones beneath a loaded file's project.
func (s *Session) SetLibraryRoots(roots []string) {
	defer s.enter()()
	s.libraryRoots = append([]string(nil), roots...)
	// The workspace index searches the installed projects too, so it is built
	// again over the new roots.
	s.workspace = nil
}

// LibraryRoots reports the directories installed projects are looked for under.
func (s *Session) LibraryRoots() []string {
	s.state.Lock()
	defer s.state.Unlock()
	return append([]string(nil), s.libraryRoots...)
}

// discovery is the search for installed projects under the session's library roots.
func (s *Session) discovery() project.Discovery {
	d := project.Discovery{Roots: s.libraryRoots}
	if s.fileGuard != nil {
		d.Guard = func(path string) error { return s.guardFile(path, true) }
	}
	return d
}

// reading is what readSources read: the sources a load submits, and what the
// notebooks among them report of their cells.
type reading struct {
	files []SourceFile
	notes []string
	// deps is the same text under the paths the dependency search starts
	// from, a notebook's cells under the notebook's path.
	deps []project.Source
}

// readWithDependencies reads paths, then the model files beside and below them
// that declare a root namespace they reference and neither they, the library
// nor the rest of the buffer declares.
func (s *Session) readWithDependencies(paths []string, opts loadOptions) (reading, error) {
	read, err := s.readSources(paths, opts)
	if err != nil {
		return reading{}, err
	}
	deps := s.discovery().DependenciesOf(read.deps, s.knownRootBesides(read.files))
	if len(deps) == 0 {
		return read, nil
	}
	if err := s.guardFiles(deps, true); err != nil {
		return reading{}, err
	}
	more, err := s.readSources(deps, loadOptions{})
	if err != nil {
		return reading{}, err
	}
	read.files = append(read.files, more.files...)
	return read, nil
}

// knownRootBesides reports the root namespace names the library or the buffer
// declares, what the files being read declared before left out — the cells of
// a notebook read whole too: a reload supersedes that, so a reference to such
// a name is looked for beside them.
func (s *Session) knownRootBesides(files []SourceFile) func(name string) bool {
	replacing, whole := map[string]bool{}, map[string]bool{}
	for _, f := range files {
		if key := f.key(); key != "" {
			replacing[key] = true
		}
		if f.Whole {
			whole[fileKeyOf(f.Of)] = true
		}
	}
	declared := s.declaredRoots(func(sn snippet) bool {
		return sn.key != "" && replacing[sn.key] || sn.of != "" && whole[sn.of]
	})
	return func(name string) bool { return declared[name] || s.ws.IsLibraryRoot(name) }
}

// expandHomes expands a leading ~ in every path.
func expandHomes(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, expandHome(p))
	}
	return out
}

// readSources reads every path into the sources a load submits, under the names
// they are reported by — a notebook's cells each under its own; the error is a
// *ReadError, a *ReservedNameError, or a notebook's refusal. opts picks the
// cells of the notebooks; it is an error when no notebook was named.
func (s *Session) readSources(paths []string, opts loadOptions) (reading, error) {
	var read reading
	notebooks := 0
	for _, path := range paths {
		if notebook.IsNotebook(path) {
			notebooks++
			cells, note, err := s.readNotebook(path, opts.cells)
			if err != nil {
				return reading{}, err
			}
			read.files = append(read.files, cells...)
			read.notes = append(read.notes, note...)
			for _, cell := range cells {
				read.deps = append(read.deps, project.Source{Path: path, Content: []byte(cell.Text)})
			}
			continue
		}
		name, data, err := project.ReadFile(path)
		if err != nil {
			return reading{}, readError(name, err)
		}
		if err := reservedName(name); err != nil {
			return reading{}, err
		}
		var warnings []string
		text, converted := data, false
		if s.sourceConverter != nil {
			var err error
			text, converted, err = s.sourceConverter(name, data, func(message string) {
				warnings = append(warnings, message)
			})
			if err != nil {
				return reading{}, fmt.Errorf("cannot convert %s: %w", name, err)
			}
		}
		var kind source.Kind
		if converted {
			kind = source.KindSysML
		}
		read.files = append(read.files, SourceFile{Name: name, Text: string(text), Kind: kind, Warnings: warnings})
		read.deps = append(read.deps, project.Source{Path: path, Content: text})
	}
	if notebooks == 0 && !opts.cells.IsZero() {
		return reading{}, errors.New("--cells picks the cells of a notebook, and no .ipynb was named")
	}
	return read, nil
}

// ReservedNameError is a file a load refused because its name is the one the
// session keeps its typed text under, which a loaded file cannot share.
type ReservedNameError struct {
	Name string
}

func (e *ReservedNameError) Error() string {
	return fmt.Sprintf("cannot load %s: the name is reserved for the text typed at the prompt", e.Name)
}

// reservedName is the *ReservedNameError refusing a file named as the
// transcript, nil for any other name.
func reservedName(name string) error {
	if name != docName {
		return nil
	}
	return &ReservedNameError{Name: name}
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
