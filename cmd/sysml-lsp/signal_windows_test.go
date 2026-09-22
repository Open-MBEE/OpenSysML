//go:build windows

package main

import "os"

var quitSignal os.Signal = os.Kill
