// Command genkeywords writes the Emacs mode's keyword lists from the lexer's
// keyword list, so highlighting cannot drift from the language.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	dir := flag.String("out", ".", "directory to write sysml-mode-keywords.el into")
	flag.Parse()

	data, err := Render()
	if err != nil {
		fmt.Fprintf(os.Stderr, "genkeywords: %v\n", err)
		os.Exit(1)
	}
	path := filepath.Join(*dir, File)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "genkeywords: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n", path)
}
