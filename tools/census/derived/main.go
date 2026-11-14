package derived

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
	"github.com/Open-MBEE/OpenSysML/tools/referee/diff"
)

// options are the parsed command line.
type options struct {
	repo, launcher, out string
	roots               []string
	timeout             time.Duration
	update, check       bool
	requireExporter     bool
	libraries           string
	log                 io.Writer
}

// Main runs the census with the command-line arguments and returns the exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet(toolName, flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoFlag := flags.String("repo", "", "repository root (default: module root)")
	launcherFlag := flags.String("exporter", "", "pilot JSON exporter launcher (default: <repo>/build/pilot-json-exporter/export-json)")
	outFlag := flags.String("out", "", "output directory (default: <repo>/build/"+toolName+")")
	rootsFlag := flags.String("roots", "", "comma-separated subset of the corpus roots to measure (default: all)")
	timeoutFlag := flags.Duration("timeout", 30*time.Minute, "time allowed for the pilot's export of one root")
	updateFlag := flags.Bool("update", false, "re-record "+CommittedBaseline+" from this run")
	checkFlag := flags.Bool("check", false, "fail unless this run reproduces "+CommittedBaseline+" and no served rule is newly silent where the pilot answers")
	requireFlag := flags.Bool("require-exporter", false, "fail rather than reuse the recorded pilot column when the exporter is absent")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "%s: unexpected argument %q\n", toolName, flags.Arg(0))
		return 2
	}
	root, err := repo.Choose(*repoFlag)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", toolName, err)
		return 1
	}
	opts := options{repo: root, timeout: *timeoutFlag, update: *updateFlag, check: *checkFlag, requireExporter: *requireFlag, log: stdout}
	if opts.update && opts.check {
		fmt.Fprintf(stderr, "%s: -update and -check are exclusive\n", toolName)
		return 2
	}
	opts.launcher = repo.Resolve(root, *launcherFlag)
	if opts.launcher == "" {
		opts.launcher = filepath.Join(root, "build", "pilot-json-exporter", "export-json")
	}
	opts.out = repo.Resolve(root, *outFlag)
	if opts.out == "" {
		opts.out = filepath.Join(root, "build", toolName)
	}
	if *rootsFlag != "" {
		if opts.check {
			fmt.Fprintf(stderr, "%s: -check compares every root; drop -roots\n", toolName)
			return 2
		}
		opts.roots = strings.Split(*rootsFlag, ",")
		for _, name := range opts.roots {
			if !slices.ContainsFunc(roots(), func(r diff.Root) bool { return r.Name == name }) {
				fmt.Fprintf(stderr, "%s: unknown root %q; the roots are %s\n", toolName, name, strings.Join(rootNames(), ", "))
				return 2
			}
		}
	}
	if opts.libraries, err = diff.LibrariesHandedOver(root); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", toolName, err)
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
	committed := filepath.Join(opts.repo, filepath.FromSlash(CommittedBaseline))
	exporter, err := exporterPresent(opts)
	if err != nil {
		return err
	}
	var recorded *Report
	if exporter {
		if err := underDeclaredJVM(opts, record.PilotJava); err != nil {
			return err
		}
	}
	if !exporter {
		if recorded, err = readCommitted(committed); err != nil && !os.IsNotExist(err) {
			return err
		}
		if opts.update {
			if err := reusable(recorded, record.PilotJava); err != nil {
				return err
			}
		}
		fmt.Fprintf(opts.log, "pilot JSON exporter is absent at %s; the pilot column is the one %s records (run ./scripts/download-pilot-json-exporter.sh to re-export it)\n", opts.launcher, CommittedBaseline)
	}
	classes := newMetaclasses()
	report := &Report{Provenance: record}
	for _, root := range roots() {
		if len(opts.roots) > 0 && !slices.Contains(opts.roots, root.Name) {
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
		ours, err := censusRoot(dir, files)
		if err != nil {
			return fmt.Errorf("%s: %w", root.Name, err)
		}
		entry := RootReport{Name: root.Name, Dir: root.Dir, Files: len(files), Rejected: len(ours.Rejected),
			Elements: ours.Elements, Implied: ours.Implied, Rules: map[string]RuleCounts{}}
		for name, counts := range ours.Rules {
			entry.Rules[name] = RuleCounts{Ours: counts}
		}
		if exporter {
			pilot, err := runExporter(opts.launcher, dir, opts.libraries, files, opts.timeout, classes)
			if err != nil {
				return err
			}
			entry.PilotElements, entry.PilotImplied, entry.PilotDuplicates, entry.PilotFailed = pilot.Elements, pilot.Implied, pilot.Duplicates, len(pilot.Failed)
			for name, counts := range pilot.Rules {
				row := entry.Rules[name]
				row.Pilot = counts
				entry.Rules[name] = row
			}
		} else if previous, ok := recorded.root(root.Name); ok {
			entry.PilotElements, entry.PilotImplied, entry.PilotDuplicates, entry.PilotFailed = previous.PilotElements, previous.PilotImplied, previous.PilotDuplicates, previous.PilotFailed
			for name, counts := range previous.Rules {
				row := entry.Rules[name]
				row.Pilot = counts.Pilot
				entry.Rules[name] = row
			}
		}
		report.Roots = append(report.Roots, entry)
	}
	report.finish()
	if _, err := writeReports(opts.out, report, opts.log); err != nil {
		return err
	}
	if opts.update {
		if len(opts.roots) > 0 {
			return fmt.Errorf("-update records every root; drop -roots")
		}
		head, err := committedHeader(report)
		if err != nil {
			return err
		}
		if err := baseline.Write(committed, head, opts.log); err != nil {
			return err
		}
		if err := os.WriteFile(recordsPath(committed), encodeRecords(report), 0o600); err != nil {
			return err
		}
		fmt.Fprintf(opts.log, "recorded %s\n", recordsPath(committed))
		return nil
	}
	if opts.check {
		if err := baseline.CheckCommitted(committed, refreshCommand, record); err != nil {
			return err
		}
		if recorded == nil {
			if recorded, err = readCommitted(committed); err != nil {
				return err
			}
		}
		if newly := newlySilent(recorded, report); len(newly) > 0 {
			return fmt.Errorf("served rules answer nothing on every element where the pilot answers, and %s does not record them as silent:\n  %s",
				CommittedBaseline, strings.Join(newly, "\n  "))
		}
		head, err := committedHeader(report)
		if err != nil {
			return err
		}
		if err := baseline.Reproduces(committed, head); err != nil {
			return err
		}
		return recordsReproduce(recordsPath(committed), encodeRecords(report))
	}
	return nil
}

// underDeclaredJVM holds the export to the declared JVM: a record is written under no
// other, and a run that only compares says which JVM the pilot's column came from.
func underDeclaredJVM(opts options, declared string) error {
	jvm, err := exporterJVM(opts.launcher)
	if err != nil {
		return err
	}
	if jvm == declared {
		return nil
	}
	if opts.update {
		return fmt.Errorf("the pilot exporter runs under Java %s and %s declares Java %s for the record: put a Java %s first on PATH and re-run",
			jvm, baseline.PinPath, declared, declared)
	}
	fmt.Fprintf(opts.log, "the pilot exporter runs under Java %s, not the Java %s %s declares; a difference in the pilot's column may be the JVM's\n",
		jvm, declared, baseline.PinPath)
	return nil
}

// reusable refuses to re-record a pilot column that was recorded under an undeclared JVM.
func reusable(recorded *Report, declared string) error {
	if recorded == nil || recorded.Provenance.PilotJava == declared {
		return nil
	}
	return fmt.Errorf("%s holds a pilot column recorded under Java %q and %s declares Java %s: provision the exporter and re-record under that Java",
		CommittedBaseline, recorded.Provenance.PilotJava, baseline.PinPath, declared)
}

// exporterPresent reports whether the pilot's exporter is provisioned; its
// absence is an error only when the run requires it.
func exporterPresent(opts options) (bool, error) {
	_, err := os.Stat(opts.launcher)
	switch {
	case err == nil:
		return true, nil
	case os.IsNotExist(err) && opts.requireExporter:
		return false, fmt.Errorf("pilot JSON exporter is absent at %s; run ./scripts/download-pilot-json-exporter.sh to provision it", opts.launcher)
	case os.IsNotExist(err):
		return false, nil
	}
	return false, fmt.Errorf("inspect exporter: %w", err)
}

// readCommitted reads the recorded census.
func readCommitted(path string) (*Report, error) {
	content, err := os.ReadFile(path) // #nosec G304 -- the committed baseline in this repository
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var report Report
	if err := json.Unmarshal(content, &report); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	records, err := os.ReadFile(recordsPath(path)) // #nosec G304 -- the committed records beside the baseline
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", recordsPath(path), err)
	}
	if err := decodeRecords(records, &report); err != nil {
		return nil, err
	}
	report.finish()
	return &report, nil
}

// recordsPath names the records file beside a committed baseline header.
func recordsPath(committed string) string {
	return filepath.Join(filepath.Dir(committed), filepath.Base(filepath.FromSlash(CommittedRecords)))
}

// committedHeader encodes the header the committed baseline keeps.
func committedHeader(report *Report) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(header(report)); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// recordsReproduce reports the records that differ between the committed file and a fresh run.
func recordsReproduce(path string, fresh []byte) error {
	committed, err := os.ReadFile(path) // #nosec G304 -- the committed records in this repository
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if bytes.Equal(committed, fresh) {
		return nil
	}
	was, now := recordLines(committed), recordLines(fresh)
	var moved []string
	for key, line := range now {
		if was[key] != line {
			moved = append(moved, "  "+line)
		}
	}
	for key, line := range was {
		if _, ok := now[key]; !ok {
			moved = append(moved, "  gone: "+line)
		}
	}
	sort.Strings(moved)
	return fmt.Errorf("%s does not reproduce; records moved:\n%s\nadjudicate the movement, then re-record with %s", path, strings.Join(moved, "\n"), refreshCommand)
}

func recordLines(data []byte) map[string]string {
	lines := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue
		}
		lines[fields[0]+"\t"+fields[1]] = line
	}
	return lines
}

func (r *Report) root(name string) (RootReport, bool) {
	if r == nil {
		return RootReport{}, false
	}
	for _, root := range r.Roots {
		if root.Name == name {
			return root, true
		}
	}
	return RootReport{}, false
}

// newlySilent lists the rules silent in fresh that recorded does not list.
func newlySilent(recorded, fresh *Report) []string {
	var out []string
	for _, name := range fresh.Silent {
		if !slices.Contains(recorded.Silent, name) {
			out = append(out, name)
		}
	}
	return out
}

// writeReports writes the JSON report and a text summary under out and
// returns the JSON bytes.
func writeReports(out string, report *Report, log io.Writer) ([]byte, error) {
	if err := os.MkdirAll(out, 0o750); err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return nil, err
	}
	jsonPath := filepath.Join(out, toolName+".json")
	if err := os.WriteFile(jsonPath, buffer.Bytes(), 0o600); err != nil {
		return nil, err
	}
	text := summarize(report)
	textPath := filepath.Join(out, toolName+".txt")
	if err := os.WriteFile(textPath, []byte(text), 0o600); err != nil {
		return nil, err
	}
	fmt.Fprint(log, text)
	fmt.Fprintf(log, "wrote %s and %s\n", jsonPath, textPath)
	return buffer.Bytes(), nil
}

// summarize renders the report for a reader: the roots, the classes and the
// silent rules with their counts.
func summarize(report *Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "structure: %s\n", report.Structure)
	for _, root := range report.Roots {
		fmt.Fprintf(&b, "%s: files %d (rejected %d), elements %d (implied %d); pilot elements %d (implied %d, copies dropped %d, files failed %d)\n",
			root.Name, root.Files, root.Rejected, root.Elements, root.Implied, root.PilotElements, root.PilotImplied, root.PilotDuplicates, root.PilotFailed)
	}
	s := report.Summary
	fmt.Fprintf(&b, "served rules %d: answering %d, empty everywhere %d, not derived %d, carried by no element %d; the pilot answers %d\n",
		s.Served, s.Answering, s.Empty, s.NotDerived, s.Uncarried, s.PilotAnswering)
	fmt.Fprintf(&b, "silent (ours never a value, the pilot a value somewhere): %d\n", s.Silent)
	for _, name := range report.Silent {
		row := report.Totals[name]
		fmt.Fprintf(&b, "  %s: ours empty %d, not derived %d of %d; pilot values %d of %d\n",
			name, row.Ours.Empty, row.Ours.NotDerived, row.Ours.Carriers, row.Pilot.Value, row.Pilot.Carriers)
	}
	return b.String()
}
