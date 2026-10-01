//go:build sysml_prod

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func init() { cliBuildFlags = []string{"-tags", "sysml_prod"} }

// TestProdBuild checks the sysml_prod binary refuses the flags of the groups it
// leaves out and keeps them out of -help. The REPL keeps %print, %save and
// %query and -query stays a core flag; only %features ... json is left out.
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
		for _, flag := range []string{"-migrate", "-compile", "-sync-diff", "-cpuprofile"} {
			if strings.Contains(got.stdout, "\n  "+flag+" ") {
				t.Errorf("-help lists %s, which the build leaves out", flag)
			}
		}
		if !strings.Contains(got.stdout, "\n  -query ") {
			t.Errorf("-help does not list -query, which the build keeps:\n%s", got.stdout)
		}
	})

	t.Run("repl", func(t *testing.T) {
		saved := filepath.Join(t.TempDir(), "out.sysml")
		got := runBinary(t, binary,
			"%help\n%print\n%save "+saved+"\n%query sysml:name=%22A%22\n%features A json\n", []string{model})
		if got.status != 0 {
			t.Fatalf("the prompt exited %d:\n%s%s", got.status, got.stdout, got.stderr)
		}
		out := got.stdout + got.stderr
		for _, cmd := range []string{"%print", "%save", "%query"} {
			if !strings.Contains(out, cmd+" ") {
				t.Errorf("%%help does not list %s:\n%s", cmd, out)
			}
			if strings.Contains(out, `unknown command "`+cmd+`"`) {
				t.Errorf("%s is an unknown command:\n%s", cmd, out)
			}
		}
		data, err := os.ReadFile(saved)
		if err != nil || !strings.Contains(string(data), "part def A") {
			t.Errorf("%%save wrote %v (%v), want the session model", data, err)
		}
		// The build leaves out only %features ... json: %features itself is
		// listed, and its help offers no json form.
		featuresHelp := out[strings.Index(out, "%features"):]
		if i := strings.Index(featuresHelp, "\n"); i >= 0 {
			featuresHelp = featuresHelp[:i]
		}
		if strings.Contains(featuresHelp, "json") {
			t.Errorf("%%features help offers json, which the build leaves out:\n%s", featuresHelp)
		}
	})
}
