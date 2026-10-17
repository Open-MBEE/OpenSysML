package repl

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/codes"
)

// doExplainCode answers %explain-code: each diagnostic code named is explained
// in turn, and no code at all lists the catalogue with one-line summaries.
func (s *Session) doExplainCode(args []string) []string {
	if len(args) == 0 {
		return strings.Split(strings.TrimRight(codes.List(), "\n"), "\n")
	}
	var out []string
	for _, code := range args {
		text, err := codes.Explain(code)
		if err != nil {
			out = append(out, errPrefix+err.Error())
			continue
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, strings.Split(strings.TrimRight(text, "\n"), "\n")...)
	}
	return out
}
