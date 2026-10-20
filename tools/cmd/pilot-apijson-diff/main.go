// Command pilot-apijson-diff compares this repository's API element JSON with
// the pinned pilot's export, property by property, over the corpus roots; see
// docs/project/api-json-differential.md.
package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/referee/apijson"
)

func main() {
	os.Exit(apijson.Main(os.Args[1:], os.Stderr))
}
