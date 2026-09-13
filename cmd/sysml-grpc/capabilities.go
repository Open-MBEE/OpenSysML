package main

import (
	"os"
	"strings"
)

const testWithholdCapabilitiesEnv = "OPENSYSML_TEST_WITHHOLD_CAPABILITIES"

func unavailableCapabilitiesForTesting() []string {
	return splitNames(os.Getenv(testWithholdCapabilitiesEnv))
}

// splitNames reads a comma-separated list, dropping blanks.
func splitNames(list string) []string {
	var names []string
	for _, name := range strings.Split(list, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}
