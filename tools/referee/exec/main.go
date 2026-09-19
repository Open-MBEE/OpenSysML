// Package exec is the pilot execution referee: it evaluates the shipped
// expression cases with both this implementation and the OMG SysML v2 Pilot
// Implementation's headless evaluator, and buckets every case by agreement.
package exec

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
	reports "github.com/Open-MBEE/OpenSysML/tools/oracle/report"
)

// bucketKindOnly is the verdict for values that agree once their kinds are
// reconciled.
const bucketKindOnly = "kind-only"

var bucketNames = []string{
	"agree",
	bucketKindOnly,
	"order-only",
	"disagree",
	"pilot-unevaluated",
	"pilot-silent",
	"pilot-error",
	"ours-error",
	"ours-undetermined",
	"both-error",
	"nondeterministic",
}

type caseReport struct {
	ID            string     `json:"id"`
	Models        []string   `json:"models"`
	Target        string     `json:"target"`
	Expression    string     `json:"expression"`
	RawPilot      string     `json:"rawPilot"`
	RawOurs       string     `json:"rawOurs"`
	Pilot         normalized `json:"pilotNormalized"`
	Ours          normalized `json:"oursNormalized"`
	Bucket        string     `json:"bucket"`
	RealPrecision string     `json:"realPrecision"`
}

type execReport struct {
	PilotArtifact string         `json:"pilotArtifact"`
	Scope         string         `json:"scope"`
	Note          string         `json:"note"`
	Cases         []caseReport   `json:"cases"`
	Buckets       map[string]int `json:"buckets"`
}

// defaultCases is the shipped case corpus, relative to the repository root.
const defaultCases = "tools/referee/exec/testdata/cases"

// toolName is the command's name in its flags, messages and output directory.
const toolName = "pilot-exec-diff"

// Main runs the pilot-exec-diff command over args and returns its exit status.
func Main(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet(toolName, flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoFlag := flags.String("repo", "", "repository root (default: module root)")
	casesFlag := flags.String("cases", "", "directory containing .cases files")
	outFlag := flags.String("out", "", "output directory (default: <repo>/build/pilot-exec-diff)")
	launcherFlag := flags.String("launcher", "", "pilot evaluator launcher (default: <repo>/build/pilot-evaluator/eval-sysml)")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	root, err := repo.Choose(*repoFlag)
	if err != nil {
		fmt.Fprintf(stderr, toolName+": %v\n", err)
		return 1
	}
	launcher := repo.Resolve(root, *launcherFlag)
	if launcher == "" {
		launcher = filepath.Join(root, "build", "pilot-evaluator", "eval-sysml")
	}
	if _, err := os.Stat(launcher); os.IsNotExist(err) {
		fmt.Println(artifactAbsentMessage(launcher))
		return 0
	} else if err != nil {
		fmt.Fprintf(stderr, toolName+": inspect launcher: %v\n", err)
		return 1
	}

	casesDir := repo.Resolve(root, *casesFlag)
	if casesDir == "" {
		casesDir = filepath.Join(root, filepath.FromSlash(defaultCases))
	}
	caseFiles, err := readCaseFiles(casesDir)
	if err != nil {
		fmt.Fprintf(stderr, toolName+": %v\n", err)
		return 1
	}
	out := repo.Resolve(root, *outFlag)
	if out == "" {
		out = filepath.Join(root, "build", toolName)
	}
	report, err := execute(root, launcher, caseFiles)
	if err != nil {
		fmt.Fprintf(stderr, toolName+": %v\n", err)
		return 1
	}
	if err := writeReport(out, report); err != nil {
		fmt.Fprintf(stderr, toolName+": %v\n", err)
		return 1
	}
	printSummary(report)
	return 0
}

func artifactAbsentMessage(launcher string) string {
	return fmt.Sprintf("pilot execution artifact is absent at %s; run ./scripts/download-pilot-evaluator.sh to provision it", launcher)
}

func execute(repo, launcher string, files []execCaseFile) (*execReport, error) {
	report := &execReport{
		PilotArtifact: filepath.ToSlash(relativeTo(repo, launcher)),
		Scope:         "expressions only",
		Note:          "Action, state-machine, and exhibit/perform execution are OUT OF REACH of the pinned artifact and are not compared here. Single-element sequence renderings are unwrapped on both sides; scalar-vs-singleton distinction is unobservable in this report. The pilot cannot distinguish \"no value\" from \"declined to evaluate\" when it emits no output.",
		Buckets:       make(map[string]int, len(bucketNames)),
	}
	for _, name := range bucketNames {
		report.Buckets[name] = 0
	}
	for _, file := range files {
		models, err := resolveModels(repo, file.Path, file.Models)
		if err != nil {
			return nil, err
		}
		pilotOne, err := runPilot(launcher, models, file.Cases)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file.Path, err)
		}
		pilotTwo, err := runPilot(launcher, models, file.Cases)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file.Path, err)
		}
		oursOne := runOurs(models, file.Cases)
		oursTwo := runOurs(models, file.Cases)
		for _, testCase := range file.Cases {
			pilotRaw := pilotOne[testCase.ID]
			oursRaw := oursOne[testCase.ID]
			pilot := normalizePilot(pilotRaw)
			ours := normalizeOurs(oursRaw.Raw, oursRaw.Error)
			bucket := bucketResults(pilot, ours)
			if canonicalPilot(pilotOne[testCase.ID]) != canonicalPilot(pilotTwo[testCase.ID]) ||
				oursOne[testCase.ID].Raw != oursTwo[testCase.ID].Raw ||
				oursOne[testCase.ID].Error != oursTwo[testCase.ID].Error {
				bucket = "nondeterministic"
			}
			report.Cases = append(report.Cases, caseReport{
				ID: testCase.ID, Models: modelPaths(file.Models),
				Target: testCase.Target, Expression: testCase.Expression,
				RawPilot: pilotRaw, RawOurs: oursRaw.Raw,
				Pilot: pilot.Value, Ours: ours.Value,
				Bucket: bucket, RealPrecision: "real values compared after rounding both sides to 2 decimal places",
			})
			report.Buckets[bucket]++
		}
	}
	return report, nil
}

func resolveModels(repo, casePath string, models []execModel) ([]string, error) {
	paths := make([]string, len(models))
	for i, model := range models {
		path := filepath.Join(repo, filepath.FromSlash(model.Path))
		if info, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("%s:%d: model %s: %w", casePath, model.Line, model.Path, err)
		} else if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s:%d: model %s is not a regular file", casePath, model.Line, model.Path)
		}
		paths[i] = path
	}
	return paths, nil
}

func modelPaths(models []execModel) []string {
	paths := make([]string, len(models))
	for i, model := range models {
		paths[i] = model.Path
	}
	return paths
}

func runPilot(launcher string, models []string, cases []execCase) (map[string]string, error) {
	tsv, err := os.CreateTemp("", toolName+"-*.tsv")
	if err != nil {
		return nil, fmt.Errorf("create pilot cases: %w", err)
	}
	path := tsv.Name()
	defer os.Remove(path)
	for _, testCase := range cases {
		if _, err := fmt.Fprintf(tsv, "%s\t%s\t%s\n", testCase.ID, testCase.Target, testCase.Expression); err != nil {
			_ = tsv.Close()
			return nil, fmt.Errorf("write pilot cases: %w", err)
		}
	}
	if err := tsv.Close(); err != nil {
		return nil, fmt.Errorf("close pilot cases: %w", err)
	}
	args := []string{"--cases", path}
	for _, model := range models {
		args = append(args, "--model", model)
	}
	// #nosec G204 -- launcher is an explicit CLI override or repo-local artifact.
	command := exec.Command(launcher, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		result := make(map[string]string, len(cases))
		for _, testCase := range cases {
			result[testCase.ID] = "ERROR:launcher: " + message
		}
		return result, nil
	}
	result, err := parsePilotCases(stdout.String(), cases)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func parsePilotCases(output string, cases []execCase) (map[string]string, error) {
	wanted := make(map[string]bool, len(cases))
	for _, testCase := range cases {
		wanted[testCase.ID] = true
	}
	results := make(map[string]string, len(cases))
	var current string
	var lines []string
	flush := func() {
		if current != "" {
			results[current] = strings.Join(lines, "\n")
		}
	}
	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, "== case "):
			flush()
			current = strings.TrimPrefix(line, "== case ")
			lines = nil
		case strings.HasPrefix(line, "== end "):
			flush()
			current, lines = "", nil
		case current != "":
			lines = append(lines, line)
		}
	}
	flush()
	for id := range wanted {
		if _, ok := results[id]; !ok {
			return nil, fmt.Errorf("pilot output did not contain case %s", id)
		}
	}
	return results, nil
}

func runOurs(models []string, cases []execCase) map[string]sideResult {
	results := make(map[string]sideResult, len(cases))
	session := repl.NewSession()
	report, err := session.LoadPathsReport(models)
	if err != nil {
		for _, testCase := range cases {
			results[testCase.ID] = sideResult{Raw: "sysml: " + err.Error(), Error: true}
		}
		return results
	}
	if report.Errors {
		raw := "sysml: model did not analyse cleanly"
		if len(report.Found) > 0 {
			limit := min(5, len(report.Found))
			raw += "\n" + strings.Join(report.Found[:limit], "\n")
		}
		for _, testCase := range cases {
			results[testCase.ID] = sideResult{Raw: raw, Error: true}
		}
		return results
	}
	for _, testCase := range cases {
		var (
			lines []string
			err   error
		)
		if testCase.Target != "" {
			lines, _, err = session.RunMeta("%eval in " + testCase.Target + " : " + testCase.Expression)
		} else {
			lines, err = session.EvalExpr(testCase.Expression)
		}
		if err != nil {
			results[testCase.ID] = sideResult{Raw: "sysml: " + err.Error(), Error: true}
			continue
		}
		results[testCase.ID] = sideResult{Raw: strings.Join(lines, "\n")}
	}
	return results
}

func canonicalPilot(raw string) string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		lines[i] = pilotUUID.ReplaceAllString(line, "")
	}
	return strings.Join(lines, "\n")
}

func writeReport(dir string, report *execReport) error {
	files, err := reports.Open(dir, toolName)
	if err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	if _, err := files.JSON(report); err != nil {
		return fmt.Errorf("write JSON report: %w", err)
	}
	var text strings.Builder
	fmt.Fprintf(&text, "pilot execution referee (%s)\n\n", report.Scope)
	fmt.Fprintf(&text, "%s\n\n", report.Note)
	for _, result := range report.Cases {
		fmt.Fprintf(&text, "%s: %s [%s]\n", result.ID, result.Bucket, strings.Join(result.Models, ", "))
		fmt.Fprintf(&text, "  target: %s\n  expression: %s\n", result.Target, result.Expression)
		fmt.Fprintf(&text, "  raw pilot:\n%s\n  raw ours:\n%s\n", result.RawPilot, result.RawOurs)
		fmt.Fprintf(&text, "  pilot normalized: %s\n  ours normalized: %s\n", normalizedText(result.Pilot), normalizedText(result.Ours))
		fmt.Fprintf(&text, "  real comparison: %s\n\n", result.RealPrecision)
	}
	fmt.Fprintln(&text, "bucket counts:")
	for _, name := range bucketNames {
		fmt.Fprintf(&text, "  %s: %d\n", name, report.Buckets[name])
	}
	if err := files.Text(text.String()); err != nil {
		return fmt.Errorf("write text report: %w", err)
	}
	return nil
}

func printSummary(report *execReport) {
	fmt.Println(toolName + ": expressions only")
	fmt.Println(report.Note)
	for _, name := range bucketNames {
		fmt.Printf("%s: %d\n", name, report.Buckets[name])
	}
	fmt.Printf("%d case(s)\n", len(report.Cases))
}

func relativeTo(repo, path string) string {
	rel, err := filepath.Rel(repo, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return filepath.ToSlash(rel)
}
