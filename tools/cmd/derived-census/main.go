package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/census/derived"
)

func main() {
	os.Exit(derived.Main(os.Args[1:], os.Stdout, os.Stderr))
}
