package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var fakeShards = []string{"cli", "graph", "rest", "runtime", "smt", "tools"}

// fakeRoot writes a tree whose shard script is the single source the runner
// reads; nothing about the shards is compiled into the runner.
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

func allShardsScript() string {
	return "printf '%s\\n' " + strings.Join(fakeShards, " ") + "\n"
}

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
				key := "SHARDS="
				if tier == "full" {
					key = "SHARD="
				}
				found := false
				for _, field := range fields {
					if strings.HasPrefix(field, key) {
						planned = append(planned, strings.TrimPrefix(field, key))
						found = true
					}
				}
				if !found {
					t.Errorf("command %q names no %s", line, key)
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
		root:        root,
		tier:        "quick",
		concurrency: 3,
		pkgParallel: 1,
		shards:      fakeShards,
		stdout:      &stdout,
	}
	r.exec = func(_ context.Context, command []string) (string, error) {
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
			return "smt: failing output\n", errors.New("exit status 1")
		}
		return "ok " + shard + "\n", nil
	}
	commands := r.commands()
	results, err := r.run(context.Background(), commands)
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
	var stdout bytes.Buffer
	var calls atomic.Int64
	r := &runner{
		root:        root,
		tier:        "quick",
		concurrency: 2,
		shards:      []string{"cli", "smt", "tools"},
		stdout:      &stdout,
	}
	r.exec = func(_ context.Context, command []string) (string, error) {
		calls.Add(1)
		if slices.Equal(command, snapshotCommand()) {
			return "snapshot\n", nil
		}
		return "", nil
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
	var stdout bytes.Buffer
	var calls atomic.Int64
	r := &runner{
		root:        root,
		tier:        "quick",
		concurrency: 2,
		shards:      []string{"cli", "smt"},
		stdout:      &stdout,
	}
	r.exec = func(_ context.Context, _ []string) (string, error) {
		calls.Add(1)
		return "snapshot failed\n", errors.New("exit status 2")
	}
	results, err := r.run(context.Background(), r.commands())
	if err == nil || !strings.Contains(err.Error(), "snapshot") {
		t.Fatalf("run error = %v, want the snapshot failure", err)
	}
	if results != nil || calls.Load() != 1 {
		t.Fatalf("ran after the snapshot failed: %d commands, %d results", calls.Load(), len(results))
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
	} {
		stdout.Reset()
		stderr.Reset()
		if err := run(args, &stdout, &stderr); err == nil {
			t.Errorf("run %v accepted bad input", args)
		}
	}
}

func TestChildEnvSkipsTheSharedSnapshot(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=readonly")
	env := childEnv(2)
	if !slices.Contains(env, snapshotSkipEnv+"="+snapshotSkipOn) {
		t.Errorf("child env %v does not skip the snapshot prerequisite", env)
	}
	if !slices.Contains(env, "GOFLAGS=-mod=readonly -p=2") {
		t.Errorf("child env %v does not bound package parallelism", env)
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
		{flags: "-p -v -mod=readonly", p: 5, want: "-mod=readonly -p=5"},
	} {
		if got := goFlagsWithP(tc.flags, tc.p); got != tc.want {
			t.Errorf("goFlagsWithP(%q, %d) = %q, want %q", tc.flags, tc.p, got, tc.want)
		}
	}
}

func TestEnvLimit(t *testing.T) {
	t.Setenv(concurrencyEnv, "")
	if got, err := envLimit(); err != nil || got != defaultLimit {
		t.Errorf("unset limit = %d, %v; want %d", got, err, defaultLimit)
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
