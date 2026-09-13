package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/repl"
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

func main() {
	repoFlag := flag.String("repo", "", "repository root (default: module root)")
	casesFlag := flag.String("cases", "", "directory containing .cases files")
	outFlag := flag.String("out", "", "output directory (default: <repo>/build/pilot-exec-diff)")
	launcherFlag := flag.String("launcher", "", "pilot evaluator launcher (default: <repo>/build/pilot-evaluator/eval-sysml)")
	flag.Parse()

	repo, err := chooseRepo(*repoFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pilot-exec-diff: %v\n", err)
		os.Exit(1)
	}
	launcher := *launcherFlag
	if launcher == "" {
		launcher = filepath.Join(repo, "build", "pilot-evaluator", "eval-sysml")
	}
	if _, err := os.Stat(launcher); os.IsNotExist(err) {
		fmt.Println(artifactAbsentMessage(launcher))
		return
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "pilot-exec-diff: inspect launcher: %v\n", err)
		os.Exit(1)
	}

	casesDir := *casesFlag
	if casesDir == "" {
		casesDir = filepath.Join(repo, "cmd", "pilot-exec-diff", "testdata", "cases")
	}
	caseFiles, err := readCaseFiles(casesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pilot-exec-diff: %v\n", err)
		os.Exit(1)
	}
	out := *outFlag
	if out == "" {
		out = filepath.Join(repo, "build", "pilot-exec-diff")
	}
	report, err := execute(repo, launcher, caseFiles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pilot-exec-diff: %v\n", err)
		os.Exit(1)
	}
	if err := writeReport(out, report); err != nil {
		fmt.Fprintf(os.Stderr, "pilot-exec-diff: %v\n", err)
		os.Exit(1)
	}
	printSummary(report)
}

func artifactAbsentMessage(launcher string) string {
	return fmt.Sprintf("pilot execution artifact is absent at %s; run ./scripts/download-pilot-evaluator.sh to provision it", launcher)
}

func chooseRepo(path string) (string, error) {
	if path != "" {
		return filepath.Abs(path)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found above the working directory; pass -repo")
		}
		dir = parent
	}
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
	tsv, err := os.CreateTemp("", "pilot-exec-diff-*.tsv")
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
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pilot-exec-diff.json"), append(encoded, '\n'), 0o600); err != nil {
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
	if err := os.WriteFile(filepath.Join(dir, "pilot-exec-diff.txt"), []byte(text.String()), 0o600); err != nil {
		return fmt.Errorf("write text report: %w", err)
	}
	return nil
}

func printSummary(report *execReport) {
	fmt.Println("pilot-exec-diff: expressions only")
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
