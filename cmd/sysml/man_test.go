package main

import (
	"flag"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/usage"
)

// The shipped page is generated, so a flag or a section added to the
// description without regenerating it is a failure here rather than a page that
// documents a command that no longer exists.
func TestTheShippedManualPageIsWhatTheCommandRenders(t *testing.T) {
	needs(t, features...) // the shipped page is the full build's
	t.Setenv("SOURCE_DATE_EPOCH", "")
	if err := usage.CheckShippedPage("../..", doc(), docFlags()); err != nil {
		t.Error(err)
	}
}

func TestTheManualPageDocumentsEveryFlag(t *testing.T) {
	page := string(usage.Page(doc(), docFlags()))
	docFlags().VisitAll(func(f *flag.Flag) {
		if _, standIn := f.Value.(*omittedFlag); standIn {
			return // refused by this build, so documented by none of its pages
		}
		// A hyphen means itself in roff, so every one of them is escaped.
		name := "\\-" + strings.ReplaceAll(f.Name, "-", "\\-")
		if !strings.Contains(page, name+" ") && !strings.Contains(page, name+"\n") {
			t.Errorf("the manual page does not document -%s", f.Name)
		}
	})
}
