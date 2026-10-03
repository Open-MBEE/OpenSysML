package codegen

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/hostcap"
)

// Target is a code generation backend.
type Target string

const (
	// TargetC emits C and builds it with the system C compiler.
	TargetC Target = "c"
	// TargetGo emits Go and builds it with the Go toolchain.
	TargetGo Target = "go"
)

// Targets lists the backends in the order they are documented.
func Targets() []Target { return []Target{TargetC, TargetGo} }

// CCompilerEnvVar names the C compiler TargetC drives (default: cc).
const CCompilerEnvVar = "OPENSYSML_CC"

// GoCommandEnvVar names the go command TargetGo drives (default: go).
const GoCommandEnvVar = "OPENSYSML_GO"

func compilerName(target Target) string {
	envVar, defaultName := CCompilerEnvVar, "cc"
	if target == TargetGo {
		envVar, defaultName = GoCommandEnvVar, "go"
	}
	if name := os.Getenv(envVar); name != "" {
		return name
	}
	return defaultName
}

// goCommand resolves name to an absolute executable path once, before it runs.
func goCommand(name string) (string, error) {
	resolved, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	return filepath.Abs(resolved)
}

// CFlags are the C compiler options; the prelude's explicit checks keep -O3 safe,
// and no contraction keeps every Real operation rounded as the interpreter rounds it.
var CFlags = []string{"-O3", "-flto", "-ffp-contract=off", "-std=gnu11", "-Wall", "-Wextra", "-Wno-unused-function", "-Wno-unused-parameter"}

// Source renders program for target, as a complete program with a main.
func Source(p *Program, target Target) ([]byte, error) {
	if p.Target != target {
		return nil, fmt.Errorf("codegen: program compiled for target %q, not %q", p.Target, target)
	}
	var buf bytes.Buffer
	var err error
	switch target {
	case TargetC:
		err = EmitC(&buf, p, true)
	case TargetGo:
		err = EmitGo(&buf, p)
	default:
		return nil, fmt.Errorf("codegen: unknown target %q; targets are c and go", target)
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SourceExtension is the file extension of target's source.
func SourceExtension(target Target) string {
	if target == TargetGo {
		return ".go"
	}
	return ".c"
}

// Build compiles program for target into the executable at output, leaving the
// generated source beside it as output plus the target's extension.
func Build(p *Program, target Target, output string) error {
	src, err := Source(p, target)
	if err != nil {
		return err
	}
	// Check before writing source or looking up a tool the host cannot run.
	if err := hostcap.CheckSpawn(compilerName(target)); err != nil {
		return fmt.Errorf("codegen: %w", err)
	}
	srcPath := output + SourceExtension(target)
	if err := os.WriteFile(srcPath, src, 0o600); err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch target {
	case TargetC:
		cc := compilerName(TargetC)
		args := append(append([]string{}, CFlags...), "-o", output, srcPath, "-lm")
		cmd = exec.Command(cc, args...) // #nosec G204 -- the compiler is the operator's choice, the arguments are ours
	case TargetGo:
		// A module of its own, so the generated program is built like any
		// user's, not as part of this repository.
		dir, err := os.MkdirTemp("", "sysml-codegen-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module sysmlcompiled\n\ngo 1.23\n"), 0o600); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "main.go"), src, 0o600); err != nil {
			return err
		}
		abs, err := filepath.Abs(output)
		if err != nil {
			return err
		}
		goBin, err := goCommand(compilerName(TargetGo))
		if err != nil {
			return fmt.Errorf("codegen: no go command: %w", err)
		}
		cmd = exec.Command(goBin, "build", "-o", abs, ".") // #nosec G204 -- the toolchain is the operator's choice, the arguments are ours
		cmd.Dir = dir
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("codegen: %s failed: %w\n%s", cmd.Args[0], err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
