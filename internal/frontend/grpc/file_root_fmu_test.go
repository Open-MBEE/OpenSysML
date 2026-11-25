//go:build !sysml_re && !sysml_nofmi

package grpc

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// An FMU named relative to the root converts from the root: the importer opens the
// file again, and must open the one the service read, not one beside the working directory.
func TestConfinedServiceConvertsAnFMUNamedRelativeToItsRoot(t *testing.T) {
	const description = `<fmiModelDescription fmiVersion="2.0" modelName="BouncingBall" guid="{1}">
	<CoSimulation modelIdentifier="BouncingBall"/>
	<ModelVariables>
		<ScalarVariable name="h" valueReference="2" causality="output"><Real start="1.0"/></ScalarVariable>
	</ModelVariables>
</fmiModelDescription>`
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	entry, err := w.Create("modelDescription.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(description)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ball.fmu"), archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	srv, err := NewService(10, "test", ConfineFilesTo(root))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(srv.Close)
	resp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source: &pb.ConvertRequest_FilePath{FilePath: "ball.fmu"}, ToFormat: "sysml",
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if resp.Error != "" || !strings.Contains(resp.Content, "calc def BouncingBall") {
		t.Errorf("Convert of a root-relative FMU: error %q, output:\n%s", resp.Error, resp.Content)
	}
}
