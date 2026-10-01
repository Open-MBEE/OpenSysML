//go:build js

package main

import (
	"context"
	"syscall/js"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/syntax"
)

// serve installs globalThis.sysmlSyntax — call(method, paramsJSON) answering
// the JSON-RPC response body, synchronous on the JS thread — and blocks
// forever, unless -stdio asks for the pipe protocol instead.
func serve(useStdio bool) int {
	if useStdio {
		return serveStdio()
	}
	js.Global().Set("sysmlSyntax", map[string]any{
		"version": Version,
		"call": js.FuncOf(func(_ js.Value, args []js.Value) any {
			method, params := "", ""
			if len(args) > 0 {
				method = args[0].String()
			}
			if len(args) > 1 {
				params = args[1].String()
			}
			var body []byte
			var err error
			if params == "" {
				body, err = syntax.Call(context.Background(), method, nil)
			} else {
				body, err = syntax.Call(context.Background(), method, []byte(params))
			}
			return jsonrpc.Envelope(body, err)
		}),
	})
	select {}
}
