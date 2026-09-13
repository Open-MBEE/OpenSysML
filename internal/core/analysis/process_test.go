package analysis

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// OPENSYSML_TOOL_MAX_OUTPUT is bytes or bytes with a binary suffix; anything else is the default.
func TestOutputLimitFromEnv(t *testing.T) {
	cases := map[string]int{
		"":             DefaultOutputLimit,
		" 4096 ":       4096,
		"16K":          16 << 10,
		"2M":           2 << 20,
		"1G":           1 << 30,
		"0":            DefaultOutputLimit,
		"-5":           DefaultOutputLimit,
		"lots":         DefaultOutputLimit,
		"1.5M":         DefaultOutputLimit,
		"99999999999G": DefaultOutputLimit,
	}
	for text, want := range cases {
		t.Setenv(OutputLimitEnv, text)
		if got := outputLimitFromEnv(); got != want {
			t.Errorf("%s=%q: %d, want %d", OutputLimitEnv, text, got, want)
		}
	}
}

// A bounded buffer keeps the prefix within its limit, reports the overflow once and stops
// the process; a limit below zero keeps nothing.
func TestBoundedBufferKeepsThePrefixAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := newBoundedBuffer(8, cancel)
	if n, err := b.Write([]byte("abcd")); n != 4 || err != nil || b.Over() {
		t.Fatalf("within the limit: %d %v over %v", n, err, b.Over())
	}
	if n, err := b.Write([]byte("efghij")); n != 6 || err != nil || !b.Over() {
		t.Fatalf("past the limit: %d %v over %v; want the write accepted and the overflow recorded", n, err, b.Over())
	}
	if got := string(b.Bytes()); got != "abcdefgh" {
		t.Errorf("kept %q, want the first eight bytes", got)
	}
	if ctx.Err() == nil {
		t.Error("the process was not stopped at the overflow")
	}
	empty := newBoundedBuffer(-1, nil)
	if _, err := empty.Write([]byte("x")); err != nil || !empty.Over() || len(empty.Bytes()) != 0 {
		t.Errorf("limit below zero: %v over %v kept %q; want nothing kept and the overflow recorded", err, empty.Over(), empty.Bytes())
	}
}

// The tool's overflow is measured at OPENSYSML_TOOL_MAX_OUTPUT and names the variable.
func TestToolOverflowNamesTheBound(t *testing.T) {
	t.Setenv(OutputLimitEnv, "64K")
	t.Setenv(standinMode, "flood")
	p := parsePilot(t)
	r := toolRegistry(t, manifestDir(t, pilotEntry(standin(t))))
	_, _, err := p.perform(t, r, p.context(), "Once")
	var fault *runtime.ToolError
	if !errors.As(err, &fault) || fault.Kind != runtime.ToolMalformed {
		t.Fatalf("perform = %v, want the malformed reply of an overflow", err)
	}
	if !strings.Contains(err.Error(), "wrote more than 65536 bytes") || !strings.Contains(err.Error(), OutputLimitEnv) {
		t.Errorf("overflow %q does not name the bound and its variable", err)
	}
}
