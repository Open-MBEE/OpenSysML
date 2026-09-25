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
