// Command validation-census runs the census gate implemented by package validation.
package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/census/validation"
)

func main() {
	os.Exit(validation.Main(os.Args[1:], os.Stdout, os.Stderr))
}
