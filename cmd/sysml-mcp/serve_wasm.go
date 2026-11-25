//go:build wasm

package main

import "fmt"

// httpRefusal refuses -http: a wasm build's listener accepts only from inside
// its own process, so serving on it would wait on an address no client could dial.
func httpRefusal(addr string) error {
	return fmt.Errorf("-http %s binds an address, and a WebAssembly build's network reaches only "+
		"the process it runs in, so no client outside that process could connect; leave -http "+
		"out to speak the protocol on stdin/stdout, which binds no address", addr)
}
