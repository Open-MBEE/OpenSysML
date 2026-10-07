package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// The kernelspec's defaults: the directory notebooks bind to by name, and what
// front ends list the kernel under.
const (
	defaultKernelName  = "sysml"
	defaultDisplayName = "SysML v2 (OpenSysML)"
)

// kernelspec is a kernel.json: how Jupyter starts the kernel.
type kernelspec struct {
	Argv          []string          `json:"argv"`
	DisplayName   string            `json:"display_name"`
	Language      string            `json:"language"`
	InterruptMode string            `json:"interrupt_mode"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// kernelspecOf is the kernelspec that starts this binary, as the options name it.
func kernelspecOf(opts *options) (kernelspec, error) {
	if opts.name == "" || opts.name != filepath.Base(opts.name) {
		return kernelspec{}, fmt.Errorf("-name %q must be a directory name", opts.name)
	}
	self, err := os.Executable()
	if err != nil {
		return kernelspec{}, fmt.Errorf("locate this binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	return kernelspec{
		Argv:          []string{self, "-connection-file", "{connection_file}"},
		DisplayName:   opts.displayName,
		Language:      "sysml",
		InterruptMode: "message",
		Metadata:      map[string]string{"implementation": kernelCommand},
	}, nil
}

// printKernelspec writes the kernelspec to stdout.
func printKernelspec(spec kernelspec) int {
	raw, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, errPrefix, err)
		return 1
	}
	fmt.Println(string(raw))
	return 0
}

// installKernelspec writes the kernelspec where Jupyter finds kernels.
func installKernelspec(opts *options, spec kernelspec) int {
	if opts.user && opts.prefix != "" {
		fmt.Fprintf(os.Stderr, "%s -user and -prefix name different places; give one\n", errPrefix)
		return 2
	}
	dir, err := kernelsDir(opts.prefix)
	if err != nil {
		fmt.Fprintln(os.Stderr, errPrefix, err)
		return 2
	}
	target := filepath.Join(dir, opts.name)
	// #nosec G301 G306 -- a kernelspec under --prefix or --system is read by every user's Jupyter.
	if err := os.MkdirAll(target, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, errPrefix, err)
		return 1
	}
	raw, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, errPrefix, err)
		return 1
	}
	// #nosec G306 -- see above.
	if err := os.WriteFile(filepath.Join(target, "kernel.json"), append(raw, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, errPrefix, err)
		return 1
	}
	fmt.Printf("Installed kernelspec %s in %s\n", opts.name, target)
	return 0
}

// kernelsDir is where the kernelspec goes: under the prefix given, or the
// user's Jupyter data directory, which JUPYTER_DATA_DIR overrides.
func kernelsDir(prefix string) (string, error) {
	if prefix != "" {
		return filepath.Join(prefix, "share", "jupyter", "kernels"), nil
	}
	if dir := os.Getenv("JUPYTER_DATA_DIR"); dir != "" {
		return filepath.Join(dir, "kernels"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate the user's Jupyter directory: %w", err)
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Jupyter", "kernels"), nil
	case "windows":
		base := os.Getenv("APPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(base, "jupyter", "kernels"), nil
	default:
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, "jupyter", "kernels"), nil
	}
}
