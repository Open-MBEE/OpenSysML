// Package testbudget applies an opt-in multiplier to wall-clock test deadlines.
//
// Set OPENSYSML_TEST_BUDGET_SCALE to a positive number to stretch deadlines on
// a contended host. Unset, every deadline is exactly the value its test
// declares, so ordinary local runs and CI are unchanged. Only deadlines are
// scaled: no expected value, model budget, step bound or assertion changes.
package testbudget

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// EnvName names the environment variable that holds the scale.
const EnvName = "OPENSYSML_TEST_BUDGET_SCALE"

// maxScale keeps a scaled deadline inside time.Duration's range and prevents a
// typo from turning a seconds-long deadline into a day-long hang.
const maxScale = 1000

// Parse interprets a scale value. Empty or whitespace means 1, the default;
// anything else must be a finite positive number no greater than maxScale.
func Parse(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 1, nil
	}
	scale, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s=%q: want a positive number: %w", EnvName, value, err)
	}
	if math.IsNaN(scale) || math.IsInf(scale, 0) || scale <= 0 || scale > maxScale {
		return 0, fmt.Errorf("%s=%q: want a finite number in (0, %d]", EnvName, value, maxScale)
	}
	return scale, nil
}

// Scale returns the scale named by the environment, or 1 when it is unset.
func Scale() (float64, error) {
	return Parse(os.Getenv(EnvName))
}

// Apply returns d multiplied by the configured scale. It returns an error
// rather than panicking when the configuration is invalid.
func Apply(d time.Duration) (time.Duration, error) {
	scale, err := Scale()
	if err != nil {
		return 0, err
	}
	if float64(d) > float64(math.MaxInt64)/scale {
		return 0, fmt.Errorf("%s: scaling %s by %g overflows a duration", EnvName, d, scale)
	}
	return time.Duration(float64(d) * scale), nil
}

// Fataler is the part of testing.TB the Deadline helper needs, kept as an
// interface so this package never imports testing into a shipped package.
type Fataler interface {
	Fatalf(format string, args ...any)
}

// Deadline returns d stretched by the configured scale, failing t when the
// configuration is invalid. Call it at the start of a test, before the work a
// deadline bounds, so a misconfiguration fails at the point of use.
func Deadline(t Fataler, d time.Duration) time.Duration {
	scaled, err := Apply(d)
	if err != nil {
		t.Fatalf("test budget: %v", err)
	}
	return scaled
}
