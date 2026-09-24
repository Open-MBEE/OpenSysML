package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSuite writes one scenario file into a fresh directory.
func writeSuite(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestLoadScenariosReadsFilesInOrder verifies scenarios arrive in file then
// declaration order, which is what makes a run's report stable.
func TestLoadScenariosReadsFilesInOrder(t *testing.T) {
	dir := writeSuite(t, "02-second.json", `{"scenarios":[
		{"id":"b","rpc":"Evaluate","request":{},"expect":{}},
		{"id":"c","rpc":"Evaluate","request":{},"expect":{}}]}`)
	first := `{"scenarios":[{"id":"a","rpc":"GetServerInfo","request":{},"expect":{}}]}`
	if err := os.WriteFile(filepath.Join(dir, "01-first.json"), []byte(first), 0o600); err != nil {
		t.Fatal(err)
	}

	scenarios, err := loadScenarios(dir)
	if err != nil {
		t.Fatalf("loadScenarios failed: %v", err)
	}
	var ids []string
	for _, scenario := range scenarios {
		ids = append(ids, scenario.ID)
	}
	want := []string{"a", "b", "c"}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v, want %v", ids, want)
		}
	}
}

// TestLoadScenariosRejectsUnknownFields verifies a misspelled expectation is an
// error, since silently ignoring it would leave the scenario asserting nothing.
func TestLoadScenariosRejectsUnknownFields(t *testing.T) {
	dir := writeSuite(t, "s.json", `{"scenarios":[
		{"id":"a","rpc":"Evaluate","request":{},"expect":{"respones":{}}}]}`)
	if _, err := loadScenarios(dir); err == nil {
		t.Fatal("an unknown expectation field was accepted")
	}
}

// TestLoadScenariosRejectsDuplicateIDs verifies two scenarios cannot share an
// id, which a report and a -run filter both address them by.
func TestLoadScenariosRejectsDuplicateIDs(t *testing.T) {
	dir := writeSuite(t, "s.json", `{"scenarios":[
		{"id":"a","rpc":"Evaluate","request":{},"expect":{}},
		{"id":"a","rpc":"Evaluate","request":{},"expect":{}}]}`)
	if _, err := loadScenarios(dir); err == nil {
		t.Fatal("a duplicate scenario id was accepted")
	}
}

// TestLoadScenariosRequiresIDAndRPC verifies a scenario naming no RPC is an
// error rather than a scenario that runs nothing.
func TestLoadScenariosRequiresIDAndRPC(t *testing.T) {
	dir := writeSuite(t, "s.json", `{"scenarios":[{"id":"a","request":{},"expect":{}}]}`)
	if _, err := loadScenarios(dir); err == nil {
		t.Fatal("a scenario without an rpc was accepted")
	}
}

// TestLoadScenariosRejectsAnEmptyDirectory verifies an empty suite fails rather
// than passing with nothing run.
func TestLoadScenariosRejectsAnEmptyDirectory(t *testing.T) {
	if _, err := loadScenarios(t.TempDir()); err == nil {
		t.Fatal("a suite with no scenario files was accepted")
	}
}

// TestScenarioMethodAcceptsEitherSpelling verifies a scenario may name its RPC
// bare or qualified, since the qualified form is one transport's spelling.
func TestScenarioMethodAcceptsEitherSpelling(t *testing.T) {
	for _, rpc := range []string{"Evaluate", "sysml.SysMLService/Evaluate", "/sysml.SysMLService/Evaluate"} {
		scenario := &Scenario{RPC: rpc}
		if got := scenario.method(); got != "Evaluate" {
			t.Errorf("method(%q) = %q, want Evaluate", rpc, got)
		}
	}
}
