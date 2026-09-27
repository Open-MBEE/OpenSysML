//go:build wasm

package main

import "fmt"

// serveRefusal refuses a transport that binds an address, and answers nil for
// -transport stdio, which binds none.
//
// Go's net on js/wasm and wasip1 is net/fd_fake.go: net.Listen succeeds, but the
// listener it hands back accepts only from inside the same process. Serving on one
// would report an address no client outside the process could dial and wait forever,
// so the transport is refused here — before the service is built, before anything
// listens — rather than serving nobody.
func serveRefusal(transport string) error {
	if transport == transportStdio {
		return nil
	}
	return fmt.Errorf("sysml-grpc: -transport %s binds an address, and a WebAssembly build's "+
		"network reaches only the process it runs in, so no client outside that process could "+
		"connect; use -transport stdio, which binds no address and speaks the protocol on "+
		"stdin/stdout", transport)
}
