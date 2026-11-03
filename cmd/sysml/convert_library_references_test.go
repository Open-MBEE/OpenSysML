package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestConvertLibraryReferencesCLI(t *testing.T) {
	dir := t.TempDir()
	input, table := filepath.Join(dir, "units.sysml"), filepath.Join(dir, "names.json")
	model := []byte(`package P { private import SI::*; attribute time = 20 [s]; }`)
	if err := os.WriteFile(input, model, 0o600); err != nil {
		t.Fatal(err)
	}
	bin := buildCLI(t)
	plain, err := exec.Command(bin, input, "-convert", "api-json").Output()
	if err != nil {
		t.Fatal(err)
	}
	named, err := exec.Command(bin, input, "-convert", "api-json", "-library-references", table).Output()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plain, named) {
		t.Fatal("name side file changed the array")
	}
	data, err := os.ReadFile(table)
	if err != nil {
		t.Fatal(err)
	}
	var refs []map[string]string
	if err := json.Unmarshal(data, &refs); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ref := range refs {
		if ref["qualifiedName"] == "SI::second" && ref["@id"] == "0c539f7c-20a1-520d-bae0-f3f855517aea" {
			found = true
		}
	}
	if !found {
		t.Fatalf("table = %s", data)
	}
	legacy, err := exec.Command(bin, input, "-convert", "api-json", "-include-library-stubs").Output()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(plain, []byte(`"isLibraryElement": true`)) || !bytes.Contains(legacy, []byte(`"isLibraryElement": true`)) {
		t.Fatal("legacy stubs were not opt-in")
	}
	for _, args := range [][]string{
		{input, "-library-references", table},
		{input, "-convert", "ttl", "-include-library-stubs"},
		{input, "-convert", "api-json", "-library-references", ""},
		{input, "-convert", "api-json", "-library-references", input},
		{input, "-convert", "api-json", "-library-references", table, "-o", table},
	} {
		if out, err := exec.Command(bin, args...).CombinedOutput(); err == nil {
			t.Fatalf("accepted %v: %s", args, out)
		}
	}
	got, err := os.ReadFile(input)
	if err != nil || !bytes.Equal(got, model) {
		t.Fatal("side file replaced input")
	}
}
