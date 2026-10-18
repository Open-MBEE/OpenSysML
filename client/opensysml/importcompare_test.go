package opensysml_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const importCompareVehicles = `package V {
	private import ScalarValues::*;
	part def Vehicle {
		attribute count : Integer default 1;
		attribute label : String default "plain";
	}
	part vehicle : Vehicle;
	part car : Vehicle {
		attribute :>> count = 2;
	}
}`

const importCompareProbe = `package Cfg {
	private import ScalarValues::*;
	part def Probe {
		attribute total : Real = 1.0;
		action steady { first start; then assign total := 4.0; then done; }
	}
	action def 'Group 1' { part target : Probe; perform action run ::> target.steady; }
}`

const importCompareResults = `{"source":"probe.xmi","configurations":[
 {"id":"_c1","name":"Cfg::'Group 1'","runs":2,"target":"target","behavior":"run","observables":["total"],
  "snapshots":[{"id":"_r1","values":{"total":4.0}},{"id":"_r2","values":{"total":6.0}}]},
 {"id":"_cn","name":"Cfg::Nothing","observables":[],"snapshots":[]}]}`

func TestImportDataAgainstService(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, importCompareVehicles)
	ctx := context.Background()
	result, err := client.ImportData(ctx, model, []opensysml.DataImport{
		{Name: "fleet.csv", Content: "element,count,label\nV::vehicle,7,fast\nV::car,9,\n"},
		{Name: "labels.txt", Content: `[{"element":"V::car","label":"slow"}]`, Format: "json"},
	})
	if err != nil {
		t.Fatalf("ImportData: %v", err)
	}
	var descriptions []string
	for _, file := range result.Files {
		for _, a := range file.Assignments {
			descriptions = append(descriptions, a.Description)
		}
	}
	want := []string{
		"V::vehicle::count = 7 (redefines V::Vehicle::count)",
		`V::vehicle::label = "fast" (redefines V::Vehicle::label)`,
		"V::car::count = 9",
		`V::car::label = "slow" (redefines V::Vehicle::label)`,
	}
	if strings.Join(descriptions, "\n") != strings.Join(want, "\n") {
		t.Errorf("assignments:\n%s\nwant:\n%s", strings.Join(descriptions, "\n"), strings.Join(want, "\n"))
	}
	if len(result.Files) != 2 || result.Files[0].Values != 3 || result.Files[0].Elements != 2 || result.Files[1].Name != "labels.txt" {
		t.Errorf("files = %+v; want fleet.csv (3 values, 2 elements) then labels.txt", result.Files)
	}
	if result.Files[0].Assignments[0].Redefines != "V::Vehicle::count" || result.Files[0].Assignments[0].Where == "" {
		t.Errorf("first assignment = %+v; want it to redefine V::Vehicle::count and say where", result.Files[0].Assignments[0])
	}
	if !strings.Contains(result.Content, "attribute :>> count = 7;") || !strings.Contains(result.Content, `attribute :>> label = "slow";`) {
		t.Errorf("content:\n%s\nwant the values set", result.Content)
	}
	if result.ModelHash == "" || result.ModelHash == model.Hash || len(result.Documents) != 1 {
		t.Errorf("model hash %q (from %q), %d documents; want a new one-document model", result.ModelHash, model.Hash, len(result.Documents))
	}
	imported := &opensysml.Model{Hash: result.ModelHash}
	if sym, err := client.LookupSymbol(ctx, imported, "V::car"); err != nil || sym.Name != "car" {
		t.Errorf("the imported model does not resolve V::car: %v %+v", err, sym)
	}

	dry, err := client.ImportData(ctx, model, []opensysml.DataImport{
		{Name: "fleet.tsv", Content: "element\tcount\nV::vehicle\t7\n"},
	}, opensysml.ImportDryRun())
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if len(dry.Files) != 1 || len(dry.Files[0].Assignments) != 1 || dry.ModelHash != "" || dry.Content != "" || len(dry.Documents) != 0 {
		t.Errorf("dry run = %+v; want the one assignment and the model untouched", dry)
	}

	_, err = client.ImportData(ctx, model, []opensysml.DataImport{
		{Name: "fleet.csv", Content: "element,count\nV::car,9\nV::Nowhere,1\n"},
	})
	var failure *opensysml.FailureError
	if !errors.As(err, &failure) || !strings.Contains(failure.Message, "V::Nowhere") {
		t.Errorf("an unknown element: %v, want a FailureError naming V::Nowhere", err)
	}
	_, err = client.ImportData(ctx, model, nil)
	var status *opensysml.StatusError
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
		t.Errorf("no imports: %v, want INVALID_ARGUMENT", err)
	}
}

func TestCompareResultsAgainstService(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, importCompareProbe)
	ctx := context.Background()
	comparison, err := client.CompareResults(ctx, model, importCompareResults, opensysml.CompareSeed(7))
	if err != nil {
		t.Fatalf("CompareResults: %v", err)
	}
	if comparison.Source != "probe.xmi" || comparison.Holds() || len(comparison.Configurations) != 2 {
		t.Fatalf("comparison = %+v; want probe.xmi's two configurations, one unresolved", comparison)
	}
	group, nothing := comparison.Configurations[0], comparison.Configurations[1]
	if group.Name != "Cfg::'Group 1'" || group.Status != "holds" || group.Runs != 2 || group.Completed != 2 || group.StoredRuns != 2 || group.Seed == nil || *group.Seed != 7 || len(group.Rows) != 2 {
		t.Errorf("group = %+v; want 2 of 2 seeded runs beside 2 stored", group)
	}
	if len(group.Observables) != 1 {
		t.Fatalf("observables = %+v; want total", group.Observables)
	}
	total := group.Observables[0]
	if total.Stored != "total" || total.Feature != "target.total" || !total.Compared || total.Tool == nil || total.Run == nil || total.Difference == nil {
		t.Fatalf("total = %+v; want it compared with both distributions", total)
	}
	if total.Tool.Count != 2 || total.Tool.Mean != 5 || total.Run.Mean != 4 || total.Difference.Mean != "-20.0%" {
		t.Errorf("total: tool %+v, run %+v, difference %+v", total.Tool, total.Run, total.Difference)
	}
	if len(group.Lines) == 0 || nothing.Status != "unresolved" || nothing.Unresolved == "" {
		t.Errorf("lines %d, nothing = %+v; want tabled lines and an unresolved configuration", len(group.Lines), nothing)
	}

	selected, err := client.CompareResults(ctx, model, importCompareResults, opensysml.CompareActions("Group 1"), opensysml.CompareRuns(1), opensysml.CompareSeed(3))
	if err != nil {
		t.Fatalf("selected: %v", err)
	}
	if len(selected.Configurations) != 1 || selected.Configurations[0].Runs != 1 || !selected.Holds() {
		t.Errorf("selected = %+v; want the one configuration run once", selected.Configurations)
	}

	_, err = client.CompareResults(ctx, model, importCompareResults, opensysml.CompareRuns(-1))
	var status *opensysml.StatusError
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
		t.Errorf("negative runs: %v, want INVALID_ARGUMENT", err)
	}
	_, err = client.CompareResults(ctx, model, "not json")
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
		t.Errorf("malformed results: %v, want INVALID_ARGUMENT", err)
	}
}
