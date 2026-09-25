// Command pilot-exec-diff runs the referee implemented by package exec.
package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/referee/exec"
)

func main() {
	os.Exit(exec.Main(os.Args[1:], os.Stderr))
}
