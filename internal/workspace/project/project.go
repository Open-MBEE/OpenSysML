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

// Expand turns the paths named on a command line (or at a %load prompt) into
// the model files to load: a directory contributes every model file under it,
// a pattern contributes the model files among its matches, and any other path
// is taken as named.
// Files come back in a deterministic order — the inputs in the order given,
// each directory walk and each pattern match sorted by path — and duplicates
// are dropped, so naming a file twice loads it once.
func Expand(inputs []string) ([]string, error) {
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
		files, err := expandOne(in)
		if err != nil {
			return nil, err
		}
		add(files...)
	}
	return out, nil
}

// expandOne expands a single input path.
func expandOne(input string) ([]string, error) {
	// Standard input is neither statted nor globbed: "-" names the stream
	// wherever it is written, even where a file of that name exists.
	if IsStdin(input) {
		return []string{input}, nil
	}
	info, err := os.Stat(input)
	switch {
	case err == nil && info.IsDir():
		return ModelFiles(input)
	case err == nil:
		return []string{input}, nil
	case hasMeta(input):
		return expandPattern(input)
	case errors.Is(err, fs.ErrNotExist):
		// Reported by the loader, which names the file it could not read.
		return []string{input}, nil
	default:
		return nil, err
	}
}

// expandPattern expands a glob pattern, walking any directory it matches.
func expandPattern(pattern string) ([]string, error) {
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
		if info.IsDir() {
			var files []string
			if err := walk(m, map[string]bool{}, &files); err != nil {
				return nil, err
			}
			sort.Strings(files)
			out = append(out, files...)
			continue
		}
		if IsModelFile(m) {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no model files match %q", pattern)
	}
	return out, nil
}

// ModelFiles walks dir and returns every .sysml/.kerml file under it, sorted by
// path. Hidden directories are skipped, so a repository's .git or a build cache
// under a dot-directory contributes nothing. Symlinked directories are walked —
// a project may keep a shared library as a link — each at most once, so a cycle
// terminates.
func ModelFiles(dir string) ([]string, error) {
	var out []string
	if err := walk(dir, map[string]bool{}, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no .sysml or .kerml files in %s", dir)
	}
	sort.Strings(out)
	return out, nil
}

// walk appends the model files under dir to out, descending into
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
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if err := walk(path, visited, out); err != nil {
				return err
			}
		case IsModelFile(path):
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
