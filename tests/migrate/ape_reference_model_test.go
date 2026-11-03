package migrate_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

func TestAPEReferenceModelMigration(t *testing.T) {
	const root = "testdata/ape-reference-model"
	const archive = "APE-ReferenceModel.mdzip"
	data, err := os.ReadFile(filepath.Join(root, archive))
	if os.IsNotExist(err) && os.Getenv("OPENSYSML_REQUIRE_APE_REFERENCE_MODEL") != "1" {
		t.Skip("supply the APE archive described in testdata/ape-reference-model/README.md; set OPENSYSML_REQUIRE_APE_REFERENCE_MODEL=1 to require it")
	}
	if err != nil {
		t.Fatalf("read required source archive: %v", err)
	}
	manifestData, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Files []struct {
			Path   string `json:"path"`
			Bytes  int    `json:"bytes"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	archiveFound := false
	for _, f := range manifest.Files {
		if f.Path == archive {
			archiveFound = true
			apeFixtureDigest(t, archive, data, f.Bytes, f.SHA256)
		}
	}
	if !archiveFound {
		t.Fatal("manifest has no source archive checksum")
	}

	// Use the conversion entry point so malformed generated notation fails its syntax gate.
	r, err := convert.Migrate(archive, data, convert.FormatSysML, convert.MigrateOptions{})
	if err != nil {
		t.Fatalf("migrate APE through the public conversion path: %v", err)
	}
	if len(r.Output) == 0 || r.Report == nil || len(r.Report.Entries) == 0 {
		t.Fatal("migration produced no notation or element report")
	}
	for _, name := range []string{"MAPS Spectrum Mean", "MAPS Spectrum Variance"} {
		want := "::'Test Requirements'::'MAPS Quality of turbulence'::'" + name + "';"
		if !strings.Contains(string(r.Output), want) {
			t.Errorf("missing qualified reference to nested requirement %q", name)
		}
	}
	seen := map[string]bool{}
	for _, entry := range r.Report.Entries {
		if entry.ID == "" || seen[entry.ID] {
			t.Errorf("missing or duplicate source identity in migration report: %q", entry.ID)
		}
		seen[entry.ID] = true
	}
	images := 0
	for _, f := range manifest.Files {
		if !strings.HasPrefix(f.Path, "generated/images/") {
			continue
		}
		images++
		name := strings.TrimPrefix(f.Path, "generated/")
		image, ok := r.Files[name]
		if !ok {
			t.Errorf("migration lost attached image %q", name)
			continue
		}
		apeFixtureDigest(t, name, image, f.Bytes, f.SHA256)
	}
	if images == 0 || len(r.Files) != images {
		t.Errorf("attached files: got %d, want the %d images recorded in the source inventory", len(r.Files), images)
	}
	t.Log(r.Report.Summary())
}

func apeFixtureDigest(t *testing.T, name string, data []byte, size int, want string) {
	t.Helper()
	digest := sha256.Sum256(data)
	if len(data) != size || hex.EncodeToString(digest[:]) != want {
		t.Fatalf("%s does not match the fixture inventory (size %d, SHA-256 %x)", name, len(data), digest)
	}
}
