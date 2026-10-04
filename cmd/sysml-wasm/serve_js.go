//go:build js

package main

import (
	"context"
	"syscall/js"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/combined"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
)

func serve(server *combined.Server, useStdio bool) int {
	if useStdio {
		return serveStdio(server)
	}
	js.Global().Set("sysmlWasm", map[string]any{
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
				body, err = server.Call(context.Background(), method, nil)
			} else {
				body, err = server.Call(context.Background(), method, []byte(params))
			}
			return jsonrpc.Envelope(body, err)
		}),
	})
	select {}
}
