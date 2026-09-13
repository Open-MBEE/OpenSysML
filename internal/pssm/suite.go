package pssm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// SuiteFile is the file name of the PSSM test suite within its root.
const SuiteFile = "PSSM_TestSuite.xmi"

// RequireEnv names the environment variable CI sets so an absent suite fails
// a gate instead of skipping it.
const RequireEnv = "OPENSYSML_REQUIRE_PSSM_SUITE"

// DefaultRoot is the suite root relative to the repository root, where
// scripts/download-pssm-suite.sh installs the pinned suite.
const DefaultRoot = "build/pssm"

// ErrSuiteAbsent reports that no suite file exists at the root looked in.
var ErrSuiteAbsent = errors.New("PSSM test suite is absent")

// Locate returns the suite file under root, or an error wrapping
// ErrSuiteAbsent with the provisioning hint when it does not exist.
func Locate(root string) (string, error) {
	path := filepath.Join(root, SuiteFile)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w at %s; run ./scripts/download-pssm-suite.sh to provision it", ErrSuiteAbsent, path)
		}
		return "", err
	}
	return path, nil
}

// Required reports whether the environment demands the suite be present.
func Required() bool { return os.Getenv(RequireEnv) != "" }
