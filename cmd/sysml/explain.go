package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/codes"
)

// explainDiagnostics answers -explain: each code named is explained in turn, and
// no code at all lists the catalogue. An unknown code is a usage error, as a
// misspelt flag is, so a script that asked about a code that was renamed hears it.
func explainDiagnostics(out, errw io.Writer, names []string) int {
	if len(names) == 0 {
		fmt.Fprint(out, codes.List())
		return 0
	}
	status, printed := 0, false
	for _, name := range names {
		text, err := codes.Explain(name)
		if err != nil {
			fmt.Fprintf(errw, "sysml: %v\n", err)
			status = 2
			continue
		}
		if printed {
			fmt.Fprintln(out)
		}
		fmt.Fprint(out, text)
		printed = true
	}
	return status
}

// codeNames are -explain's codes. The flag takes an optional value: permuteArgs
// joins the word after a bare -explain as its code and spells a truly bare one
// `-explain=`, so no code word is ever read as a boolean.
type codeNames struct {
	codes []string
	given bool
}

func (c *codeNames) String() string { return strings.Join(c.codes, ",") }

func (c *codeNames) Set(value string) error {
	c.given = true
	if value != "" {
		c.codes = append(c.codes, value)
	}
	return nil
}

// TakesOptionalValue marks the flag for permuteArgs.
func (*codeNames) TakesOptionalValue() bool { return true }
