//go:build js || wasip1

package markup

// runAsciiDoc reports the converter missing: no WebAssembly host starts a
// process, and leaving os/exec out keeps the core build within its size budget.
func runAsciiDoc(string) ([]byte, error) {
	name, named := asciiDocName()
	detail := "this host starts no process, so PATH was not searched"
	if named {
		detail = AsciiDocEnv + " names it, but this host starts no process"
	}
	return nil, &ToolError{Tool: name, EnvVar: AsciiDocEnv, Named: named, Missing: true, Detail: detail}
}
