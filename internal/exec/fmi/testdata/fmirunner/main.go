// Command fmirunner stands in for the OPENSYSML_FMI_RUNNER executable in the
// engine tests: it records the request it read on standard input to
// $TMPDIR/fmirunner-request.json, then acts as $TMPDIR/fmirunner-mode says —
// absent or "answer" writes $TMPDIR/fmirunner-reply.json to standard output,
// "error:<msg>" writes a reply carrying an error member, "exit:<msg>" writes
// msg to standard error and exits nonzero, "hang" sleeps past any timeout.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func scratch(name string) string {
	return filepath.Join(os.TempDir(), "fmirunner-"+name)
}

func main() {
	request, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
		os.Exit(2)
	}
	if err := os.WriteFile(scratch("request.json"), request, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "record request: %v\n", err)
		os.Exit(2)
	}
	mode, _ := os.ReadFile(scratch("mode.txt"))
	text := strings.TrimSpace(string(mode))
	switch {
	case text == "" || text == "answer":
		reply, err := os.ReadFile(scratch("reply.json"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "read reply: %v\n", err)
			os.Exit(2)
		}
		fmt.Print(string(reply))
	case strings.HasPrefix(text, "error:"):
		fmt.Printf(`{"protocol": 1, "error": %q}`+"\n", strings.TrimPrefix(text, "error:"))
	case strings.HasPrefix(text, "exit:"):
		fmt.Fprintln(os.Stderr, strings.TrimPrefix(text, "exit:"))
		os.Exit(3)
	case text == "hang":
		time.Sleep(time.Hour)
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", text)
		os.Exit(2)
	}
}
