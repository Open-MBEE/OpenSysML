//go:build js

package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// exposeCompletion lets a page complete the line being typed:
// globalThis.sysmlReplComplete(textBeforeCursor) answers {candidates, prefix} as JSON.
func exposeCompletion(sess *repl.Session) {
	js.Global().Set("sysmlReplComplete", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 || args[0].Type() != js.TypeString {
			return "{}"
		}
		line := args[0].String()
		c := sess.Complete(line, len(line))
		out, err := json.Marshal(struct {
			Candidates []string `json:"candidates"`
			Prefix     string   `json:"prefix"`
		}{c.Candidates, c.Prefix})
		if err != nil {
			return "{}"
		}
		return string(out)
	}))
}
