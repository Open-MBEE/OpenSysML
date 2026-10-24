package view

import (
	"strings"
	"testing"
)

// A rendering with nothing to show says why in the terms of the view's exposure:
// what it exposes is filtered out (naming the filter), does not resolve, or holds
// nothing; and the reason reaches the text form and the machine form alike.
func TestEmptyRenderingReasons(t *testing.T) {
	cases := []struct {
		view string
		want string
	}{
		{"EmptyReasons::filteredOut", "every element the view exposes is rejected by its filter @ActionUsage; the rendering is empty"},
		{"EmptyReasons::inheritedFilter", "every element the view exposes is rejected by its filter @ActionUsage; the rendering is empty"},
		{"EmptyReasons::unresolved", "the view exposes the members of Nowhere, which does not resolve; the rendering is empty"},
		{"EmptyReasons::hollow", "the view exposes the members of Hollow, and there is nothing there; the rendering is empty"},
		{"EmptyReasons::hollowFiltered", "the view exposes the members of Hollow, and there is nothing there; the rendering is empty"},
		{"MatrixReasons::unnamedFiltered", "every element the view exposes is rejected by its filter @SysML::AllocationUsage; the rendering is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.view, func(t *testing.T) {
			rendering := render(t, "empty-reasons.sysml", tc.view)
			if !rendering.Empty() {
				t.Fatalf("rendering is not empty: %+v", rendering.Roots)
			}
			if got := rendering.EmptyReason(); got != tc.want {
				t.Errorf("EmptyReason() = %q, want %q", got, tc.want)
			}
			if text := rendering.Text(); !strings.Contains(text, tc.want) {
				t.Errorf("text does not say why:\n%s", text)
			}
			data := rendering.Data()
			if len(data.Nodes) != 0 || len(data.Edges) != 0 || len(data.Rows) != 0 {
				t.Fatalf("data is not empty: %+v", data)
			}
			if len(data.Notices) != 1 || data.Notices[0] != tc.want {
				t.Errorf("Data().Notices = %q, want the reason alone", data.Notices)
			}
		})
	}
}

// The machine form of every blank rendering carries the reason the text form
// gives, whichever way it came to be blank.
func TestBlankDataCarriesTheReason(t *testing.T) {
	cases := []struct {
		file, view, want string
	}{
		{"errors.sysml", "ErrorViews::emptyView", "the view exposes nothing; the rendering is empty"},
		{"errors.sysml", "ErrorViews::emptySequenceView", "the view exposes nothing; the rendering is empty"},
		{"errors.sysml", "ErrorViews::misfitView", "the rendering is empty: nothing the view exposes is shown by an action rendering"},
		{"pictures.sysml", "Site::pictureOnlyView", "the view shows 1 picture(s) and no element"},
	}
	for _, tc := range cases {
		t.Run(tc.view, func(t *testing.T) {
			data := render(t, tc.file, tc.view).Data()
			if len(data.Nodes) != 0 || len(data.Edges) != 0 || len(data.Rows) != 0 {
				t.Fatalf("data is not blank: %d nodes, %d edges, %d rows", len(data.Nodes), len(data.Edges), len(data.Rows))
			}
			if len(data.Notices) == 0 || data.Notices[len(data.Notices)-1] != tc.want {
				t.Errorf("Data().Notices = %q, want to end with %q", data.Notices, tc.want)
			}
		})
	}
	drawn := render(t, "filters.sysml", "FilteredViews::safetyView").Data()
	for _, notice := range drawn.Notices {
		if strings.Contains(notice, "the rendering is empty") {
			t.Errorf("a drawn rendering carries an empty-rendering notice: %q", drawn.Notices)
		}
	}
}

// Elements rendered without a view say so in the document's terms.
func TestEmptyExposedRenderingSaysWhy(t *testing.T) {
	r, _ := loadFixture(t, "errors.sysml")
	rendering, err := r.RenderExposed(nil, KindTree, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := "the document declares no element; the rendering is empty"; rendering.EmptyReason() != want {
		t.Errorf("EmptyReason() = %q, want %q", rendering.EmptyReason(), want)
	}
}
