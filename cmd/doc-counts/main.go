// Command doc-counts rewrites the documentation lines that are a function of the
// committed oracle baselines, so no contributor types them. It reads them through
// internal/doccounts, which the guard in cmd/pilot-diff reads too, and rewrites
// nothing else in the files it touches. The compliance map's own row census is not
// written anywhere: the documentation build counts it. Run it with `make docs-counts`.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doccounts"
)

func main() {
	root := flag.String("root", ".", "repository root the documentation paths are relative to")
	checkOnly := flag.Bool("check", false, "verify that generated documentation is current without writing")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "doc-counts: unexpected argument %q\n", flag.Arg(0))
		os.Exit(2)
	}
	var rewritten int
	var err error
	if *checkOnly {
		rewritten, err = check(*root, os.Stdout)
	} else {
		rewritten, err = run(*root, os.Stdout)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "doc-counts: %v\n", err)
		os.Exit(1)
	}
	if *checkOnly && rewritten > 0 {
		os.Exit(1)
	}
	if rewritten == 0 {
		fmt.Fprintln(os.Stdout, "doc-counts: already current")
	}
}

// run restates every derived line from the census and reports how many files it
// changed. A file already stating the census is left untouched, which is what
// makes a second run a no-op.
func run(root string, out io.Writer) (int, error) {
	pending, err := pendingRewrites(root)
	if err != nil {
		return 0, err
	}

	for _, file := range pending {
		if err := checkWritable(root, file.path); err != nil {
			return 0, err
		}
	}
	for _, file := range pending {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(file.path)), []byte(file.content), file.mode); err != nil {
			return 0, err
		}
		fmt.Fprintf(out, "doc-counts: rewrote %s\n", file.path)
	}
	return len(pending), nil
}

// check reports generated differences without modifying any file.
func check(root string, out io.Writer) (int, error) {
	pending, err := pendingRewrites(root)
	if err != nil {
		return 0, err
	}
	for _, file := range pending {
		fmt.Fprintf(out, "doc-counts: %s is stale\n", file.path)
		fmt.Fprint(out, diffReport(file.path, file.current, file.content))
	}
	return len(pending), nil
}

type rewrite struct {
	path    string
	current string
	content string
	mode    fs.FileMode
}

func pendingRewrites(root string) ([]rewrite, error) {
	compliance, _, err := readFile(root, doccounts.SpecCompliancePath)
	if err != nil {
		return nil, err
	}
	counts := doccounts.CountRules(compliance)
	if counts.Total == 0 {
		return nil, fmt.Errorf("%s states no rule rows", doccounts.SpecCompliancePath)
	}
	if counts.KnownFailure != 0 {
		return nil, fmt.Errorf("%s: %d 🚧 rows; give them a status the census states", doccounts.SpecCompliancePath, counts.KnownFailure)
	}
	refereed, err := doccounts.ReadRefereedCounts(root)
	if err != nil {
		return nil, err
	}

	var pending []rewrite
	for _, path := range paths() {
		content, mode, err := readFile(root, path)
		if err != nil {
			return nil, err
		}
		updated := content
		for _, line := range doccounts.BaselineLines() {
			if line.Path != path {
				continue
			}
			if updated, err = doccounts.RewriteBaselineLine(updated, line, refereed); err != nil {
				return nil, err
			}
		}
		for _, block := range doccounts.Blocks() {
			if block.Path != path {
				continue
			}
			if updated, err = doccounts.RewriteBlock(updated, block, refereed); err != nil {
				return nil, err
			}
		}
		if updated == content {
			continue
		}
		pending = append(pending, rewrite{path: path, current: content, content: updated, mode: mode})
	}
	return pending, nil
}

func diffReport(path, current, generated string) string {
	currentLines := strings.Split(current, "\n")
	generatedLines := strings.Split(generated, "\n")
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s (current)\n+++ %s (generated)\n", path, path)
	for i := 0; i < len(currentLines) || i < len(generatedLines); i++ {
		var old, fresh string
		if i < len(currentLines) {
			old = currentLines[i]
		}
		if i < len(generatedLines) {
			fresh = generatedLines[i]
		}
		if old == fresh {
			continue
		}
		fmt.Fprintf(&b, "@@ line %d @@\n", i+1)
		if i < len(currentLines) {
			fmt.Fprintf(&b, "-%s\n", old)
		}
		if i < len(generatedLines) {
			fmt.Fprintf(&b, "+%s\n", fresh)
		}
	}
	return b.String()
}

// checkWritable reports whether a file can be opened for writing, so an
// unwritable file is refused before any file is rewritten.
func checkWritable(root, path string) error {
	file, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(path)), os.O_WRONLY, 0) // #nosec G304 -- a documentation path this command declares
	if err != nil {
		return err
	}
	return file.Close()
}

// paths lists the files carrying a derived line, in the order the lines declare
// them and without repeating a file that carries more than one.
func paths() []string {
	var ordered []string
	seen := map[string]bool{}
	for _, line := range doccounts.BaselineLines() {
		if seen[line.Path] {
			continue
		}
		seen[line.Path] = true
		ordered = append(ordered, line.Path)
	}
	for _, block := range doccounts.Blocks() {
		if seen[block.Path] {
			continue
		}
		seen[block.Path] = true
		ordered = append(ordered, block.Path)
	}
	return ordered
}

func readFile(root, path string) (string, fs.FileMode, error) {
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Stat(full)
	if err != nil {
		return "", 0, err
	}
	content, err := os.ReadFile(full) // #nosec G304 -- a documentation path this command declares
	if err != nil {
		return "", 0, err
	}
	return string(content), info.Mode().Perm(), nil
}
