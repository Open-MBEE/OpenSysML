package ast

import "testing"

// TestNotationVariation checks a variation is named with `variation` before its
// kind keyword, after any other prefix, for definitions and usages alike.
func TestNotationVariation(t *testing.T) {
	cases := []struct {
		name string
		node Node
		want string
	}{
		{"part def", &Definition{Kind: DefPart, Keyword: "part", HasDefKeyword: true}, "part def"},
		{"variation part def", &Definition{Kind: DefPart, Keyword: "part", HasDefKeyword: true, IsVariation: true}, "variation part def"},
		{"variation classifier", &Definition{Kind: DefPart, Keyword: "classifier", IsVariation: true}, "variation classifier"},
		{"part", &Usage{Kind: UsagePart, Keyword: "part"}, "part"},
		{"variation part", &Usage{Kind: UsagePart, Keyword: "part", IsVariation: true}, "variation part"},
		{"variation attribute", &Usage{Kind: UsageAttribute, Keyword: "attribute", IsVariation: true}, "variation attribute"},
		{"variation with prefix", &Usage{Kind: UsagePart, Keyword: "part", PrefixKeyword: "ref", IsVariation: true}, "variation ref part"},
		{"variant", &Usage{Kind: UsagePart, Keyword: "variant", IsVariant: true}, "variant"},
		{"variant part", &Usage{Kind: UsagePart, Keyword: "part", IsVariant: true}, "variant part"},
		{"variant attribute", &Usage{Kind: UsageAttribute, Keyword: "attribute", IsVariant: true}, "variant attribute"},
		{"variant ref part", &Usage{Kind: UsagePart, Keyword: "part", PrefixKeyword: "ref", IsVariant: true}, "variant ref part"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Notation(tc.node); got != tc.want {
				t.Errorf("Notation = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestNotationAbstract checks an abstract definition or usage is named with
// `abstract` before its kind keyword, and a variation with `variation` alone.
func TestNotationAbstract(t *testing.T) {
	cases := []struct {
		name string
		node Node
		want string
	}{
		{"abstract part def", &Definition{Kind: DefPart, Keyword: "part", HasDefKeyword: true, IsAbstract: true}, "abstract part def"},
		{"abstract classifier", &Definition{Kind: DefPart, Keyword: "classifier", IsAbstract: true}, "abstract classifier"},
		{"abstract variation part def", &Definition{Kind: DefPart, Keyword: "part", HasDefKeyword: true, IsAbstract: true, IsVariation: true}, "variation part def"},
		{"abstract part", &Usage{Kind: UsagePart, Keyword: "part", IsAbstract: true}, "abstract part"},
		{"abstract attribute", &Usage{Kind: UsageAttribute, Keyword: "attribute", IsAbstract: true}, "abstract attribute"},
		{"abstract with prefix", &Usage{Kind: UsagePart, Keyword: "part", PrefixKeyword: "ref", IsAbstract: true}, "abstract ref part"},
		{"abstract variation part", &Usage{Kind: UsagePart, Keyword: "part", IsAbstract: true, IsVariation: true}, "variation part"},
		{"variant abstract part", &Usage{Kind: UsagePart, Keyword: "part", IsVariant: true, IsAbstract: true}, "variant abstract part"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Notation(tc.node); got != tc.want {
				t.Errorf("Notation = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNotationKeyword(t *testing.T) {
	for notation, want := range map[string]string{
		"part def":              "part def",
		"abstract part def":     "part def",
		"variation part def":    "part def",
		"abstract classifier":   "classifier",
		"variant":               "variant",
		"variant ref part":      "ref part",
		"variant abstract part": "part",
		"entry action":          "entry action",
		"":                      "",
	} {
		if got := NotationKeyword(notation); got != want {
			t.Errorf("NotationKeyword(%q) = %q, want %q", notation, got, want)
		}
	}
}
