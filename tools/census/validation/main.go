// Package validation keeps the census of the pilot's named validation
// constraints honest: docs/project/constraint-manifest.json records the names
// extracted from the pinned pilot jar with each one's spec clause,
// implementation, diagnostic code, probe evidence and census status,
// docs/project/validation-constraints.md is the page a reader consults, and
// this program is what ties the two to each other and to the jar.
//
// A plain run restates the derived lines and the census table of the document
// from the manifest. -check verifies instead of writing: the manifest is
// internally consistent, the document's derived lines and table are what the
// manifest renders, every implemented row has a probe under testdata/probes,
// every implemented row cites an implementation location and every cited one
// is a declared function, every listed negative case exists, is attributed by
// its header to the row's constraint and is bucketed by
// tools/referee/reject consistently with the row's status, every case the corpus
// attributes to a constraint is listed on its row,
// the summary figures are current, and — when the pinned jar is provisioned or
// -require-jar is set — the manifest still lists what the jar contains. -update
// re-extracts the list from the jar, keeping every recorded row. Run it with
// `go run -C tools ./cmd/validation-census`.
package validation

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/census/doccounts"
	"github.com/Open-MBEE/OpenSysML/tools/census/manifest"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// Main is the validation-census command; it returns the process exit status.
func Main(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("validation-census", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoDir := flags.String("repo", "", "repository root (default: the module root above the working directory)")
	jar := flags.String("jar", "", "pinned pilot jar (default: the artifact scripts/download-pilot-validator.sh provisions under build/)")
	check := flags.Bool("check", false, "verify the manifest, the census document and the jar agree, without writing")
	requireJar := flags.Bool("require-jar", false, "fail rather than skip the jar comparison when the jar is absent")
	update := flags.Bool("update", false, "re-extract the constraint list from the jar into the manifest, keeping recorded rows")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "validation-census: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	root, err := repo.Choose(*repoDir)
	if err != nil {
		fmt.Fprintf(stderr, "validation-census: %v\n", err)
		return 1
	}
	opts := options{jar: repo.Resolve(root, *jar), requireJar: *requireJar}
	switch {
	case *update && *check:
		err = fmt.Errorf("-update and -check are exclusive")
	case *update:
		err = runUpdate(root, opts, stdout)
	case *check:
		err = runCheck(root, opts, stdout)
	default:
		err = runWrite(root, opts, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "validation-census: %v\n", err)
		return 1
	}
	return 0
}

type options struct {
	jar        string
	requireJar bool
}

// recordRows lists the rows of a manifest re-recorded from an extraction: one
// pilot row per extracted name, keeping the row already recorded under it — a
// spec-sourced row the jar now names becomes that pilot row, keeping its
// clause, text and evidence — then the spec-sourced rows the jar still does
// not name.
func recordRows(previous *manifest.Manifest, extracted []Extracted) []manifest.Constraint {
	rows := make(map[string]manifest.Constraint, len(previous.Constraints))
	for _, c := range previous.Constraints {
		rows[c.Name] = c
	}
	named := make(map[string]bool, len(extracted))
	var out []manifest.Constraint
	for _, e := range extracted {
		named[e.Name] = true
		c, known := rows[e.Name]
		if !known {
			c = manifest.Constraint{Name: e.Name, Kind: "validate", Implementation: "—", Message: "—", NegativeCase: "none",
				Codes: []string{}, Passes: []string{}, Status: manifest.StatusUnknown, Rationale: "new to the pinned jar; not yet assessed"}
		}
		c.Raw, c.Source, c.OCL = e.Raw, e.Source, ""
		out = append(out, c)
	}
	for _, c := range previous.SpecRows() {
		if !named[c.Name] {
			out = append(out, c)
		}
	}
	return out
}

// pinnedJarName is the filename of the artifact the pin designates.
func pinnedJarName(pin baseline.Pin) string {
	return fmt.Sprintf("jupyter-sysml-kernel-%s-all.jar", pin.Artifact)
}

// jarPath resolves the jar to compare against and whether it is present.
func (o options) jarPath(root string, pin baseline.Pin) (string, bool, error) {
	path := o.jar
	if path == "" {
		path = filepath.Join(root, "build", "pilot-validator", "target", "sysml-download", "sysml", pinnedJarName(pin))
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) && o.jar == "" && !o.requireJar {
			return path, false, nil
		}
		return path, false, fmt.Errorf("pinned jar %s: %w (run ./scripts/download-pilot-validator.sh)", path, err)
	}
	return path, true, nil
}

// runWrite restates the derived lines and table of the census document from
// the manifest and reports jar disagreement without touching the manifest.
func runWrite(root string, opts options, out io.Writer) error {
	base, err := manifest.Load(root)
	if err != nil {
		return err
	}
	if err := validateRecord(base); err != nil {
		return err
	}
	docPath := filepath.Join(root, filepath.FromSlash(censusDocPath))
	content, err := os.ReadFile(docPath) // #nosec G304 -- fixed repository path
	if err != nil {
		return err
	}
	rewritten, err := rewriteDerivedLines(string(content), base)
	if err != nil {
		return err
	}
	if rewritten != string(content) {
		err := os.WriteFile(docPath, []byte(rewritten), 0o644) // #nosec G306 G703 -- fixed repository documentation path
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "validation-census: rewrote the derived lines and table of %s\n", censusDocPath)
	} else {
		fmt.Fprintln(out, "validation-census: derived lines and table already current")
	}
	return compareJar(root, base, opts, out)
}

// runCheck is the gate: every consistency rule, none of the writing.
func runCheck(root string, opts options, out io.Writer) error {
	base, err := manifest.Load(root)
	if err != nil {
		return err
	}
	if err := validateRecord(base); err != nil {
		return err
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(censusDocPath))) // #nosec G304 -- fixed repository path
	if err != nil {
		return err
	}
	compliance, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(doccounts.SpecCompliancePath))) // #nosec G304 -- fixed repository path
	if err != nil {
		return err
	}
	var failures []string
	for _, err := range []error{
		checkDocument(root, string(content), base),
		checkCompliance(root, string(compliance), base),
		checkProbes(root, base),
		compareJar(root, base, opts, out),
	} {
		if err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "\n"))
	}
	fmt.Fprintf(out, "validation-census: %d constraints, census document, compliance map and manifest agree\n", len(base.PilotRows()))
	return nil
}

// runUpdate re-extracts the constraint list and rewrites the manifest around the
// rows already recorded; a name new to the manifest starts as an unknown row
// to be filled in. The jar is recorded under its pinned name, wherever -jar read it from.
func runUpdate(root string, opts options, out io.Writer) error {
	pin, err := baseline.ReadPin(root)
	if err != nil {
		return err
	}
	opts.requireJar = true
	jar, _, err := opts.jarPath(root, pin)
	if err != nil {
		return err
	}
	extracted, err := extractFromJar(jar)
	if err != nil {
		return err
	}
	digest, err := baseline.DigestFile(jar)
	if err != nil {
		return err
	}
	previous, err := manifest.Load(root)
	if err != nil {
		return err
	}
	next := *previous
	next.PilotTag, next.PilotCommit, next.PilotArtifact = pin.Tag, pin.Commit, pin.Artifact
	next.Jar = manifest.JarRecord{Name: pinnedJarName(pin), Digest: digest}
	next.Extraction = extractionRecord()
	next.Constraints = recordRows(previous, extracted)
	next.Recorded = recordedDate(previous, &next)
	if err := next.Validate(); err != nil {
		return err
	}
	if err := manifest.Write(root, &next); err != nil {
		return err
	}
	fmt.Fprintf(out, "validation-census: recorded %d constraints from %s\n", len(extracted), filepath.Base(jar))
	return nil
}

// compareJar checks the baseline's list and digest against the jar when it is
// available, and says so when it is not.
func compareJar(root string, base *manifest.Manifest, opts options, out io.Writer) error {
	pin, err := baseline.ReadPin(root)
	if err != nil {
		return err
	}
	if base.PilotTag != pin.Tag || base.PilotCommit != pin.Commit || base.PilotArtifact != pin.Artifact {
		return fmt.Errorf("%s records pilot %s/%s/%s but %s pins %s/%s/%s: re-record with -update",
			manifest.Path, base.PilotTag, base.PilotCommit, base.PilotArtifact, baseline.PinPath, pin.Tag, pin.Commit, pin.Artifact)
	}
	if want := pinnedJarName(pin); base.Jar.Name != want {
		return fmt.Errorf("%s records jar %q but the pin names %q: re-record with -update", manifest.Path, base.Jar.Name, want)
	}
	jar, present, err := opts.jarPath(root, pin)
	if err != nil {
		return err
	}
	if !present {
		fmt.Fprintf(out, "validation-census: pinned jar not provisioned at %s; skipping the jar comparison\n", jar)
		return nil
	}
	digest, err := baseline.DigestFile(jar)
	if err != nil {
		return err
	}
	if digest != base.Jar.Digest {
		return fmt.Errorf("%s digests %s as %s but %s records %s: the pinned artifact changed underneath the baseline", filepath.Base(jar), jar, digest, manifest.Path, base.Jar.Digest)
	}
	extracted, err := extractFromJar(jar)
	if err != nil {
		return err
	}
	if err := matches(base, extracted); err != nil {
		return err
	}
	fmt.Fprintf(out, "validation-census: %s lists the %d constraints %s contains\n", manifest.Path, len(extracted), filepath.Base(jar))
	return nil
}
