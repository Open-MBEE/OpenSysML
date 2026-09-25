package repl

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
	}
	for _, c := range cases {
		if got := needsContinuation(c.src); got != c.want {
			t.Errorf("needsContinuation(%q) = %v, want %v", c.src, got, c.want)
		}
	}
}
