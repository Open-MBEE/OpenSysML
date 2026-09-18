// Command pilot-reject runs the referee implemented by package reject.
package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/referee/reject"
)

func main() {
	os.Exit(reject.Main(os.Args[1:], os.Stderr))
}
