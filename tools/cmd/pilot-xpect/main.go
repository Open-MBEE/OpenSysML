// Command pilot-xpect runs the referee implemented by package xpect.
package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/referee/xpect"
)

func main() {
	os.Exit(xpect.Main(os.Args[1:], os.Stderr))
}
