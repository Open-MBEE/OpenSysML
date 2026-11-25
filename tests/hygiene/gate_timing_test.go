package hygiene

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gateTimingRun runs the opt-in timing script with a controlled HOME, so a
// hostile value cannot direct writes at the real cache.
func gateTimingRun(t *testing.T, home, value string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", "scripts/gate-timing.sh")
	cmd.Dir = repoRoot
	var env []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "HOME=") && !strings.HasPrefix(entry, "GATE_TIMING_TMP=") {
			env = append(env, entry)
		}
	}
	cmd.Env = append(env, "HOME="+home, "GATE_TIMING_TMP="+value)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestGateTimingRejectsHostileTmp(t *testing.T) {
	for _, tc := range []struct {
		name, value, message string
	}{
		{"absolute", "/tmp/gate-timing-hostile", "must be relative"},
		{"traversal", "../escape", "must not contain '..'"},
		{"nested traversal", "a/../../escape", "must not contain '..'"},
		{"unsupported character", "a b", "unsupported component"},
		{"newline escapes validation", "safe\n/tmp/escape", "control characters"},
		{"newline traversal", "safe\n../escape", "control characters"},
		{"carriage return", "safe\rother", "control characters"},
		{"tab", "safe\tother", "control characters"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			out, err := gateTimingRun(t, home, tc.value)
			if err == nil {
				t.Fatalf("accepted GATE_TIMING_TMP=%q", tc.value)
			}
			if !strings.Contains(out, tc.message) {
				t.Errorf("output %q lacks %q", out, tc.message)
			}
			root := filepath.Join(home, ".cache", "opensysml")
			if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("created %s for a hostile value: %v", root, err)
			}
		})
	}
}

func TestGateTimingRejectsSymlinkedTmp(t *testing.T) {
	// The script creates tmp and temp below the requested directory: all three
	// paths must be checked before mkdir can follow a link into the target.
	for _, name := range []string{"link", "tmp", "temp"} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			root := filepath.Join(home, ".cache", "opensysml")
			outside := filepath.Join(home, "outside")
			for _, dir := range []string{root, outside} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			rel := "link"
			if name != "link" {
				rel = "gate-timing/" + name
				if err := os.MkdirAll(filepath.Join(root, "gate-timing"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(outside, filepath.Join(root, rel)); err != nil {
				t.Fatal(err)
			}
			out, err := gateTimingRun(t, home, strings.SplitN(rel, "/", 2)[0])
			if err == nil {
				t.Fatal("accepted a symlinked GATE_TIMING_TMP path")
			}
			if !strings.Contains(out, "symlinked path component") {
				t.Errorf("output %q does not name the symlink", out)
			}
			if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
				t.Errorf("wrote through the symlink into %s: %v, %v", outside, entries, err)
			}
		})
	}
}

// TestGateTimingForcesUncachedRuns pins the methodology: both timed arms run
// after GOFLAGS gains -count=1, so neither can replay a result the other cached.
func TestGateTimingForcesUncachedRuns(t *testing.T) {
	source := readRepoFile(t, "scripts/gate-timing.sh")
	flags := strings.Index(source, "export GOFLAGS=")
	serial := strings.Index(source, "make test-quick SHARDS=")
	runner := strings.Index(source, "go run -C tools ./cmd/gate-runner")
	if flags < 0 || serial < 0 || runner < 0 {
		t.Fatalf("timing script no longer has the GOFLAGS export and both timed arms:\n%s", source)
	}
	if flags > serial || flags > runner {
		t.Errorf("GOFLAGS is exported after a timed arm (flags=%d serial=%d runner=%d)", flags, serial, runner)
	}
	for _, want := range []string{`export GOFLAGS="$GOFLAGS -count=1"`, `export GOFLAGS='-count=1'`} {
		if !strings.Contains(source, want) {
			t.Errorf("timing script lacks %q", want)
		}
	}
	doc := readRepoFile(t, "docs/project/test-gate-throughput.md")
	for _, want := range []string{"GOFLAGS=-count=1", "not cache hits"} {
		if !strings.Contains(doc, want) {
			t.Errorf("the timing documentation no longer says %q", want)
		}
	}
}

// TestGateTimingWarmsTheExactTimedShardFlags pins that neither arm pays a cold
// compilation the other reuses, and that the warmup sees the generated snapshot
// and the same Go flags as the timed shards.
func TestGateTimingWarmsTheExactTimedShardFlags(t *testing.T) {
	source := readRepoFile(t, "scripts/gate-timing.sh")
	snapshot := strings.Index(source, "make --no-print-directory stdlib-snapshot")
	warm := strings.Index(source, `make --no-print-directory test-quick SHARDS="grpc rest"`)
	timed := strings.Index(source, "--- $serial ---")
	runner := strings.Index(source, "go run -C tools ./cmd/gate-runner")
	if snapshot < 0 || warm < 0 || timed < 0 || runner < 0 || !(snapshot < warm && warm < timed && timed < runner) {
		t.Fatalf("timing script lacks a snapshot and warmup before both arms: snapshot=%d warm=%d timed=%d runner=%d", snapshot, warm, timed, runner)
	}
	warmup := source[warm:timed]
	for _, want := range []string{`SHARDS="grpc rest"`, `SKIP_STDLIB_SNAPSHOT=1`, `TEST_SHARD_RUN="-run '^$'"`} {
		if !strings.Contains(warmup, want) {
			t.Errorf("warmup lacks %q:\n%s", want, warmup)
		}
	}
	makefile := readRepoFile(t, "Makefile")
	recipe := makefile[strings.Index(makefile, "test-shard:"):strings.Index(makefile, "\ntest-quick:")]
	if got := strings.Count(recipe, "-pgo=off -timeout 55m $(TEST_SHARD_RUN)"); got != 2 {
		t.Errorf("%d of the shard's two go test invocations use the exact timed flags", got)
	}
	for _, target := range []string{"test-quick:", "test-queue:"} {
		section := makefile[strings.Index(makefile, target):]
		if end := strings.Index(section, "\n\n"); end >= 0 {
			section = section[:end]
		}
		if !strings.Contains(section, `TEST_SHARD_RUN="$(TEST_SHARD_RUN)"`) {
			t.Errorf("%s does not pass the compile-only flags to its shards", target)
		}
	}
}

// cleanupSnippet is the published shell code, extracted so the test executes
// exactly what a reader would copy rather than a second copy of the guard.
func cleanupSnippet(t *testing.T) string {
	t.Helper()
	doc := readRepoFile(t, "docs/project/test-gate-throughput.md")
	start := strings.Index(doc, `: "${HOME:?`)
	if start < 0 {
		t.Fatal("cleanup snippet has no guard")
	}
	end := strings.Index(doc[start:], "rm -rf -- ")
	if end < 0 {
		t.Fatal("cleanup snippet has no removal")
	}
	line := strings.IndexByte(doc[start+end:], '\n')
	if line < 0 {
		t.Fatal("cleanup snippet's removal has no line end")
	}
	return doc[start : start+end+line]
}

func runCleanupSnippet(t *testing.T, home, gotmp string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", "-c", cleanupSnippet(t))
	env := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "HOME=") && !strings.HasPrefix(entry, "GOTMPDIR=") {
			env = append(env, entry)
		}
	}
	cmd.Env = append(env, "HOME="+home)
	if gotmp != "" {
		cmd.Env = append(cmd.Env, "GOTMPDIR="+gotmp)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestGateCleanupSnippetConfinesDeletion executes the documented cleanup with
// hostile cache paths: only entries inside the approved root may be removed.
func TestGateCleanupSnippetConfinesDeletion(t *testing.T) {
	write := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("approved cache", func(t *testing.T) {
		home := t.TempDir()
		cache := filepath.Join(home, ".cache", "opensysml", "go-tmp")
		stale := filepath.Join(cache, "stale")
		write(stale)
		out, err := runCleanupSnippet(t, home, cache)
		if err != nil {
			t.Fatalf("approved cleanup failed: %v\n%s", err, out)
		}
		if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("stale entry survived: %v", err)
		}
	})
	t.Run("unset GOTMPDIR", func(t *testing.T) {
		out, err := runCleanupSnippet(t, t.TempDir(), "")
		if err == nil || !strings.Contains(out, "GOTMPDIR:") {
			t.Fatalf("unset GOTMPDIR was not rejected: %v\n%s", err, out)
		}
	})

	for _, tc := range []struct {
		name, message string
		setup         func(t *testing.T, home, root, outside string) (gotmp, keep string)
	}{
		{
			name: "symlink inside the root", message: "symlinked cache path",
			setup: func(t *testing.T, home, root, outside string) (string, string) {
				t.Helper()
				inside := filepath.Join(root, "inside")
				if err := os.MkdirAll(filepath.Join(inside, "go-tmp"), 0o755); err != nil {
					t.Fatal(err)
				}
				link := filepath.Join(root, "link")
				if err := os.Symlink(inside, link); err != nil {
					t.Fatal(err)
				}
				keep := filepath.Join(inside, "keep")
				write(keep)
				return link, keep
			},
		},
		{
			name: "symlinked parent to outside", message: "refusing to remove outside",
			setup: func(t *testing.T, home, root, outside string) (string, string) {
				t.Helper()
				cache := filepath.Join(outside, "cache")
				if err := os.MkdirAll(cache, 0o755); err != nil {
					t.Fatal(err)
				}
				link := filepath.Join(root, "parent")
				if err := os.Symlink(outside, link); err != nil {
					t.Fatal(err)
				}
				keep := filepath.Join(cache, "keep")
				write(keep)
				return filepath.Join(link, "cache"), keep
			},
		},
		{
			name: "symlinked cache root component", message: "symlinked cache path",
			setup: func(t *testing.T, home, root, outside string) (string, string) {
				t.Helper()
				cache := filepath.Join(home, ".cache")
				moved := filepath.Join(outside, ".cache")
				target := filepath.Join(moved, "opensysml")
				if err := os.MkdirAll(filepath.Join(target, "go-tmp"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.RemoveAll(cache); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(moved, cache); err != nil {
					t.Fatal(err)
				}
				keepDir := filepath.Join(target, "keep-dir")
				if err := os.MkdirAll(keepDir, 0o755); err != nil {
					t.Fatal(err)
				}
				keep := filepath.Join(keepDir, "keep")
				write(keep)
				return filepath.Join(root, "go-tmp"), keep
			},
		},
		{
			name: "parent traversal", message: "refusing to remove outside",
			setup: func(t *testing.T, home, root, outside string) (string, string) {
				t.Helper()
				evil := filepath.Join(home, ".cache", "opensysml-evil")
				if err := os.MkdirAll(evil, 0o755); err != nil {
					t.Fatal(err)
				}
				keep := filepath.Join(evil, "keep")
				write(keep)
				return filepath.Join(root, "..", "opensysml-evil"), keep
			},
		},
		{
			name: "prefix sibling", message: "refusing to remove outside",
			setup: func(t *testing.T, home, root, outside string) (string, string) {
				t.Helper()
				evil := root + "-evil"
				if err := os.MkdirAll(evil, 0o755); err != nil {
					t.Fatal(err)
				}
				keep := filepath.Join(evil, "keep")
				write(keep)
				return evil, keep
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			root := filepath.Join(home, ".cache", "opensysml")
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(home, "outside")
			gotmp, keep := tc.setup(t, home, root, outside)
			out, err := runCleanupSnippet(t, home, gotmp)
			if err == nil {
				t.Fatalf("accepted GOTMPDIR=%q", gotmp)
			}
			if !strings.Contains(out, tc.message) {
				t.Errorf("output %q lacks %q", out, tc.message)
			}
			if _, err := os.Stat(keep); err != nil {
				t.Errorf("deleted outside the approved root: %v", err)
			}
		})
	}
}

// TestPublishedPagesLinkToGateGuideSource pins that pages present on the built
// site do not point at the engineering record excluded from the site.
func TestPublishedPagesLinkToGateGuideSource(t *testing.T) {
	const source = "https://github.com/Open-MBEE/OpenSysML/blob/develop/docs/project/test-gate-throughput.md"
	for _, page := range []string{"docs/reference/environment.md", "docs/internals/testing.md"} {
		content := readRepoFile(t, page)
		if strings.Contains(content, "../project/test-gate-throughput.md") {
			t.Errorf("%s links to a page excluded from the site", page)
		}
		if !strings.Contains(content, source) {
			t.Errorf("%s lacks a source link to the gate guide", page)
		}
	}
}

// TestGateGuideIsAccountedForInMkDocs pins that the strict site build sees the
// new engineering record as excluded rather than as an omitted page.
func TestGateGuideIsAccountedForInMkDocs(t *testing.T) {
	config := readRepoFile(t, "mkdocs.yml")
	page := "project/test-gate-throughput.md"
	count := strings.Count(config, page)
	nav := strings.Index(config, "\nnav:")
	if count != 1 || nav < 0 || strings.Index(config, page) > nav {
		t.Errorf("%s appears %d times; want once in the pre-nav exclusions", page, count)
	}
}
