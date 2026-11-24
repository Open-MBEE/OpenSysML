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

func TestConfiningToAMissingDirectoryIsAConfigurationError(t *testing.T) {
	if srv, err := NewService(10, "test", ConfineFilesTo(filepath.Join(t.TempDir(), "absent"))); err == nil {
		srv.Close()
		t.Error("NewService accepted a file root that does not exist")
	}
}
