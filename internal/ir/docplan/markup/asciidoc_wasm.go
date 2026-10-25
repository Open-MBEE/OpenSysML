//go:build js || wasip1

package markup

import (
	"os"
	"strings"
)

// runAsciiDoc reports the converter missing: no WebAssembly host starts a
// process, and leaving os/exec out keeps the core build within its size budget.
func runAsciiDoc(string) ([]byte, error) {
	name := strings.TrimSpace(os.Getenv(AsciiDocEnv))
	if name == "" {
		name = asciiDocTool
	}
	return nil, &ToolError{Tool: name, EnvVar: AsciiDocEnv, Missing: true, Detail: "this host starts no process"}
}
