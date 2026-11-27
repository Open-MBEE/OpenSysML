package gobuild

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// Args only reads the environment: the runtime's variable is whatever it was before the call.
func TestArgsChangesNoEnvironment(t *testing.T) {
	t.Setenv(DirEnv, t.TempDir())
	t.Setenv(runtimeDirEnv, "as it was")
	args := Args("out")
	if !slices.Contains(args, "-cover") {
		t.Errorf("a coverage run builds without -cover: %v", args)
	}
	if got := os.Getenv(runtimeDirEnv); got != "as it was" {
		t.Errorf("Args set %s to %q", runtimeDirEnv, got)
	}

	t.Setenv(DirEnv, "")
	if args := Args("out", "-trimpath"); !slices.Equal(args, []string{"build", "-trimpath", "-o", "out", "."}) {
		t.Errorf("outside a coverage run: %v", args)
	}
}

// The counter directory reaches the runtime's variable through exportDir alone.
func TestExportDirHandsTheCounterDirectoryToTheRuntime(t *testing.T) {
	t.Setenv(DirEnv, "")
	t.Setenv(runtimeDirEnv, "as it was")
	if err := exportDir(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(runtimeDirEnv); got != "as it was" {
		t.Errorf("outside a coverage run %s became %q", runtimeDirEnv, got)
	}
	dir := t.TempDir()
	t.Setenv(DirEnv, dir)
	if err := exportDir(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(runtimeDirEnv); got != dir {
		t.Errorf("%s = %q, want %q", runtimeDirEnv, got, dir)
	}
}

// A Scratch is one directory however often it is asked for, and Remove leaves nothing behind.
func TestScratchIsRemoved(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	var unused Scratch
	if err := unused.Remove(); err != nil {
		t.Errorf("removing a scratch never used: %v", err)
	}
	scratch := Scratch{Pattern: "gobuild-scratch"}
	dir, err := scratch.Dir()
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := scratch.Dir(); again != dir {
		t.Errorf("a second directory %s beside %s", again, dir)
	}
	if err := os.WriteFile(filepath.Join(dir, "built"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := scratch.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("%s is still there: %v", dir, err)
	}
}

// TestScratchBuildsEachCommandOnce pins that a command asked for twice is built once, into
// the directory Remove deletes.
func TestScratchBuildsEachCommandOnce(t *testing.T) {
	scratch := Scratch{Pattern: "gobuild-test-build"}
	pkgDir := filepath.Join("testdata", "hello")
	first, err := scratch.Build("hello", pkgDir)
	if err != nil {
		t.Fatal(err)
	}
	built, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := scratch.Build("hello", pkgDir)
	if err != nil {
		t.Fatal(err)
	}
	again, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	if second != first || !again.ModTime().Equal(built.ModTime()) {
		t.Errorf("built again: %s at %v, then %s at %v", first, built.ModTime(), second, again.ModTime())
	}
	if dir, _ := scratch.Dir(); filepath.Dir(first) != dir {
		t.Errorf("%s is not in the scratch directory %s", first, dir)
	}
	if err := scratch.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Errorf("%s is still there: %v", first, err)
	}
}

// TestScratchReportsAFailedBuild pins that a build that fails is an error, not a path.
func TestScratchReportsAFailedBuild(t *testing.T) {
	scratch := Scratch{Pattern: "gobuild-test-failed"}
	t.Cleanup(func() { _ = scratch.Remove() })
	if path, err := scratch.Build("absent", filepath.Join("testdata", "absent")); err == nil {
		t.Errorf("building a package that is not there gave %s", path)
	}
}

// TestRemoveAbandonedKeepsWhatARunningProcessOwns pins which directories a new run removes:
// those named for a process that is gone, and nothing else.
func TestRemoveAbandonedKeepsWhatARunningProcessOwns(t *testing.T) {
	gone := exec.Command(os.Args[0], "-test.run=^$")
	if err := gone.Run(); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	const prefix = "builds-"
	abandoned := prefix + strconv.Itoa(gone.Process.Pid) + "-1"
	kept := []string{
		prefix + strconv.Itoa(os.Getpid()) + "-1",
		"other-" + strconv.Itoa(gone.Process.Pid) + "-1",
		prefix + "unowned",
		prefix + strconv.Itoa(gone.Process.Pid) + "-more-1",
	}
	for _, name := range append([]string{abandoned}, kept...) {
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	file := prefix + strconv.Itoa(gone.Process.Pid) + "-2"
	if err := os.WriteFile(filepath.Join(root, file), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	removeAbandoned(root, prefix)

	if _, err := os.Stat(filepath.Join(root, abandoned)); !os.IsNotExist(err) {
		t.Errorf("%s, whose process is gone, is still there: %v", abandoned, err)
	}
	for _, name := range append(kept, file) {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("%s was removed: %v", name, err)
		}
	}
}

// scratchHolderEnv makes this test binary hold a Scratch directory until it is signalled.
const scratchHolderEnv = "GOBUILD_TEST_HOLD_SCRATCH"

// TestScratchHolder is the process TestScratchIsRemovedOnInterrupt signals: it prints the
// scratch directories it still holds, having removed one already, and waits.
func TestScratchHolder(t *testing.T) {
	if os.Getenv(scratchHolderEnv) == "" {
		t.Skip("run by TestScratchIsRemovedOnInterrupt")
	}
	removed := Scratch{Pattern: "gobuild-test-held"}
	if _, err := removed.Dir(); err != nil {
		t.Fatal(err)
	}
	if err := removed.Remove(); err != nil {
		t.Fatal(err)
	}
	held := make([]Scratch, 16)
	dirs := make([]string, len(held))
	for i := range held {
		held[i].Pattern = "gobuild-test-held"
		dir, err := held[i].Dir()
		if err != nil {
			t.Fatal(err)
		}
		dirs[i] = dir
	}
	fmt.Println(strings.Join(dirs, string(os.PathListSeparator)))
	select {}
}

// TestScratchIsRemovedOnInterrupt pins that an interrupted run, whose TestMain never gets
// to remove its directories, leaves none of them.
func TestScratchIsRemovedOnInterrupt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a process cannot be sent an interrupt on Windows")
	}
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			holder := exec.Command(os.Args[0], "-test.run=^TestScratchHolder$")
			holder.Env = append(os.Environ(), scratchHolderEnv+"=1")
			out, err := holder.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := holder.Start(); err != nil {
				t.Fatal(err)
			}
			line, err := bufio.NewReader(out).ReadString('\n')
			if err != nil {
				_ = holder.Process.Kill()
				t.Fatal(err)
			}
			dirs := strings.Split(strings.TrimSpace(line), string(os.PathListSeparator))
			for _, dir := range dirs {
				if _, err := os.Stat(dir); err != nil {
					_ = holder.Process.Kill()
					t.Fatalf("the holder's directory: %v", err)
				}
			}
			if err := holder.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			if err := holder.Wait(); err == nil {
				t.Error("the interrupted run reported success")
			}
			for _, dir := range dirs {
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Errorf("%s is still there: %v", dir, err)
				}
			}
		})
	}
}
