//go:build !wasm

package layout

// checkSpawn returns nil: every native host starts processes.
func checkSpawn() error { return nil }
