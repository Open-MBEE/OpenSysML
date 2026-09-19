package codegen

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// TestCPrintRealMatchesFormatReal drives the prelude's sysml_print_real over
// every power of two and its neighbours, where the shortest spelling is
// hardest to pick, plus random bit patterns, and compares with FormatReal.
func TestCPrintRealMatchesFormatReal(t *testing.T) {
	cc := os.Getenv(CCompilerEnvVar)
	if cc == "" {
		cc = "cc"
	}
	if _, err := exec.LookPath(cc); err != nil {
		t.Skip("no C compiler on PATH")
	}

	var values []float64
	for e := -1074; e <= 1023; e++ {
		p := math.Ldexp(1, e)
		values = append(values, p, -p, math.Nextafter(p, 0), math.Nextafter(p, math.Inf(1)))
	}
	rng := rand.New(rand.NewSource(20260902))
	for len(values) < 20000 {
		f := math.Float64frombits(rng.Uint64())
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			values = append(values, f)
		}
	}
	for _, f := range []float64{0, 0.1, 0.3, 0.1 + 0.2, 1e-4, 1e21, 9.5e20, 123456789012345680, 5e-324, math.MaxFloat64} {
		values = append(values, f, -f)
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "print.c")
	program := fmt.Sprintf("#define SYSML_MAX_CALC_DEPTH %d\n", runtime.DefaultMaxCalcDepth) + cPrelude + `
int main(void) {
	char line[64];
	while (fgets(line, sizeof line, stdin)) {
		uint64_t bits = strtoull(line, NULL, 16);
		sysml_real r;
		memcpy(&r, &bits, sizeof r);
		sysml_print_real(r);
	}
	return 0;
}
`
	if err := os.WriteFile(src, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "print")
	build := exec.Command(cc, append(append([]string{}, CFlags...), "-o", bin, src, "-lm")...) // #nosec G204 -- test-owned arguments
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s", err, out)
	}

	var in bytes.Buffer
	for _, f := range values {
		fmt.Fprintf(&in, "%016x\n", math.Float64bits(f))
	}
	run := exec.Command(bin)
	run.Stdin = &in
	out, err := run.Output()
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	mismatches := 0
	for i, f := range values {
		if !sc.Scan() {
			t.Fatalf("output ended after %d of %d values", i, len(values))
		}
		if got, want := strings.TrimSpace(sc.Text()), semantics.FormatReal(f); got != want {
			mismatches++
			if mismatches <= 10 {
				t.Errorf("%016x: C prints %s, FormatReal %s", math.Float64bits(f), got, want)
			}
		}
	}
	if mismatches > 10 {
		t.Errorf("%d mismatches in total", mismatches)
	}
}

// commaLocales are locales whose decimal point is ',' under which a host that
// has called setlocale runs the prelude; the first one installed is used.
var commaLocales = []string{"de_DE.UTF-8", "de_DE.utf8", "fr_FR.UTF-8", "fr_FR.utf8", "nl_NL.UTF-8", "pt_BR.UTF-8", "ru_RU.UTF-8"}

// TestCRealNotationIsLocaleIndependent runs the prelude's Real parsing and
// printing under a locale whose decimal point is ',' and checks that '.'
// notation still reads and prints as under the C locale, with overflow,
// underflow and non-decimal notation classified as before.
func TestCRealNotationIsLocaleIndependent(t *testing.T) {
	cc := os.Getenv(CCompilerEnvVar)
	if cc == "" {
		cc = "cc"
	}
	if _, err := exec.LookPath(cc); err != nil {
		t.Skip("no C compiler on PATH")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "locale.c")
	program := fmt.Sprintf("#define SYSML_MAX_CALC_DEPTH %d\n", runtime.DefaultMaxCalcDepth) + cPrelude + `
#include <locale.h>
/* argv[1] is the locale, argv[2] the Real to parse and print; exits 3 when the locale is not installed. */
int main(int argc, char **argv) {
	if (argc != 3) return 4;
	if (setlocale(LC_ALL, argv[1]) == NULL) return 3;
	if (strcmp(localeconv()->decimal_point, ",") != 0) return 3;
	sysml_print_real(sysml_parse_real(argv[2], "x"));
	return 0;
}
`
	if err := os.WriteFile(src, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "locale")
	build := exec.Command(cc, append(append([]string{}, CFlags...), "-o", bin, src, "-lm")...) // #nosec G204 -- test-owned arguments
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s", err, out)
	}

	locale := ""
	for _, candidate := range commaLocales {
		if err := exec.Command(bin, candidate, "1.0").Run(); err == nil { // #nosec G204 -- test-owned arguments
			locale = candidate
			break
		}
	}
	if locale == "" {
		t.Skipf("none of %v is installed", commaLocales)
	}

	for _, text := range []string{"1.5", "2.5e3", "0.1", "-0.000123", "1e21", "9.5e20", "5e-324", "123456789012345680", "1.7976931348623157e308", "0.0", "-0.5E-10"} {
		want, err := semantics.ParseReal(text)
		if err != nil {
			t.Fatalf("ParseReal(%q): %v", text, err)
		}
		out, err := exec.Command(bin, locale, text).Output() // #nosec G204 -- test-owned arguments
		if err != nil {
			t.Errorf("%s under %s: %v", text, locale, err)
			continue
		}
		if got := strings.TrimSpace(string(out)); got != semantics.FormatReal(want) {
			t.Errorf("%s under %s prints %s, want %s", text, locale, got, semantics.FormatReal(want))
		}
	}
	for _, tc := range []struct {
		text   string
		status int
		stderr string
	}{
		{"1e400", 1, "arithmetic overflow: 1e400 is outside the Real range"},
		{"1e-400", 1, "arithmetic overflow: 1e-400 is outside the Real range"},
		{"1,5", 2, "1,5 is not a finite Real in decimal notation"},
		{"0x1p-2", 2, "is not a finite Real in decimal notation"},
	} {
		var stderr bytes.Buffer
		cmd := exec.Command(bin, locale, tc.text) // #nosec G204 -- test-owned arguments
		cmd.Stderr = &stderr
		err := cmd.Run()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != tc.status {
			t.Errorf("%s under %s: %v, want exit status %d", tc.text, locale, err, tc.status)
		}
		if !strings.Contains(stderr.String(), tc.stderr) {
			t.Errorf("%s under %s: stderr %q, want %q", tc.text, locale, stderr.String(), tc.stderr)
		}
	}
}
