package notebook

import "testing"

func TestNeedsContinuation(t *testing.T) {
	cases := []struct {
		src  string
		want bool
	}{
		{"", false},
		{"package P;", false},
		{"package P {", true},
		{"package P {\n}", false},
		{"package P { namespace N {", true},
		{"import P::N;", false},
		{"( [ {", true},
		{"f(", true},
		{"f()", false},
		{"a[0]", false},
		{"package P } }", false},
		{"} package P {", true},
		// A string, quoted name, comment or note still open at the end of the
		// line continues: the literal may span lines, and the prompt should not
		// submit half of one.
		{`attribute s = "line one`, true},
		{"attribute s = \"line one\nline two\";", false},
		{"part def 'multi", true},
		{"part def 'multi\nline';", false},
		{"doc /* body", true},
		{"doc /* body\n*/", false},
		{"//* note", true},
		{"// note", false},
		{`attribute s = "a\"b";`, false},
	}
	for _, c := range cases {
		if got := NeedsContinuation(c.src); got != c.want {
			t.Errorf("NeedsContinuation(%q) = %v, want %v", c.src, got, c.want)
		}
	}
}
