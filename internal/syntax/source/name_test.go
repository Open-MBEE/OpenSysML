package source

import (
	"reflect"
	"slices"
	"testing"
)

func TestQualifiedNameSegments(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"A::B", []string{"A", "B"}},
		{"'Sep::Pkg'::x", []string{"Sep::Pkg", "x"}},
		{`'a\'b'::c`, []string{`a'b`, "c"}},
		{`'a\\b'::'c\td'`, []string{`a\b`, "c\td"}},
		{"x", []string{"x"}},
	} {
		got, ok := QualifiedNameSegments(tc.text)
		if !ok {
			t.Errorf("QualifiedNameSegments(%q) failed", tc.text)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("QualifiedNameSegments(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
	if _, ok := QualifiedNameSegments("A::"); ok {
		t.Error("QualifiedNameSegments(\"A::\") succeeded, want a failure")
	}
}

func TestMetadataPathSegments(t *testing.T) {
	for _, tc := range []struct {
		text       string
		definition []string
		feature    string
		ok         bool
	}{
		{"Meta::tag", []string{"Meta"}, "tag", true},
		{"Meta::'tag with space'", []string{"Meta"}, "tag with space", true},
		{"'My Meta'::tag", []string{"My Meta"}, "tag", true},
		{"Meta::x.y", []string{"Meta"}, "x.y", true},
		{"Meta::'x.y'", []string{"Meta"}, "x.y", true},
		{"Meta::'x::y'", []string{"Meta"}, "x::y", true},
		{"'A::B'::tag", []string{"A::B"}, "tag", true},
		{"Meta::", nil, "", false},
		{"::tag", nil, "", false},
		{"tag", nil, "", false},
	} {
		gotDefinition, gotFeature, ok := MetadataPathSegments(tc.text)
		if ok != tc.ok {
			t.Errorf("MetadataPathSegments(%q) succeeded = %v, want %v", tc.text, ok, tc.ok)
			continue
		}
		if !reflect.DeepEqual(gotDefinition, tc.definition) || gotFeature != tc.feature {
			t.Errorf(
				"MetadataPathSegments(%q) = %v, %q, want %v, %q",
				tc.text, gotDefinition, gotFeature, tc.definition, tc.feature,
			)
		}
	}
}

// A typing's references each end in the name a diagram heads the type by; a
// conjugation is kept, and text that is not a reference list stands as it is.
func TestReferenceEndNames(t *testing.T) {
	for _, tc := range []struct{ text, want string }{
		{"Pump", "Pump"},
		{"Plant::Pumps::Pump", "Pump"},
		{"$::ISQ::LengthValue", "LengthValue"},
		{"~Ports::FuelPort", "~FuelPort"},
		{"Plant::Pumps::Pump, ~Ports::FuelPort", "Pump, ~FuelPort"},
		{"'Sep::Pkg'::'x::y'", "'x::y'"},
		{"vehicle.engine.Cylinder", "Cylinder"},
		{"Plant::pump.'fuel in'", "'fuel in'"},
		{"", ""},
		{"T<U>", "T<U>"},
		{"A::", "A::"},
		{"A,B", "A,B"},
	} {
		if got := ReferenceEndNames(tc.text); got != tc.want {
			t.Errorf("ReferenceEndNames(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

func TestReferenceQualifiedNames(t *testing.T) {
	for _, tc := range []struct {
		text string
		want [][]string
	}{
		{"Pump", [][]string{{"Pump"}}},
		{"Plant::Pumps::Pump", [][]string{{"Plant", "Pumps", "Pump"}}},
		{"$::ISQ::LengthValue", [][]string{{"ISQ", "LengthValue"}}},
		{"~Ports::FuelPort", [][]string{{"Ports", "FuelPort"}}},
		{"Plant::Pumps::Pump, ~Ports::FuelPort", [][]string{{"Plant", "Pumps", "Pump"}, {"Ports", "FuelPort"}}},
		{"'Sep::Pkg'::'x::y'", [][]string{{"Sep::Pkg", "x::y"}}},
		{"vehicle.engine.Cylinder", nil},
		{"Plant::pump.'fuel in', Pump", [][]string{{"Pump"}}},
		{"", nil},
		{"T<U>", nil},
		{"A::", nil},
		{"A,B", nil},
	} {
		if got := ReferenceQualifiedNames(tc.text); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ReferenceQualifiedNames(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

// Writing names and reading the text back gives the names again, whatever they hold.
func TestWrittenNamesReadBackAsTheNamesThemselves(t *testing.T) {
	names := []string{
		"x", "part", "two words", "it's", `'`, `''quoted''`, `a\b`, `\`, `\\`, `\'`, `ends\`,
		`q"d`, "x::y", "::", "a.b", ".", "\b", "\t", "\n", "\f", "\r", "a\tb\nc\rd\be\ff",
		`\n`, `\t`, "é", "名前", "naïve 'name'", "@1", "$", "1st",
	}
	paths := [][]string{names}
	for _, name := range names {
		paths = append(paths, []string{name}, []string{"P", name}, []string{name, "x"}, []string{name, name})
	}
	for _, path := range paths {
		text := QualifiedNameOf(path)
		if got, ok := QualifiedNameSegments(text); !ok || !slices.Equal(got, path) {
			t.Errorf("QualifiedNameSegments(%q) = %q, %v, want %q", text, got, ok, path)
		}
		text = MemberPathOf(path)
		if got, ok := MemberPathSegments(text); !ok || !slices.Equal(got, path) {
			t.Errorf("MemberPathSegments(%q) = %q, %v, want %q", text, got, ok, path)
		}
	}
	for _, name := range names {
		text := UnrestrictedNameText(name)
		if got := Unescape(text[1 : len(text)-1]); got != name {
			t.Errorf("Unescape of UnrestrictedNameText(%q) = %q", name, got)
		}
	}
}
