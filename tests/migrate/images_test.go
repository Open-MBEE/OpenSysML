package migrate_test

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// zipWith zips the named fixture with the extra entries, as an mdzip holds an
// attachment beside the model document.
func zipWith(t *testing.T, fixture string, extra map[string][]byte) []byte {
	t.Helper()
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	return zipData(t, data, extra)
}

// zipData zips the model bytes with the extra entries, as zipWith does for a file.
func zipData(t *testing.T, data []byte, extra map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entries := map[string][]byte{
		"com.nomagic.magicdraw.uml_model.model": data,
	}
	for name, content := range extra {
		entries[name] = content
	}
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestImageParagraphsFromArchive migrates the documents fixture as an mdzip
// holding the images its image paragraphs attach: each becomes an Image block
// whose location names the written sidecar file, and the archive's bytes are
// registered for writing.
func TestImageParagraphsFromArchive(t *testing.T) {
	fleet := []byte("\x89PNG\r\n\x1a\n fleet bytes")
	depot := []byte("\x89PNG\r\n\x1a\n depot bytes")
	r, err := migrate.Migrate("documents.mdzip", zipWith(t, "testdata/xmi/documents.xmi", map[string][]byte{
		"attachments/fleet.png": fleet,
		"attachments/depot.png": depot,
	}))
	if err != nil {
		t.Fatal(err)
	}
	wantLine(t, r.Notation, "attribute redefines location = \"images/fleet.png\";")
	wantLine(t, r.Notation, "attribute redefines location = \"images/depot.png\";")
	if !strings.Contains(string(r.Notation), ": DocumentQueries::Image") {
		t.Fatalf("notation lacks an Image block:\n%s", r.Notation)
	}
	if !bytes.Equal(r.Files["images/fleet.png"], fleet) || !bytes.Equal(r.Files["images/depot.png"], depot) {
		t.Errorf("Files = %v", keysOf(r.Files))
	}
	if r.Report.Images != 2 {
		t.Errorf("Report.Images = %d, want 2", r.Report.Images)
	}
}

// TestImageParagraphFromStream migrates the layout Cameo writes for real: the
// comment's ATTACHED_FILE extension names the archive entry by streamContentID
// and the AttachedFile tag names the file, so the entry's bytes are written
// under the tag's name.
func TestImageParagraphFromStream(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/documents.xmi")
	if err != nil {
		t.Fatal(err)
	}
	const stream = "BINARY-5f61da68-044e-4f13-848c-1aa8b6afd6fd"
	jpeg := []byte("\xFF\xD8\xFF\xE0 Clipboard33")
	doc := strings.Replace(string(data),
		`<ownedComment xmi:type="uml:Comment" xmi:id="_note_image" body="Figure: the fleet at the depot"/>`,
		`<ownedComment xmi:type="uml:Comment" xmi:id="_note_image" body="Figure: the fleet at the depot">
			<xmi:Extension extender="MagicDraw UML 2022x">
				<md_extensions.ATTACHED_FILE>
					<MDFoundation:MDExtension source="ATTACHED_FILE">
						<element href="#_note_image" xsi:type="uml:Comment"/>
						<contents streamContentID="`+stream+`" xsi:type="binary:StreamIdentityBinaryObject"/>
					</MDFoundation:MDExtension>
				</md_extensions.ATTACHED_FILE>
			</xmi:Extension>
		</ownedComment>`, 1)
	doc = strings.Replace(doc, `file="fleet.png"`, `file="Clipboard33.jpg"`, 1)
	if doc == string(data) {
		t.Fatal("the fixture lacks the image comment")
	}
	r, err := migrate.Migrate("documents.mdzip", zipData(t, []byte(doc), map[string][]byte{stream: jpeg}))
	if err != nil {
		t.Fatal(err)
	}
	wantLine(t, r.Notation, `attribute redefines location = "images/Clipboard33.jpg";`)
	if !bytes.Equal(r.Files["images/Clipboard33.jpg"], jpeg) {
		t.Errorf("Files = %v", keysOf(r.Files))
	}
	wantOneNote(t, r, "_st_note_blank_image", migrate.Unmapped, `the attached image "depot.png" is not in the archive`)
}

// TestImageParagraphMissingFromArchive reports what an image paragraph earns
// when the archive holds no entry for its attached file: a captioned one
// falls back to the paragraph its caption makes, with the reason noted, and
// a captionless one is refused.
func TestImageParagraphMissingFromArchive(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/documents.xmi")
	if err != nil {
		t.Fatal(err)
	}
	r, err := migrate.Migrate("documents.xmi", data)
	if err != nil {
		t.Fatal(err)
	}
	wantOneNote(t, r, "_st_note_image", migrate.Approximated, `the attached image "fleet.png" is not in the archive; its caption stands as the paragraph`)
	wantLine(t, r.Notation, `attribute redefines text = "Figure: the fleet at the depot";`)
	wantOneNote(t, r, "_st_note_blank_image", migrate.Unmapped, `the attached image "depot.png" is not in the archive`)
	if len(r.Files) != 0 {
		t.Errorf("Files = %v, want none", keysOf(r.Files))
	}
}

func keysOf(files map[string][]byte) []string {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	return keys
}

// TestImageParagraphServedByBaseURL migrates an image paragraph whose comment
// embeds a server-relative <img src>: with -image-base-url the image resolves
// to a remote location, without it the caption stands as the paragraph with
// the note saying how to show it.
func TestImageParagraphServedByBaseURL(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/documents.xmi")
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.Replace(string(data), `xmi:id="_note_blank_image"/`,
		`xmi:id="_note_blank_image" body="&lt;img src=&quot;/projects/x/png&quot;&gt;Figure A"/`, 1)
	doc = strings.Replace(doc, `file="depot.png"`, `file=""`, 1)
	if doc == string(data) {
		t.Fatal("the fixture lacks the blank image comment")
	}
	r, err := migrate.MigrateOptions("documents.xmi", []byte(doc), migrate.Options{ImageBaseURL: "https://mms.example.org"})
	if err != nil {
		t.Fatal(err)
	}
	wantLine(t, r.Notation, `attribute redefines location = "https://mms.example.org/projects/x/png";`)
	if len(r.Files) != 0 {
		t.Errorf("Files = %v, want none for a remote image", keysOf(r.Files))
	}

	r, err = migrate.MigrateOptions("documents.xmi", []byte(doc), migrate.Options{})
	if err != nil {
		t.Fatal(err)
	}
	wantOneNote(t, r, "_st_note_blank_image", migrate.Approximated,
		`the image "/projects/x/png" is served by the View Editor; pass -image-base-url to show it; its caption stands as the paragraph`)
	wantLine(t, r.Notation, `attribute redefines text = "Figure A";`)
}

// TestParagraphBodyImageServedByBaseURL plans the first <img> of a regular
// collaborator paragraph's body as an Image block: the body text is its
// caption and the img's alt its alt text.
func TestParagraphBodyImageServedByBaseURL(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/documents.xmi")
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.Replace(string(data), `body="First note."`,
		`body="&lt;p&gt;&lt;img alt=&quot;&quot; src=&quot;/projects/y/png&quot;&gt;&lt;/p&gt;&lt;p&gt;Figure 1. Caption&lt;/p&gt;"`, 1)
	if doc == string(data) {
		t.Fatal("the fixture lacks the first note")
	}
	r, err := migrate.MigrateOptions("documents.xmi", []byte(doc), migrate.Options{ImageBaseURL: "https://mms.example.org"})
	if err != nil {
		t.Fatal(err)
	}
	wantLine(t, r.Notation, `attribute redefines location = "https://mms.example.org/projects/y/png";`)
	wantLine(t, r.Notation, `attribute redefines caption = "Figure 1. Caption";`)
	wantLine(t, r.Notation, `attribute redefines alt = "Figure 1. Caption";`)
}

// TestImageBaseURLRejected validates the base URL is an absolute http(s) URL.
func TestImageBaseURLRejected(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/documents.xmi")
	if err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{"ftp://x", "mms.example.org"} {
		if _, err := migrate.MigrateOptions("documents.xmi", data, migrate.Options{ImageBaseURL: base}); err == nil ||
			!strings.Contains(err.Error(), "not an absolute http(s) URL") {
			t.Errorf("ImageBaseURL %q: err = %v", base, err)
		}
	}
}

// TestFigureNoteImage turns a figure diagram that draws nothing but whose note
// holds an <img> into an Image block: with -image-base-url the server path
// resolves, without it the figure stays left out with the hint in its note.
func TestFigureNoteImage(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/figures.xmi")
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.Replace(string(data),
		`<ownedDiagram xmi:type="uml:Diagram" xmi:id="_diag_unlisted" name="Unlisted" ownerOfDiagram="_pkg_plant">`,
		`<ownedDiagram xmi:type="uml:Diagram" xmi:id="_diag_unlisted" name="Unlisted" ownerOfDiagram="_pkg_plant">
			<ownedComment xmi:type="uml:Comment" xmi:id="_cmt_unlisted" body="&lt;p&gt;&lt;img alt=&quot;&quot; src=&quot;/projects/z/png&quot;&gt;&lt;/p&gt;&lt;p&gt;Figure 2. Caption&lt;/p&gt;"/>`,
		1)
	if doc == string(data) {
		t.Fatal("the fixture lacks the unlisted diagram")
	}
	r, err := migrate.MigrateOptions("figures.xmi", []byte(doc), migrate.Options{ImageBaseURL: "https://mms.example.org"})
	if err != nil {
		t.Fatal(err)
	}
	wantLine(t, r.Notation, `attribute redefines location = "https://mms.example.org/projects/z/png";`)
	wantLine(t, r.Notation, `attribute redefines caption = "Unlisted";`)
	wantLine(t, r.Notation, `attribute redefines text = "Figure 2. Caption";`)
	wantOneNote(t, r, "_st_pictures_image", migrate.Approximated,
		`the figure shows the image the diagram's note carries, https://mms.example.org/projects/z/png`)

	r, err = migrate.MigrateOptions("figures.xmi", []byte(doc), migrate.Options{})
	if err != nil {
		t.Fatal(err)
	}
	wantOneNote(t, r, "_st_pictures_image", migrate.Approximated,
		`the note's image "/projects/z/png" is served by the View Editor; pass -image-base-url to show it`)
}

// TestImageOnlyBody plans an Image block for a body holding only an <img> —
// the resolvable source wins over the empty-body refusal; an unresolvable
// server path without a base URL still refuses.
func TestImageOnlyBody(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/documents.xmi")
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.Replace(string(data), `body="First note."`,
		`body="&lt;img src=&quot;https://example.org/plate.png&quot;&gt;"`, 1)
	r, err := migrate.Migrate("documents.xmi", []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	wantLine(t, r.Notation, `attribute redefines location = "https://example.org/plate.png";`)

	doc = strings.Replace(string(data), `body="First note."`,
		`body="&lt;img src=&quot;/projects/q/png&quot;&gt;"`, 1)
	r, err = migrate.Migrate("documents.xmi", []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	wantOneNote(t, r, "_st_note_first", migrate.Unmapped, "the paragraph's comment has no body")
}

// TestImageTagMissingFallsToSrc resolves an image paragraph through its <img
// src> when the AttachedFile tag names a file no archive entry holds.
func TestImageTagMissingFallsToSrc(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/documents.xmi")
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.Replace(string(data), `file="fleet.png"`, `file="missing.png"`, 1)
	doc = strings.Replace(doc, `body="Figure: the fleet at the depot"`,
		`body="&lt;img src=&quot;/projects/p/png&quot;&gt;Plate"`, 1)
	if doc == string(data) {
		t.Fatal("the fixture lacks the image tag")
	}
	r, err := migrate.MigrateOptions("documents.xmi", []byte(doc), migrate.Options{ImageBaseURL: "https://mms.example.org"})
	if err != nil {
		t.Fatal(err)
	}
	wantLine(t, r.Notation, `attribute redefines location = "https://mms.example.org/projects/p/png";`)
	if len(r.Files) != 0 {
		t.Errorf("Files = %v, want none for a remote image", keysOf(r.Files))
	}
}

// TestImageAmbiguousBaseName refuses an attachment whose base name several
// archive entries share, the reason saying so.
func TestImageAmbiguousBaseName(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/documents.xmi")
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.Replace(string(data), `file="fleet.png"`, `file="figure.png"`, 1)
	r, err := migrate.Migrate("documents.mdzip", zipData(t, []byte(doc), map[string][]byte{
		"a/figure.png": []byte("\x89PNG\r\n\x1a\n one"),
		"b/figure.png": []byte("\x89PNG\r\n\x1a\n two"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	wantOneNote(t, r, "_st_note_image", migrate.Approximated,
		`the attached image "figure.png" matches 2 archive entries; the attachment names no stream`)
}

// TestImageNotAnImage refuses an attachment whose bytes are not an image, the
// reason naming the content type it read.
func TestImageNotAnImage(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/documents.xmi")
	if err != nil {
		t.Fatal(err)
	}
	r, err := migrate.Migrate("documents.mdzip", zipData(t, data, map[string][]byte{
		"attachments/fleet.png": []byte("a manifest, not a picture"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	wantOneNote(t, r, "_st_note_image", migrate.Approximated,
		`the attached image "fleet.png" is not an image (content type text/plain; charset=utf-8)`)
}

// TestImageUnnamedStreamNamesSVG an SVG stream the AttachedFile tag leaves
// unnamed is written under the stream id, suffixed by the type the bytes read.
func TestImageUnnamedStreamNamesSVG(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/documents.xmi")
	if err != nil {
		t.Fatal(err)
	}
	const stream = "BINARY-aaaabbbb-0000-0000-0000-0000000000ff"
	svg := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`)
	doc := strings.Replace(string(data),
		`<ownedComment xmi:type="uml:Comment" xmi:id="_note_image" body="Figure: the fleet at the depot"/>`,
		`<ownedComment xmi:type="uml:Comment" xmi:id="_note_image" body="Figure: the fleet at the depot">
			<xmi:Extension extender="MagicDraw UML 2022x">
				<md_extensions.ATTACHED_FILE>
					<MDFoundation:MDExtension source="ATTACHED_FILE">
						<element href="#_note_image" xsi:type="uml:Comment"/>
						<contents streamContentID="`+stream+`" xsi:type="binary:StreamIdentityBinaryObject"/>
					</MDFoundation:MDExtension>
				</md_extensions.ATTACHED_FILE>
			</xmi:Extension>
		</ownedComment>`, 1)
	doc = strings.Replace(doc, `file="fleet.png"`, `file=""`, 1)
	if doc == string(data) {
		t.Fatal("the fixture lacks the image comment")
	}
	r, err := migrate.Migrate("documents.mdzip", zipData(t, []byte(doc), map[string][]byte{stream: svg}))
	if err != nil {
		t.Fatal(err)
	}
	wantLine(t, r.Notation, `attribute redefines location = "images/`+stream+`.svg";`)
	if !bytes.Equal(r.Files["images/"+stream+".svg"], svg) {
		t.Errorf("Files = %v", keysOf(r.Files))
	}
}

// TestDocGenParagraphBodyImage a DocGen Paragraph step whose body is only an
// <img> tag becomes an Image when the source resolves.
func TestDocGenParagraphBodyImage(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/documents.xmi")
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.Replace(string(data),
		`base_CallBehaviorAction="_per_para" body="One truck."`,
		`base_CallBehaviorAction="_per_para" body="&lt;img src=&quot;https://example.org/plate.png&quot;&gt;"`, 1)
	if doc == string(data) {
		t.Fatal("the fixture lacks the paragraph step")
	}
	r, err := migrate.Migrate("documents.xmi", []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	wantLine(t, r.Notation, `attribute redefines location = "https://example.org/plate.png";`)
}

// TestImageParagraphNotesExtraImages an image paragraph whose body holds
// several <img>s shows the first and says how many more it left out.
func TestImageParagraphNotesExtraImages(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/documents.xmi")
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.Replace(string(data),
		`body="Figure: the fleet at the depot"`,
		`body="&lt;img src=&quot;https://a/x.png&quot;&gt;&lt;img src=&quot;https://a/y.png&quot;&gt;"`, 1)
	if doc == string(data) {
		t.Fatal("the fixture lacks the image comment")
	}
	r, err := migrate.Migrate("documents.xmi", []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	wantLine(t, r.Notation, `attribute redefines location = "https://a/x.png";`)
	if strings.Contains(string(r.Notation), `location = "https://a/y.png"`) {
		t.Error("the second image was planned too:\n" + string(r.Notation))
	}
	wantOneNote(t, r, "_st_note_image", migrate.Approximated, "1 more images in the body are left out")
}

// TestImageNameStaysInsideImages writes an attachment whose tag name climbs
// directories under images/ as a plain base name, and under the suffix its
// bytes call for rather than the one the tag states.
func TestImageNameStaysInsideImages(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/documents.xmi")
	if err != nil {
		t.Fatal(err)
	}
	fleet := []byte("\x89PNG\r\n\x1a\n fleet bytes")
	for _, tc := range []struct{ file, entry, want string }{
		{`../../fleet.txt`, "attachments/fleet.txt", "images/fleet.png"},
		{`..\..\fleet`, "attachments/fleet", "images/fleet.png"},
		{`..`, "attachments/..", "images/image.png"},
		{`fleet.PNG`, "attachments/fleet.PNG", "images/fleet.PNG"},
	} {
		doc := strings.Replace(string(data), `file="fleet.png"`, `file="`+tc.file+`"`, 1)
		if doc == string(data) {
			t.Fatal("the fixture lacks the image tag")
		}
		r, err := migrate.Migrate("documents.mdzip", zipData(t, []byte(doc), map[string][]byte{
			tc.entry:                fleet,
			"attachments/depot.png": []byte("\x89PNG\r\n\x1a\n depot bytes"),
		}))
		if err != nil {
			t.Fatal(err)
		}
		wantLine(t, r.Notation, `attribute redefines location = "`+tc.want+`";`)
		if !bytes.Equal(r.Files[tc.want], fleet) {
			t.Errorf("%q: Files = %v, want %s", tc.file, keysOf(r.Files), tc.want)
		}
		for name := range r.Files {
			if !strings.HasPrefix(name, "images/") || strings.Contains(name[len("images/"):], "/") || strings.Contains(name, "..") {
				t.Errorf("%q: file %q is not a plain name under images/", tc.file, name)
			}
		}
	}
}

// TestFigureNoteImageKeepsALongerNote keeps a note that begins with the
// figure's title but says more as the caption paragraph, and drops one the
// title already says.
func TestFigureNoteImageKeepsALongerNote(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/figures.xmi")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		note string
		kept bool
	}{
		{"Unlisted", false},
		{"Unlisted plant, as installed", true},
	} {
		doc := strings.Replace(string(data),
			`<ownedDiagram xmi:type="uml:Diagram" xmi:id="_diag_unlisted" name="Unlisted" ownerOfDiagram="_pkg_plant">`,
			`<ownedDiagram xmi:type="uml:Diagram" xmi:id="_diag_unlisted" name="Unlisted" ownerOfDiagram="_pkg_plant">
			<ownedComment xmi:type="uml:Comment" xmi:id="_cmt_unlisted" body="&lt;p&gt;&lt;img alt=&quot;&quot; src=&quot;/projects/z/png&quot;&gt;&lt;/p&gt;&lt;p&gt;`+tc.note+`&lt;/p&gt;"/>`,
			1)
		r, err := migrate.MigrateOptions("figures.xmi", []byte(doc), migrate.Options{ImageBaseURL: "https://mms.example.org"})
		if err != nil {
			t.Fatal(err)
		}
		wantLine(t, r.Notation, `attribute redefines caption = "Unlisted";`)
		if got := bytes.Contains(r.Notation, []byte(`attribute redefines text = "`+tc.note+`";`)); got != tc.kept {
			t.Errorf("note %q kept as a paragraph = %v, want %v:\n%s", tc.note, got, tc.kept, r.Notation)
		}
	}
}
