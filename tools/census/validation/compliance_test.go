package validation

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/census/manifest"
)

const compliancePage = "# Compliance\n\n" +
	"Prose may cite `validateA` and the helper `validateHelper`.\n\n" +
	"| Semantic Rule | Implementation | Test Case | Status |\n" +
	"|---|---|---|---|\n" +
	"| `validateA` holds | `a.go` `validateHelper` | t | ✅ Faithful |\n" +
	"| `validateB_` holds (raw spelling) | b | t | ⚠️ Approximate |\n" +
	"| `validateCFormal` by its formal name | c | t | ⚠️ Approximate |\n" +
	"| a sub-case of `validateA` (census faithful), beside `validateD` (census unknown; unobservable) | d | probes `validateD.sysml` (census unknown) | 🧩 Extension |\n" +
	"| the specification's `spec:validateLib` | e | t | ⚠️ Approximate |\n" +
	"| a probe `tools/census/validation/testdata/probes/validateD.sysml` (census unknown) and the helper `checker.validateHelper` | f | p | ✅ Faithful |\n"

func complianceManifest() *manifest.Manifest {
	rowB := testRow("validateB", "kerml", manifest.StatusApproximate)
	rowB.Raw = "validateB_"
	rowC := testRow("validateC", "kerml", manifest.StatusApproximate)
	rowC.Spec = &manifest.SpecRef{Document: "kerml", Clause: "8.3.2.1.2", Name: "validateCFormal"}
	return testManifest(testRow("validateA", "kerml", manifest.StatusFaithful), rowB, rowC, testRow("validateD", "sysml", manifest.StatusUnknown))
}

func complianceRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestRepo(t, root, map[string]string{
		validationLibraryPath:        "library package SysMLValidation {\n    constraint def validateLib {\n        in element : KerML::Feature;\n        true\n    }\n}\n",
		"internal/check/passes/a.go": "package passes\n\ntype checker struct{}\n\nfunc (c *checker) validateHelper() {}\n\nfunc validateOther[T any](x T) {}\n",
	})
	return root
}

// TestCheckComplianceAcceptsTheConventions: the pilot name, the raw identifier,
// the formal document's name, a `(census …)` reference stating the right status,
// a `spec:` name the library states, a probe path and a Go helper all pass.
func TestCheckComplianceAcceptsTheConventions(t *testing.T) {
	if err := checkCompliance(complianceRoot(t), compliancePage, complianceManifest()); err != nil {
		t.Fatalf("a consistent page must pass: %v", err)
	}
}

// TestCheckComplianceRejectsDrift: each gate fails on a mutated page or manifest
// and names the line.
func TestCheckComplianceRejectsDrift(t *testing.T) {
	root := complianceRoot(t)
	mutate := func(old, replacement string) string {
		if strings.Count(compliancePage, old) != 1 {
			t.Fatalf("%q must occur once in the page", old)
		}
		return strings.Replace(compliancePage, old, replacement, 1)
	}
	pageCases := []struct{ name, page, want string }{
		{"unknown name", mutate("`validateA` holds", "`validateAa` holds"), ":7: `validateAa` is neither a constraint"},
		{"unknown name in prose", mutate("Prose may cite `validateA`", "Prose may cite `validateNope`"), ":3: `validateNope` is neither a constraint"},
		{"typo in a raw spelling", mutate("`validateB_`", "`validateBb_`"), ":8: `validateBb_` is neither"},
		{"row status differs from the census", mutate("| t | ✅ Faithful |", "| t | ⚠️ Approximate |"), ":7: a ⚠ row cites `validateA`, whose census status is faithful"},
		{"formal name claims the wrong status", mutate("| c | t | ⚠️ Approximate |", "| c | t | ✅ Faithful |"), ":9: a ✅ row cites `validateCFormal`, whose census status is approximate"},
		{"extension row claims a pilot constraint", mutate("`validateA` (census faithful)", "`validateA`"), ":10: a 🧩 row cites `validateA`, whose census status is faithful"},
		{"reference states the wrong status", mutate("`validateA` (census faithful)", "`validateA` (census approximate)"), ":10: `validateA` is cited as (census approximate) but the census row is faithful"},
		{"probe reference states the wrong status", mutate("`validateD.sysml` (census unknown)", "`validateD.sysml` (census faithful)"), ":10: `validateD` is cited as (census faithful) but the census row is unknown"},
		{"spec name the library does not state", mutate("`spec:validateLib`", "`spec:validateLibrary`"), ":11: `spec:validateLibrary` is not a constraint of SysMLValidation.sysml"},
		{"spec name cited bare", mutate("`spec:validateLib`", "`validateLib`"), ":11: `validateLib` is neither a constraint"},
		{"a span is checked whole", mutate("`validateA` holds", "`validateA-extra` holds"), ":7: `validateA-extra` is not a constraint citation"},
		{"a brace shorthand is no citation", mutate("`validateA` holds", "`validateA{One,Two}` holds"), ":7: `validateA{One,Two}` is not a constraint citation"},
		{"a prose name is taken whole", mutate("Prose may cite `validateA`", "Prose may cite validateA-extra"), ":3: `validateA-extra` is neither a constraint"},
		{"a reference covers its own citation only", mutate("| a sub-case of `validateA` (census faithful), beside", "| `validateA` bare and a sub-case of `validateA` (census faithful), beside"), ":10: a 🧩 row cites `validateA`, whose census status is faithful"},
		{"a reference must follow the citation", mutate("`validateD.sysml` (census unknown)", "`validateD.sysml` and (census unknown)"), ":10: a 🧩 row cites `validateD`, whose census status is unknown"},
	}
	for _, tc := range pageCases {
		err := checkCompliance(root, tc.page, complianceManifest())
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want an error containing %q, got %v", tc.name, tc.want, err)
		}
	}
	for _, tc := range []struct{ name, constraint, status, want string }{
		{"manifest row moves under a claiming row", "validateA", manifest.StatusApproximate, ":7: a ✅ row cites `validateA`, whose census status is approximate"},
		{"manifest row moves under a reference", "validateD", manifest.StatusFaithful, ":10: `validateD` is cited as (census unknown) but the census row is faithful"},
	} {
		m := complianceManifest()
		for i := range m.Constraints {
			if m.Constraints[i].Name == tc.constraint {
				m.Constraints[i].Status = tc.status
				m.Constraints[i].Rationale = "moved"
			}
		}
		err := checkCompliance(root, compliancePage, m)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want an error containing %q, got %v", tc.name, tc.want, err)
		}
	}
}

// TestValidateAcceptsAFormalNameThePilotMisspells: a spec name differing from
// the row's name is kept, unless it is another row's name or no constraint name.
func TestValidateAcceptsAFormalNameThePilotMisspells(t *testing.T) {
	row := testRow("validateViewDefinitionOnlyOnvViewRendering", "sysml", manifest.StatusFaithful)
	row.Spec = &manifest.SpecRef{Document: "kerml", Clause: "8.3.26.7", Name: "validateViewDefinitionOnlyOneViewRendering"}
	if err := testManifest(row).Validate(); err != nil {
		t.Fatalf("a formal name must validate: %v", err)
	}
	other := testRow("validateViewDefinitionOnlyOneViewRendering", "sysml", manifest.StatusFaithful)
	if err := testManifest(row, other).Validate(); err == nil || !strings.Contains(err.Error(), "own name") {
		t.Fatalf("a spec name that is another row's name must be refused, got %v", err)
	}
	row.Spec.Name = "ViewRendering"
	if err := testManifest(row).Validate(); err == nil || !strings.Contains(err.Error(), "not a constraint name") {
		t.Fatalf("a spec name that is no constraint name must be refused, got %v", err)
	}
}

// TestCheckComplianceLeavesSpecSourcedRowsUnbound: a row citing a spec-sourced
// constraint resolves the name but is bound to no census status, since such a
// row is outside the census and states no verdict.
func TestCheckComplianceLeavesSpecSourcedRowsUnbound(t *testing.T) {
	specRow := testRow("validateSpecOnly", manifest.SourceSpec, manifest.StatusUnknown)
	specRow.Spec.Name = "validateSpecOnly"
	m := complianceManifest()
	m.Constraints = append(m.Constraints, specRow)
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	page := compliancePage + "| the documents' `validateSpecOnly`, not yet assessed | g | t | ✅ Faithful |\n"
	if err := checkCompliance(complianceRoot(t), page, m); err != nil {
		t.Fatalf("a row citing a spec-sourced constraint must not be held to its ❔: %v", err)
	}
}

// TestCheckComplianceRefusesTheSpecPrefixOnAPilotConstraint: `spec:` is for a
// library constraint outside the census; on a pilot constraint the library also
// states it would bypass the status gate, so it is refused.
func TestCheckComplianceRefusesTheSpecPrefixOnAPilotConstraint(t *testing.T) {
	root := complianceRoot(t)
	writeTestRepo(t, root, map[string]string{
		validationLibraryPath: "library package SysMLValidation {\n    constraint def validateLib {\n        in element : KerML::Feature;\n        true\n    }\n    constraint def validateD {\n        in element : KerML::Feature;\n        true\n    }\n}\n",
	})
	page := compliancePage + "| the library's `spec:validateD` | h | t | ✅ Faithful |\n"
	err := checkCompliance(root, page, complianceManifest())
	if err == nil || !strings.Contains(err.Error(), ":13: `spec:validateD` names a pilot constraint; cite it bare") {
		t.Fatalf("spec: on a pilot constraint must be refused, got %v", err)
	}
}
