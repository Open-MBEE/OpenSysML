//go:build sysml_prod || sysml_nosync

package main

import (
	"errors"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// syncTokenEnv is unnamed: the help leaves the section naming it out.
const syncTokenEnv = ""

var errSyncNotLinked = errors.New("a repository branch is not available in this build (built without sync)")

func runSyncDiff([]string) int { return fail(errSyncNotLinked) }

func runSyncApply([]string) int { return fail(errSyncNotLinked) }

// convertBranch refuses a conversion either side of which names a repository.
func convertBranch(input string, _ convert.Format) (int, bool, error) {
	if namesRepository(input) || namesRepository(outputPath) {
		return 0, true, errSyncNotLinked
	}
	return 0, false, nil
}

// migrateBranch refuses a migration either side of which names a repository.
func migrateBranch(input string, _ convert.Format, _ producer) (int, bool, error) {
	if namesRepository(input) || namesRepository(outputPath) {
		return 0, true, errSyncNotLinked
	}
	return 0, false, nil
}

// branchURL refuses a path naming a repository; any other is a file.
func branchURL(path string) (string, bool, error) {
	if namesRepository(path) {
		return "", false, errSyncNotLinked
	}
	return "", false, nil
}

func namesRepository(path string) bool {
	for _, scheme := range []string{"flexo://", "http://", "https://"} {
		if strings.HasPrefix(path, scheme) {
			return true
		}
	}
	return false
}
