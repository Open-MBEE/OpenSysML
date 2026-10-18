package migrate

import "strings"

// hasBlock reports whether notation holds want's lines consecutively, each at
// one same indentation: a declaration reads alike at whatever depth its
// package nests it.
func hasBlock(notation, want string) bool {
	for _, line := range strings.Split(notation, "\n") {
		indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
		block := indent + strings.ReplaceAll(want, "\n", "\n"+indent)
		if strings.Contains(notation, block) {
			return true
		}
	}
	return false
}
