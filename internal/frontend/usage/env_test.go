package usage

import (
	"flag"
	"io"
	"strings"
	"testing"
)

func TestEnvName(t *testing.T) {
	for name, want := range map[string]string{"log-level": "OPENSYSML_MCP_LOG_LEVEL", "http": "OPENSYSML_MCP_HTTP", "max-body-bytes": "OPENSYSML_MCP_MAX_BODY_BYTES", "v": "OPENSYSML_MCP_V"} {
		if got := EnvName("OPENSYSML_MCP_", name); got != want {
			t.Errorf("EnvName(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestApplyEnvironmentFillsUnsetFlags checks a variable sets the flag the
// command line left alone, of every kind, and a flag given wins over it.
func TestApplyEnvironmentFillsUnsetFlags(t *testing.T) {
	fs := flag.NewFlagSet("cmd", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	level := fs.String("log-level", "warn", "")
	size := fs.Int("cache-size", 100, "")
	strict := fs.Bool("strict", false, "")
	root := fs.String("root", ".", "")
	token := fs.String("token", "", "")
	t.Setenv("TEST_CMD_LOG_LEVEL", "debug")
	t.Setenv("TEST_CMD_CACHE_SIZE", "7")
	t.Setenv("TEST_CMD_STRICT", "true")
	t.Setenv("TEST_CMD_ROOT", "/models")
	t.Setenv("TEST_CMD_TOKEN", "from-the-environment")
	if err := fs.Parse([]string{"-cache-size", "3"}); err != nil {
		t.Fatal(err)
	}
	if err := ApplyEnvironment(fs, "TEST_CMD_", "token"); err != nil {
		t.Fatalf("ApplyEnvironment: %v", err)
	}
	if *level != "debug" || *size != 3 || !*strict || *root != "/models" {
		t.Errorf("after the environment: log-level %q, cache-size %d, strict %v, root %q", *level, *size, *strict, *root)
	}
	if *token != "" {
		t.Errorf("the excepted flag was set from the environment: %q", *token)
	}
}

// TestApplyEnvironmentLeavesEmptyVariables checks a variable set to nothing is
// as good as unset, so an `export OPENSYSML_MCP_STRICT=` leaves the default.
func TestApplyEnvironmentLeavesEmptyVariables(t *testing.T) {
	fs := flag.NewFlagSet("cmd", flag.ContinueOnError)
	strict := fs.Bool("strict", false, "")
	t.Setenv("TEST_CMD_STRICT", "")
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if err := ApplyEnvironment(fs, "TEST_CMD_"); err != nil || *strict {
		t.Errorf("ApplyEnvironment with an empty variable: %v, strict %v", err, *strict)
	}
}

// TestApplyEnvironmentRefusesWhatTheFlagRefuses checks a value the flag's own
// parser rejects is an error naming the variable and the flag.
func TestApplyEnvironmentRefusesWhatTheFlagRefuses(t *testing.T) {
	fs := flag.NewFlagSet("cmd", flag.ContinueOnError)
	fs.Int("cache-size", 100, "")
	t.Setenv("TEST_CMD_CACHE_SIZE", "many")
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	err := ApplyEnvironment(fs, "TEST_CMD_")
	if err == nil {
		t.Fatal("ApplyEnvironment accepted a non-integer for an int flag")
	}
	for _, want := range []string{`"many"`, "TEST_CMD_CACHE_SIZE", "-cache-size", "parse error"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}
