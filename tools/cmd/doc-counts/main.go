// Command doc-counts runs the documentation-figure gate implemented by package doccounts.
package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/census/doccounts"
)

func main() {
	os.Exit(doccounts.Main(os.Args[1:], os.Stdout, os.Stderr))
}
