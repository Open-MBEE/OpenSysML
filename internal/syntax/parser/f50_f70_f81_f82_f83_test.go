package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func parseFeatureFix(t *testing.T, name, input string) (*ast.RootNamespace, []Diagnostic) {
	t.Helper()
	p := New(source.New(name, []byte(input)))
	root := p.ParseFile()
	return root, p.Diagnostics
}

func TestF50F70F81F82F83AndF62F63Parse(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
		// file overrides the default `<name>.sysml`, for KerML-only notation.
		file string
	}{
		{
			"variable_and_member",
			"package P { classifier C; classifier D { abstract var feature x [0..*]; member abstract feature y [0..*] featured by C; } }",
			"variable=true",
			"",
		},
		{
			"variable_before_modifier",
			"package P { classifier D { var abstract feature x; } }",
			"variable=true",
			"",
		},
		{
			"representation_keywords_as_names",
			"package P { classifier D { feature rep; feature language; } }",
			`name="language"`,
			"",
		},
		{
			"textual_representation",
			"package P { classifier C { inv check { rep inOCL language \"ocl\" /* body */ } } }",
			`TextualRepresentation language="\"ocl\"" name="inOCL"`,
			"",
		},
		{
			"differences",
			"package P { classifier A; classifier B; classifier D differences A, B; }",
			`kind="differences"`,
			"",
		},
		// A keyword-first Disjoining is a KerML member of its own
		// (KerML.xtext:426) with ordered ends; SysML.xtext has no such member.
		{
			"namespace_disjoint",
			"package P { classifier A; classifier B; disjoint B from A; }",
			`(RelationshipMember visibility="default" kind="disjoint" name="" keyword="disjoint" source="B" target="A"`,
			"namespace_disjoint.kerml",
		},
		{
			"global_disjoint_end",
			"package P { classifier A; classifier B; disjoint $::P::A from B; }",
			`(RelationshipMember visibility="default" kind="disjoint" name="" keyword="disjoint" source="$::P::A" target="B"`,
			"global_disjoint_end.kerml",
		},
		{
			"keyword_named_disjoint_end",
			"package P { classifier B; feature part : B; disjoining part disjoint part from B; }",
			`(RelationshipMember visibility="default" kind="disjoint" name="part" keyword="disjoint" source="part" target="B"`,
			"keyword_named_disjoint_end.kerml",
		},
		{
			"classifier_multiplicity",
			"package P { classifier A; classifier B [1] specializes A; }",
			"(Multiplicity",
			"",
		},
		{
			"metaclass_multiplicity",
			"package P { classifier A; metaclass S [1] specializes A; }",
			`(Definition kind="metaclass"`,
			"",
		},
		{
			"exhibit_chain",
			"package P { state def S { state on; } part vehicle : S { exhibit vehicleStates.on; } }",
			"FeatureChainExpr",
			"",
		},
		{
			"reference_body",
			"package P { part medicalDevice { ref patient { event occurrence therapyDelayed; } } }",
			`name="patient"`,
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := tt.file
			if file == "" {
				file = tt.name + ".sysml"
			}
			root, diags := parseFeatureFix(t, file, tt.src)
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(diags) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if dump := ast.Dump(root); !strings.Contains(dump, tt.want) {
				t.Fatalf("AST dump does not contain %q:\n%s", tt.want, dump)
			}
		})
	}
}

func TestNegativeF50F70F81F82F83AndF62F63(t *testing.T) {
	tests := []struct {
		name string
		src  string
		// file overrides the default `<name>.sysml`, for notation whose
		// malformedness is KerML-specific.
		file string
	}{
		// `var` is a KerML keyword (KerML.xtext BasicFeaturePrefix), so it
		// cannot name the feature there; in SysML it is an ordinary name and
		// the reference accepts `abstract var;` as a DefaultReferenceUsage.
		{"abstract_var", "package P { abstract var; }", "abstract_var.kerml"},
		{"member", "package P { classifier D { member ; } }", ""},
		{"representation", "package P { classifier D { inv c { rep language; } } }", ""},
		{"differences", "package P { classifier D differences ; }", ""},
		{"disjoint", "package P { disjoint from A; }", ""},
		{"disjoint_missing_end", "package P { disjoint from A; }", "disjoint_missing_end.kerml"},
		{"disjoint_missing_from", "package P { disjoint B A; }", "disjoint_missing_from.kerml"},
		{"disjoint_missing_target", "package P { disjoint B from ; }", "disjoint_missing_target.kerml"},
		{"disjoining_without_disjoint", "package P { disjoining D B from A; }", "disjoining_without_disjoint.kerml"},
		// SysML.xtext declares no keyword-first Disjoining, so the member is
		// refused there as `subtype A specializes B;` is.
		{"sysml_disjoint_member", "package P { part def A; part def B; disjoint B from A; }", ""},
		{"sysml_disjoint_body_member", "package P { part def A; part def B; part def C { disjoint B from A; } }", ""},
		{"sysml_subset_body_member", "package P { part def A; part def C { part x : A; part y : A; subset x subsets y; } }", ""},
		{"multiplicity", "package P { classifier B [ specializes A; }", ""},
		{"sysml_definition_multiplicity", "package P { part def P [1] :> A; }", ""},
		{"sysml_attribute_definition_multiplicity", "package P { attribute def A [1]; }", ""},
		{"sysml_calc_definition_multiplicity", "package P { calc def C [1] { return x; } }", ""},
		{"exhibit", "package P { part v { exhibit .on { } } }", ""},
		{"reference", "package P { part v { ref { } } }", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			file := tt.file
			if file == "" {
				file = tt.name + ".sysml"
			}
			root, diags := parseFeatureFix(t, file, tt.src)
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(diags) == 0 {
				t.Fatal("expected a diagnostic")
			}
		})
	}
}

func TestF83ClassifierMultiplicitySyntaxBoundary(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"classifier", "package P { classifier S [1] specializes A; }"},
		{"class", "package P { class S [1] specializes A; }"},
		{"datatype", "package P { datatype S [1] specializes A; }"},
		{"struct", "package P { struct S [1] specializes A; }"},
		{"assoc", "package P { assoc S [1] specializes A; }"},
		{"behavior", "package P { behavior S [1] specializes A; }"},
		{"function", "package P { function S [1] specializes A; }"},
		{"predicate", "package P { predicate S [1] specializes A; }"},
		{"interaction", "package P { interaction S [1] specializes A; }"},
		{"metaclass", "package P { metaclass S [1] specializes A; }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, diags := parseFeatureFix(t, tt.name+".kerml", tt.src)
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(diags) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
		})
	}
}

func TestNoPanicF50F70F81F82F83AndF62F63Truncated(t *testing.T) {
	sources := []string{
		"package P { classifier D { abstract var feature x [0..*]; } }",
		"package P { classifier D { inv c { rep inOCL language \"ocl\" /* body */ } } }",
		"package P { classifier D differences A, B; }",
		"package P { disjoint B from A; }",
		"package P { classifier B [1] specializes A; }",
		"package P { part v { exhibit vehicleStates.on { doc /* body */ } } }",
		"package P { part v { ref patient { event occurrence therapyDelayed; } } }",
	}
	for _, src := range sources {
		for i := 0; i <= len(src); i++ {
			prefix := src[:i]
			t.Run("", func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic at prefix %d: %v", i, r)
					}
				}()
				root, _ := parseFeatureFix(t, "truncated.sysml", prefix)
				if root == nil {
					t.Fatal("ParseFile returned nil")
				}
			})
		}
	}
}
