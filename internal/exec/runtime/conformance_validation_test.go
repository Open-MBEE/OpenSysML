package runtime

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// The validation part of the execution-conformance harness: what an instance
// fixture's `validation` key pins, and the check of a report against it.

// ExpectedValidation is the report of validating an instance as a whole: each verdict in
// order, whether the object is shown valid, whether the walk was bounded, the unreadable values.
type ExpectedValidation struct {
	Verdicts []ExpectedObjectVerdict `json:"verdicts"`
	Valid    *bool                   `json:"valid,omitempty"`
	Bounded  bool                    `json:"bounded,omitempty"`
	Unread   []string                `json:"unread,omitempty"`
}

// ExpectedObjectVerdict is one verdict of a validation: the assertion's kind and
// text, the object's path from the root (empty for the root), status and error text.
type ExpectedObjectVerdict struct {
	Kind      string `json:"kind"`
	Assertion string `json:"assertion"`
	Object    string `json:"object,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	// Verifications are the runs of the verification cases verifying the requirement
	// the verdict is about, in order; absent, they are not checked, empty, there are none.
	Verifications []ExpectedVerificationRun `json:"verifications,omitempty"`
}

// ExpectedVerificationRun is one run of a verification case beside a validation verdict: the case,
// the path of the object it ran on (empty for the root), its verdict, and the run's objective/subcase verdicts.
type ExpectedVerificationRun struct {
	Case          string            `json:"case"`
	Object        string            `json:"object,omitempty"`
	Verdict       string            `json:"verdict"`
	VerdictDetail string            `json:"verdictDetail,omitempty"`
	Verdicts      map[string]string `json:"verdicts,omitempty"`
	Subcases      map[string]string `json:"subcases,omitempty"`
}

// validateValidation checks what validating the instance as a whole reports
// against what the case states, when it states it: the verdicts in order.
func validateValidation(t *testing.T, ctx *Context, scopes []*symbols.Scope, inst *Instance, expected *ExpectedValidation) {
	t.Helper()
	if expected == nil {
		return
	}
	report, err := ctx.ValidateObject(inst, scopes)
	if err != nil {
		t.Fatalf("validation: %v", err)
	}
	if expected.Valid != nil && report.Valid() != *expected.Valid {
		t.Errorf("validation: valid = %v, want %v", report.Valid(), *expected.Valid)
	}
	if report.Bounded != expected.Bounded {
		t.Errorf("validation: bounded = %v, want %v", report.Bounded, expected.Bounded)
	}
	if len(report.Unread) != len(expected.Unread) {
		t.Errorf("validation: %d unread %v, want %d %v", len(report.Unread), report.Unread, len(expected.Unread), expected.Unread)
	} else {
		for i, want := range expected.Unread {
			requireError(t, fmt.Sprintf("validation unread %d", i+1), report.Unread[i], want)
		}
	}
	var got []string
	for _, v := range report.Verdicts {
		got = append(got, fmt.Sprintf("%s %q on %q: %s", v.Kind, v.Text, strings.Join(v.Path, "."), v.Status))
	}
	if len(report.Verdicts) != len(expected.Verdicts) {
		t.Errorf("validation: %d verdict(s), want %d:\n%s", len(report.Verdicts), len(expected.Verdicts), strings.Join(got, "\n"))
		return
	}
	for i, want := range expected.Verdicts {
		v := report.Verdicts[i]
		wantText := fmt.Sprintf("%s %q on %q: %s", want.Kind, want.Assertion, want.Object, want.Status)
		if got[i] != wantText {
			t.Errorf("validation verdict %d: %s, want %s", i+1, got[i], wantText)
			continue
		}
		if v.Element == nil {
			t.Errorf("validation verdict %d: names no element", i+1)
		}
		if v.Subject == nil {
			t.Errorf("validation verdict %d: names no subject", i+1)
		}
		switch {
		case want.Error != "":
			requireError(t, fmt.Sprintf("validation verdict %d", i+1), v.Err, want.Error)
		case v.Status == ValidationHolds && v.Err != nil:
			t.Errorf("validation verdict %d holds yet carries %v", i+1, v.Err)
		case v.Status != ValidationHolds && v.Err == nil:
			t.Errorf("validation verdict %d is %s yet carries no error", i+1, v.Status)
		}
		if want.Verifications != nil {
			checkVerdictVerifications(t, ctx, scopes, inst, i+1, v, want.Verifications)
		}
	}
}

// checkVerdictVerifications runs the verification cases verifying a validation verdict's
// requirement, as a report does beside the verdict, and checks each run against the outcome in order.
func checkVerdictVerifications(t *testing.T, ctx *Context, scopes []*symbols.Scope, root *Instance, n int, v ObjectVerdict, want []ExpectedVerificationRun) {
	t.Helper()
	label := fmt.Sprintf("validation verdict %d", n)
	var results []VerificationResult
	if v.Requirement != nil {
		results = ctx.VerificationResultsOn(scopes, v.Requirement, v.Subject)
	}
	paths := heldObjectPaths(ctx, root)
	var got []string
	for _, r := range results {
		var path []string
		if r.Verdict.Subject != nil {
			path = paths[r.Verdict.Subject.ID]
		}
		got = append(got, fmt.Sprintf("%s on %q: %s", r.Verdict.Case, strings.Join(path, "."), r.Verdict.Kind))
	}
	if len(results) != len(want) {
		t.Errorf("%s: %d verification run(s), want %d:\n%s", label, len(results), len(want), strings.Join(got, "\n"))
		return
	}
	for i, w := range want {
		r := results[i]
		run := fmt.Sprintf("%s verification %d", label, i+1)
		wantText := fmt.Sprintf("%s on %q: %s", w.Case, w.Object, w.Verdict)
		if got[i] != wantText {
			t.Errorf("%s: %s (%s), want %s", run, got[i], r.Verdict.Detail, wantText)
			continue
		}
		if w.VerdictDetail != "" && !strings.Contains(r.Verdict.Detail, w.VerdictDetail) {
			t.Errorf("%s: detail = %q, want it to contain %q", run, r.Verdict.Detail, w.VerdictDetail)
		}
		checkSubcaseVerdicts(t, r.Subcases, w.Subcases)
		if r.Verdict.Kind != verdictError {
			checkAnalysisVerdicts(t, run, r.Run.Verdicts, w.Verdicts)
		}
	}
}

// caseDocumentRoots are the root scopes of a case's model, its own document first,
// which a validation reads as a report over the documents loaded does.
func caseDocumentRoots(idx *symbols.Index, path string, expected ExpectedOutcome) []*symbols.Scope {
	scopes := []*symbols.Scope{idx.DocumentRoot(path)}
	for _, name := range expected.Documents {
		scopes = append(scopes, idx.DocumentRoot(filepath.Join(filepath.Dir(path), name)))
	}
	return scopes
}

// heldObjectPaths maps each object root holds, by ID, to its path from root as a
// validation verdict names it.
func heldObjectPaths(ctx *Context, root *Instance) map[int64][]string {
	paths := map[int64][]string{}
	for _, obj := range ctx.walkHeldObjects(root, maxMaterializeBudget).objects {
		paths[obj.inst.ID] = obj.path
	}
	return paths
}
