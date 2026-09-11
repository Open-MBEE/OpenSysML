package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// TestNegativePrefixAlternatives pins as syntax errors the prefix spellings the pinned
// grammars forbid; each case cites its production and the token the error sits on.
func TestNegativePrefixAlternatives(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		src     string
		at      string
		message string
	}{
		// composite and portion are alternatives (KerML.xtext:514 BasicFeaturePrefix).
		{"composite_portion", "a.kerml", "package P { class A { composite portion feature j; } }", "portion",
			"'portion' cannot follow 'composite': a prefix says 'composite' or 'portion', not both"},
		{"portion_composite", "a.kerml", "package P { class A { portion composite feature j; } }", "composite",
			"'composite' cannot follow 'portion': a prefix says 'composite' or 'portion', not both"},
		// abstract and variation are alternatives (SysML.xtext:490 BasicDefinitionPrefix).
		{"abstract_variation_definition", "a.sysml", "package P { abstract variation part def V; }", "variation",
			"'variation' cannot follow 'abstract': a prefix says 'abstract' or 'variation', not both"},
		{"variation_abstract_definition", "a.sysml", "package P { variation abstract part def V; }", "abstract",
			"'abstract' cannot follow 'variation': a prefix says 'abstract' or 'variation', not both"},
		// abstract and variation are alternatives (SysML.xtext:556 RefPrefix).
		{"abstract_variation_usage", "a.sysml", "package P { part def V; abstract variation part v : V; }", "variation",
			"'variation' cannot follow 'abstract': a prefix says 'abstract' or 'variation', not both"},
		{"variation_abstract_usage", "a.sysml", "package P { part def V; variation abstract part v : V; }", "abstract",
			"'abstract' cannot follow 'variation': a prefix says 'abstract' or 'variation', not both"},
		// A direction is read at most once (KerML.xtext:506 FeatureDirection, SysML.xtext:554).
		{"in_out_sysml", "a.sysml", "package P { port def Q { in out item x; } }", "out",
			"'out' cannot follow 'in': a prefix says one direction ('in', 'out' or 'inout'), not both"},
		{"in_out_kerml", "a.kerml", "package P { class Q { in out feature x; } }", "out",
			"'out' cannot follow 'in': a prefix says one direction ('in', 'out' or 'inout'), not both"},
		{"inout_in_parameter", "a.sysml", "package P { action def A { inout in x : Integer; } }", "in",
			"'in' cannot follow 'inout': a prefix says one direction ('in', 'out' or 'inout'), not both"},
		// DefinitionPrefix has no ref; only BasicUsagePrefix does (SysML.xtext:498, :563).
		{"ref_definition", "a.sysml", "package P { ref part def R; }", "ref",
			"'ref' is a usage prefix: a definition prefix admits only 'abstract' or 'variation'"},
		// constant is RefPrefix only (SysML.xtext:560); a variant is a usage (SysML.xtext:700 VariantUsageElement).
		{"constant_definition", "a.sysml", "package P { constant part def D; }", "constant",
			"'constant' is a usage prefix: a definition prefix admits only 'abstract' or 'variation'"},
		{"variant_definition", "a.sysml", "package P { variation part def V { variant part def D; } }", "variant",
			"'variant' is a usage prefix: a definition prefix admits only 'abstract' or 'variation'"},
		// TypePrefix admits abstract only (KerML.xtext:313); const/derived are BasicFeaturePrefix (KerML.xtext:514).
		{"const_class_kerml", "a.kerml", "package P { const class C; }", "const",
			"'const' is a usage prefix: a definition prefix admits only 'abstract'"},
		{"derived_struct_kerml", "a.kerml", "package P { derived struct S; }", "derived",
			"'derived' is a usage prefix: a definition prefix admits only 'abstract'"},
		{"in_datatype_kerml", "a.kerml", "package P { in datatype T; }", "in",
			"'in' is a usage prefix: a definition prefix admits only 'abstract'"},
		// variation is a SysML prefix (SysML.xtext:491, :559); no KerML.xtext prefix spells it.
		{"variation_class_kerml", "a.kerml", "package P { variation class C; }", "variation",
			"`variation` is SysML notation: the KerML grammar has no such prefix, so move the declaration to a .sysml file"},
		{"variation_feature_kerml", "a.kerml", "package P { class A { variation feature f; } }", "variation",
			"`variation` is SysML notation: the KerML grammar has no such prefix, so move the declaration to a .sysml file"},
		// A crossing feature's prefix is the same BasicFeaturePrefix (KerML.xtext:514, :602 OwnedCrossingFeature).
		{"cross_feature_in_out", "a.sysml", "package P { assoc A { end in out x : T feature e; } }", "out",
			"'out' cannot follow 'in': a prefix says one direction ('in', 'out' or 'inout'), not both"},
		{"cross_feature_composite_portion", "a.kerml", "package P { assoc A { end composite portion x : T feature e; } }", "portion",
			"'portion' cannot follow 'composite': a prefix says 'composite' or 'portion', not both"},
		{"cross_feature_abstract_variation", "a.sysml", "package P { assoc A { end abstract variation x : T feature e; } }", "variation",
			"'variation' cannot follow 'abstract': a prefix says 'abstract' or 'variation', not both"},
		// A bad escape is reported once the token is consumed, so the crossing-feature
		// try-parse that reads it first and restores does not lose it (KerMLExpressions.xtext UNRESTRICTED_NAME).
		{"bad_escape_after_cross_feature_attempt", "a.sysml", "package P { interface def I { end <'bad \\q'>; } }", "\\q",
			"invalid escape '\\q' in an unrestricted name: only \\b \\t \\n \\f \\r \\\" \\' \\\\ are admitted"},
		{"bad_escape_in_end_type", "a.sysml", "package P { assoc A { end x : 'bad \\q'; } }", "\\q",
			"invalid escape '\\q' in an unrestricted name: only \\b \\t \\n \\f \\r \\\" \\' \\\\ are admitted"},
		// A metadata name follows # (SysML.xtext:127 PrefixMetadataAnnotation, KerML.xtext:1073 PrefixMetadataMember).
		{"prefix_metadata_keyword_sysml", "a.sysml", "package P { # part def D; }", "#",
			"expected a metadata feature name after '#': 'part' is a keyword"},
		{"prefix_metadata_keyword_kerml", "a.kerml", "package P { # namespace N; }", "#",
			"expected a metadata feature name after '#': 'namespace' is a keyword"},
		// A bound is a literal or feature reference (KerML.xtext:780 MultiplicityExpressionMember).
		{"negative_multiplicity_bound", "a.sysml", "package P { part def D; part many [-1] : D; }", "-",
			"a multiplicity bound cannot start with '-': a bound is a literal or a feature name (KerML.xtext MultiplicityExpressionMember)"},
		{"negative_multiplicity_upper_bound", "a.sysml", "package P { part def D; part many [0..-1] : D; }", "-",
			"a multiplicity bound cannot start with '-': a bound is a literal or a feature name (KerML.xtext MultiplicityExpressionMember)"},
		// KerML spells the prefix const (KerML.xtext:514 BasicFeaturePrefix isConstant ?= 'const').
		{"constant_in_kerml", "a.kerml", "package P { constant feature n; }", "constant",
			"`constant` is SysML notation: the KerML grammar spells the prefix `const`, so write `const` here or move the declaration to a .sysml file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := source.New(tt.file, []byte(tt.src))
			p := New(sf)
			if root := p.ParseFile(); root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) != 1 {
				t.Fatalf("want one diagnostic for %q, got %v", tt.src, p.Diagnostics)
			}
			d := p.Diagnostics[0]
			if d.Message != tt.message {
				t.Errorf("message = %q, want %q", d.Message, tt.message)
			}
			if got := sf.Text(d.Span); got != tt.at {
				t.Errorf("diagnostic sits on %q, want %q", got, tt.at)
			}
		})
	}
}

// TestPrefixAlternativesStillAccepted keeps the single-keyword forms and the
// arithmetic multiplicity-bound extension parsing clean.
func TestPrefixAlternativesStillAccepted(t *testing.T) {
	tests := []struct {
		name string
		file string
		src  string
	}{
		{"composite", "a.kerml", "package P { class A { composite feature j; } }"},
		{"portion", "a.kerml", "package P { class A { portion feature j; } }"},
		{"const", "a.kerml", "package P { class A { const feature n; } }"},
		{"abstract_class_kerml", "a.kerml", "package P { abstract class C; }"},
		{"individual_definition", "a.sysml", "package P { individual part def D; }"},
		{"variant_usage", "a.sysml", "package P { variation part def V { variant part v; } }"},
		{"cross_feature_direction", "a.kerml", "package P { assoc A { end in x : T feature e; } }"},
		{"abstract_definition", "a.sysml", "package P { abstract part def V; }"},
		{"variation_definition", "a.sysml", "package P { variation part def V; }"},
		{"variation_usage", "a.sysml", "package P { part def V; variation part v : V; }"},
		{"inout", "a.sysml", "package P { port def Q { inout item x; } }"},
		{"ref_usage", "a.sysml", "package P { part def R; ref part r : R; }"},
		{"prefix_metadata_name", "a.sysml", "package P { metadata def M; #M part def D; }"},
		{"prefix_metadata_quoted_keyword", "a.sysml", "package P { metadata def 'part'; #'part' part def D; }"},
		{"multiplicity_feature_bound", "a.sysml", "package P { attribute n : Integer; part def D; part many [n] : D; }"},
		{"multiplicity_arithmetic_bound", "a.sysml", "package P { attribute n : Integer; part def D; part many [n+1] : D; }"},
		{"multiplicity_range", "a.sysml", "package P { part def D; part many [0..*] : D; }"},
		// A constraint body is a CalculationBody, so it reads the transition extension a calc body does.
		{"transition_in_constraint_def", "a.sysml", "package P { constraint def C { action a; action b; transition first a then b; } }"},
		{"transition_in_constraint_usage", "a.sysml", "package P { constraint c { action a; action b; transition first a then b; } }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(source.New(tt.file, []byte(tt.src)))
			if root := p.ParseFile(); root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
			}
		})
	}
}

// TestNegativeBodyContext pins members admitted by one body kind only as syntax errors
// naming the construct and that body (SysML.xtext StateBodyItem vs DefinitionBodyItem).
func TestNegativeBodyContext(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		at      string
		message string
	}{
		{"transition_in_part_def", "package P { part def D { state s1; state s2; transition first s1 then s2; } }", "transition",
			"'transition' declares a transition between states and is only allowed in a state body; move it into the state it belongs to"},
		{"transition_in_part_usage", "package P { part d { state s1; state s2; transition first s1 then s2; } }", "transition",
			"'transition' declares a transition between states and is only allowed in a state body; move it into the state it belongs to"},
		{"transition_in_package", "package P { state s1; state s2; transition first s1 then s2; }", "transition",
			"'transition' declares a transition between states and is only allowed in a state body; move it into the state it belongs to"},
		// A member that fails to parse inside an enumeration body is reported, not dereferenced.
		{"enum_direction_only_member", "package P { enum def E { in; } }", ";",
			"expected a feature after 'in': write `in <name> : <Type>`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := source.New(tt.name+".sysml", []byte(tt.src))
			p := New(sf)
			if root := p.ParseFile(); root == nil {
				t.Fatal("ParseFile returned nil")
			}
			var found bool
			for _, d := range p.Diagnostics {
				if d.Message == tt.message && sf.Text(d.Span) == tt.at {
					found = true
				}
			}
			if !found {
				t.Fatalf("want %q at %q, got %v", tt.message, tt.at, p.Diagnostics)
			}
		})
	}
}
