// Command notation-census measures how much of the SysML v2 graphical notation
// (§8.2.3) the view renderers draw and holds the committed baseline to it.
package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/census/notation"
)

func main() {
	os.Exit(notation.Main(os.Args[1:], os.Stdout, os.Stderr))
}
