package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A kernelspec's name is one directory under the kernels directory; a path, or
// a name beginning with a dot, would put the spec elsewhere.
func TestAKernelspecNameIsOneDirectoryName(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../escape", "a/b", `a\b`, ".hidden"} {
		if _, err := kernelspecOf(&options{name: name, displayName: "x"}); err == nil {
			t.Errorf("kernelspecOf accepted -name %q", name)
		}
	}
	for _, name := range []string{"sysml", "SysML2", "sysml-dev", "sysml.v2_x"} {
		if _, err := kernelspecOf(&options{name: name, displayName: "x"}); err != nil {
			t.Errorf("kernelspecOf refused -name %q: %v", name, err)
		}
	}
}

func TestInstallWritesTheSpecAndTheLogosWhereJupyterLooks(t *testing.T) {
	data := t.TempDir()
	t.Setenv("JUPYTER_DATA_DIR", data)
	opts := &options{name: "sysml-test", displayName: "SysML (test)"}
	spec, err := kernelspecOf(opts)
	if err != nil {
		t.Fatal(err)
	}
	if code := installKernelspec(opts, spec); code != 0 {
		t.Fatalf("installKernelspec = %d", code)
	}
	dir := filepath.Join(data, "kernels", "sysml-test")
	raw, err := os.ReadFile(filepath.Join(dir, "kernel.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got kernelspec
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "SysML (test)" || got.Language != "sysml" || got.InterruptMode != "message" || len(got.Argv) != 3 {
		t.Errorf("kernel.json = %+v", got)
	}
	for _, name := range []string{"logo-32x32.png", "logo-64x64.png"} {
		want, err := logos.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		written, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(written, want) || !bytes.HasPrefix(written, []byte("\x89PNG")) {
			t.Errorf("%s was not written as embedded", name)
		}
	}
}
