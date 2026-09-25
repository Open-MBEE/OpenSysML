//go:build !wasm

package hostcap

// CheckSpawn returns nil: every native host starts processes, so nothing is refused
// here and every caller goes on to look the tool up and run it as it always has.
func CheckSpawn(string) error { return nil }
