//go:build wasm

package hostcap

// CheckSpawn names subject as unable to run. No Go WebAssembly target starts a
// process: os/exec resolves the executable, then fails when it comes to starting
// one, reporting the platform's own "pipe: not implemented" with no word of the
// tool or the reason for it — so the reason is answered before that, and the
// lookup's own "not found" advice, which installing cannot satisfy, never appears.
func CheckSpawn(subject string) error { return &UnsupportedError{Subject: subject} }
