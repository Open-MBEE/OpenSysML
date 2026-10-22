package specexamples

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// pinStamp names the file in the text directory that records the release the
// PDFs were fetched at.
const pinStamp = ".pilot-pin"

// Main is the spec-examples command; it returns the process exit status.
func Main(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("spec-examples", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoDir := flags.String("repo", "", "repository root (default: the module root containing this command)")
	textDir := flags.String("text", "", "directory holding <document>.txt, the `pdftotext -layout` rendering of each PDF (default: <repo>/build/spec-docs)")
	out := flags.String("out", "", "corpus root to write (default: <repo>/examples/spec-examples)")
	release := flags.String("release", "", "release tag the PDFs were fetched at, for the fixture headers (default: the first word of <text>/.pilot-pin)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if err := run(*repoDir, *textDir, *out, *release, stderr); err != nil {
		fmt.Fprintf(stderr, "spec-examples: %v\n", err)
		return 1
	}
	return 0
}

// run extracts every document and replaces the corpus root with the result; the
// root is written only when every document yielded examples.
func run(repoDir, textDir, out, release string, log io.Writer) error {
	repoDir, err := repo.Choose(repoDir)
	if err != nil {
		return err
	}
	if textDir == "" {
		textDir = filepath.Join(repoDir, "build", "spec-docs")
	}
	if out == "" {
		out = filepath.Join(repoDir, "examples", "spec-examples")
	}
	pin, err := os.ReadFile(filepath.Join(textDir, pinStamp)) // #nosec G304 -- the directory is named on the command line.
	if err != nil {
		return fmt.Errorf("the text directory carries no pin stamp (run scripts/download-spec-examples.sh): %w", err)
	}
	if release == "" {
		fields := strings.Fields(string(pin))
		if len(fields) == 0 {
			return fmt.Errorf("%s: empty pin stamp", filepath.Join(textDir, pinStamp))
		}
		release = fields[0]
	}

	var manifest strings.Builder
	manifest.WriteString("# Fixtures of examples/spec-examples: <path>\t<layout text line>\t<imports added>\n")
	var fixtures []Fixture
	var excluded []string
	for _, doc := range Documents {
		textPath := filepath.Join(textDir, doc.Name+".txt")
		text, err := os.ReadFile(textPath) // #nosec G304 -- the directory is named on the command line.
		if err != nil {
			return err
		}
		res := Extract(doc, string(text))
		if len(res.Blocks) == 0 {
			return fmt.Errorf("%s: no example extracted; the layout text does not look like the pinned specification", textPath)
		}
		fixtures = append(fixtures, Complete(res.Title, release, res.Blocks)...)
		for _, e := range res.Excluded {
			excluded = append(excluded, fmt.Sprintf("%s clause %s page %s line %d: %s (%s)", doc.Name, e.Clause, e.Page, e.Line, e.Reason, e.First))
		}
		fmt.Fprintf(log, "%s: %d example(s) in %d clause(s), %d excluded\n", doc.Name, len(res.Blocks), clauseCount(res.Blocks), len(res.Excluded))
	}

	staging := out + ".new"
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	for _, f := range fixtures {
		path := filepath.Join(staging, filepath.FromSlash(f.Block.Path()))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(f.Content), 0o600); err != nil {
			return err
		}
		fmt.Fprintf(&manifest, "%s\t%d\t%s\n", f.Block.Path(), f.Block.Line, strings.Join(f.Imports, ","))
	}
	manifest.WriteString("# Excluded blocks: <document> clause <clause> page <page> line <line>: <rule> (<first line>)\n")
	sort.Strings(excluded)
	for _, e := range excluded {
		manifest.WriteString("# " + e + "\n")
	}
	if err := os.WriteFile(filepath.Join(staging, "manifest.txt"), []byte(manifest.String()), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, pinStamp), pin, 0o600); err != nil {
		return err
	}
	if err := install(staging, out); err != nil {
		return err
	}
	fmt.Fprintf(log, "Wrote %d fixture(s) to %s\n", len(fixtures), out)
	return nil
}

func clauseCount(blocks []Block) int {
	seen := map[string]bool{}
	for _, b := range blocks {
		seen[b.Clause] = true
	}
	return len(seen)
}

// install swaps the staged corpus into place, recovering an orphaned `<out>.old`
// first and discarding a backup only once the new corpus is installed.
func install(staging, out string) error {
	backup := out + ".old"
	if _, err := os.Stat(out); os.IsNotExist(err) {
		if err := os.Rename(backup, out); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if _, err := os.Stat(out); err == nil {
		if err := os.RemoveAll(backup); err != nil {
			return err
		}
		if err := os.Rename(out, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(staging, out); err != nil {
		if restore := os.Rename(backup, out); restore != nil && !os.IsNotExist(restore) {
			return fmt.Errorf("%w; restoring the previous corpus failed too: %v", err, restore)
		}
		return err
	}
	return os.RemoveAll(backup)
}
