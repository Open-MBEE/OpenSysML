package qvts_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

var update = flag.Bool("update", false, "update the QVTS example recordings")

type recording struct {
	name    string
	args    []string
	command string
	stdin   string
	status  int
	view    func(string) (string, error)
}

func TestExampleRecordings(t *testing.T) {
	root := repoRoot(t)
	binary := buildCLI(t, root)
	models := []string{
		"examples/qvts/lander.sysml",
		"examples/qvts/queries.sysml",
	}
	duplicateModels := append(append([]string(nil), models[:1]...), "examples/qvts/duplicates.sysml", models[1])
	query := func(name string, jsonOutput bool) []string {
		args := append([]string(nil), models...)
		if name == "AmbiguousPart" || name == "DoubleMassValue" {
			args = duplicateModels
		}
		args = append(args, "-run-query", "QVTSDemo::"+name)
		if jsonOutput {
			args = append(args, "-json")
		}
		return args
	}
	runs := []recording{
		{name: "cabin-mass", args: query("CabinMass", false), status: 0},
		{
			name:    "repl-cabin-mass",
			args:    append([]string(nil), models...),
			command: "printf '%s\\n' '%run-query QVTSDemo::CabinMass' | bin/sysml examples/qvts/lander.sysml examples/qvts/queries.sysml",
			stdin:   "%run-query QVTSDemo::CabinMass\n",
			status:  0,
		},
		{name: "empty-selection", args: query("EmptySelection", false), status: 0},
		{name: "misspelled-name", args: query("MisspelledName", false), status: 2},
		{name: "empty-package-member", args: query("EmptyPackageMember", false), status: 2},
		{name: "missing-mass-project", args: query("MissingMassProject", false), status: 2},
		{name: "missing-mass-filter", args: query("MissingMassFilter", false), status: 2},
		{name: "unvalued-mass", args: query("UnvaluedMass", false), status: 0},
		{name: "ambiguous-part", args: query("AmbiguousPart", false), status: 2},
		{name: "double-mass-value", args: query("DoubleMassValue", false), status: 0},
		{name: "tolerated-mass-project", args: query("ToleratedMassProject", false), status: 0},
		{name: "tolerated-mass-filter", args: query("ToleratedMassFilter", false), status: 0},
		{name: "mass-table", args: query("MassTable", false), status: 0},
		{name: "ordered-masses", args: query("OrderedMasses", false), status: 0},
		{name: "mass-table-json", args: query("MassTable", true), status: 0},
		{name: "document-table", args: append(append([]string(nil), models...), "-render-document", "QVTSDemo::MassReport"), status: 0},
		{name: "exact-part-usages", args: query("ExactPartUsages", false), status: 0},
		{name: "metamodel-properties", args: query("MetamodelProperties", false), status: 0},
		{name: "changed-masses", args: query("ChangedMasses", false), status: 0},
		{name: "misspelled-differ-key", args: query("MisspelledDifferKey", false), status: 2},
		{name: "failing-query-json", args: query("MissingMassProject", true), status: 2},
		{name: "model-error-json", args: query("AmbiguousPart", true), status: 2},
		{name: "ttl-cabin-excerpt", args: []string{"examples/qvts/lander.sysml", "-convert", "ttl"}, status: 0, view: rdfCabinExcerpt},
		{name: "metamodel-partusage", args: []string{"-metamodel-table"}, status: 0, view: metamodelPartUsage},
	}
	for i := range runs {
		if runs[i].command == "" {
			runs[i].command = "bin/sysml " + strings.Join(runs[i].args, " ")
		}
	}

	readme, err := os.ReadFile(filepath.Join(root, "examples", "qvts", "README.md"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err != nil && !*update {
		t.Fatal(err)
	}
	var readmeFences []markdownFence
	if err == nil && !*update {
		readmeFences = markdownFenceBlocks(string(readme))
	}
	for _, run := range runs {
		t.Run(run.name, func(t *testing.T) {
			cmd := exec.Command(binary, run.args...)
			cmd.Dir = root
			if run.stdin != "" {
				cmd.Stdin = strings.NewReader(run.stdin)
			}
			output, runErr := cmd.CombinedOutput()
			status := 0
			if runErr != nil {
				exitErr, ok := runErr.(*exec.ExitError)
				if !ok {
					t.Fatalf("run %v: %v\n%s", run.args, runErr, output)
				}
				status = exitErr.ExitCode()
			}
			if status != run.status {
				t.Fatalf("exit = %d, want %d\n%s", status, run.status, output)
			}
			recorded := string(output)
			if run.view != nil {
				recorded, err = run.view(recorded)
				if err != nil {
					t.Fatalf("extract output excerpt: %v\n%s", err, output)
				}
			}
			extension := ".txt"
			if strings.HasSuffix(run.name, "-json") {
				extension = ".json"
			}
			path := filepath.Join(root, "examples", "qvts", "expected", run.name+extension)
			if *update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(recorded), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal([]byte(recorded), want) {
				t.Errorf("recording differs from %s\n--- got ---\n%s\n--- want ---\n%s", path, recorded, want)
			}
			if !commandHasRecording(string(readme), readmeFences, run.command, string(want), run.status) {
				t.Errorf("%s command, recording, and exit status are not in the expected order in README.md", run.name)
			}
		})
	}
	if !*update {
		guide, err := os.ReadFile(filepath.Join(root, "docs", "guide", "qvts.md"))
		if err != nil {
			t.Fatal(err)
		}
		expected := make(map[string]string, len(runs))
		for _, run := range runs {
			extension := ".txt"
			if strings.HasSuffix(run.name, "-json") {
				extension = ".json"
			}
			path := filepath.Join(root, "examples", "qvts", "expected", run.name+extension)
			recorded, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			expected[run.name] = string(recorded)
		}
		if err := guideOutputsMatchRecordings(markdownFenceBlocks(string(guide)), runs, expected); err != nil {
			t.Error(err)
		}
	}
}

func buildCLI(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sysml")
	cmd := exec.Command("go", gobuild.Args(path)...)
	cmd.Dir = filepath.Join(root, "cmd", "sysml")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build sysml: %v\n%s", err, output)
	}
	return path
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller did not locate this test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

type markdownFence struct {
	info    string
	body    string
	endLine int
}

func markdownFenceBlocks(markdown string) []markdownFence {
	var blocks []markdownFence
	lines := strings.Split(markdown, "\n")
	for i := 0; i < len(lines); i++ {
		opening := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(opening, "```") {
			continue
		}
		info := strings.TrimSpace(strings.TrimPrefix(opening, "```"))
		start := i + 1
		for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
		}
		if i < len(lines) {
			blocks = append(blocks, markdownFence{
				info:    info,
				body:    strings.Join(lines[start:i], "\n") + "\n",
				endLine: i,
			})
		}
	}
	return blocks
}

func commandHasRecording(markdown string, blocks []markdownFence, command, recording string, status int) bool {
	lines := strings.Split(markdown, "\n")
	for i, block := range blocks {
		if block.info != "console" || strings.TrimSpace(block.body) != "$ "+command || i+1 >= len(blocks) {
			continue
		}
		output := blocks[i+1]
		if (output.info != "text" && output.info != "json") || output.body != recording {
			continue
		}
		line := output.endLine + 1
		for line < len(lines) && strings.TrimSpace(lines[line]) == "" {
			line++
		}
		if line < len(lines) && lines[line] == fmt.Sprintf("Exit status: %d", status) {
			return true
		}
	}
	return false
}

func guideOutputsMatchRecordings(blocks []markdownFence, runs []recording, expected map[string]string) error {
	seen := make(map[string]int)
	outputBlocks := 0
	for i, block := range blocks {
		if block.info != "text" && block.info != "json" {
			continue
		}
		outputBlocks++
		if i == 0 || blocks[i-1].info != "console" {
			return fmt.Errorf("guide output block %d is not preceded by a console command", outputBlocks)
		}
		command := strings.TrimSpace(blocks[i-1].body)
		matched := ""
		for _, run := range runs {
			if command == "$ "+run.command && expected[run.name] == block.body {
				if block.info == "json" && !strings.HasSuffix(run.name, "-json") {
					continue
				}
				if block.info == "text" && strings.HasSuffix(run.name, "-json") {
					continue
				}
				matched = run.name
				break
			}
		}
		if matched == "" {
			return fmt.Errorf("guide output block %d does not match a recording and its command", outputBlocks)
		}
		seen[matched]++
	}
	for _, name := range []string{"cabin-mass", "failing-query-json", "repl-cabin-mass"} {
		if seen[name] != 1 {
			return fmt.Errorf("guide must contain exactly one command/output pair for %s; found %d", name, seen[name])
		}
	}
	return nil
}

func rdfCabinExcerpt(output string) (string, error) {
	lines := strings.Split(output, "\n")
	var subject string
	var triples []string
	for i, line := range lines {
		if !strings.Contains(line, `sysml:qualifiedName "Lander::lander::cabin"`) {
			continue
		}
		for j := i - 1; j >= 0 && !strings.HasPrefix(strings.TrimSpace(lines[j]), "#"); j-- {
			if strings.HasPrefix(strings.TrimSpace(lines[j]), "elmt:") {
				subject = lines[j]
				break
			}
		}
		if subject == "" {
			return "", fmt.Errorf("could not find the subject for the cabin qualified-name triple")
		}
		for j := i - 1; j < len(lines) && !strings.HasSuffix(strings.TrimSpace(lines[j]), "."); j++ {
			line := lines[j]
			if strings.Contains(line, "a sysml:PartUsage ;") ||
				strings.Contains(line, `sysml:qualifiedName "Lander::lander::cabin"`) ||
				strings.Contains(line, "sysml:elementId ") {
				triples = append(triples, line)
			}
		}
		break
	}
	if subject == "" || len(triples) != 3 {
		return "", fmt.Errorf("found subject %q and %d of 3 requested cabin triples", subject, len(triples))
	}
	return strings.Join(append([]string{subject}, triples...), "\n") + "\n", nil
}

func metamodelPartUsage(output string) (string, error) {
	start := strings.Index(output, `    {
      "name": "PartUsage"`)
	if start < 0 {
		return "", fmt.Errorf("PartUsage class is absent from metamodel output")
	}
	end := strings.Index(output[start:], "\n    },")
	if end < 0 {
		return "", fmt.Errorf("could not find the end of the PartUsage class")
	}
	return output[start:start+end+len("\n    }")] + "\n", nil
}
