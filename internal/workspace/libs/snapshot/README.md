`stdlib.snapshot` is the bundled library parsed, indexed, expanded and frozen, written here by
`make stdlib-snapshot` (`go generate ./internal/workspace/libs`) and embedded into every binary the
Makefile builds. It is generated, gitignored and never edited: a tree without it, such as the one
`go install github.com/Open-MBEE/OpenSysML/cmd/sysml@latest` compiles, loads the library files at
start-up instead. This file keeps the directory in the module so the `//go:embed snapshot`
directive in `../snapshot.go` always matches.
