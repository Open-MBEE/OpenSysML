package solve

import (
	"bufio"
	"errors"
	"strings"
	"testing"
)

// parse reads one S-expression from text, as the driver reads a reply.
func parse(t *testing.T, text string) sexpr {
	t.Helper()
	got, err := readSexpr(bufio.NewReader(strings.NewReader(text)))
	if err != nil {
		t.Fatalf("read %q: %v", text, err)
	}
	return got
}

func TestReadSexpr(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"atom", "sat\n", "sat"},
		{"atom at end of output", "sat", "sat"},
		{"empty list", "()\n", "()"},
		{"nested list", "((x 1) (y (- 2)))\n", "((x 1) (y (- 2)))"},
		{"leading comment", "; thinking\nunsat\n", "unsat"},
		{"quoted symbol", "|Check::C::a b|\n", "Check::C::a b"},
		{"string literal", `"a ""quoted"" reply"` + "\n", `"a ""quoted"" reply"`},
		{"comment inside a list", "(x ; note\n 1)\n", "(x 1)"},
		{"decimal", "(/ 1.0 3.0)\n", "(/ 1.0 3.0)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parse(t, c.in).String(); got != c.want {
				t.Errorf("read %q as %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestReadSexprReportsAnEndedOutputAsEOF(t *testing.T) {
	for _, in := range []string{"", "   \n", "; only a comment\n"} {
		_, err := readSexpr(bufio.NewReader(strings.NewReader(in)))
		if !errors.Is(err, errEOF) {
			t.Errorf("read %q gave %v, want errEOF", in, err)
		}
	}
}

func TestReadSexprRejectsMalformedReplies(t *testing.T) {
	for _, in := range []string{")\n", "(x\n", "|unterminated\n"} {
		_, err := readSexpr(bufio.NewReader(strings.NewReader(in)))
		if err == nil {
			t.Errorf("read %q gave no error", in)
		}
	}
}

func TestSexprError(t *testing.T) {
	msg, ok := parse(t, `(error "line 3: unsupported")`).isError()
	if !ok || msg != "line 3: unsupported" {
		t.Errorf("isError gave %q, %v", msg, ok)
	}
	if _, ok := parse(t, "unsat").isError(); ok {
		t.Error("a verdict was read as an error reply")
	}
}
