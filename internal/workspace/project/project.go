// Package project turns the paths a user names — files, directories and glob
// patterns — into the model files to load, in a deterministic order.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/notebook"
)

// Stdin is the path that names standard input, by the convention that a lone
// "-" is read from the stream rather than from a file of that name. A file
// really called "-" is named as "./-".
const Stdin = "-"

// StdinName is what a model read from standard input is called in diagnostics.
const StdinName = "<stdin>"

// IsStdin reports whether path names standard input.
func IsStdin(path string) bool { return path == Stdin }

// modelExts are the file extensions a directory walk collects.
var modelExts = []string{".sysml", ".kerml"}

// IsModelFile reports whether path names a SysML or KerML model file.
func IsModelFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, want := range modelExts {
		if ext == want {
			return true
		}
	}
	return false
}

// IsLoadable reports whether path names a source a load reads: a model file or
// a notebook, whose code cells are the model.
func IsLoadable(path string) bool {
	return IsModelFile(path) || notebook.IsNotebook(path)
}

// Expand turns the paths named on a command line (or at a %load prompt) into
// the sources to load: a directory contributes every model file and notebook
// under it, a pattern contributes the model files and notebooks among its
// matches, and any other path is taken as named.
// Files come back in a deterministic order — the inputs in the order given,
// each directory walk and each pattern match sorted by path — and duplicates
// are dropped, so naming a file twice loads it once.
func Expand(inputs []string) ([]string, error) {
	return Discovery{}.Expand(inputs)
}

// Expand is the package's Expand with the discovery's library roots: a named
// directory holding a project interchange file contributes, after its own
// files, those of the installed projects the project uses.
func (d Discovery) Expand(inputs []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	add := func(paths ...string) {
		for _, p := range paths {
			key := absKey(p)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, p)
		}
	}
	for _, in := range inputs {
		files, err := d.expandOne(in)
		if err != nil {
			return nil, err
		}
		add(files...)
	}
	return out, nil
}

// expandOne expands a single input path.
func (d Discovery) expandOne(input string) ([]string, error) {
	// Standard input is neither statted nor globbed: "-" names the stream
	// wherever it is written, even where a file of that name exists.
	if IsStdin(input) {
		return []string{input}, nil
	}
	info, err := os.Stat(input)
	switch {
	case err == nil && info.IsDir() && IsProjectDir(input):
		return d.projectFiles(input)
	case err == nil && info.IsDir():
		return ModelFiles(input)
	case err == nil && info.Mode().IsRegular() && IsArchive(input):
		return d.archiveFiles(input)
	case err == nil:
		return []string{input}, nil
	case hasMeta(input):
		return d.expandPattern(input)
	case errors.Is(err, fs.ErrNotExist):
		// Reported by the loader, which names the file it could not read.
		return []string{input}, nil
	default:
		return nil, err
	}
}

// expandPattern expands a glob pattern: a project directory or archive it
// matches contributes what naming it would, any other directory its files.
func (d Discovery) expandPattern(pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("bad pattern %q: %w", pattern, err)
	}
	sort.Strings(matches)
	var out []string
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			return nil, err
		}
		// A pattern names its matches only collectively, so one that catches a
		// doc directory or a README contributes nothing rather than failing.
		switch {
		case info.IsDir() && IsProjectDir(m):
			files, err := d.projectFiles(m)
			if err != nil {
				return nil, err
			}
			out = append(out, files...)
		case info.IsDir():
			var files []string
			if err := walk(m, map[string]bool{}, &files); err != nil {
				return nil, err
			}
			sort.Strings(files)
			out = append(out, files...)
		case info.Mode().IsRegular() && IsArchive(m):
			files, err := d.archiveFiles(m)
			if err != nil {
				return nil, err
			}
			out = append(out, files...)
		case IsLoadable(m):
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no model files match %q", pattern)
	}
	return out, nil
}

// projectFiles is ModelFiles over a project directory, followed by the model
// files of the installed projects the project uses: the project as its package
// manager sees it. A project with no files of its own is loaded as what it uses.
func (d Discovery) projectFiles(dir string) ([]string, error) {
	var own []string
	if err := walk(dir, map[string]bool{}, &own); err != nil {
		return nil, err
	}
	sort.Strings(own)
	out := append(own, projectFiles(d.Around([]string{dir}).Used)...)
	if len(out) == 0 {
		return nil, fmt.Errorf("no .sysml or .kerml files or .ipynb notebooks in %s or the projects it uses", dir)
	}
	return out, nil
}

// archiveFiles lists the model members of a project archive, then the files of
// the installed projects it uses. An archive that is not a project interchange
// file is left to the loader, which reports it under the archive's path.
func (d Discovery) archiveFiles(archive string) ([]string, error) {
	a, err := openCached(archive)
	if err != nil {
		return []string{archive}, nil
	}
	installed, _ := installedProjects(d.libraryRoots(EnclosingProjects(archive)))
	uses, _ := used([]*Interchange{a.Project}, installed)
	out := append(a.Files(), projectFiles(uses)...)
	if len(out) == 0 {
		return nil, fmt.Errorf("no .sysml or .kerml files in %s or the projects it uses", archive)
	}
	return out, nil
}

// ModelFiles walks dir and returns every .sysml/.kerml file and .ipynb notebook
// under it, sorted by path. Hidden directories are skipped, so a repository's .git or a build cache
// under a dot-directory contributes nothing, as are the well-known library
// directories of a project directory, whose contents its usages select. Symlinked directories are walked —
// a project may keep a shared library as a link — each at most once, so a cycle
// terminates.
func ModelFiles(dir string) ([]string, error) {
	var out []string
	if err := walk(dir, map[string]bool{}, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no .sysml or .kerml files or .ipynb notebooks in %s", dir)
	}
	sort.Strings(out)
	return out, nil
}

// walk appends the model files and notebooks under dir to out, descending into
// subdirectories, symlinked ones included. visited holds the resolved
// directories already walked, which is what keeps a link cycle finite.
func walk(dir string, visited map[string]bool, out *[]string) error {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	if visited[resolved] {
		return nil
	}
	visited[resolved] = true
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		info, err := os.Stat(path) // resolves a link to what it points at
		if err != nil {
			// A dangling link names no model file; the walk is not about it.
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
		switch {
		case info.IsDir():
			if strings.HasPrefix(e.Name(), ".") || IsLibraryDir(dir, e.Name()) {
				continue
			}
			if err := walk(path, visited, out); err != nil {
				return err
			}
		case IsLoadable(path):
			*out = append(*out, path)
		}
	}
	return nil
}

// absKey is the identity a path is deduplicated under: its absolute form when
// that can be computed, the path itself otherwise. Standard input keeps its own
// identity, being no file, least of all the one called "-" beside it.
func absKey(path string) string {
	if IsStdin(path) {
		return Stdin
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

// hasMeta reports whether path is written as a glob pattern.
func hasMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}
