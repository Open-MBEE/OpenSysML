package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// -keep-line-breaks makes a notation → notation conversion keep the author's
// line breaks, as a save does, and is refused with any other conversion.
func TestConvertKeepLineBreaksKeepsAOneLineBody(t *testing.T) {
	binary := buildCLI(t)
	const oneLiner = "package P { part def A; }\n"
	model := filepath.Join(t.TempDir(), "p.sysml")
	if err := os.WriteFile(model, []byte(oneLiner), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run(t, binary, model, "-convert", "sysml"); got != "package P {\n    part def A;\n}\n" {
		t.Errorf("-convert sysml wrote %q, want the statement on its own line", got)
	}
	if got := run(t, binary, model, "-convert", "sysml", "-keep-line-breaks"); got != oneLiner {
		t.Errorf("-convert sysml -keep-line-breaks wrote %q, want the one-liner unchanged", got)
	}
	if got := run(t, binary, model, "-convert", "kerml", "-keep-line-breaks"); got != oneLiner {
		t.Errorf("-convert kerml -keep-line-breaks wrote %q, want the one-liner unchanged", got)
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"without -convert", []string{model, "-keep-line-breaks"}, "-keep-line-breaks requires -convert sysml or kerml"},
		{"with -convert ttl", []string{model, "-convert", "ttl", "-keep-line-breaks"}, "-keep-line-breaks applies to -convert sysml or kerml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := exec.Command(binary, tc.args...).CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 {
				t.Fatalf("exit = %v, want status 2\n%s", err, out)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Errorf("stderr = %q, want it to say %q", out, tc.want)
			}
		})
	}
}
