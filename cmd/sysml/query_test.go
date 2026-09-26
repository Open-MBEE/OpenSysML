package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueryFlagIdentifiesElements(t *testing.T) {
	binary := buildCLI(t)
	model := filepath.Join(t.TempDir(), "model.sysml")
	const queryModel = `package Demo { part def Wheel; part wheel : Wheel; }`
	if err := os.WriteFile(model, []byte(queryModel), 0o644); err != nil {
		t.Fatal(err)
	}
	out := run(t, binary, "-query", `oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name`, model)
	if !strings.Contains(out, "Demo::wheel  PartUsage") {
		t.Fatalf("query output = %s", out)
	}
}

func TestQueryFlagIdentifiesUnnamedSatisfyAndItsEnds(t *testing.T) {
	binary := buildCLI(t)
	model := filepath.Join(t.TempDir(), "model.sysml")
	const queryModel = `package Demo {
		part def Toaster;
		requirement def EnergyReq { subject t : Toaster; }
		requirement r : EnergyReq { subject t : Toaster; }
		part t : Toaster;
		assert satisfy r by t;
	}`
	if err := os.WriteFile(model, []byte(queryModel), 0o644); err != nil {
		t.Fatal(err)
	}
	out := run(t, binary, "-query", `oslc.where=rdf:type="SatisfyRequirementUsage"&oslc.select=sysml:satisfiedRequirement,sysml:satisfyingFeature`, model)
	want := "Demo::@4  SatisfyRequirementUsage  sysml:satisfiedRequirement=Demo::r  sysml:satisfyingFeature=Demo::t"
	if !strings.Contains(out, want) {
		t.Fatalf("query output = %s, want it to contain %q", out, want)
	}
}

func TestQueryFlagReportsNoMatchOnStderr(t *testing.T) {
	binary := buildCLI(t)
	model := filepath.Join(t.TempDir(), "model.sysml")
	const queryModel = `package Demo { part def Wheel; part wheel : Wheel; }`
	if err := os.WriteFile(model, []byte(queryModel), 0o644); err != nil {
		t.Fatal(err)
	}
	outcome := runCommand(t, exec.Command(binary, "-query", `sysml:name="spare"`, model))
	if outcome.status != 0 {
		t.Fatalf("outcome = %#v", outcome)
	}
	if !strings.Contains(outcome.stderr, "no elements matched") {
		t.Fatalf("stderr = %q, want a no-match report", outcome.stderr)
	}
	if strings.Contains(outcome.stdout, "no elements matched") {
		t.Fatalf("stdout = %q, want it to carry only matches", outcome.stdout)
	}
}

func TestQueryFlagRejectsEmptyQueryText(t *testing.T) {
	binary := buildCLI(t)
	model := filepath.Join(t.TempDir(), "model.sysml")
	const queryModel = `package Demo { part def Wheel; part wheel : Wheel; }`
	if err := os.WriteFile(model, []byte(queryModel), 0o644); err != nil {
		t.Fatal(err)
	}
	outcome := runCommand(t, exec.Command(binary, "-query", "", model))
	if outcome.status != 2 || !strings.Contains(outcome.stderr, "-query is empty") {
		t.Fatalf("outcome = %#v", outcome)
	}

	// -version reports the build whatever else the run asked for.
	outcome = runCommand(t, exec.Command(binary, "-version", "-query", "", model))
	if outcome.status != 0 || !strings.Contains(outcome.stdout, "sysml ") {
		t.Fatalf("version outcome = %#v", outcome)
	}
}

func TestQueryFlagRefusesWildcardValue(t *testing.T) {
	binary := buildCLI(t)
	model := filepath.Join(t.TempDir(), "model.sysml")
	const queryModel = `package Demo { part def Wheel; part wheel : Wheel[*]; }`
	if err := os.WriteFile(model, []byte(queryModel), 0o644); err != nil {
		t.Fatal(err)
	}
	outcome := runCommand(t, exec.Command(binary, "-query", `sysml:name=*`, model))
	if outcome.status == 0 || !strings.Contains(outcome.stderr, "value wildcards are not implemented") {
		t.Fatalf("outcome = %#v", outcome)
	}
	out := run(t, binary, "-query", `oslc.where=sysml:multiplicityUpper%3D*&oslc.select=sysml:multiplicityUpper`, model)
	if !strings.Contains(out, "Demo::wheel  PartUsage  sysml:multiplicityUpper=*") {
		t.Fatalf("query output = %s", out)
	}
}

func TestQueryFlagRejectsConversion(t *testing.T) {
	binary := buildCLI(t)
	model := filepath.Join(t.TempDir(), "model.sysml")
	const queryModel = `package Demo { part def Wheel; part wheel : Wheel; }`
	if err := os.WriteFile(model, []byte(queryModel), 0o644); err != nil {
		t.Fatal(err)
	}
	outcome := runCommand(t, exec.Command(binary, "-query", `rdf:type="PartUsage"`, "-convert", "ttl", model))
	if outcome.status == 0 || !strings.Contains(outcome.stderr, "mutually exclusive") {
		t.Fatalf("outcome = %#v", outcome)
	}
}
