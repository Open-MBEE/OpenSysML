// Command grammar-metaclass-map runs the advisory grammar-to-metaclass map.
package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/census/metaclassmap"
)

func main() {
	os.Exit(metaclassmap.Main(os.Args[1:], os.Stderr))
}
