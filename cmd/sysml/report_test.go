package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// TestUndecidedVerdictTakesTheCommandPrefix checks the boundary the command
// reports a verdict through: whatever the prompt renders a check it could not
// make as, the command reports it under its own prefix, on stderr.
func TestUndecidedVerdictTakesTheCommandPrefix(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  []string
	}{{
		name:  "a line the prompt prefixes is restated in the command's prefix",
		lines: []string{"error: unresolved reference: Nope"},
		want:  []string{"sysml: unresolved reference: Nope"},
	}, {
		name:  "every line of the verdict is restated, not only the first",
		lines: []string{"error: calc invocation failed: Fall", "error: unbound parameter: b"},
		want:  []string{"sysml: calc invocation failed: Fall", "sysml: unbound parameter: b"},
	}, {
		name:  "a line locating a finding in the source keeps its own shape",
		lines: []string{"model.sysml:1:42: error: unresolved reference: Nope"},
		want:  []string{"model.sysml:1:42: error: unresolved reference: Nope"},
	}, {
		name:  "a line that says error of something else is not a prefix",
		lines: []string{"the error: state was never reached"},
		want:  []string{"the error: state was never reached"},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errs bytes.Buffer
			r := &reporter{out: &out, err: &errs}
			r.verdict(repl.Verdict{Subject: "Nope", Status: repl.VerdictUnresolved, Lines: tc.lines})

			want := strings.Join(tc.want, "\n") + "\n"
			if errs.String() != want {
				t.Errorf("stderr = %q, want %q", errs.String(), want)
			}
			if out.String() != "" {
				t.Errorf("a verdict that was never decided should not reach stdout, got %q", out.String())
			}
		})
	}
}

// A result's inputs and assumptions, and the values a witness fixes with the
// file it was written to, reach the JSON report; a witness of choices alone
// reports no inputs and no path.
func TestCheckResultsReportInputsAndWitnessValues(t *testing.T) {
	witness := runtime.Witness{Inputs: []runtime.InputTaken{{Feature: "limit", Written: "-1"}}}
	plan := &analysis.Plan{Steps: []analysis.Step{{
		Engine: analysis.SMTEngineName,
		Result: &analysis.Result{
			Engine:   analysis.SMTEngineName,
			Claim:    analysis.ClaimViolated,
			Strength: analysis.Witnessed,
			Inputs: []analysis.Input{
				{Name: "n", Type: "Integer", Value: "1"},
				{Name: "limit", Type: "Integer", Free: true, Value: "-1"},
				{Name: "u", Type: "Natural", Domain: ">= 0", Free: true},
				{Name: "z", Type: "Integer", Free: true, Optional: true, Value: "null"},
			},
			Assumptions: []string{"constraint wide"},
			Witness:     &analysis.Witness{Schedule: runtime.ReplayOf(witness), Inputs: witness.Inputs, Written: "/tmp/w.witness"},
		},
	}, {
		Engine: analysis.CheckEngineName,
		Result: &analysis.Result{
			Engine:  analysis.CheckEngineName,
			Claim:   analysis.ClaimViolated,
			Witness: &analysis.Witness{Schedule: runtime.ReplayOf(runtime.Witness{})},
		},
	}}}
	got, err := json.Marshal(checkResultsOf(plan))
	if err != nil {
		t.Fatal(err)
	}
	var results []struct {
		Engine string `json:"engine"`
		Inputs []struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Domain   string `json:"domain"`
			Free     bool   `json:"free"`
			Optional bool   `json:"optional"`
			Value    string `json:"value"`
		} `json:"inputs"`
		Assumptions []string `json:"assumptions"`
		Witness     struct {
			Schedule string `json:"schedule"`
			Inputs   []struct {
				Feature string `json:"feature"`
				Value   string `json:"value"`
			} `json:"inputs"`
			Choices []string `json:"choices"`
			Path    string   `json:"path"`
		} `json:"witness"`
	}
	if err := json.Unmarshal(got, &results); err != nil {
		t.Fatal(err)
	}
	smt := results[0]
	if len(smt.Inputs) != 4 || smt.Inputs[0].Free || smt.Inputs[0].Value != "1" ||
		!smt.Inputs[1].Free || smt.Inputs[1].Optional || smt.Inputs[1].Value != "-1" || smt.Inputs[1].Type != "Integer" ||
		smt.Inputs[2].Domain != ">= 0" || smt.Inputs[2].Value != "" ||
		!smt.Inputs[3].Optional || smt.Inputs[3].Value != "null" {
		t.Errorf("inputs = %+v", smt.Inputs)
	}
	if !strings.Contains(string(got), `{"name":"z","type":"Integer","free":true,"optional":true,"value":"null"}`) {
		t.Errorf("an optional input reports optional, and its absence as null: %s", got)
	}
	if strings.Contains(string(got), `"optional":false`) {
		t.Errorf("optional is emitted rather than omitted for a mandatory input: %s", got)
	}
	if len(smt.Assumptions) != 1 || smt.Assumptions[0] != "constraint wide" {
		t.Errorf("assumptions = %v", smt.Assumptions)
	}
	if smt.Witness.Schedule != "replay" || len(smt.Witness.Inputs) != 1 || smt.Witness.Inputs[0].Feature != "limit" ||
		smt.Witness.Inputs[0].Value != "-1" || smt.Witness.Choices == nil || smt.Witness.Path != "/tmp/w.witness" {
		t.Errorf("witness = %+v", smt.Witness)
	}
	if !strings.Contains(string(got), `"witness":{"schedule":"replay","inputs":[{"feature":"limit","value":"-1"}]`) {
		t.Errorf("a witness input is keyed feature and value: %s", got)
	}
	if !strings.Contains(string(got), `"choices":[]`) {
		t.Errorf("a witness of no choice reports choices as []: %s", got)
	}
	check := results[1]
	if check.Inputs != nil || check.Assumptions != nil || check.Witness.Inputs != nil || check.Witness.Path != "" {
		t.Errorf("a result without inputs reports none: %s", got)
	}
	for _, key := range []string{`"inputs":null`, `"assumptions":null`, `"path":""`} {
		if strings.Contains(string(got), key) {
			t.Errorf("%s is emitted rather than omitted: %s", key, got)
		}
	}
}
