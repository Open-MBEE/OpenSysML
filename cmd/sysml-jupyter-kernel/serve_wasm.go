//go:build wasm

package main

import "errors"

// serveRefusal refuses to serve: a wasm build's listeners accept only from inside
// its own process, so the kernel would wait on addresses no front end could dial.
func serveRefusal() error {
	return errors.New("a kernel binds the addresses its connection file names, and a WebAssembly " +
		"build's network reaches only the process it runs in, so no Jupyter front end could " +
		"connect; run a native build of the kernel")
}
