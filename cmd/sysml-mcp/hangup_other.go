//go:build !unix

package main

import "os"

// hangup is empty: js/wasm and Windows deliver no SIGHUP, so the credentials
// are reloaded only as their files change.
var hangup []os.Signal
