package docplan

import (
	"testing"
)

// TestCompileImage locks the image block: its file location, caption and alt
// text arrive as planned attributes.
func TestCompileImage(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part photo : Image {
				attribute redefines location = "images/scope.png";
				attribute redefines caption = "The telescope at first light";
				attribute redefines alt = "a telescope";
			}
			part remote : Image {
				attribute redefines location = "https://example.test/dish.png";
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	content := plan.Content()
	if len(content) != 2 {
		t.Fatalf("content = %d, want 2", len(content))
	}
	photo := content[0]
	if photo.Kind() != ContentImage || photo.Name() != "photo" {
		t.Fatalf("image = %s %q", photo.Kind(), photo.Name())
	}
	if photo.Location() != "images/scope.png" || photo.Caption() != "The telescope at first light" || photo.Alt() != "a telescope" {
		t.Errorf("location = %q caption = %q alt = %q", photo.Location(), photo.Caption(), photo.Alt())
	}
	remote := content[1]
	if remote.Kind() != ContentImage || remote.Location() != "https://example.test/dish.png" || remote.Caption() != "" || remote.Alt() != "" {
		t.Errorf("remote = %s %q caption %q alt %q", remote.Kind(), remote.Location(), remote.Caption(), remote.Alt())
	}
}

// TestCompileImageRequiresLocation checks an image stating no location — or a
// blank one — is diagnosed the way a formula without a source is.
func TestCompileImageRequiresLocation(t *testing.T) {
	for _, attribute := range []string{"", "attribute redefines location = \"   \";"} {
		fixture := loadPlanningFixture(t, `
			part def Report :> Document {
				attribute redefines title = "Report";
				part photo : Image {
					`+attribute+`
				}
			}
		`)
		_, err := fixture.compile(t, "Report")
		planning := planningError(t, err)
		if planning.Kind != ErrorMissingImageLocation || planning.Content != "Observatory::Report::photo" {
			t.Fatalf("attribute %q: error = %+v", attribute, planning)
		}
	}
}
