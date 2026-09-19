// Command pilot-diff runs the referee implemented by package diff.
package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/referee/diff"
)

func main() {
	os.Exit(diff.Main(os.Args[1:], os.Stderr))
}
