//go:build !windows

package main

import (
	"os"
	"syscall"
)

var quitSignal os.Signal = syscall.SIGQUIT
