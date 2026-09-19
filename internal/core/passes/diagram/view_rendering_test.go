package diagram_test

import (
	"strings"
	"testing"
)

func TestW8DViewWithSecondRendering(t *testing.T) {
	const src = `package P {
		view def V {
			render rendering r1;
			render rendering r2;
			rendering r3;
		}
		view v {
			render rendering r4;
			render rendering r5;
		}
	}`
	w8dWantLines(t, src, "only-one-view-rendering", 4, 9)
	for _, d := range only(w8dDiags(t, src), "only-one-view-rendering") {
		line := w8dLine(src, d.Span)
		wantDef := line == 4
		isDef := strings.Contains(d.Message, "view definition")
		if wantDef != isDef {
			t.Fatalf("line %d: unexpected message %q", line, d.Message)
		}
	}
}

// One `render` per view is legal, and plain rendering members are no view
// renderings however many a view owns.
func TestW8DSingleViewRenderingStaysSilent(t *testing.T) {
	const src = `package P {
		rendering def RD;
		view def V {
			render rendering r1 : RD;
			rendering r2 : RD;
			rendering r3 : RD;
		}
		view v : V {
			render rendering r4 : RD;
		}
	}`
	if diags := only(w8dDiags(t, src), "only-one-view-rendering"); len(diags) != 0 {
		t.Fatalf("single renderings reported: %v", diags)
	}
}
