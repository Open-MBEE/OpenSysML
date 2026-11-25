// Command gate-runner runs every CI Go test shard with bounded concurrency,
// one warm build cache and a small cap on each child's GOMAXPROCS.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"os/signal"
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
	procsEnv        = "OPENSYSML_GATE_GOMAXPROCS"
	logTailEnv      = "OPENSYSML_GATE_LOG_TAIL"
	defaultLogTail  = 64 << 10
	skipBanner      = "GATE NOT RUN"
)

// tierTarget maps each tier to the Makefile target CI invokes for it.
var tierTarget = map[string]string{
	"quick": "test-quick",
	"queue": "test-queue",
	"full":  "test-shard",
}

// result is one shard's outcome, indexed by shard order.
type result struct {
	shard     string
	command   string
	output    string
	truncated bool
	warnings  int
	err       error
	elapsed   time.Duration
}

// FailureError reports the shards that failed.
type FailureError struct {
	Shards []string
}

func (e *FailureError) Error() string {
	return fmt.Sprintf("%d shard(s) failed: %s", len(e.Shards), strings.Join(e.Shards, ", "))
}

// executor streams a child's combined output to out and returns its error.
type executor func(ctx context.Context, command []string, out io.Writer) error

// warningCounter finds skip banners in a stream, including ones split across writes.
// Its carry is bounded to the marker width, independent of the log tail.
type warningCounter struct {
	carry []byte
	count int
}

func (c *warningCounter) scan(p []byte) {
	if len(p) == 0 {
		return
	}
	marker := []byte(skipBanner)
	width := len(marker)
	// Banners starting in this write: count each non-overlapping occurrence.
	for offset := 0; offset+width <= len(p); {
		found := bytes.Index(p[offset:], marker)
		if found < 0 {
			break
		}
		c.count++
		offset += found + width
	}
	// A banner starting in carry ends in this write; carry holds fewer than
	// width bytes, so it cannot hold a banner already counted on an earlier write.
	keep := width - 1
	head := keep
	if head > len(p) {
		head = len(p)
	}
	boundary := append(append([]byte(nil), c.carry...), p[:head]...)
	for offset := 0; offset < len(c.carry) && offset+width <= len(boundary); {
		found := bytes.Index(boundary[offset:], marker)
		if found < 0 {
			break
		}
		start := offset + found
		if start >= len(c.carry) {
			break
		}
		c.count++
		offset = start + width
	}
	if len(p) < keep {
		combined := append(append([]byte(nil), c.carry...), p...)
		if len(combined) > keep {
			combined = combined[len(combined)-keep:]
		}
		c.carry = combined
		return
	}
	c.carry = append(c.carry[:0], p[len(p)-keep:]...)
}

// tailBuffer keeps only the last limit bytes, streaming to w when set.
type tailBuffer struct {
	mu        sync.Mutex
	buf       []byte
	w         io.Writer
	limit     int
	truncated bool
	warnings  warningCounter
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	if t.w != nil {
		if _, err := t.w.Write(p); err != nil {
			return 0, err
		}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.warnings.scan(p)
	if len(p) >= t.limit {
		t.buf = append(t.buf[:0], p[len(p)-t.limit:]...)
		t.truncated = true
		return len(p), nil
	}
	if room := t.limit - len(p); len(t.buf) > room {
		copy(t.buf, t.buf[len(t.buf)-room:])
		t.buf = t.buf[:room]
		t.truncated = true
	}
	t.buf = append(t.buf, p...)
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

func (t *tailBuffer) wasTruncated() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.truncated
}

func (t *tailBuffer) warningCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.warnings.count
}

type runner struct {
	root        string
	tier        string
	concurrency int
	pkgParallel int
	procs       int
	logTail     int
	shards      []string
	verbose     bool
	stdout      io.Writer
	exec        executor
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), interruptSignals()...)
	defer stop()
	if err := runCtx(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		var failed *FailureError
		if !errors.As(err, &failed) {
			fmt.Fprintf(os.Stderr, "gate-runner: %v\n", err)
		}
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	return runCtx(context.Background(), args, stdout, stderr)
}

func runCtx(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("gate-runner", flag.ContinueOnError)
	fs.SetOutput(stderr)
	list := fs.Bool("list", false, "print every discovered shard, one per line, and exit")
	plan := fs.Bool("plan", false, "print the commands a run would execute and exit")
	tier := fs.String("tier", "queue", "tier to run: quick, queue or full")
	limit := fs.Int("concurrency", 0, fmt.Sprintf("maximum shards in flight (default derived, or $%s)", concurrencyEnv))
	parallel := fs.Int("p", 0, "package parallelism (-p) inside each shard; 0 leaves Go's default")
	procs := fs.Int("gomaxprocs", 0, fmt.Sprintf("GOMAXPROCS per shard (default derived, or $%s)", procsEnv))
	logTail := fs.String("log-tail", "", fmt.Sprintf("bytes of a failing shard's log to keep (default $%s or 64K)", logTailEnv))
	subset := fs.String("shards", "", "comma- or space-separated subset of the discovered shards")
	root := fs.String("root", "", "repository root (default: walk up from the working directory)")
	verbose := fs.Bool("v", false, "stream every shard's output as it arrives")
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

	cpus, mem := hostCapacity()
	concurrency := *limit
	if concurrency == 0 {
		concurrency, err = envLimit(cpus, mem)
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
	// The fleet is resolved before sizing shards: an override to the shard
	// subset or the concurrency must change the CPU budget per shard, not add
	// workers with the default fleet's GOMAXPROCS.
	workers := min(concurrency, len(shards))
	procsPerShard, err := goMaxProcs(*procs, workers, cpus)
	if err != nil {
		return err
	}
	tail, err := logTailBytes(*logTail)
	if err != nil {
		return err
	}

	r := &runner{
		root:        repo,
		tier:        *tier,
		concurrency: concurrency,
		pkgParallel: *parallel,
		procs:       procsPerShard,
		logTail:     tail,
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
	r.exec = processExecutor(repo, r.pkgParallel, r.procs)
	results, err := r.run(ctx, commands)
	if results != nil {
		printSummary(r, results)
	}
	return err
}

// findRoot walks up from start to the directory owning the shard script.
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

// discover reads the shard list from the script, never from a local copy.
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

func envLimit(cpus int, mem uint64) (int, error) {
	value := os.Getenv(concurrencyEnv)
	if strings.TrimSpace(value) == "" {
		return defaultShards(cpus, mem), nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s=%q: want a positive integer", concurrencyEnv, value)
	}
	return n, nil
}

// goMaxProcs returns 0 when the caller explicitly set GOMAXPROCS, leaving the
// caller's value untouched in the child environment. Otherwise it divides the
// host's CPU budget over the workers that will actually run.
func goMaxProcs(explicit, workers, cpus int) (int, error) {
	if explicit < 0 {
		return 0, fmt.Errorf("-gomaxprocs must not be negative, got %d", explicit)
	}
	if explicit > 0 {
		return explicit, nil
	}
	if value := os.Getenv(procsEnv); strings.TrimSpace(value) != "" {
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n < 1 {
			return 0, fmt.Errorf("%s=%q: want a positive integer", procsEnv, value)
		}
		return n, nil
	}
	if os.Getenv("GOMAXPROCS") != "" {
		return 0, nil
	}
	return procsForWorkers(cpus, workers), nil
}

func logTailBytes(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = strings.TrimSpace(os.Getenv(logTailEnv))
	}
	if value == "" {
		return defaultLogTail, nil
	}
	multiplier := uint64(1)
	switch last := strings.ToUpper(value[len(value)-1:]); last {
	case "K":
		multiplier, value = 1<<10, value[:len(value)-1]
	case "M":
		multiplier, value = 1<<20, value[:len(value)-1]
	case "G":
		multiplier, value = 1<<30, value[:len(value)-1]
	}
	n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || n == 0 || n > uint64(math.MaxInt32)/multiplier {
		return 0, fmt.Errorf("%s=%q: want a positive size with K, M or G suffix", logTailEnv, value)
	}
	return int(n * multiplier), nil
}

func snapshotCommand() []string {
	return []string{"make", "--no-print-directory", snapshotTarget}
}

// commands builds one make invocation per shard; the snapshot guard reaches
// recursive make calls, which a command-line -o does not.
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

// goFlagsWithP replaces any -p in GOFLAGS with the runner's bound.
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

// childEnv bounds each shard's CPU demand and package parallelism.
func childEnv(pkgParallel, procs int) []string {
	env := append(os.Environ(), snapshotSkipEnv+"="+snapshotSkipOn)
	if procs > 0 {
		env = append(env, "GOMAXPROCS="+strconv.Itoa(procs))
	}
	if pkgParallel > 0 {
		env = append(env, "GOFLAGS="+goFlagsWithP(os.Getenv("GOFLAGS"), pkgParallel))
	}
	return env
}

func processExecutor(root string, pkgParallel, procs int) executor {
	return func(ctx context.Context, command []string, out io.Writer) error {
		cmd := exec.CommandContext(ctx, command[0], command[1:]...)
		cmd.Dir = root
		cmd.Env = childEnv(pkgParallel, procs)
		cmd.Stdout, cmd.Stderr = out, out
		setProcessGroup(cmd)
		cmd.Cancel = func() error { return killProcessGroup(cmd) }
		cmd.WaitDelay = 2 * time.Second
		return cmd.Run()
	}
}

// run generates the snapshot once, then runs every shard within the cap.
func (r *runner) run(ctx context.Context, commands [][]string) ([]result, error) {
	if len(commands) == 0 {
		return nil, errors.New("no shards to run")
	}
	if r.logTail <= 0 {
		r.logTail = defaultLogTail
	}
	snapshot := &tailBuffer{limit: r.logTail}
	if err := r.exec(ctx, snapshotCommand(), snapshot); err != nil {
		return nil, fmt.Errorf("generate the bundled-library snapshot: %w\n%s", err, snapshot)
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
				sink := &tailBuffer{limit: r.logTail}
				if r.verbose {
					sink.w = r.stdout
				}
				started := time.Now()
				err := r.exec(ctx, commands[i], sink)
				output := ""
				truncated := false
				if err != nil {
					output = sink.String()
					truncated = sink.wasTruncated()
				}
				results[i] = result{
					shard:     r.shards[i],
					command:   strings.Join(commands[i], " "),
					output:    output,
					truncated: truncated,
					warnings:  sink.warningCount(),
					err:       err,
					elapsed:   time.Since(started),
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
	failed, warned := 0, 0
	for _, res := range results {
		status := "PASS"
		switch {
		case res.err != nil:
			status = "FAIL"
			failed++
		case res.warnings > 0:
			status = "WARN"
			warned++
		}
		fmt.Fprintf(r.stdout, "[%s] %-24s %s\n", status, res.shard, res.elapsed.Round(time.Millisecond))
		if res.err != nil {
			fmt.Fprintf(r.stdout, "----- %s: %s -----\n%s\n", res.shard, res.command, res.output)
			if res.truncated {
				fmt.Fprintf(r.stdout, "----- %s: only the last %d bytes retained -----\n", res.shard, r.logTail)
			}
			fmt.Fprintf(r.stdout, "----- %s: %v -----\n", res.shard, res.err)
		}
		if res.warnings > 0 {
			fmt.Fprintf(r.stdout, "----- %s: %d gate(s) not run (coverage incomplete) -----\n", res.shard, res.warnings)
		}
	}
	if failed == 0 && warned == 0 {
		fmt.Fprintf(r.stdout, "gate runner: all %d shards passed\n", len(results))
		return
	}
	if failed == 0 {
		fmt.Fprintf(r.stdout, "gate runner: all %d shards exited zero, %d incomplete (gate skipped)\n", len(results), warned)
		return
	}
	fmt.Fprintf(r.stdout, "gate runner: %d of %d shards failed", failed, len(results))
	if warned > 0 {
		fmt.Fprintf(r.stdout, ", %d incomplete (gate skipped)", warned)
	}
	fmt.Fprintln(r.stdout)
}
