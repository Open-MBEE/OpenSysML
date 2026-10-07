package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
