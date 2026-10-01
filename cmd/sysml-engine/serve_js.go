//go:build js

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"syscall/js"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
)

// codeUnknown is the canonical status code an unclassified failure is answered
// under, as connect.CodeOf maps a non-status error.
const codeUnknown uint32 = 2

// serve installs globalThis.sysmlEngine — call(method, paramsJSON) answering
// the JSON-RPC response body, synchronous on the JS thread — and blocks
// forever, unless -stdio asks for the pipe protocol instead.
func serve(eng *engine.Engine, useStdio bool) int {
	if useStdio {
		return serveStdio(eng)
	}
	js.Global().Set("sysmlEngine", map[string]any{
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
				body, err = eng.Call(context.Background(), method, nil)
			} else {
				body, err = eng.Call(context.Background(), method, []byte(params))
			}
			if err != nil {
				code, message := codeUnknown, err.Error()
				var engErr *engine.Error
				if errors.As(err, &engErr) {
					code, message = engErr.Code, engErr.Message
				}
				return fmt.Sprintf(`{"jsonrpc":"2.0","id":null,"error":{"code":%d,"message":%s}}`,
					code, jsString(message))
			}
			return fmt.Sprintf(`{"jsonrpc":"2.0","id":null,"result":%s}`, body)
		}),
	})
	select {}
}

// jsString quotes a message for the JSON envelope it is written into.
func jsString(s string) string {
	out, err := json.Marshal(s)
	if err != nil {
		return `"error"`
	}
	return string(out)
}
