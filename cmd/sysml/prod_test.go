//go:build sysml_prod

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func init() { cliBuildFlags = []string{"-tags", "sysml_prod"} }

// TestProdBuild checks the sysml_prod binary refuses the flags of the groups it
// leaves out, keeps them out of -help, and serves no %print, %save or %query.
func TestProdBuild(t *testing.T) {
	binary := buildCLI(t)
	model := write(t, filepath.Join(t.TempDir(), "m.sysml"), "package P { part def A; }\n")

	t.Run("excluded flags", func(t *testing.T) {
		for _, tc := range []struct {
			args []string
			want string
		}{
			{[]string{"-migrate", "sysml"}, "sysml: -migrate is not available in this build (built without v1)"},
			{[]string{"-compile", "P::A"}, "sysml: -compile is not available in this build (built without codegen)"},
			{[]string{"-sync-diff", "repo.ttl"}, "sysml: -sync-diff is not available in this build (built without sync)"},
			{[]string{"-cpuprofile", "cpu.out"}, "sysml: -cpuprofile is not available in this build (built without profile)"},
		} {
			got := runFiles(t, binary, []string{model}, tc.args...)
			if got.status != 2 || strings.TrimSpace(got.stderr) != tc.want {
				t.Errorf("sysml %v: status %d, stderr %q; want 2, %q", tc.args, got.status, got.stderr, tc.want)
			}
		}
	})

	t.Run("help", func(t *testing.T) {
		got := runBinary(t, binary, "", []string{"-help"})
		if got.status != 0 {
			t.Fatalf("-help: status %d, stderr %q", got.status, got.stderr)
		}
		for _, flag := range []string{"-migrate", "-compile", "-sync-diff", "-cpuprofile", "-query"} {
			if strings.Contains(got.stdout, "\n  "+flag+" ") {
				t.Errorf("-help lists %s, which the build leaves out", flag)
			}
		}
	})

	t.Run("repl", func(t *testing.T) {
		got := runBinary(t, binary, "%help\n%print\n%save out.sysml\n%query x\n", nil)
		help, _, _ := strings.Cut(got.stdout+got.stderr, "unknown command")
		for _, cmd := range []string{"%print", "%save", "%query"} {
			if strings.Contains(help, cmd+" ") || strings.Contains(help, cmd+"\n") {
				t.Errorf("%%help lists %s", cmd)
			}
			if !strings.Contains(got.stdout+got.stderr, `unknown command "`+cmd+`"`) {
				t.Errorf("%s is not an unknown command:\n%s%s", cmd, got.stdout, got.stderr)
			}
		}
	})
}
