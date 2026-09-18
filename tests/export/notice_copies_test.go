package export_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// The Python client cannot import the Go constant, so it keeps a fallback copy
// for a service too old to send its own notice. This pins the two together.
func TestPythonFallbackNoticeMatchesTheConstant(t *testing.T) {
	path := filepath.Join("..", "..", "clients", "python", "opensysml", "conversion.py")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the Python client: %v", err)
	}
	copied, err := pythonStringConstant(string(source), "EXPERIMENTAL_NOTICE")
	if err != nil {
		t.Fatalf("read EXPERIMENTAL_NOTICE from %s: %v", path, err)
	}
	if copied != export.ExperimentalNotice {
		t.Errorf("the Python fallback notice has drifted from the constant\npython: %q\ngo:     %q", copied, export.ExperimentalNotice)
	}
}

// pythonStringConstant reads a module-level constant assigned a run of adjacent
// string literals, parenthesized or not.
func pythonStringConstant(source, name string) (string, error) {
	at := strings.Index(source, "\n"+name+" = ")
	if at < 0 {
		return "", errors.New("no assignment to " + name)
	}
	rest := strings.TrimPrefix(strings.TrimLeft(source[at+len(name)+4:], " \t"), "(")
	var built strings.Builder
	for {
		rest = strings.TrimLeft(rest, " \t\r\n")
		if rest == "" || (rest[0] != '"' && rest[0] != '\'') {
			break
		}
		text, remainder, err := pythonLiteral(rest)
		if err != nil {
			return "", err
		}
		built.WriteString(text)
		rest = remainder
	}
	if built.Len() == 0 {
		return "", errors.New(name + " is not a string literal")
	}
	return built.String(), nil
}

// pythonLiteral unquotes the literal at the head of source and returns what
// follows it. Python and Go escape these the same way.
func pythonLiteral(source string) (string, string, error) {
	quote := source[0]
	for i := 1; i < len(source); i++ {
		switch source[i] {
		case '\\':
			i++
		case quote:
			body := strings.ReplaceAll(source[1:i], `"`, `\"`)
			text, err := strconv.Unquote(`"` + body + `"`)
			if err != nil {
				return "", "", err
			}
			return text, source[i+1:], nil
		}
	}
	return "", "", errors.New("unterminated string literal")
}
