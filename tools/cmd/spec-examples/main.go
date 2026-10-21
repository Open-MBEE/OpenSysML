// Command spec-examples extracts the specification examples; see package specexamples.
package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/specexamples"
)

func main() {
	os.Exit(specexamples.Main(os.Args[1:], os.Stderr))
}
