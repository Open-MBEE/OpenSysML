package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var fakeShards = []string{"cli", "graph", "rest", "runtime", "smt", "tools"}

// fakeRoot writes a tree whose shard script is the single source the runner reads.
func fakeRoot(t *testing.T, script string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "race-shard.sh"), []byte("#!/usr/bin/env bash\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Makefile"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func allShardsScript() string { return "printf '%s\\n' " + strings.Join(fakeShards, " ") + "\n" }

func TestDiscoverReadsTheScript(t *testing.T) {
	root := fakeRoot(t, allShardsScript())
	got, err := discover(root)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	want := slices.Clone(fakeShards)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("discover = %v, want %v", got, want)
	}
}

func TestDiscoverRejectsAnEmptyOrDuplicatedList(t *testing.T) {
	for name, script := range map[string]string{
		"empty":     "exit 0\n",
		"duplicate": "printf '%s\\n' cli cli\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := fakeRoot(t, script)
			if _, err := discover(root); err == nil {
				t.Fatal("discover accepted a bad shard list")
			}
		})
	}
}

func TestPlanCoversEveryDiscoveredShardAtEveryTier(t *testing.T) {
	root := fakeRoot(t, allShardsScript())
	for tier, target := range tierTarget {
		t.Run(tier, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := run([]string{"-root", root, "-tier", tier, "-plan"}, &stdout, &stderr); err != nil {
				t.Fatalf("run -plan: %v\n%s", err, stderr.String())
			}
			lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
			if len(lines) != len(fakeShards)+1 {
				t.Fatalf("plan printed %d commands, want the snapshot and %d shards:\n%s", len(lines), len(fakeShards), stdout.String())
			}
			if got, want := lines[0], strings.Join(snapshotCommand(), " "); got != want {
				t.Errorf("first command %q, want %q", got, want)
			}
			var planned []string
			for _, line := range lines[1:] {
				fields := strings.Fields(line)
				if !strings.HasPrefix(line, snapshotSkipEnv+"="+snapshotSkipOn+" ") {
					t.Errorf("command %q does not tell the shard the snapshot is current", line)
				}
				if !slices.Contains(fields, target) {
					t.Errorf("command %q does not call %s", line, target)
				}
				for _, field := range fields {
					if strings.HasPrefix(field, "SHARD=") || strings.HasPrefix(field, "SHARDS=") {
						planned = append(planned, strings.TrimPrefix(strings.TrimPrefix(field, "SHARDS="), "SHARD="))
					}
				}
			}
			want := slices.Clone(fakeShards)
			slices.Sort(want)
			if !slices.Equal(planned, want) {
				t.Errorf("plan covers %v, want every shard %v", planned, want)
			}
		})
	}
}

func TestRunBoundsConcurrencyAndReportsEveryFailure(t *testing.T) {
	root := fakeRoot(t, allShardsScript())
	var stdout bytes.Buffer
	var inFlight, peak atomic.Int64
	var mu sync.Mutex
	var seen []string
	r := &runner{
		root: root, tier: "quick", concurrency: 3, pkgParallel: 1,
		shards: fakeShards, stdout: &stdout,
	}
	r.exec = func(_ context.Context, command []string, out io.Writer) error {
		n := inFlight.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		inFlight.Add(-1)
		var shard string
		for _, field := range command {
			if strings.HasPrefix(field, "SHARDS=") {
				shard = strings.TrimPrefix(field, "SHARDS=")
			}
		}
		if shard != "" {
			mu.Lock()
			seen = append(seen, shard)
			mu.Unlock()
		}
		if shard == "smt" {
			_, _ = io.WriteString(out, "smt: failing output\n")
			return errors.New("exit status 1")
		}
		_, _ = io.WriteString(out, "ok "+shard+"\n")
		return nil
	}
	results, err := r.run(context.Background(), r.commands())
	var failed *FailureError
	if !errors.As(err, &failed) {
		t.Fatalf("run error = %v, want a FailureError", err)
	}
	if !slices.Equal(failed.Shards, []string{"smt"}) {
		t.Errorf("failed shards = %v, want [smt]", failed.Shards)
	}
	if len(results) != len(fakeShards) {
		t.Fatalf("got %d results, want %d", len(results), len(fakeShards))
	}
	if peak.Load() > 3 || peak.Load() < 2 {
		t.Errorf("peak in-flight shards = %d, want at most 3 and greater than 1", peak.Load())
	}
	slices.Sort(seen)
	want := slices.Clone(fakeShards)
	slices.Sort(want)
	if !slices.Equal(seen, want) {
		t.Errorf("ran %v, want every shard %v", seen, want)
	}
	for _, res := range results {
		if res.shard == "smt" && !strings.Contains(res.output, "failing output") {
			t.Errorf("failing shard's output was lost: %q", res.output)
		}
	}
	printSummary(r, results)
	if summary := stdout.String(); !strings.Contains(summary, "[FAIL] smt") || !strings.Contains(summary, "1 of 6 shards failed") {
		t.Errorf("summary = %q, want the failure named and counted", summary)
	}
}

func TestRunGeneratesTheSnapshotOnce(t *testing.T) {
	root := fakeRoot(t, allShardsScript())
	var calls atomic.Int64
	r := &runner{
		root: root, tier: "quick", concurrency: 2,
		shards: []string{"cli", "smt", "tools"}, stdout: io.Discard,
	}
	r.exec = func(_ context.Context, _ []string, _ io.Writer) error {
		calls.Add(1)
		return nil
	}
	if _, err := r.run(context.Background(), r.commands()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, want := calls.Load(), int64(4); got != want {
		t.Errorf("ran %d commands, want one snapshot and three shards", got)
	}
}

func TestSnapshotFailureStopsTheRun(t *testing.T) {
	root := fakeRoot(t, allShardsScript())
	var calls atomic.Int64
	r := &runner{
		root: root, tier: "quick", concurrency: 2,
		shards: []string{"cli", "smt"}, stdout: io.Discard,
	}
	r.exec = func(_ context.Context, _ []string, out io.Writer) error {
		calls.Add(1)
		_, _ = io.WriteString(out, "snapshot failed\n")
		return errors.New("exit status 2")
	}
	results, err := r.run(context.Background(), r.commands())
	if err == nil || !strings.Contains(err.Error(), "snapshot") {
		t.Fatalf("run error = %v, want the snapshot failure", err)
	}
	if results != nil || calls.Load() != 1 {
		t.Fatalf("ran after the snapshot failed: %d commands, %d results", calls.Load(), len(results))
	}
}

func TestLargeShardOutputIsBounded(t *testing.T) {
	root := fakeRoot(t, allShardsScript())
	var stdout bytes.Buffer
	r := &runner{
		root: root, tier: "quick", concurrency: 1,
		shards: []string{"cli"}, stdout: &stdout,
	}
	r.exec = func(_ context.Context, command []string, out io.Writer) error {
		if !slices.Contains(command, "test-quick") {
			return nil
		}
		if _, err := io.WriteString(out, strings.Repeat("x", 11<<20)); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "DIAGNOSTIC: shard failed\n")
		return errors.New("exit status 1")
	}
	results, err := r.run(context.Background(), r.commands())
	if err == nil {
		t.Fatal("run accepted a failing shard")
	}
	if got := len(results[0].output); got == 0 || got > r.logTail {
		t.Fatalf("retained %d bytes, want a bounded non-empty tail of %d", got, r.logTail)
	}
	if !strings.Contains(results[0].output, "DIAGNOSTIC: shard failed") {
		t.Error("the failing shard's last diagnostic was not retained")
	}
	if !results[0].truncated {
		t.Error("truncation was not reported")
	}
	printSummary(r, results)
	if summary := stdout.String(); !strings.Contains(summary, "DIAGNOSTIC: shard failed") || !strings.Contains(summary, "only the last 65536 bytes retained") {
		t.Errorf("summary = %q, want the diagnostic and the retention note", summary)
	}
}

func TestVerboseStreamsPassingOutput(t *testing.T) {
	root := fakeRoot(t, allShardsScript())
	var stdout bytes.Buffer
	r := &runner{
		root: root, tier: "quick", concurrency: 1, verbose: true,
		shards: []string{"cli"}, stdout: &stdout,
	}
	r.exec = func(_ context.Context, _ []string, out io.Writer) error {
		_, _ = io.WriteString(out, "streamed while running\n")
		return nil
	}
	results, err := r.run(context.Background(), r.commands())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout.String(), "streamed while running") {
		t.Error("verbose output was not streamed")
	}
	if results[0].output != "" {
		t.Errorf("a passing shard retained %d bytes", len(results[0].output))
	}
}

func TestCancellationKillsShardDescendants(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups are a POSIX facility")
	}
	dir := t.TempDir()
	pidFile, killedFile := filepath.Join(dir, "pid"), filepath.Join(dir, "killed")
	script := "#!/usr/bin/env bash\nsleep 30 &\nchild=$!\nprintf '%s\\n' \"$child\" > \"$GATE_TEST_PID_FILE\"\n" +
		"trap 'printf killed > \"$GATE_TEST_KILLED\"; kill \"$child\" 2>/dev/null; exit 0' TERM\nwait \"$child\"\n"
	if err := os.WriteFile(filepath.Join(dir, "make"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GATE_TEST_PID_FILE", pidFile)
	t.Setenv("GATE_TEST_KILLED", killedFile)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- processExecutor(dir, 0, 0)(ctx, []string{"make", "--no-print-directory", "test-quick", "SHARDS=cli"}, io.Discard)
	}()
	var pid []byte
	for i := 0; i < 200; i++ {
		if pid, _ = os.ReadFile(pidFile); len(pid) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(pid) == 0 {
		cancel()
		t.Fatal("the fake shard never started its child")
	}
	if _, err := strconv.Atoi(strings.TrimSpace(string(pid))); err != nil {
		t.Fatalf("bad child pid %q: %v", pid, err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("the executor did not return after cancellation")
	}
	if _, err := os.Stat(killedFile); err != nil {
		t.Fatalf("the shard child was not signalled: %v", err)
	}
}

func TestListAndSubsetValidation(t *testing.T) {
	root := fakeRoot(t, allShardsScript())
	var stdout, stderr bytes.Buffer
	if err := run([]string{"-root", root, "-list"}, &stdout, &stderr); err != nil {
		t.Fatalf("run -list: %v", err)
	}
	var got []string
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		got = append(got, strings.TrimSpace(line))
	}
	want := slices.Clone(fakeShards)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("-list printed %v, want %v", got, want)
	}
	for _, args := range [][]string{
		{"-root", root, "-shards", "not-a-shard", "-plan"},
		{"-root", root, "-shards", "cli,cli", "-plan"},
		{"-root", root, "-tier", "nope", "-plan"},
		{"-root", root, "-concurrency", "0x", "-plan"},
		{"-root", root, "-gomaxprocs", "-1", "-plan"},
	} {
		stdout.Reset()
		stderr.Reset()
		if err := run(args, &stdout, &stderr); err == nil {
			t.Errorf("run %v accepted bad input", args)
		}
	}
}

func TestChildEnvBoundsProcsAndPackages(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=readonly")
	t.Setenv("GOMAXPROCS", "")
	t.Setenv(procsEnv, "")
	env := childEnv(2, 3)
	for _, want := range []string{snapshotSkipEnv + "=" + snapshotSkipOn, "GOMAXPROCS=3", "GOFLAGS=-mod=readonly -p=2"} {
		if !slices.Contains(env, want) {
			t.Errorf("child env %v lacks %s", env, want)
		}
	}
	t.Setenv("GOMAXPROCS", "9")
	if got, err := goMaxProcs(0); err != nil || got != 0 {
		t.Errorf("explicit GOMAXPROCS: goMaxProcs = %d, %v; want 0, nil", got, err)
	}
	if got := childEnv(0, 0); slices.Contains(got, "GOMAXPROCS=2") {
		t.Errorf("child env %v overrode an explicit GOMAXPROCS", got)
	}
}

func TestGoMaxProcsConfiguration(t *testing.T) {
	t.Setenv("GOMAXPROCS", "")
	t.Setenv(procsEnv, "")
	_, derived := deriveDefaults()
	if got, err := goMaxProcs(0); err != nil || got != derived {
		t.Errorf("default = %d, %v; want the derived %d", got, err, derived)
	}
	t.Setenv(procsEnv, " 5 ")
	if got, err := goMaxProcs(0); err != nil || got != 5 {
		t.Errorf("env = %d, %v; want 5", got, err)
	}
	if got, err := goMaxProcs(7); err != nil || got != 7 {
		t.Errorf("flag = %d, %v; want 7", got, err)
	}
	t.Setenv(procsEnv, "nope")
	if _, err := goMaxProcs(0); err == nil {
		t.Error("accepted an invalid procs value")
	}
}

func TestLogTailBytes(t *testing.T) {
	t.Setenv(logTailEnv, "")
	if got, err := logTailBytes(""); err != nil || got != defaultLogTail {
		t.Errorf("default = %d, %v; want %d", got, err, defaultLogTail)
	}
	for value, want := range map[string]int{"128K": 128 << 10, "2M": 2 << 20, "4096": 4096} {
		if got, err := logTailBytes(value); err != nil || got != want {
			t.Errorf("logTailBytes(%q) = %d, %v; want %d", value, got, err, want)
		}
	}
	t.Setenv(logTailEnv, "1M")
	if got, err := logTailBytes(""); err != nil || got != 1<<20 {
		t.Errorf("env = %d, %v; want 1M", got, err)
	}
	for _, value := range []string{"0", "-1", "oops", "1T"} {
		if _, err := logTailBytes(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}

func TestDerivedDefaultsStayWithinTheHost(t *testing.T) {
	cpus, _ := hostCapacity()
	concurrency, procs := deriveDefaults()
	if concurrency < 1 || procs < 1 {
		t.Fatalf("derived %d shards and %d procs", concurrency, procs)
	}
	if concurrency > cpus || concurrency*procs > cpus {
		t.Errorf("derived %d shards x %d procs exceeds the host's %d CPUs", concurrency, procs, cpus)
	}
}

func TestGoFlagsWithP(t *testing.T) {
	for _, tc := range []struct {
		flags string
		p     int
		want  string
	}{
		{flags: "", p: 1, want: "-p=1"},
		{flags: "-mod=readonly -v", p: 2, want: "-mod=readonly -v -p=2"},
		{flags: "-p 8 -tags sysml_re", p: 3, want: "-tags sysml_re -p=3"},
		{flags: "-p=16 -trimpath", p: 4, want: "-trimpath -p=4"},
	} {
		if got := goFlagsWithP(tc.flags, tc.p); got != tc.want {
			t.Errorf("goFlagsWithP(%q, %d) = %q, want %q", tc.flags, tc.p, got, tc.want)
		}
	}
}

func TestEnvLimit(t *testing.T) {
	t.Setenv(concurrencyEnv, "")
	derived, _ := deriveDefaults()
	if got, err := envLimit(); err != nil || got != derived {
		t.Errorf("unset limit = %d, %v; want the derived %d", got, err, derived)
	}
	t.Setenv(concurrencyEnv, " 7 ")
	if got, err := envLimit(); err != nil || got != 7 {
		t.Errorf("set limit = %d, %v; want 7", got, err)
	}
	for _, value := range []string{"0", "-1", "two"} {
		t.Setenv(concurrencyEnv, value)
		if _, err := envLimit(); err == nil {
			t.Errorf("accepted %s=%q", concurrencyEnv, value)
		}
	}
}

func TestFindRoot(t *testing.T) {
	root := fakeRoot(t, allShardsScript())
	nested := filepath.Join(root, "tools", "cmd", "gate-runner")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := findRoot(nested)
	if err != nil {
		t.Fatalf("findRoot: %v", err)
	}
	if got != root {
		t.Errorf("findRoot = %q, want %q", got, root)
	}
	if _, err := findRoot(t.TempDir()); err == nil {
		t.Error("findRoot accepted a directory outside a repository")
	}
}

func TestFailureErrorMessage(t *testing.T) {
	err := &FailureError{Shards: []string{"smt", "cli"}}
	if got := err.Error(); !strings.Contains(got, "smt") || !strings.Contains(got, "cli") || !strings.Contains(got, "failed") {
		t.Fatal(fmt.Sprintf("Error() = %q", got))
	}
}
