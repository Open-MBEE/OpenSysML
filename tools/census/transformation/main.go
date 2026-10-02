// Package transformation keeps the census of OMG's SysML v1 to v2
// transformation model honest:
// docs/project/sysml-v1-transformation-census-baseline.json records the
// mapping classes extracted from the pinned model with each one's census
// status, docs/project/sysml-v1-transformation-census.md is the table a reader
// consults, and this program is what ties the two to each other and to the
// XMI.
//
// A plain run rewrites the generated blocks of the census document from the
// baseline. -check verifies instead of writing: the baseline is internally
// consistent, every adjudication rule holds (a status is a known marker, a
// non-faithful row states a reason, a backed row cites an implementation that
// resolves to a declared function and a test that resolves to a declared
// TestFunc, a not-implemented or approximate row names measured scope tokens),
// the document's generated blocks are current and its table rows name exactly
// the baseline's mappings, the recorded measurement still counts the same —
// recomputed while the pinned PSSM suite is provisioned, skipped when it is
// absent unless OPENSYSML_REQUIRE_PSSM_SUITE=1 — and, when the pinned XMI is
// provisioned or -require-xmi is set, the baseline still lists what the model
// contains. -update re-extracts the rows from the XMI, keeping every recorded
// verdict, and -measure recomputes the scope counts. Run it with
// `go run -C tools ./cmd/transformation-census`.
package transformation

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// Main is the transformation-census command; it returns the process exit status.
func Main(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("transformation-census", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoDir := flags.String("repo", "", "repository root (default: the module root above the working directory)")
	xmi := flags.String("xmi", "", "pinned transformation model (default: the file scripts/download-sysml-v1tov2.sh provisions under build/)")
	check := flags.Bool("check", false, "verify the baseline, the census document and the model agree, without writing")
	requireXMI := flags.Bool("require-xmi", false, "fail rather than skip the model comparison when the XMI is absent")
	update := flags.Bool("update", false, "re-extract the mapping list from the XMI into the baseline, keeping recorded verdicts")
	measureFlag := flags.Bool("measure", false, "recompute the scope-token counts over the corpora into the baseline")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "transformation-census: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	root, err := repo.Choose(*repoDir)
	if err != nil {
		fmt.Fprintf(stderr, "transformation-census: %v\n", err)
		return 1
	}
	opts := options{xmi: repo.Resolve(root, *xmi), requireXMI: *requireXMI}
	switch {
	case count(*check, *update, *measureFlag) > 1:
		err = fmt.Errorf("-check, -update and -measure are exclusive")
	case *update:
		err = runUpdate(root, opts, stdout)
	case *measureFlag:
		err = runMeasure(root, stdout)
	case *check:
		err = runCheck(root, opts, stdout)
	default:
		err = runWrite(root, opts, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "transformation-census: %v\n", err)
		return 1
	}
	return 0
}

func count(flags ...bool) int {
	n := 0
	for _, f := range flags {
		if f {
			n++
		}
	}
	return n
}

type options struct {
	xmi        string
	requireXMI bool
}

// xmiPath resolves the model to compare against and whether it is present.
func (o options) xmiPath(root string, pin Pin) (string, bool, error) {
	path := o.xmi
	if path == "" {
		path = modelPath(root, pin.File, "")
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) && o.xmi == "" && !o.requireXMI && os.Getenv(RequireEnv) != "1" {
			return path, false, nil
		}
		return path, false, fmt.Errorf("pinned model %s: %w (run ./scripts/download-sysml-v1tov2.sh)", path, err)
	}
	return path, true, nil
}

// runWrite rewrites the generated blocks of the census document from the
// baseline and reports model disagreement without touching the baseline.
func runWrite(root string, opts options, out io.Writer) error {
	base, err := loadBaseline(root)
	if err != nil {
		return err
	}
	if err := base.validate(); err != nil {
		return err
	}
	docPath := filepath.Join(root, filepath.FromSlash(censusDocPath))
	content, err := os.ReadFile(docPath) // #nosec G304 -- fixed repository path
	if err != nil {
		return err
	}
	rewritten, err := rewriteBlocks(string(content), base)
	if err != nil {
		return err
	}
	if rewritten != string(content) {
		err := os.WriteFile(docPath, []byte(rewritten), 0o644) // #nosec G306 G703 -- fixed repository documentation path
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "transformation-census: rewrote the generated blocks of %s\n", censusDocPath)
	} else {
		fmt.Fprintln(out, "transformation-census: generated blocks already current")
	}
	return compareXMI(root, base, opts, out)
}

// runCheck is the gate: every consistency rule, none of the writing.
func runCheck(root string, opts options, out io.Writer) error {
	base, err := loadBaseline(root)
	if err != nil {
		return err
	}
	if err := base.validate(); err != nil {
		return err
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(censusDocPath))) // #nosec G304 -- fixed repository path
	if err != nil {
		return err
	}
	var failures []string
	for _, err := range []error{
		checkDocument(string(content), base),
		checkSource(root, base),
		checkCitesError(root, base),
		checkMeasurement(root, base, out),
		compareXMI(root, base, opts, out),
	} {
		if err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "\n"))
	}
	fmt.Fprintf(out, "transformation-census: %d mapping classes, census document and baseline agree\n", len(base.Mappings))
	return nil
}

func checkCitesError(root string, base *Baseline) error {
	problems := checkCites(newDeclarations(root), base)
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("the citations do not resolve:\n  %s", strings.Join(problems, "\n  "))
}

// checkSource verifies the recorded provenance is the pin's.
func checkSource(root string, base *Baseline) error {
	pin, err := ReadPin(root)
	if err != nil {
		return err
	}
	s := base.Source
	if s.Document != pin.Document || s.Version != pin.Version || s.URL != pin.URL ||
		s.Digest != "sha256:"+pin.SHA256 || s.File != pin.File {
		return fmt.Errorf("%s records source %+v but %s pins %s %s %s %s: re-record with -update",
			baselinePath, s, PinPath, pin.Document, pin.Version, pin.SHA256, pin.URL)
	}
	return nil
}

// runUpdate re-extracts the mapping list and rewrites the baseline around the
// verdicts already recorded; a name new to the baseline starts as unknown.
func runUpdate(root string, opts options, out io.Writer) error {
	pin, err := ReadPin(root)
	if err != nil {
		return err
	}
	opts.requireXMI = true
	xmi, _, err := opts.xmiPath(root, pin)
	if err != nil {
		return err
	}
	if err := verifyPin(pin, xmi); err != nil {
		return err
	}
	fresh, err := extract(xmi)
	if err != nil {
		return err
	}
	previous, err := loadBaseline(root)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	adjudicated := map[string]Mapping{}
	if previous != nil {
		for _, m := range previous.Mappings {
			adjudicated[m.Name] = m
		}
	}
	next := &Baseline{
		Source: Source{
			Document: pin.Document, Version: pin.Version, URL: pin.URL,
			Digest: "sha256:" + pin.SHA256, File: pin.File,
			Packages: fresh.Packages, Classes: fresh.Classes,
			Mappings: len(fresh.Mappings), OCLBodies: fresh.OCLBodies,
			OCLSpecifications: fresh.OCLSpecifications,
			OCLPostconditions: fresh.OCLPostconditions,
			OCLOwnedRules:     fresh.OCLOwnedRules,
		},
	}
	if previous != nil {
		next.Measurement = previous.Measurement
		next.Beyond = previous.Beyond
	}
	for _, e := range fresh.Mappings {
		if old, ok := adjudicated[e.Name]; ok {
			e.Status, e.Implementation, e.Tests, e.Reason, e.Scope =
				old.Status, old.Implementation, old.Tests, old.Reason, old.Scope
		} else {
			e.Status, e.Reason = StatusUnknown, "not yet adjudicated"
			e.Implementation, e.Tests, e.Scope = []string{}, []string{}, []string{}
		}
		if e.Generals == nil {
			e.Generals = []string{}
		}
		if e.Operations == nil {
			e.Operations = []string{}
		}
		next.Mappings = append(next.Mappings, e)
	}
	if next.Measurement.Counts == nil {
		next.Measurement = Measurement{Corpora: []Corpus{}, Counts: map[string]TokenCount{}}
	}
	if next.Beyond == nil {
		next.Beyond = []BeyondEntry{}
	}
	if err := writeBaseline(root, next); err != nil {
		return err
	}
	fmt.Fprintf(out, "transformation-census: recorded %d mapping classes from %s\n", len(next.Mappings), filepath.Base(xmi))
	return runWrite(root, opts, out)
}

// runMeasure recomputes the scope counts and rewrites the baseline and document.
func runMeasure(root string, out io.Writer) error {
	base, err := loadBaseline(root)
	if err != nil {
		return err
	}
	if err := measure(root, base, out); err != nil {
		return err
	}
	if err := writeBaseline(root, base); err != nil {
		return err
	}
	return runWrite(root, options{}, out)
}

// verifyPin refuses a model file whose bytes are not the pinned ones.
func verifyPin(pin Pin, xmi string) error {
	digest, err := Digest(xmi)
	if err != nil {
		return err
	}
	if digest != pin.SHA256 {
		return fmt.Errorf("%s has sha256 %s, not the pinned %s; re-run ./scripts/download-sysml-v1tov2.sh", xmi, digest, pin.SHA256)
	}
	return nil
}

// compareXMI checks the baseline's list and counts against the model when it
// is available, and says so when it is not.
func compareXMI(root string, base *Baseline, opts options, out io.Writer) error {
	pin, err := ReadPin(root)
	if err != nil {
		return err
	}
	xmi, present, err := opts.xmiPath(root, pin)
	if err != nil {
		return err
	}
	if !present {
		fmt.Fprintf(out, "transformation-census: pinned model not provisioned at %s; skipping the model comparison\n", xmi)
		return nil
	}
	if err := verifyPin(pin, xmi); err != nil {
		return err
	}
	fresh, err := extract(xmi)
	if err != nil {
		return err
	}
	s := base.Source
	if fresh.Packages != s.Packages || fresh.Classes != s.Classes ||
		len(fresh.Mappings) != s.Mappings || fresh.OCLBodies != s.OCLBodies ||
		fresh.OCLSpecifications != s.OCLSpecifications ||
		fresh.OCLPostconditions != s.OCLPostconditions || fresh.OCLOwnedRules != s.OCLOwnedRules {
		return fmt.Errorf("%s records %d packages, %d classes, %d mappings, %d/%d OCL bodies/specifications but the model holds %d, %d, %d, %d/%d: re-record with -update",
			baselinePath, s.Packages, s.Classes, s.Mappings, s.OCLBodies, s.OCLSpecifications,
			fresh.Packages, fresh.Classes, len(fresh.Mappings), fresh.OCLBodies, fresh.OCLSpecifications)
	}
	if err := base.matchesExtracted(fresh.Mappings); err != nil {
		return err
	}
	fmt.Fprintf(out, "transformation-census: %s lists the %d mapping classes %s contains\n", baselinePath, len(fresh.Mappings), filepath.Base(xmi))
	return nil
}
