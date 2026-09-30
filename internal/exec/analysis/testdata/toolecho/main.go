// Command toolecho stands in for an external tool started from an invocation block: it
// answers with the command it was started as — argv, environment, working directory,
// standard input, and the input file and output directory two variables name — as string
// outputs of the protocol's reply object. It is built by the tests that run it and lives
// under testdata, out of every build.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InputEnv names the input file to read back; OutputDirEnv the directory to write into.
const (
	InputEnv     = "TOOL_ECHO_INPUT"
	OutputDirEnv = "TOOL_ECHO_OUTDIR"
)

// value is one protocol value.
type value struct {
	Value string `json:"value"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "toolecho:", err)
		os.Exit(2)
	}
}

func run() error {
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	env := append([]string(nil), os.Environ()...)
	sort.Strings(env)
	var input string
	if path := os.Getenv(InputEnv); path != "" {
		data, err := os.ReadFile(path) // #nosec G304 -- the test names the input file
		if err != nil {
			return err
		}
		input = string(data)
	}
	outDir := os.Getenv(OutputDirEnv)
	if outDir != "" {
		if err := os.WriteFile(filepath.Join(outDir, "result.txt"), []byte("done\n"), 0o600); err != nil {
			return err
		}
	}
	argv, err := json.Marshal(os.Args[1:])
	if err != nil {
		return err
	}
	outputs := map[string]value{
		"argv":   text(string(argv)),
		"env":    text(strings.Join(env, "\n")),
		"cwd":    text(cwd),
		"stdin":  text(string(stdin)),
		"input":  text(input),
		"outDir": text(outDir),
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"outputs": outputs})
}

// text is a string value.
func text(s string) value { return value{Value: s} }
