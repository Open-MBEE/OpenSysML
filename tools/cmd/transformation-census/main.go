// Command transformation-census runs the census gate implemented by package transformation.
package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/census/transformation"
)

func main() {
	os.Exit(transformation.Main(os.Args[1:], os.Stdout, os.Stderr))
}
