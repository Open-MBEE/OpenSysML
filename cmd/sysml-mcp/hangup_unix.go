//go:build unix

package main

import (
	"os"
	"syscall"
)

// hangup is the signal that has serve reload the credentials: SIGHUP, the
// signal a service is told to re-read its configuration by.
var hangup = []os.Signal{syscall.SIGHUP}
