// Command gengrammar writes the VS Code extension's TextMate grammars and the
// JupyterLab extension's CodeMirror syntax table from the lexer's keyword list,
// so highlighting cannot drift from the language.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	dir := flag.String("out", ".", "directory to write the grammar files into")
	table := flag.String("codemirror-out", "", "file to write the CodeMirror syntax table into (none when empty)")
	flag.Parse()

	for _, g := range Grammars() {
		data, err := Render(g)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gengrammar: %v\n", err)
			os.Exit(1)
		}
		path := filepath.Join(*dir, g.File)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "gengrammar: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", path)
	}
	if *table == "" {
		return
	}
	data, err := RenderSyntaxTable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gengrammar: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*table, data, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "gengrammar: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n", *table)
}
