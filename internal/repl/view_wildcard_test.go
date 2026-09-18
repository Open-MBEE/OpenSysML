package repl

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

const wildcardViewModel = `package Fleet {
    metadata def Safety;
    #Safety part def Airbag;
    part def Radio;
    part def Bus {
        part radio : Radio;
    }
    view everything {
        expose Fleet::*;
    }
    view safety {
        expose Fleet::**[@Fleet::Safety];
    }
}`

func wildcardViewSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	res := s.Submit(wildcardViewModel)
	for _, d := range res.Diagnostics {
		if d.Severity == diag.SeverityError {
			t.Fatalf("model did not load: %v", res.Diagnostics)
		}
	}
	return s
}

// A wildcard expose reaches a namespace's members through the members' own scope
// and through the index, which build a symbol each; the element is one, so it is
// listed and rendered once.
func TestWildcardExposeNamesEachElementOnce(t *testing.T) {
	for _, tc := range []struct{ cmd, elem string }{
		{"%view Fleet::everything", "Fleet::Radio"},
		{"%render Fleet::everything", "part def Fleet::Radio"},
		{"%view Fleet::safety", "Fleet::Airbag"},
		{"%render Fleet::safety", "part def Fleet::Airbag"},
	} {
		out, _, err := wildcardViewSession(t).RunMeta(tc.cmd)
		if err != nil {
			t.Fatalf("%s: %v", tc.cmd, err)
		}
		text := strings.Join(out, "\n")
		if n := strings.Count(text, tc.elem); n != 1 {
			t.Errorf("%s named %s %d times, want once:\n%s", tc.cmd, tc.elem, n, text)
		}
	}
}
