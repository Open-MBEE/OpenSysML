// Command gate-runner runs every declared CI Go test shard with a bounded
// number of shards in flight and one warm build cache. Shards come from
// scripts/race-shard.sh --all, the same source the Makefile and CI read, so a
// new shard cannot be left out of a local run.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	shardScript     = "scripts/race-shard.sh"
	snapshotTarget  = "stdlib-snapshot"
	snapshotSkipEnv = "SKIP_STDLIB_SNAPSHOT"
	snapshotSkipOn  = "1"
	concurrencyEnv  = "OPENSYSML_GATE_CONCURRENCY"
	defaultLimit    = 4
)

// tierTarget names the Makefile target that runs each shard at a tier. The
// queue tier keeps the sweeps' construct-cover economy and the quick tier drops
// -race; both are the targets CI itself invokes, so the runner cannot drift
// from what CI does.
var tierTarget = map[string]string{
	"quick": "test-quick",
	"queue": "test-queue",
	"full":  "test-shard",
}

// result is one shard's outcome, kept by index so the summary is in shard
// order rather than completion order.
type result struct {
	shard   string
	command string
	output  string
	err     error
	elapsed time.Duration
}

// FailureError reports the shards that failed.
type FailureError struct {
	Shards []string
}

func (e *FailureError) Error() string {
	return fmt.Sprintf("%d shard(s) failed: %s", len(e.Shards), strings.Join(e.Shards, ", "))
}

type executor func(ctx context.Context, command []string) (string, error)

type runner struct {
	root        string
	tier        string
	concurrency int
	pkgParallel int
	shards      []string
	verbose     bool
	stdout      io.Writer
	exec        executor
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		var failed *FailureError
		if !errors.As(err, &failed) {
			fmt.Fprintf(os.Stderr, "gate-runner: %v\n", err)
		}
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("gate-runner", flag.ContinueOnError)
	fs.SetOutput(stderr)
	list := fs.Bool("list", false, "print every discovered shard, one per line, and exit")
	plan := fs.Bool("plan", false, "print the commands a run would execute and exit")
	tier := fs.String("tier", "queue", "tier to run: quick, queue or full")
	limit := fs.Int("concurrency", 0, fmt.Sprintf("maximum shards in flight (default $%s or %d)", concurrencyEnv, defaultLimit))
	parallel := fs.Int("p", 1, "package parallelism (-p) inside each shard; 0 leaves Go's default")
	subset := fs.String("shards", "", "comma- or space-separated subset of the discovered shards")
	root := fs.String("root", "", "repository root (default: walk up from the working directory)")
	verbose := fs.Bool("v", false, "print each shard's output, including passing shards")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	repo := *root
	if repo == "" {
		var err error
		repo, err = findRoot(".")
		if err != nil {
			return err
		}
	}
	discovered, err := discover(repo)
	if err != nil {
		return err
	}
	if *list {
		for _, shard := range discovered {
			fmt.Fprintln(stdout, shard)
		}
		return nil
	}
	if _, ok := tierTarget[*tier]; !ok {
		return fmt.Errorf("unknown tier %q: want quick, queue or full", *tier)
	}

	concurrency := *limit
	if concurrency == 0 {
		concurrency, err = envLimit()
		if err != nil {
			return err
		}
	}
	if concurrency < 1 {
		return fmt.Errorf("-concurrency must be at least 1, got %d", concurrency)
	}
	if *parallel < 0 {
		return fmt.Errorf("-p must not be negative, got %d", *parallel)
	}

	shards := discovered
	if fields := strings.FieldsFunc(*subset, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' }); len(fields) > 0 {
		known := make(map[string]bool, len(discovered))
		for _, shard := range discovered {
			known[shard] = true
		}
		seen := make(map[string]bool, len(fields))
		for _, shard := range fields {
			if !known[shard] {
				return fmt.Errorf("shard %q is not declared by %s", shard, shardScript)
			}
			if seen[shard] {
				return fmt.Errorf("shard %q was named twice", shard)
			}
			seen[shard] = true
		}
		shards = fields
	}

	r := &runner{
		root:        repo,
		tier:        *tier,
		concurrency: concurrency,
		pkgParallel: *parallel,
		shards:      shards,
		verbose:     *verbose,
		stdout:      stdout,
	}
	commands := r.commands()
	if *plan {
		fmt.Fprintln(stdout, strings.Join(snapshotCommand(), " "))
		for _, command := range commands {
			fmt.Fprintf(stdout, "%s=%s %s\n", snapshotSkipEnv, snapshotSkipOn, strings.Join(command, " "))
		}
		return nil
	}
	r.exec = processExecutor(repo, *parallel)
	results, err := r.run(context.Background(), commands)
	if results != nil {
		printSummary(r, results)
	}
	return err
}

// findRoot walks up from start to the directory that owns the shard script and
// the Makefile, as the Makefile itself does with dirname "$0"/...
func findRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, shardScript)); err == nil && !info.IsDir() {
			if info, err := os.Stat(filepath.Join(dir, "Makefile")); err == nil && !info.IsDir() {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no repository root with %s found above %s", shardScript, start)
		}
		dir = parent
	}
}

// discover reads the shard list from the script rather than keeping one here.
func discover(root string) ([]string, error) {
	cmd := exec.Command("bash", shardScript, "--all")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list shards with %s --all: %w", shardScript, err)
	}
	var shards []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		shard := strings.TrimSpace(line)
		if shard == "" {
			continue
		}
		if seen[shard] {
			return nil, fmt.Errorf("%s --all names %q twice", shardScript, shard)
		}
		seen[shard] = true
		shards = append(shards, shard)
	}
	if len(shards) == 0 {
		return nil, fmt.Errorf("%s --all named no shards", shardScript)
	}
	sort.Strings(shards)
	return shards, nil
}

func envLimit() (int, error) {
	value := os.Getenv(concurrencyEnv)
	if strings.TrimSpace(value) == "" {
		return defaultLimit, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s=%q: want a positive integer", concurrencyEnv, value)
	}
	return n, nil
}

func snapshotCommand() []string {
	return []string{"make", "--no-print-directory", snapshotTarget}
}

// commands builds one make invocation per shard. The runner generates the
// snapshot once up front and sets SKIP_STDLIB_SNAPSHOT in the child
// environment, which reaches the recursive make calls inside test-quick and
// test-queue (command line options such as -o do not).
func (r *runner) commands() [][]string {
	target := tierTarget[r.tier]
	commands := make([][]string, 0, len(r.shards))
	for _, shard := range r.shards {
		command := []string{"make", "--no-print-directory"}
		if r.tier == "full" {
			command = append(command, target, "SHARD="+shard)
		} else {
			command = append(command, target, "SHARDS="+shard)
		}
		commands = append(commands, command)
	}
	return commands
}

// goFlagsWithP replaces any -p already in GOFLAGS with the runner's bound and
// leaves every other flag untouched.
func goFlagsWithP(flags string, p int) string {
	fields := strings.Fields(flags)
	kept := make([]string, 0, len(fields)+1)
	for i := 0; i < len(fields); {
		field := fields[i]
		if field == "-p" {
			i += 2 // drop its value too, or go would read the next flag's
			continue
		}
		if !strings.HasPrefix(field, "-p=") {
			kept = append(kept, field)
		}
		i++
	}
	return strings.Join(append(kept, "-p="+strconv.Itoa(p)), " ")
}

// childEnv is the environment every shard command sees: the snapshot is
// already current, and -p bounds package parallelism so the global cap is
// concurrency * p, not concurrency * the number of packages in a shard.
func childEnv(pkgParallel int) []string {
	env := append(os.Environ(), snapshotSkipEnv+"="+snapshotSkipOn)
	if pkgParallel > 0 {
		env = append(env, "GOFLAGS="+goFlagsWithP(os.Getenv("GOFLAGS"), pkgParallel))
	}
	return env
}

func processExecutor(root string, pkgParallel int) executor {
	return func(ctx context.Context, command []string) (string, error) {
		cmd := exec.CommandContext(ctx, command[0], command[1:]...)
		cmd.Dir = root
		cmd.Env = childEnv(pkgParallel)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
}

// run executes the snapshot once, then the shard commands with at most
// concurrency in flight. Every command is attempted even when one fails, so a
// summary names all the failing shards (CI's matrix is fail-fast: false).
func (r *runner) run(ctx context.Context, commands [][]string) ([]result, error) {
	if len(commands) == 0 {
		return nil, errors.New("no shards to run")
	}
	if out, err := r.exec(ctx, snapshotCommand()); err != nil {
		return nil, fmt.Errorf("generate the bundled-library snapshot: %w\n%s", err, out)
	}

	results := make([]result, len(commands))
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := r.concurrency
	if workers > len(commands) {
		workers = len(commands)
	}
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				started := time.Now()
				out, err := r.exec(ctx, commands[i])
				results[i] = result{
					shard:   r.shards[i],
					command: strings.Join(commands[i], " "),
					output:  out,
					err:     err,
					elapsed: time.Since(started),
				}
			}
		}()
	}
	for i := range commands {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	var failed []string
	for _, res := range results {
		if res.err != nil {
			failed = append(failed, res.shard)
		}
	}
	if len(failed) > 0 {
		return results, &FailureError{Shards: failed}
	}
	return results, nil
}

func printSummary(r *runner, results []result) {
	for _, res := range results {
		status := "PASS"
		if res.err != nil {
			status = "FAIL"
		}
		fmt.Fprintf(r.stdout, "[%s] %-24s %s\n", status, res.shard, res.elapsed.Round(time.Millisecond))
		if res.err != nil || r.verbose {
			fmt.Fprintf(r.stdout, "----- %s: %s -----\n%s\n", res.shard, res.command, res.output)
			if res.err != nil {
				fmt.Fprintf(r.stdout, "----- %s: %v -----\n", res.shard, res.err)
			}
		}
	}
	failed := 0
	for _, res := range results {
		if res.err != nil {
			failed++
		}
	}
	if failed == 0 {
		fmt.Fprintf(r.stdout, "gate runner: all %d shards passed\n", len(results))
		return
	}
	fmt.Fprintf(r.stdout, "gate runner: %d of %d shards failed\n", failed, len(results))
}
