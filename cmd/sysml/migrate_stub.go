//go:build sysml_prod || sysml_nov1

package main

import "github.com/Open-MBEE/OpenSysML/internal/translate/convert"

func migrateInput(string, []byte, convert.Format) ([]byte, map[string][]byte, error) {
	return nil, nil, convert.ErrMigrationNotLinked
}
