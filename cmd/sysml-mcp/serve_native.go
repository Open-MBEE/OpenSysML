//go:build !wasm

package main

// httpRefusal is nil: a native build binds an address a client outside the
// process reaches, so -http has one to serve on.
func httpRefusal(string) error { return nil }
