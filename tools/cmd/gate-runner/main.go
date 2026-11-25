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
	"slices"
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
	// One package binary at a time keeps Go's package concurrency from
	// multiplying the shard's CPU allowance.
	defaultPackageParallel = 1
	// The runner gives the tools module one owner in a local all-shards plan;
	// CI jobs keep their own copy of the whole module.
	skipToolsTestsVar = "SKIP_TOOLS_TESTS"
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
	// Resolve the fleet before sizing: shard or concurrency overrides change
	// the per-shard CPU budget, not add workers at the default GOMAXPROCS.
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
	flags, err := effectiveGoFlags()
	if err != nil {
		return err
	}
	r.pkgParallel = packageParallel(*parallel, flags)
	r.procs = perProcessProcs(procsPerShard, r.pkgParallel)
	r.exec = processExecutor(repo, r.pkgParallel, r.procs, flags)
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

// goMaxProcs returns 0 when the caller set GOMAXPROCS, else divides the host's
// CPU budget over the workers actually in flight.
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
		// Both shards would otherwise run the tools module's snapshot generator
		// against this checkout. With both in the plan, tools owns it; a plan
		// with only rest keeps rest's tools coverage.
		if shard == "rest" && slices.Contains(r.shards, "tools") {
			command = append(command, skipToolsTestsVar+"=1")
		}
		commands = append(commands, command)
	}
	return commands
}

// goflag is one GOFLAGS token. raw keeps the caller's bytes, including any
// quotes wrapping the whole token; value is what Go reads after stripping them.
type goflag struct {
	raw   string
	value string
}

func isGOFlagSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// splitGOFLAGS mirrors cmd/go's quoted.Split: whitespace separates arguments
// and a leading single or double quote may wrap one whole argument. As in Go,
// nothing is unescaped inside the quotes.
func splitGOFLAGS(s string) ([]goflag, error) {
	var out []goflag
	for i := 0; i < len(s); {
		for i < len(s) && isGOFlagSpace(s[i]) {
			i++
		}
		if i == len(s) {
			break
		}
		start := i
		var value string
		if s[i] == '\'' || s[i] == '"' {
			quote := s[i]
			i++
			valueStart := i
			for i < len(s) && s[i] != quote {
				i++
			}
			if i == len(s) {
				return nil, fmt.Errorf("unterminated %c string in GOFLAGS", quote)
			}
			value = s[valueStart:i]
			i++
		} else {
			valueStart := i
			for i < len(s) && !isGOFlagSpace(s[i]) {
				i++
			}
			value = s[valueStart:i]
		}
		out = append(out, goflag{raw: s[start:i], value: value})
	}
	return out, nil
}

// goflagName splits a GOFLAGS token into its name and value. Go accepts one or
// two leading dashes and uses -name=value; a bare -name has no attached value.
func goflagName(token string) (name, value string, hasValue bool) {
	name = token
	switch {
	case strings.HasPrefix(name, "--"):
		name = name[2:]
	case strings.HasPrefix(name, "-"):
		name = name[1:]
	}
	if i := strings.IndexByte(name, '='); i >= 0 {
		value, hasValue, name = name[i+1:], true, name[:i]
	}
	return name, value, hasValue
}

// flagsPackageParallel reads the last -p in GOFLAGS, exported or saved with
// `go env -w`, because Go applies each setting in order. Zero means the flag is
// absent or malformed.
func flagsPackageParallel(flags string) int {
	tokens, err := splitGOFLAGS(flags)
	if err != nil {
		return 0
	}
	p := 0
	for i := 0; i < len(tokens); i++ {
		name, value, hasValue := goflagName(tokens[i].value)
		if name != "p" {
			continue
		}
		if !hasValue {
			// GOFLAGS holds one flag per token. The separate-value spelling is
			// not valid Go syntax but is common in hand-written settings.
			if i+1 >= len(tokens) || strings.HasPrefix(tokens[i+1].value, "-") {
				p = 0
				continue
			}
			i++
			value = tokens[i].value
		}
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			p = n
		} else {
			p = 0
		}
	}
	return p
}

// effectiveGoFlags returns the flags Go will actually read, including a value
// saved with `go env -w` that the caller did not export.
func effectiveGoFlags() (string, error) {
	out, err := exec.Command("go", "env", "GOFLAGS").Output()
	if err != nil {
		return "", fmt.Errorf("go env GOFLAGS: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// packageParallel resolves the -p passed to every shard: an explicit flag or
// GOFLAGS entry wins, otherwise one package binary runs at a time.
func packageParallel(flag int, flags string) int {
	if flag > 0 {
		return flag
	}
	if p := flagsPackageParallel(flags); p > 0 {
		return p
	}
	return defaultPackageParallel
}

// perProcessProcs divides the shard's CPU allowance among the package test
// processes Go may run at once. Zero leaves an explicit caller GOMAXPROCS alone.
func perProcessProcs(procs, p int) int {
	if procs <= 0 {
		return 0
	}
	if p < 1 {
		p = 1
	}
	return max(1, procs/p)
}

// goFlagsWithP bounds package parallelism without rewriting unrelated flags.
// It removes only -p tokens, keeping every other token's original bytes, and
// appends an override that Go applies last.
func goFlagsWithP(flags string, p int) string {
	override := "-p=" + strconv.Itoa(p)
	tokens, err := splitGOFLAGS(flags)
	if err != nil {
		// Go reports malformed flags itself; do not corrupt the caller's text.
		if trimmed := strings.TrimSpace(flags); trimmed != "" {
			return trimmed + " " + override
		}
		return override
	}
	if flagsPackageParallel(flags) == p {
		return strings.TrimSpace(flags)
	}
	kept := make([]string, 0, len(tokens)+1)
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		name, _, hasValue := goflagName(token.value)
		if name != "p" {
			kept = append(kept, token.raw)
			continue
		}
		if !hasValue && i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1].value, "-") {
			i++ // the separate value is not a flag
		}
	}
	return strings.Join(append(kept, override), " ")
}

// childEnv bounds each shard's package processes and per-process CPU demand.
// flags is what Go will read, exported or saved with `go env -w`.
func childEnv(p, procs int, flags string) []string {
	env := append(os.Environ(), snapshotSkipEnv+"="+snapshotSkipOn)
	if procs > 0 {
		env = append(env, "GOMAXPROCS="+strconv.Itoa(procs))
	}
	if p > 0 {
		env = append(env, "GOFLAGS="+goFlagsWithP(flags, p))
	}
	return env
}

func processExecutor(root string, p, procs int, flags string) executor {
	return func(ctx context.Context, command []string, out io.Writer) error {
		cmd := exec.CommandContext(ctx, command[0], command[1:]...)
		cmd.Dir = root
		cmd.Env = childEnv(p, procs, flags)
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
