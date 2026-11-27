package gobuild

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// Scratch is the directory a test binary builds its commands into, each once. Main removes
// it after the run and an interrupt removes it at once; what a killed run left is removed
// by the next run that uses the same Pattern.
type Scratch struct {
	// Pattern names the directory; the owning process and a random suffix follow it.
	Pattern string

	once sync.Once
	dir  string
	err  error

	mu     sync.Mutex
	builds map[string]*build
}

// build is one command built into a Scratch.
type build struct {
	once sync.Once
	path string
	err  error
}

// Main runs the tests of a package, removes the directory and exits with the result. m is
// the *testing.M a TestMain is handed.
func (s *Scratch) Main(m interface{ Run() int }) {
	code := m.Run()
	if err := s.Remove(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	os.Exit(code)
}

// Dir returns the directory, made on first use.
func (s *Scratch) Dir() (string, error) {
	s.once.Do(func() {
		prefix := s.Pattern + "-"
		removeAbandoned(os.TempDir(), prefix)
		s.dir, s.err = os.MkdirTemp("", prefix+strconv.Itoa(os.Getpid())+"-*")
		if s.err == nil {
			s.removeOnInterrupt()
		}
	})
	return s.dir, s.err
}

// Build builds the package in pkgDir into the directory as name, once however often it is
// asked for, and returns the binary's path. extra is passed to `go build`.
func (s *Scratch) Build(name, pkgDir string, extra ...string) (string, error) {
	s.mu.Lock()
	if s.builds == nil {
		s.builds = map[string]*build{}
	}
	b := s.builds[name]
	if b == nil {
		b = &build{}
		s.builds[name] = b
	}
	s.mu.Unlock()
	b.once.Do(func() {
		dir, err := s.Dir()
		if err != nil {
			b.err = err
			return
		}
		b.path = filepath.Join(dir, name)
		cmd := exec.Command("go", Args(b.path, extra...)...) // #nosec G204 -- the toolchain, on a package of this module
		cmd.Dir = pkgDir
		if out, err := cmd.CombinedOutput(); err != nil {
			b.err = fmt.Errorf("go build %s: %v\n%s", name, err, out)
		}
	})
	return b.path, b.err
}

// Remove deletes the directory and what was built into it; a Scratch never used has none.
func (s *Scratch) Remove() error {
	s.once.Do(func() {})
	if s.dir == "" {
		return nil
	}
	err := os.RemoveAll(s.dir)
	interrupted.mu.Lock()
	delete(interrupted.dirs, s.dir)
	interrupted.mu.Unlock()
	return err
}

// interrupted holds every scratch directory still to remove, and the one handler that
// removes them all: a handler per directory would exit before the others had run.
var interrupted struct {
	mu     sync.Mutex
	dirs   map[string]bool
	handle sync.Once
}

// removeOnInterrupt removes the directory when the run is interrupted or terminated, which
// would otherwise end the process before Main could.
func (s *Scratch) removeOnInterrupt() {
	interrupted.mu.Lock()
	if interrupted.dirs == nil {
		interrupted.dirs = map[string]bool{}
	}
	interrupted.dirs[s.dir] = true
	interrupted.mu.Unlock()
	interrupted.handle.Do(func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-signals
			interrupted.mu.Lock()
			for dir := range interrupted.dirs {
				_ = os.RemoveAll(dir)
			}
			os.Exit(1)
		}()
	})
}

// removeAbandoned deletes the scratch directories under root whose owning process is gone:
// a run killed by its timeout or by SIGKILL cannot remove its own.
func removeAbandoned(root, prefix string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		owner, ok := ownerOf(entry.Name(), prefix)
		if !ok || !entry.IsDir() || processRuns(owner) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(root, entry.Name()))
	}
}

// ownerOf reads the process id out of a scratch directory's name, <prefix><pid>-<random>.
func ownerOf(name, prefix string) (int, bool) {
	rest, ok := strings.CutPrefix(name, prefix)
	if !ok {
		return 0, false
	}
	id, random, ok := strings.Cut(rest, "-")
	if !ok || random == "" || strings.Contains(random, "-") {
		return 0, false
	}
	pid, err := strconv.Atoi(id)
	return pid, err == nil && pid > 0
}

// processRuns reports whether a process with this id exists. Where that cannot be asked
// it answers true, so a directory is only ever removed from under a process known gone.
func processRuns(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	defer func() { _ = process.Release() }()
	if runtime.GOOS == "windows" {
		return true
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
