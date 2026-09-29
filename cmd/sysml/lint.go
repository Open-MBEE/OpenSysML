package main

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
)

// lintList is -disable-lint as written: the codes of the lints left out of the
// model's diagnostics, comma-separated or repeated.
type lintList []string

func (l *lintList) String() string { return strings.Join(*l, ",") }

func (l *lintList) Set(value string) error {
	var codes []string
	for _, code := range strings.Split(value, ",") {
		if code = strings.TrimSpace(code); code != "" {
			codes = append(codes, code)
		}
	}
	if err := passes.CheckLintCodes(codes); err != nil {
		return err
	}
	*l = append(*l, codes...)
	return nil
}
