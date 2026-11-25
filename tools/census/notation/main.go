package notation

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// RequireEnv is the workflow variable that makes an absent specification text
// a failure rather than a skipped comparison; the spec-examples gate sets it.
const RequireEnv = "OPENSYSML_REQUIRE_SPEC_EXAMPLES"

type options struct {
	text        string
	requireText bool
}

// Main runs the notation census command: -measure re-measures the census into
// the baseline and the document, -check verifies both against the repository
// and, where the rendered text is present, against a fresh measurement.
func Main(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("notation-census", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoDir := flags.String("repo", "", "repository root (default: the module root above the working directory)")
	text := flags.String("text", "", "rendered specification text (default: "+defaultTextPath+", as scripts/download-spec-examples.sh renders it)")
	check := flags.Bool("check", false, "verify the baseline, the census document and the evidence agree, and the measurement where the text is present")
	measure := flags.Bool("measure", false, "re-measure the census into the baseline and the document")
	requireText := flags.Bool("require-text", false, "fail rather than skip the measurement when the rendered text is absent")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "notation-census: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if *check && *measure {
		fmt.Fprintln(stderr, "notation-census: -check and -measure exclude each other")
		return 2
	}
	root, err := repo.Choose(*repoDir)
	if err != nil {
		fmt.Fprintf(stderr, "notation-census: %v\n", err)
		return 1
	}
	opts := options{text: repo.Resolve(root, *text), requireText: *requireText || os.Getenv(RequireEnv) != ""}
	if opts.text == "" {
		opts.text = filepath.Join(root, filepath.FromSlash(defaultTextPath))
	}
	if *check {
		err = runCheck(root, opts, stdout)
	} else {
		err = runMeasure(root, opts, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "notation-census: %v\n", err)
		return 1
	}
	return 0
}

func textRel(root, text string) string {
	if rel, err := filepath.Rel(root, text); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(text)
}

func runMeasure(root string, opts options, out io.Writer) error {
	source, err := ReadSource(root, textRel(root, opts.text))
	if err != nil {
		return err
	}
	c, err := Measure(root, opts.text)
	if err != nil {
		return err
	}
	if problems := CheckEvidence(root, c.Rows); len(problems) > 0 {
		return fmt.Errorf("the evidence does not resolve:\n  %s", strings.Join(problems, "\n  "))
	}
	b := c.Baseline(source)
	b.sortNonDrawn()
	if err := writeBaseline(root, b); err != nil {
		return err
	}
	if err := writeDocument(root, b); err != nil {
		return err
	}
	if err := writeRows(root, c); err != nil {
		return err
	}
	t := b.Totals
	fmt.Fprintf(out, "notation-census: %d productions (%d statements): %d drawn, %d approximate, %d undrawn, %d not applicable; wrote %s, %s and %s\n",
		t.Productions, t.Statements, t.Drawn, t.Approximate, t.Undrawn, t.NotApplicable, baselinePath, censusDocPath, rowsPath)
	return nil
}

func runCheck(root string, opts options, out io.Writer) error {
	b, err := loadBaseline(root)
	if err != nil {
		return err
	}
	var failures []string
	for _, err := range []error{
		b.validate(),
		checkPin(root, b),
		checkDocumentFile(root, b),
		checkStaticEvidence(root),
		checkMeasurement(root, b, opts, out),
	} {
		if err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "\n"))
	}
	fmt.Fprintf(out, "notation-census: %d productions, baseline, census document and evidence agree\n", b.Totals.Productions)
	return nil
}

// checkPin verifies the baseline records the pinned pilot release, and the
// downloaded PDFs' pin where they are present.
func checkPin(root string, b *Baseline) error {
	pin, err := baseline.ReadPin(root)
	if err != nil {
		return err
	}
	if b.Source.PilotTag != pin.Tag {
		return fmt.Errorf("%s records pilot tag %s but %s pins %s: re-measure", baselinePath, b.Source.PilotTag, baseline.PinPath, pin.Tag)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(specPinPath))) // #nosec G304 -- fixed repository path
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fields := strings.Fields(string(content)); len(fields) < 2 || fields[0] != b.Source.PilotTag || fields[1] != b.Source.ReleaseCommit {
		return fmt.Errorf("%s pins %q but %s records %s %s", specPinPath, strings.TrimSpace(string(content)), baselinePath, b.Source.PilotTag, b.Source.ReleaseCommit)
	}
	return nil
}

func checkDocumentFile(root string, b *Baseline) error {
	content, err := readDocument(root)
	if err != nil {
		return err
	}
	return checkDocument(content, b)
}

// checkStaticEvidence resolves the citations of the verdict table and of the
// notation table's rows, which need no specification text.
func checkStaticEvidence(root string) error {
	edges, err := ReadNotationTable(root)
	if err != nil {
		return err
	}
	var rows []Row
	for _, name := range AdjudicatedNames() {
		rows = append(rows, Row{Production: Production{Name: name}, Adjudication: table[name]})
	}
	for _, name := range edges.Names() {
		adj, _ := edges.adjudicate(name)
		rows = append(rows, Row{Production: Production{Name: name}, Adjudication: adj})
	}
	if problems := CheckEvidence(root, rows); len(problems) > 0 {
		return fmt.Errorf("the evidence does not resolve:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

// checkMeasurement compares a fresh measurement with the baseline where the
// rendered text is present; absent, it skips unless the text is required.
func checkMeasurement(root string, b *Baseline, opts options, out io.Writer) error {
	if _, err := os.Stat(opts.text); errors.Is(err, os.ErrNotExist) {
		if opts.requireText {
			return fmt.Errorf("%s is absent and required (-require-text or %s): run scripts/download-spec-examples.sh", textRel(root, opts.text), RequireEnv)
		}
		fmt.Fprintf(out, "notation-census: %s is absent; the measurement comparison is skipped\n", textRel(root, opts.text))
		return nil
	} else if err != nil {
		return err
	}
	source, err := ReadSource(root, textRel(root, opts.text))
	if err != nil {
		return err
	}
	c, err := Measure(root, opts.text)
	if err != nil {
		return err
	}
	fresh := c.Baseline(source)
	fresh.sortNonDrawn()
	return b.compare(fresh)
}
