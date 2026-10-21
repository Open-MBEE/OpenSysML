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
		return err
	}
	if o.Provider == "" {
		return errors.New("-provider is required")
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
	var arms []Arm
	for _, a := range strings.Split(o.Arms, ",") {
		switch Arm(a) {
		case ArmCompiler, ArmMCP:
			arms = append(arms, Arm(a))
		default:
			return fmt.Errorf("unknown arm %q", a)
		}
	}

	var provider Provider
	var recordedTools []ToolSpec
	if o.Provider == "replay" {
		if o.Fixture == "" {
			return errors.New("replay needs -fixture")
		}
		f, err := LoadFixture(o.Fixture)
		if err != nil {
			return err
		}
		provider = NewReplay(f)
		recordedTools = f.Tools
	} else {
		if o.Model == "" {
			return errors.New("-model is required")
		}
		provider, err = NewProvider(o.Provider, o.Model, o.BaseURL)
		if err != nil {
			return err
		}
	}
	var recorder *Recorder
	if o.Record != "" {
		recorder = NewRecorder(provider)
		provider = recorder
	}

	runner := &Runner{Provider: provider, Toolchain: tc, Corpus: corpus, MaxRounds: o.MaxRounds, Temperature: o.Temperature, MaxTokens: o.MaxTokens, Pilot: o.Pilot, Tools: recordedTools}
	runner.Log = func(format string, a ...any) { fmt.Fprintf(stderr, format+"\n", a...) }
	for _, a := range arms {
		if a == ArmMCP {
			arm, err := StartMCP(ctx, "llm-bench")
			if err != nil {
				return fmt.Errorf("start sysml-mcp: %w", err)
			}
			defer arm.Close(ctx)
			runner.MCP = arm
		}
	}

	if o.MaxRounds < 0 {
		return fmt.Errorf("-rounds must not be negative, got %d", o.MaxRounds)
	}
	results := &Results{Provider: provider.Name(), Model: provider.Model(), Recorded: baseline.Today()}
	results.Settings.MaxRounds, results.Settings.Samples = o.MaxRounds, o.Samples
	results.Settings.Temperature, results.Settings.MaxTokens = o.Temperature, o.MaxTokens
	results.Settings.Tasks, results.Settings.Tools = len(tasks), MCPTools
	results.Toolchain.SysmlVersion = sysmlVersion(ctx, tc.Sysml)
	if o.Pilot {
		if pin, err := baseline.ReadPin(root); err == nil {
			results.Toolchain.PilotRelease = pin.Release()
		}
	}
	for _, task := range tasks {
		for _, arm := range arms {
			for sample := 1; sample <= o.Samples; sample++ {
				res := runner.RunCell(ctx, task, arm, sample)
				results.Cells = append(results.Cells, res)
				if recorder != nil {
					if err := recorder.Save(o.Record); err != nil {
						return err
					}
				}
				if o.Out != "" {
					results.Finish()
					if err := results.Write(o.Out); err != nil {
						return err
					}
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
