//go:build sysml_prod || sysml_nodocpdf

package main

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

func renderPDF(*repl.Session) error {
	return errors.New("-doc-form pdf is not available in this build (built without docpdf)")
}
