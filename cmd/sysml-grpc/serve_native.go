//go:build !wasm

package main

// serveRefusal is nil: a native build binds an address a client outside the
// process reaches, so every transport has one to serve on.
func serveRefusal(string) error { return nil }
