package usage

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
)

// EnvName is the variable of flag name under prefix: the name in upper case with
// each dash an underscore, as OPENSYSML_MCP_LOG_LEVEL for -log-level.
func EnvName(prefix, name string) string {
	return prefix + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// ApplyEnvironment sets each flag fs.Parse left unset from its non-empty EnvName
// variable, except the named flags; a value the flag refuses is the error, naming both.
func ApplyEnvironment(fs *flag.FlagSet, prefix string, except ...string) error {
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
	var err error
	fs.VisitAll(func(f *flag.Flag) {
		if err != nil || given[f.Name] || slices.Contains(except, f.Name) {
			return
		}
		name := EnvName(prefix, f.Name)
		value := os.Getenv(name)
		if value == "" {
			return
		}
		if e := fs.Set(f.Name, value); e != nil {
			err = fmt.Errorf("invalid value %q for %s (the environment of -%s): %v", value, name, f.Name, e)
		}
	})
	return err
}
