package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The payload of a send with no argument is the value bound to the body feature
// that redefines SendAction::payload, by name or by position; a body binding
// some other input, or the sender, leaves the send without one.
func TestSendPayloadIsTheRedefinedPayloadParameter(t *testing.T) {
	cases := map[string]struct{ send, want string }{
		"named":                      {send: `send to r { in :>> payload = a; }`, want: "a"},
		"named after other inputs":   {send: `send to r { in delay = 3; in :>> payload = a; }`, want: "a"},
		"named with a chain":         {send: `send to r { in :>> SendAction::payload = a; }`, want: "a"},
		"by position":                {send: `send { in msg = a; in :>> receiver = r; }`, want: "a"},
		"unrelated input":            {send: `send to r { in delay = 3; }`},
		"sender only":                {send: `send to r { in :>> sender = r; }`},
		"receiver then a bare input": {send: `send { in :>> receiver = r; in msg = a; }`},
		"an argument is the payload": {send: `send a to r { in delay = 3; }`},
		"output":                     {send: `send to r { out done = a; }`},
		"untyped member":             {send: `send to r { attribute n = 3; }`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src := "action a { " + tc.send + " }"
			p := parser.New(source.New("send.sysml", []byte(src)))
			root := p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Fatalf("parse errors: %v", p.Diagnostics)
			}
			send, ok := unwrapMembership(unwrapMembership(root.Members[0]).(*ast.Usage).Members[0]).(*ast.SendStatement)
			if !ok {
				t.Fatalf("no send statement parsed from %q", tc.send)
			}
			got := SendPayload(send)
			switch {
			case got == nil && tc.want != "":
				t.Errorf("no payload found, want %q", tc.want)
			case got != nil && tc.want == "":
				t.Errorf("payload %q found, want none", src[got.Span().Offset:got.Span().End()])
			case got != nil:
				if spanned := strings.TrimSpace(src[got.Span().Offset:got.Span().End()]); spanned != tc.want {
					t.Errorf("payload %q, want %q", spanned, tc.want)
				}
			}
		})
	}
}
