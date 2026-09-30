package docir

import (
	"testing"
)

// TestEvaluateImage locks the image content node: the plan's location, caption
// and alt text arrive verbatim.
func TestEvaluateImage(t *testing.T) {
	fixture := loadEvaluationFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part photo : Image {
				attribute redefines location = "images/scope.png";
				attribute redefines caption = "The telescope at first light";
				attribute redefines alt = "a telescope";
			}
		}
	`)
	document := fixture.mustEvaluate(t, "Report")
	content := document.Content()
	if len(content) != 1 {
		t.Fatalf("content = %d, want 1", len(content))
	}
	photo := content[0]
	if photo.Kind() != ContentImage || photo.Name() != "photo" {
		t.Fatalf("image = %s %q", photo.Kind(), photo.Name())
	}
	if photo.Location() != "images/scope.png" || photo.Caption() != "The telescope at first light" || photo.Alt() != "a telescope" {
		t.Errorf("location = %q caption = %q alt = %q", photo.Location(), photo.Caption(), photo.Alt())
	}
	if !photo.Origin().Located() {
		t.Errorf("image has no origin")
	}
}
