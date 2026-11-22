//go:build !wasm

package main

// serveRefusal is nil: a native build binds the addresses the connection file
// names, where the front end reaches the kernel.
func serveRefusal() error { return nil }
