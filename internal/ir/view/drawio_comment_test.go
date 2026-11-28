package view

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

// A notice naming an element with a run of hyphens stays a well-formed XML comment.
func TestDrawioCommentHoldsNoDoubleHyphen(t *testing.T) {
	for _, text := range []string{"a--b", "its step load---arm (64×28 px)", "a----b", "ends-"} {
		w := &drawioWriter{}
		w.comment(text)
		w.b.WriteString("<mxfile/>")
		out := w.b.String()
		if body := strings.TrimSuffix(strings.TrimPrefix(out, "<!--"), "-->\n<mxfile/>"); strings.Contains(body, "--") {
			t.Errorf("comment of %q holds a double hyphen: %s", text, out)
		}
		d := xml.NewDecoder(strings.NewReader(out))
		for {
			if _, err := d.Token(); err == io.EOF {
				break
			} else if err != nil {
				t.Errorf("comment of %q does not parse: %v\n%s", text, err, out)
				break
			}
		}
	}
}
