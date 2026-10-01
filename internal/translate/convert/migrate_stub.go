//go:build sysml_prod || sysml_nov1

package convert

import "errors"

// ErrMigrationNotLinked is the answer to a SysML v1 input in a build made
// without the migration.
var ErrMigrationNotLinked = errors.New("SysML v1 migration is not available in this build (built without v1)")

func migrateTo(string, []byte, Format) ([]byte, error) { return nil, ErrMigrationNotLinked }
