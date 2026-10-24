// Command formal-constraints reads the formal documents' constraints into the
// constraint manifest; see package formal.
package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/census/formal"
)

func main() {
	os.Exit(formal.Main(os.Args[1:], os.Stdout, os.Stderr))
}
