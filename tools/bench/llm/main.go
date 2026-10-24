package llm

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// Options are the run command's flags.
type Options struct {
	Provider    string
	Model       string
	BaseURL     string
	Fixture     string
	Record      string
	Out         string
	Tasks       string
	Arms        string
	Samples     int
	MaxRounds   int
	Temperature float64
	MaxTokens   int
	Pilot       bool
	Sysml       string
	Root        string
}

// Main is the llm-bench command: `run` benchmarks one model, `report` renders
// committed results as Markdown, `corpus` checks the corpus.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: llm-bench run|report|corpus [flags]")
		return 2
	}
	var err error
	switch args[0] {
	case "run":
		err = runCommand(args[1:], stdout, stderr)
	case "report":
		err = reportCommand(args[1:], stdout)
	case "corpus":
		err = corpusCommand(stdout)
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}
	if err != nil {
		var missing ErrMissingKey
		if errors.As(err, &missing) {
			fmt.Fprintln(stderr, missing.Error())
			return 3
		}
		fmt.Fprintln(stderr, "llm-bench:", err)
		return 1
	}
	return 0
}

func runCommand(args []string, stdout, stderr io.Writer) error {
	o, err := parseRunOptions(args, stderr)
	if err != nil {
		return err
	}
	ctx := context.Background()
	root, err := repo.Choose(o.Root)
	if err != nil {
		return err
	}
	tc, err := toolchainFor(root, o)
	if err != nil {
		return err
	}
	corpus, err := LoadCorpus()
	if err != nil {
		return err
	}
	var ids []string
	if o.Tasks != "" {
		ids = strings.Split(o.Tasks, ",")
	}
	tasks, err := corpus.Select(ids)
	if err != nil {
		return err
	}
	arms, err := parseArms(o.Arms)
	if err != nil {
		return err
	}
	provider, recordedTools, err := providerFor(o)
	if err != nil {
		return err
	}
	var recorder *Recorder
	if o.Record != "" {
		recorder = NewRecorder(provider)
		provider = recorder
	}

	runner := &Runner{Provider: provider, Toolchain: tc, Corpus: corpus, MaxRounds: o.MaxRounds, Temperature: o.Temperature, MaxTokens: o.MaxTokens, Pilot: o.Pilot, Tools: recordedTools}
	runner.Log = func(format string, a ...any) { fmt.Fprintf(stderr, format+"\n", a...) }
	if slices.Contains(arms, ArmMCP) {
		arm, err := StartMCP(ctx, "llm-bench")
		if err != nil {
			return fmt.Errorf("start sysml-mcp: %w", err)
		}
		defer arm.Close(ctx)
		runner.MCP = arm
	}

	results := newResults(ctx, o, provider, tc, root, len(tasks))
	for _, task := range tasks {
		for _, arm := range arms {
			for sample := 1; sample <= o.Samples; sample++ {
				results.Cells = append(results.Cells, runner.RunCell(ctx, task, arm, sample))
				if err := saveProgress(o, recorder, results); err != nil {
					return err
				}
			}
		}
	}
	results.Finish()
	fmt.Fprint(stdout, MarkdownTable([]*Results{results}))
	if s := MarkdownSignificance(results); s != "" {
		fmt.Fprint(stdout, "\n"+s)
	}
	return nil
}

// parseRunOptions reads the run subcommand's flags and rejects the combinations
// no run can start from.
func parseRunOptions(args []string, stderr io.Writer) (Options, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var o Options
	fs.StringVar(&o.Provider, "provider", "", "provider: openai, xai, openrouter, ollama, vllm, anthropic or replay")
	fs.StringVar(&o.Model, "model", "", "model identifier the provider accepts")
	fs.StringVar(&o.BaseURL, "base-url", "", "override the provider's base URL")
	fs.StringVar(&o.Fixture, "fixture", "", "replay: fixture file to replay")
	fs.StringVar(&o.Record, "record", "", "record every answer into this fixture file")
	fs.StringVar(&o.Out, "out", "", "results file to write (JSON)")
	fs.StringVar(&o.Tasks, "tasks", "", "comma-separated task ids; default every task")
	fs.StringVar(&o.Arms, "arms", "compiler,mcp", "comma-separated arms to run")
	fs.IntVar(&o.Samples, "samples", 3, "samples per cell")
	fs.IntVar(&o.MaxRounds, "rounds", DefaultMaxRounds, "repair rounds after the first generation")
	fs.Float64Var(&o.Temperature, "temperature", 0, "sampling temperature")
	fs.IntVar(&o.MaxTokens, "max-tokens", DefaultMaxTokens, "completion token limit")
	fs.BoolVar(&o.Pilot, "pilot", false, "judge every compiled document with the pinned pilot validator too")
	fs.StringVar(&o.Sysml, "sysml", "", "sysml binary; default <repo>/bin/sysml")
	fs.StringVar(&o.Root, "root", "", "repository root; default the enclosing checkout")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if o.Provider == "" {
		return o, errors.New("-provider is required")
	}
	if o.MaxRounds < 0 {
		return o, fmt.Errorf("-rounds must not be negative, got %d", o.MaxRounds)
	}
	return o, nil
}

// parseArms reads the comma-separated arms a run compares.
func parseArms(list string) ([]Arm, error) {
	var arms []Arm
	for _, a := range strings.Split(list, ",") {
		switch Arm(a) {
		case ArmCompiler, ArmMCP:
			arms = append(arms, Arm(a))
		default:
			return nil, fmt.Errorf("unknown arm %q", a)
		}
	}
	return arms, nil
}

// providerFor opens the provider the options name: a replay of a recorded
// fixture, whose tools the fixture also carries, or a live model.
func providerFor(o Options) (Provider, []ToolSpec, error) {
	if o.Provider == "replay" {
		if o.Fixture == "" {
			return nil, nil, errors.New("replay needs -fixture")
		}
		f, err := LoadFixture(o.Fixture)
		if err != nil {
			return nil, nil, err
		}
		return NewReplay(f), f.Tools, nil
	}
	if o.Model == "" {
		return nil, nil, errors.New("-model is required")
	}
	provider, err := NewProvider(o.Provider, o.Model, o.BaseURL)
	return provider, nil, err
}

// newResults starts the results file with the run's settings and toolchain.
func newResults(ctx context.Context, o Options, provider Provider, tc Toolchain, root string, tasks int) *Results {
	results := &Results{Provider: provider.Name(), Model: provider.Model(), Recorded: baseline.Today()}
	results.Settings.MaxRounds, results.Settings.Samples = o.MaxRounds, o.Samples
	results.Settings.Temperature, results.Settings.MaxTokens = o.Temperature, o.MaxTokens
	results.Settings.Tasks, results.Settings.Tools = tasks, MCPTools
	results.Toolchain.SysmlVersion = sysmlVersion(ctx, tc.Sysml)
	if o.Pilot {
		if pin, err := baseline.ReadPin(root); err == nil {
			results.Toolchain.PilotRelease = pin.Release()
		}
	}
	return results
}

// saveProgress writes the fixture and results files after a cell, so an
// interrupted run keeps what it has measured.
func saveProgress(o Options, recorder *Recorder, results *Results) error {
	if recorder != nil {
		if err := recorder.Save(o.Record); err != nil {
			return err
		}
	}
	if o.Out != "" {
		results.Finish()
		return results.Write(o.Out)
	}
	return nil
}

func toolchainFor(root string, o Options) (Toolchain, error) {
	tc := Toolchain{Sysml: o.Sysml}
	if tc.Sysml == "" {
		tc.Sysml = filepath.Join(root, "bin", "sysml")
	}
	if _, err := os.Stat(tc.Sysml); err != nil {
		return tc, fmt.Errorf("sysml binary %s: %w (run make build)", tc.Sysml, err)
	}
	if o.Pilot {
		tc.Pilot = filepath.Join(root, "build", "pilot-sysml-validator", "validate-sysml-batch")
		if _, err := os.Stat(tc.Pilot); err != nil {
			return tc, fmt.Errorf("pilot validator %s: %w (run scripts/download-pilot-sysml-validator.sh)", tc.Pilot, err)
		}
		tc.PilotLibrary = filepath.Join(root, "internal", "workspace", "libs", "stdlib", "OpenSysML Libraries")
	}
	return tc, nil
}

func sysmlVersion(ctx context.Context, sysml string) string {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, sysml, "-version").Output() // #nosec G204 -- the binary is the caller's toolchain
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func reportCommand(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New("report needs at least one results file")
	}
	var runs []*Results
	for _, p := range fs.Args() {
		r, err := ReadResults(p)
		if err != nil {
			return err
		}
		runs = append(runs, r)
	}
	fmt.Fprint(stdout, MarkdownTable(runs))
	for _, r := range runs {
		if s := MarkdownSignificance(r); s != "" {
			fmt.Fprint(stdout, "\n"+s)
		}
	}
	return nil
}

func corpusCommand(stdout io.Writer) error {
	m, err := LoadCorpus()
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%d tasks\n", len(m.Tasks))
	return nil
}
