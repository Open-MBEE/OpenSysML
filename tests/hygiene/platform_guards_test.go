package hygiene

import (
	"bufio"
	"go/build/constraint"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestUnixFilesAreGuardedByUnix checks that a *_unix.go file is selected by `unix`: a negative
// guard such as `!windows && !wasm` also selects it on Plan 9, where it cannot compile.
func TestUnixFilesAreGuardedByUnix(t *testing.T) {
	cmd := exec.Command("git", "ls-files", "--", "*_unix.go")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	files := strings.Fields(string(out))
	if len(files) == 0 {
		t.Fatal("no *_unix.go file is tracked; the check has nothing to read")
	}
	for _, file := range files {
		expr := buildConstraint(t, filepath.Join("../..", file))
		if expr == nil {
			t.Errorf("%s has no //go:build line; it must be constrained to `unix`", file)
			continue
		}
		offUnix := func(tag string) bool { return tag == "plan9" || tag == "amd64" }
		if expr.Eval(offUnix) {
			t.Errorf("%s: `//go:build %s` selects the file on plan9; constrain it with `unix`", file, expr)
		}
	}
}

// buildConstraint returns the //go:build expression of a Go file, or nil when it has none.
func buildConstraint(t *testing.T, path string) constraint.Expr {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "package ") {
			break
		}
		if !constraint.IsGoBuild(line) {
			continue
		}
		expr, err := constraint.Parse(line)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		return expr
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return nil
}

// TestSourcePackageBuildsOffUnix compiles the standard-library-only internal/syntax/source for
// Plan 9, whose syscall package lacks error numbers every other target shares.
func TestSourcePackageBuildsOffUnix(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-compiles a package")
	}
	cmd := exec.Command("go", "build", "./internal/syntax/source")
	cmd.Dir = "../.."
	cmd.Env = append(os.Environ(), "GOOS=plan9", "GOARCH=amd64", "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("GOOS=plan9 go build ./internal/syntax/source: %v\n%s", err, out)
	}
}
