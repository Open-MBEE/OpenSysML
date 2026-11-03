//go:build wasm

package layout

// checkSpawn refuses before the lookup: no Go WebAssembly target starts a
// process, and os/exec would report the platform's "not implemented" with no
// word of why.
func checkSpawn() error { return ErrUnavailable }
