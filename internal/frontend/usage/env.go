package usage

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
)

// EnvName is the environment variable a command reading its flags from the
// environment under prefix reads for the flag name: the name in upper case with
// each dash an underscore, after the prefix, as OPENSYSML_MCP_LOG_LEVEL for
// -log-level under OPENSYSML_MCP_.
func EnvName(prefix, name string) string {
	return prefix + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// ApplyEnvironment sets every flag of fs the command line left unset from its
// variable EnvName(prefix, name), where that is set and not empty; the flags
// named in except have readers of their own and are left alone. It runs after
// fs.Parse, so a flag given wins over its variable. A value the flag refuses
// is an error naming the variable and the flag's own complaint, for the caller
// to report as it reports a bad argument.
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
