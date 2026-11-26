package sessiontools

import (
	"strings"
	"testing"
)

func TestReplCommandAnsweredWithItsUsageIsAnError(t *testing.T) {
	cs := connect(t, t.TempDir())
	for _, line := range []string{
		"%save",
		"%validate a b",
		"%instantiate",
		"%render-run",
		"%render-run nope",
		"%clock-step 1 2",
	} {
		if res := call(t, cs, "repl_command", args("line", line), nil); !res.IsError || !strings.Contains(errText(res), "usage: %") {
			t.Errorf("repl_command(%q) = error %v, %q; want its usage as an error", line, res.IsError, errText(res))
		}
	}
	var cmd CommandResult
	must(t, cs, "repl_command", args("line", "%help"), &cmd)
}
