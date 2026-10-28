package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// TestMetamodelTableIsItsOwnRun pins that -metamodel-table writes the committed
// table as JSON, reads no model, stands in for no other mode and never writes
// over an input file.
func TestMetamodelTableIsItsOwnRun(t *testing.T) {
	binary := buildCLI(t)

	run := runCommand(t, exec.Command(binary, "-metamodel-table"))
	wantReport(t, run, 0, `"pilotTag"`, `"definingClass": "Element"`, `"name": "PartUsage"`)
	var table ontology.Table
	if err := json.Unmarshal([]byte(run.stdout), &table); err != nil {
		t.Fatalf("the output is not the table's JSON: %v", err)
	}
	if table.Metamodel != ontology.Version || table.PilotTag != ontology.SourceTag || table.PilotCommit != ontology.SourceCommit {
		t.Errorf("header = %s %s %s, want the table's", table.Metamodel, table.PilotTag, table.PilotCommit)
	}
	if len(table.Classes) != len(ontology.Classes()) || len(table.Properties) != len(ontology.Properties()) ||
		len(table.Enumerations) != len(ontology.Enumerations()) {
		t.Errorf("counts = %d classes, %d properties, %d enumerations; want the table's",
			len(table.Classes), len(table.Properties), len(table.Enumerations))
	}

	dir := t.TempDir()
	model := filepath.Join(dir, "m.sysml")
	const source = "package P;\n"
	if err := os.WriteFile(model, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	wantReport(t, runCommand(t, exec.Command(binary, model, "-metamodel-table")),
		2, "ask for it without model files")
	wantReport(t, runCommand(t, exec.Command(binary, "-metamodel-table", "-convert", "ttl")),
		2, "ask for it in its own run")
	wantReport(t, runCommand(t, exec.Command(binary, "-metamodel-table", "-metaclass-graph")),
		2, "ask for it in its own run")
	wantReport(t, runCommand(t, exec.Command(binary, "-metamodel-table", "-from", "kerml")),
		2, "the input, repository and document options do not apply")
	if got, err := os.ReadFile(model); err != nil || string(got) != source {
		t.Fatalf("the model file changed: %q, %v", got, err)
	}

	out := filepath.Join(dir, "metamodel.json")
	wantReport(t, runCommand(t, exec.Command(binary, "-metamodel-table", "-o", out)), 0)
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != run.stdout {
		t.Errorf("-o writes the bytes that stdout carries; they differ")
	}
}
