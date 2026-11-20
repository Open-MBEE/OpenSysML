//go:build wasm

package main

import "fmt"

// httpRefusal refuses -http: Go's net on js/wasm and wasip1 hands back a listener
// that accepts only from inside the same process, so serving on it would wait
// forever on an address no client could dial.
func httpRefusal(addr string) error {
	return fmt.Errorf("-http %s binds an address, and a WebAssembly build's network reaches only "+
		"the process it runs in, so no client outside that process could connect; leave -http "+
		"out to speak the protocol on stdin/stdout, which binds no address", addr)
}
