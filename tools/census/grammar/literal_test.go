package grammar

import "testing"

func TestLocateLiteralUsesCoverageMatchingAndStripping(t *testing.T) {
	content := "part total\n// part\n\"part\" part;\n/* + */ value + other\n"
	tests := []struct {
		line    int
		literal string
		offset  int
		ok      bool
	}{
		{1, "part", 0, true},
		{1, "to", 0, false},
		{2, "part", 0, false},
		{3, "part", 26, true},
		{4, "+", 46, true},
		{5, "part", 0, false},
		{6, "part", 0, false},
		{0, "part", 0, false},
		{1, "", 0, false},
	}
	for _, test := range tests {
		got, ok := LocateLiteral(content, test.line, test.literal)
		if got != test.offset || ok != test.ok {
			t.Errorf("LocateLiteral(%d, %q) = (%d, %v), want (%d, %v)",
				test.line, test.literal, got, ok, test.offset, test.ok)
		}
	}
}
