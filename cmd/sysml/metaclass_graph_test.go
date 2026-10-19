package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMetaclassGraphIsItsOwnRun pins that -metaclass-graph reads no model,
// stands in for no other mode and never writes over an input file.
func TestMetaclassGraphIsItsOwnRun(t *testing.T) {
	binary := buildCLI(t)

	graph := runCommand(t, exec.Command(binary, "-metaclass-graph"))
	wantReport(t, graph, 0, "sysml:PartUsage", "rdfs:subClassOf sysml:ItemUsage", "sysx:abstract")

	dir := t.TempDir()
	model := filepath.Join(dir, "m.sysml")
	const source = "package P;\n"
	if err := os.WriteFile(model, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	wantReport(t, runCommand(t, exec.Command(binary, model, "-metaclass-graph")),
		2, "ask for it without model files")
	wantReport(t, runCommand(t, exec.Command(binary, model, "-convert", "ttl", "-metaclass-graph", "-o", model)),
		2, "ask for it without model files")
	wantReport(t, runCommand(t, exec.Command(binary, "-metaclass-graph", "-convert", "ttl")),
		2, "ask for it in its own run")
	wantReport(t, runCommand(t, exec.Command(binary, "-metaclass-graph", "-query", "P")),
		2, "ask for it in its own run")
	wantReport(t, runCommand(t, exec.Command(binary, "-metaclass-graph", "-html-default-css")),
		2, "ask for it in its own run")
	wantReport(t, runCommand(t, exec.Command(binary, "-metaclass-graph", "-from", "kerml")),
		2, "the input, repository and document options do not apply")
	if got, err := os.ReadFile(model); err != nil || string(got) != source {
		t.Fatalf("the model file changed: %q, %v", got, err)
	}

	out := filepath.Join(dir, "metaclasses.ttl")
	wantReport(t, runCommand(t, exec.Command(binary, "-metaclass-graph", "-o", out)), 0)
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != graph.stdout {
		t.Errorf("-o writes the bytes that stdout carries; they differ")
	}
	if !strings.Contains(string(written), "a rdfs:Class") {
		t.Errorf("the written graph names no rdfs:Class:\n%s", written)
	}
}
