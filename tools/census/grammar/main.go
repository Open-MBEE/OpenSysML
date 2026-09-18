// Package grammar measures which productions of the OMG Xtext
// grammars our test inputs exercise, on input-presence evidence: for every rule,
// fragment, enum and terminal of KerML.xtext, KerMLExpressions.xtext and
// SysML.xtext it searches the corpora we already parse for the literals the
// production requires, and buckets it as evidence, no-evidence or
// indistinguishable.
//
// The number it reports is an over-approximation. A literal in a corpus file
// means an input could have driven that production; it does not prove our
// parser took that path, nor that it handled it correctly. It is not a
// compliance measure.
//
// It is advisory: nothing in the build or the test suite depends on it, and it
// only reads the corpora. Provision the grammars with
// scripts/download-pilot-grammars.sh, then run `go run -C tools ./cmd/grammar-coverage`.
// -baseline refreshes the committed docs/project/grammar-coverage-baseline.json,
// which carries the counts and the gaps rather than every row. See
// docs/project/grammar-coverage.md.
package grammar

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// Main is the grammar-coverage command; it returns the process exit status.
func Main(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("grammar-coverage", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoDir := flags.String("repo", "", "repository root (default: the module root containing this command)")
	grammars := flags.String("grammars", "", "directory holding the .xtext grammars (default: <repo>/build/pilot-grammars)")
	out := flags.String("out", "", "output directory for the reports (default: <repo>/build/grammar-coverage)")
	baseline := flags.String("baseline", "", "also write the compact baseline JSON to this file")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if err := run(*repoDir, *grammars, *out, *baseline); err != nil {
		fmt.Fprintf(stderr, "grammar-coverage: %v\n", err)
		return 1
	}
	return 0
}

func run(repoDir, grammarDir, out, baseline string) error {
	repoDir, err := repo.Choose(repoDir)
	if err != nil {
		return err
	}
	grammarDir, out, baseline = repo.Resolve(repoDir, grammarDir), repo.Resolve(repoDir, out), repo.Resolve(repoDir, baseline)
	if grammarDir == "" {
		grammarDir = filepath.Join(repoDir, "build", "pilot-grammars")
	}
	if out == "" {
		out = filepath.Join(repoDir, "build", "grammar-coverage")
	}

	files, err := grammarFiles(grammarDir)
	if err != nil {
		return err
	}

	var parsedGrammars []*Grammar
	var literals []string
	for _, path := range files {
		// #nosec G304 -- the grammar directory is named on the command line.
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		parsed, err := ParseGrammar(filepath.Base(path), string(data))
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "%s: %d production(s)\n", parsed.Name, len(parsed.Productions))
		parsedGrammars = append(parsedGrammars, parsed)
		for _, production := range parsed.Productions {
			literals = append(literals, production.Literals()...)
		}
	}

	lits, err := newLitTable(literals)
	if err != nil {
		return err
	}
	index, err := buildLiteralIndex(repoDir, evidenceRoots, lits)
	if err != nil {
		return err
	}
	corpusFiles := 0
	for _, root := range index.Roots() {
		corpusFiles += root.Files
	}
	if corpusFiles == 0 {
		return fmt.Errorf("no .sysml or .kerml files found under %s: is -repo right?", repoDir)
	}
	fmt.Fprintf(os.Stderr, "searched %d corpus file(s) for %d distinct literal(s)\n", corpusFiles, len(lits.order))

	rows := classifyAll(parsedGrammars, newAnalyzer(parsedGrammars, lits), index)
	report := buildReport(pilotTag(grammarDir), rows, index.Roots())
	if err := writeReports(out, report); err != nil {
		return err
	}
	if baseline == "" {
		return nil
	}
	return writeBaseline(baseline, report)
}

// classifyAll classifies every production, keeping the grammars in the order
// they were read and the productions in declaration order.
func classifyAll(grammars []*Grammar, an *analyzer, index *literalIndex) []GrammarReport {
	reports := make([]GrammarReport, 0, len(grammars))
	for _, grammar := range grammars {
		report := GrammarReport{Name: grammar.Name}
		for _, production := range grammar.Productions {
			report.Productions = append(report.Productions, classify(production, an, index))
		}
		reports = append(reports, report)
	}
	return reports
}

// grammarFiles returns the grammars to measure, sorted by name so the report is
// deterministic.
func grammarFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("grammars not found at %s: run ./scripts/download-pilot-grammars.sh", dir)
		}
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".xtext" {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no .xtext grammars in %s: run ./scripts/download-pilot-grammars.sh", dir)
	}
	sort.Strings(files)
	return files, nil
}

// pilotTag reads the release the grammars were fetched at, as recorded by the
// download script.
func pilotTag(dir string) string {
	// #nosec G304 -- the grammar directory is named on the command line.
	data, err := os.ReadFile(filepath.Join(dir, "PILOT_TAG"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}
