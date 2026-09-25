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
	fleet := []byte("\x89PNG fleet bytes")
	depot := []byte("\x89PNG depot bytes")
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
