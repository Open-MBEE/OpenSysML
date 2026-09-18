package doccounts

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// Main is the doc-counts command: it rewrites the documentation lines that are a
// function of the committed oracle baselines, of the committed analysis-library
// census or of known_failures.txt, so no contributor types them, and rewrites
// nothing else in the files it touches. The figures that move with every test
// and fixture — the compliance map's row census and the test-suite inventory —
// are not written anywhere: the documentation build counts them, and -check
// refuses a tree that states one (-site-blocks prints what the build renders).
// It returns the process exit status.
func Main(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("doc-counts", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "repository root the documentation paths are relative to (default: the product module root)")
	checkOnly := flags.Bool("check", false, "verify that generated documentation is current without writing")
	siteBlocks := flags.Bool("site-blocks", false, "print, as JSON by page and block name, the blocks the documentation build renders")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "doc-counts: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if *siteBlocks && *checkOnly {
		fmt.Fprintln(stderr, "doc-counts: -site-blocks and -check exclude each other")
		return 2
	}
	dir, err := repo.Choose(*root)
	if err != nil {
		fmt.Fprintf(stderr, "doc-counts: %v\n", err)
		return 1
	}
	if *siteBlocks {
		if err := renderSiteBlocks(dir, stdout); err != nil {
			fmt.Fprintf(stderr, "doc-counts: %v\n", err)
			return 1
		}
		return 0
	}
	var rewritten int
	if *checkOnly {
		rewritten, err = check(dir, stdout)
	} else {
		rewritten, err = run(dir, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "doc-counts: %v\n", err)
		return 1
	}
	if *checkOnly && rewritten > 0 {
		return 1
	}
	if rewritten == 0 {
		fmt.Fprintln(stdout, "doc-counts: already current")
	}
	return 0
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

// renderSiteBlocks prints the blocks the documentation build renders, as
// {"<page>": {"<block>": "<text>"}}, for the MkDocs hook to splice in.
func renderSiteBlocks(root string, out io.Writer) error {
	figures, err := ReadFigures(root)
	if err != nil {
		return err
	}
	rendered, err := RenderSiteBlocks(figures)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(rendered)
}

type rewrite struct {
	path    string
	current string
	content string
	mode    fs.FileMode
}

func pendingRewrites(root string) ([]rewrite, error) {
	compliance, _, err := readFile(root, SpecCompliancePath)
	if err != nil {
		return nil, err
	}
	counts := CountRules(compliance)
	if counts.Total == 0 {
		return nil, fmt.Errorf("%s states no rule rows", SpecCompliancePath)
	}
	if counts.KnownFailure != 0 {
		return nil, fmt.Errorf("%s: %d 🚧 rows; give them a status the census states", SpecCompliancePath, counts.KnownFailure)
	}
	figures, err := ReadFigures(root)
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
		for _, line := range BaselineLines() {
			if line.Path != path {
				continue
			}
			if updated, err = RewriteBaselineLine(updated, line, figures.Refereed); err != nil {
				return nil, err
			}
		}
		for _, block := range Blocks() {
			if block.Path != path {
				continue
			}
			if updated, err = RewriteBlock(updated, block, figures); err != nil {
				return nil, err
			}
		}
		for _, block := range SiteBlocks() {
			if block.Path != path {
				continue
			}
			if err := CheckSiteBlock(updated, block); err != nil {
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

// paths lists the files carrying a derived line or a site block, in the order the
// lines declare them and without repeating a file that carries more than one.
func paths() []string {
	var ordered []string
	seen := map[string]bool{}
	for _, line := range BaselineLines() {
		if seen[line.Path] {
			continue
		}
		seen[line.Path] = true
		ordered = append(ordered, line.Path)
	}
	for _, block := range Blocks() {
		if seen[block.Path] {
			continue
		}
		seen[block.Path] = true
		ordered = append(ordered, block.Path)
	}
	for _, path := range SitePaths() {
		if seen[path] {
			continue
		}
		seen[path] = true
		ordered = append(ordered, path)
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
