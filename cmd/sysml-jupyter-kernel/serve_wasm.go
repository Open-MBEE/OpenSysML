//go:build wasm

package main

import "errors"

// serveRefusal refuses to serve: Go's net on js/wasm and wasip1 hands back
// listeners that accept only from inside the same process, so the kernel's
// sockets would wait forever on addresses no front end could dial.
func serveRefusal() error {
	return errors.New("a kernel binds the addresses its connection file names, and a WebAssembly " +
		"build's network reaches only the process it runs in, so no Jupyter front end could " +
		"connect; run a native build of the kernel")
}
