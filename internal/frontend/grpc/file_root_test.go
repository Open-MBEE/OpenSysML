package grpc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A service confined to a directory reads the files requests name under it, by a
// relative or an absolute path, and refuses one elsewhere: by an absolute path, by
// "..", or through a symbolic link that leads out.
func TestConfinedServiceReadsNamedFilesOnlyUnderItsRoot(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const model = "package P { part def A; }"
	inside := filepath.Join(root, "inside.sysml")
	secret := filepath.Join(outside, "secret.sysml")
	for _, path := range []string{inside, secret} {
		if err := os.WriteFile(path, []byte(model), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	refused := []string{secret, filepath.Join("..", filepath.Base(outside), "secret.sysml")}
	if err := os.Symlink(secret, filepath.Join(root, "link.sysml")); err == nil {
		refused = append(refused, "link.sysml", filepath.Join(root, "link.sysml"))
	}

	srv, err := NewService(10, "test", ConfineFilesTo(root))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(srv.Close)
	ctx := context.Background()

	for _, path := range []string{inside, "inside.sysml"} {
		if _, err := srv.ParseFile(ctx, &pb.ParseFileRequest{Source: &pb.ParseFileRequest_FilePath{FilePath: path}}); err != nil {
			t.Errorf("ParseFile(%q) under the root: %v", path, err)
		}
	}
	for _, path := range refused {
		calls := map[string]func() error{
			"ParseFile": func() error {
				_, err := srv.ParseFile(ctx, &pb.ParseFileRequest{Source: &pb.ParseFileRequest_FilePath{FilePath: path}})
				return err
			},
			"ParseSources": func() error {
				_, err := srv.ParseSources(ctx, &pb.ParseSourcesRequest{Documents: []*pb.SourceDocument{
					{Source: &pb.SourceDocument_FilePath{FilePath: path}},
				}})
				return err
			},
			"Convert": func() error {
				_, err := srv.Convert(ctx, &pb.ConvertRequest{Source: &pb.ConvertRequest_FilePath{FilePath: path}, ToFormat: "ttl"})
				return err
			},
		}
		for name, call := range calls {
			if err := call(); connect.CodeOf(err) != connect.CodePermissionDenied {
				t.Errorf("%s(%q) outside the root: got %v, want permission denied", name, path, err)
			}
		}
	}
}

// Without a root the service reads any path it can, as the private child a client
// starts for itself must.
func TestUnconfinedServiceReadsAnyNamedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(path, []byte("package P;"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := mustNewService(t, 10)
	if _, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_FilePath{FilePath: path}}); err != nil {
		t.Errorf("ParseFile(%q): %v", path, err)
	}
}

// A root is a directory that is there: a missing one, a regular file and an empty
// name are each refused when the service starts, not served unconfined.
func TestConfiningToWhatIsNotADirectoryIsAConfigurationError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(file, []byte("package P;"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, dir := range map[string]string{
		"a missing directory": filepath.Join(t.TempDir(), "absent"),
		"a regular file":      file,
		"an empty name":       "",
	} {
		if srv, err := NewService(10, "test", ConfineFilesTo(dir)); err == nil {
			srv.Close()
			t.Errorf("NewService accepted %s as its file root", name)
		}
	}
}

// A confined service reads the file the system would for the name: one whose name
// starts with a space, and the sibling of a link's target for a ".." after the link.
func TestConfinedServiceReadsTheFileTheNameResolvesTo(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		" spaced.sysml":                       "package Spaced;",
		"spaced.sysml":                        "package Trimmed;",
		"beside.sysml":                        "package BesideTheLink;",
		filepath.Join("deep", "beside.sysml"): "package BesideTheTarget;",
	}
	if err := os.MkdirAll(filepath.Join(root, "deep", "target"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	names := map[string]string{" spaced.sysml": "package Spaced;"}
	if err := os.Symlink(filepath.Join(root, "deep", "target"), filepath.Join(root, "link")); err == nil {
		sep := string(filepath.Separator)
		names["link"+sep+".."+sep+"beside.sysml"] = "package BesideTheTarget;"
	}
	srv, err := NewService(10, "test", ConfineFilesTo(root))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(srv.Close)
	for name, want := range names {
		if data, err := srv.readNamedFile(name); err != nil || string(data) != want {
			t.Errorf("readNamedFile(%q) = %q, %v; want %q", name, data, err, want)
		}
	}
}

// A link put where a checked file was does not lead the read out of the root.
func TestConfinedReadGoesThroughTheRoot(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(t.TempDir(), "secret.sysml")
	if err := os.WriteFile(secret, []byte("package Secret;"), 0o600); err != nil {
		t.Fatal(err)
	}
	opened, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = opened.Close() }()
	if err := os.Symlink(secret, filepath.Join(root, "swapped.sysml")); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	if data, err := opened.ReadFile("swapped.sysml"); err == nil {
		t.Errorf("a read through the root followed a link out of it: %q", data)
	}
}
