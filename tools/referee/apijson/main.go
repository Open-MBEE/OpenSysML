package apijson

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
	"github.com/Open-MBEE/OpenSysML/tools/referee/diff"
)

const toolName = "pilot-apijson-diff"

// options are the parsed command line.
type options struct {
	repo, launcher, out string
	roots               []string
	timeout             time.Duration
	update, check       bool
	lostAgainst         string
	libraries           string
	log                 io.Writer
}

// Main runs the referee with the command-line arguments and returns the exit code.
func Main(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet(toolName, flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoFlag := flags.String("repo", "", "repository root (default: module root)")
	launcherFlag := flags.String("exporter", "", "pilot JSON exporter launcher (default: <repo>/build/pilot-json-exporter/export-json)")
	outFlag := flags.String("out", "", "output directory (default: <repo>/build/"+toolName+")")
	rootsFlag := flags.String("roots", "", "comma-separated subset of the corpus roots to compare (default: all)")
	timeoutFlag := flags.Duration("timeout", 30*time.Minute, "time allowed for the pilot's export of one root")
	updateFlag := flags.Bool("update", false, "re-record "+committedBaseline+" from this run")
	checkFlag := flags.Bool("check", false, "fail unless this run reproduces "+committedBaseline)
	lostAgainstFlag := flags.String("lost-against", "", "an earlier run's "+toolName+"-matches.json; list the element matches this run lost and gained against it")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	root, err := repo.Choose(*repoFlag)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", toolName, err)
		return 1
	}
	opts := options{repo: root, timeout: *timeoutFlag, update: *updateFlag, check: *checkFlag, lostAgainst: repo.Resolve(root, *lostAgainstFlag), log: os.Stdout}
	opts.launcher = repo.Resolve(root, *launcherFlag)
	if opts.launcher == "" {
		opts.launcher = filepath.Join(root, "build", "pilot-json-exporter", "export-json")
	}
	opts.out = repo.Resolve(root, *outFlag)
	if opts.out == "" {
		opts.out = filepath.Join(root, "build", toolName)
	}
	if *rootsFlag != "" {
		opts.roots = strings.Split(*rootsFlag, ",")
	}
	if opts.libraries, err = diff.LibrariesHandedOver(root); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", toolName, err)
		return 1
	}
	if _, err := os.Stat(opts.launcher); os.IsNotExist(err) {
		message := fmt.Sprintf("pilot JSON exporter is absent at %s; run ./scripts/download-pilot-json-exporter.sh to provision it", opts.launcher)
		if opts.check {
			fmt.Fprintf(stderr, "%s: %s\n", toolName, message)
			return 1
		}
		fmt.Fprintln(opts.log, message)
		return 0
	} else if err != nil {
		fmt.Fprintf(stderr, "%s: inspect exporter: %v\n", toolName, err)
		return 1
	}
	if err := run(opts); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", toolName, err)
		return 1
	}
	return 0
}

func run(opts options) error {
	record, err := provenance(opts.repo, opts.libraries)
	if err != nil {
		return err
	}
	record.Recorded = baseline.Today()
	report := &Report{Provenance: record, Exporter: exporterDescription, Normalizations: normalizations}
	result := newComparison()
	logs := make(map[string]string)
	unkeyed := map[string]map[string]int{"ours": {}, "pilot": {}}
	shared := make(map[string]map[string][]string)
	for _, root := range roots() {
		if len(opts.roots) > 0 && !contains(opts.roots, root.Name) {
			continue
		}
		files, err := diff.CollectFiles(opts.repo, root)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			continue
		}
		dir := filepath.Join(opts.repo, filepath.FromSlash(root.Dir))
		fmt.Fprintf(opts.log, "%s: %d files\n", root.Name, len(files))
		pilot, err := runPilot(opts.launcher, dir, []string{opts.libraries}, files, opts.timeout)
		if err != nil {
			return err
		}
		// A file the pilot failed on has nothing to compare against, so its
		// elements do not become ours-only.
		failed := make(map[string]bool, len(pilot.Failed))
		for _, rel := range pilot.Failed {
			failed[rel] = true
		}
		ours, err := exportOurs(dir, files, failed)
		if err != nil {
			return err
		}
		elements, buckets := result.compareRoot(root.Name, ours.Elements, pilot.Elements)
		report.Roots = append(report.Roots, RootReport{
			Name: root.Name, Dir: root.Dir, Files: len(files),
			OursRefused: ours.Refused, PilotFailed: len(pilot.Failed),
			OursSeparate: ours.Separate, OursUnconvertible: ours.Unconvertible, OursColliding: ours.Colliding, PilotLibraryElements: pilot.Library,
			Elements: elements, Properties: buckets,
			DuplicateElements: Sides{Ours: ours.Duplicates, Pilot: pilot.Duplicates},
			AmbiguousKeys:     Sides{Ours: ours.Ambiguous, Pilot: pilot.Ambiguous},
			UnkeyableElements: Sides{Ours: ours.Unkeyable, Pilot: pilot.Unkeyable},
		})
		report.Totals.Files += len(files)
		report.Totals.OursRefused += ours.Refused
		report.Totals.OursUnconvertible += ours.Unconvertible
		report.Totals.OursColliding += ours.Colliding
		report.Totals.PilotFailed += len(pilot.Failed)
		report.Totals.PilotLibraryElements += pilot.Library
		report.Totals.DuplicateElements.Ours += ours.Duplicates
		report.Totals.DuplicateElements.Pilot += pilot.Duplicates
		report.Totals.AmbiguousKeys.Ours += ours.Ambiguous
		report.Totals.AmbiguousKeys.Pilot += pilot.Ambiguous
		report.Totals.UnkeyableElements.Ours += ours.Unkeyable
		report.Totals.UnkeyableElements.Pilot += pilot.Unkeyable
		logs[root.Name] = pilot.Log
		if len(ours.Shared)+len(pilot.Shared) > 0 {
			shared[root.Name] = map[string][]string{"ours": ours.Shared, "pilot": pilot.Shared}
		}
		for typ, count := range ours.Unkeyed {
			unkeyed["ours"][typ] += count
		}
		for typ, count := range pilot.Unkeyed {
			unkeyed["pilot"][typ] += count
		}
	}
	// A mistyped -repo or -roots would otherwise look like a clean run.
	if report.Totals.Files == 0 {
		return fmt.Errorf("no model files found under %s", opts.repo)
	}
	report.Totals.Elements = result.elements
	report.Totals.Properties = result.buckets
	report.Totals.UnkeyedReferences = result.unkeyedReferences
	report.Totals.ExtensionProperties = result.extensions
	rows := result.scorecard()
	report.setScorecard(rows)
	report.Totals.Rows = len(rows)
	examples := Examples{Examples: result.exampleList(rows), Unmatched: result.unmatchedList(), Unkeyed: unkeyed, Shared: shared}
	fresh, err := writeReports(opts.out, report, examples, logs, opts.log)
	if err != nil {
		return err
	}
	if err := writeMatches(opts, Matches{Roots: result.matches}); err != nil {
		return err
	}
	committed := filepath.Join(opts.repo, filepath.FromSlash(committedBaseline))
	if opts.update {
		if len(opts.roots) > 0 {
			return fmt.Errorf("-update records every root; drop -roots")
		}
		return baseline.Write(committed, fresh, opts.log)
	}
	if opts.check {
		return baseline.Reproduces(committed, fresh)
	}
	return nil
}

func contains(list []string, item string) bool {
	for _, candidate := range list {
		if candidate == item {
			return true
		}
	}
	return false
}
