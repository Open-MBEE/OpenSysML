// Command llm-bench benchmarks how well language models write SysML v2 when
// the OpenSysML compiler, and optionally its MCP tools, close the loop.
package main

import (
	"os"

	"github.com/Open-MBEE/OpenSysML/tools/bench/llm"
)

func main() {
	os.Exit(llm.Main(os.Args[1:], os.Stdout, os.Stderr))
}
