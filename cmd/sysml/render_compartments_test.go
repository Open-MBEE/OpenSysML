package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderCompartments checks -render-compartments on -render and
// -render-document: most folds every owned usage into compartment rows of
// its owner with the composition edge kept at the owner, ports folds only the
// ports, the default folds nothing of an unpositioned tree, and an unknown
// folding or one with nothing to render is refused.
func TestRenderCompartments(t *testing.T) {
	binary := buildCLI(t)
	fixture := filepath.Join(t.TempDir(), "compartments.sysml")
	if err := os.WriteFile(fixture, []byte(`package CompartmentDemo {
    attribute def Label; attribute def Reading; attribute def Switch;
    port def Hub { port bearing : Hub; }
    part def Sensor { attribute label : Label; attribute reading : Reading; }
    part def Controller { attribute enabled : Switch; part sensor : Sensor; port hub : Hub; }
    view overview { expose CompartmentDemo::Sensor; expose CompartmentDemo::Controller; }
}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	render := func(args ...string) runOutcome {
		t.Helper()
		got := runFiles(t, binary, []string{fixture}, append([]string{"-render", "CompartmentDemo::overview", "-render-form", "dot"}, args...)...)
		if got.status != exitHolds {
			t.Fatalf("-render %v exit status = %d, want %d\n%s", args, got.status, exitHolds, got.output())
		}
		return got
	}
	plain := render()
	if strings.Contains(plain.stdout, "compartment rows") || !strings.Contains(plain.stdout, "<b>sensor</b>") {
		t.Errorf("without -render-compartments every member is a node:\n%s", plain.stdout)
	}
	if positioned := render("-render-compartments", "positioned"); positioned.stdout != plain.stdout {
		t.Errorf("-render-compartments positioned changed an unpositioned tree:\n%s", positioned.stdout)
	}
	most := render("-render-compartments", "most")
	for _, want := range []string{"label : Label<br/>reading : Reading", "enabled : Switch", "sensor : Sensor", "hub : Hub<br/>hub.bearing : Hub", `[headlabel="sensor", dir=back, arrowtail=diamond]`, "// not represented: 6 member(s) drawn as compartment rows of their owner\n"} {
		if !strings.Contains(most.stdout, want) {
			t.Errorf("-render-compartments most lacks %q:\n%s", want, most.stdout)
		}
	}
	if strings.Count(most.stdout, "[label=") != 2 {
		t.Errorf("-render-compartments most draws two nodes:\n%s", most.stdout)
	}
	ports := render("-render-compartments", "ports")
	if !strings.Contains(ports.stdout, "hub : Hub<br/>hub.bearing : Hub") || !strings.Contains(ports.stdout, "<b>sensor</b>") {
		t.Errorf("-render-compartments ports folds the ports and leaves the parts as nodes:\n%s", ports.stdout)
	}

	dir := t.TempDir()
	runFiles(t, binary, []string{fixture}, "-render-all", dir, "-render-form", "mermaid", "-render-compartments", "most")
	all, err := os.ReadFile(filepath.Join(dir, "CompartmentDemo.overview.mmd")) // #nosec G304 -- the test chose this path.
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(all), "sensor : Sensor") {
		t.Errorf("-render-all under -render-compartments most:\n%s", all)
	}

	unknown := runFiles(t, binary, []string{fixture}, "-render", "CompartmentDemo::overview", "-render-form", "dot", "-render-compartments", "rows")
	if unknown.status != exitUnevaluable || !strings.Contains(unknown.stderr, `-render-compartments: unknown compartment folding "rows"; the foldings are positioned, most, ports`) {
		t.Errorf("unknown folding: status %d, stderr %q", unknown.status, unknown.stderr)
	}
	alone := runFiles(t, binary, []string{fixture}, "-render-compartments", "most")
	if alone.status != 2 || !strings.Contains(alone.stderr, "-render-compartments folds a tree's owned members into compartment rows") {
		t.Errorf("folding alone: status %d, stderr %q", alone.status, alone.stderr)
	}
}
