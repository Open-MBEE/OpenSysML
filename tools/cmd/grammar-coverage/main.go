// Command grammar-coverage runs the coverage census implemented by package grammar.
package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/census/grammar"
)

func main() {
	os.Exit(grammar.Main(os.Args[1:], os.Stderr))
}
