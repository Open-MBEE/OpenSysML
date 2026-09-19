package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// F71: `timeslice` names a KerML feature, so the parameter of `expr while` builds
// a symbol and `at(timeslice.interval)` resolves against it.
func TestF71TimesliceParameterResolves(t *testing.T) {
	const src = `package ExtendedOccurrences {
    class Interval;
    class Timeslice {
        feature interval : Interval;
    }
    class ExtendedOccurrence {
        expr at {
            in interval : Interval;
            return result : Timeslice;
        }

        expr while {
            in timeslice : Timeslice;
            return result : Timeslice = at(timeslice.interval);
        }

        expr snapshotInterval {
            in snapshot : Timeslice;
            return result : Interval = snapshot.interval;
        }
    }
}`

	ws := NewWorkspace()
	ws.Open("f71.kerml", []byte(src), 1)

	var errs []string
	for _, d := range ws.Diagnostics("f71.kerml") {
		if d.Severity == diag.SeverityError {
			errs = append(errs, d.Message)
		}
	}
	if len(errs) > 0 {
		t.Fatalf("expected the model to analyse cleanly, got:\n%s", strings.Join(errs, "\n"))
	}
}
