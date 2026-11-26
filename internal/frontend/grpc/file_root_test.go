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

// A name that leaves the root on its way to a file under it is refused as a missing
// file is, so the answer does not tell which paths outside the root lead into it.
func TestConfinedServiceRefusesAWayIntoTheRootFromOutside(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.sysml"), []byte("package P;"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "inside.sysml"), filepath.Join(outside, "alias.sysml")); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	for _, link := range [][2]string{
		{root, filepath.Join(outside, "back")},
		{outside, filepath.Join(root, "out")},
	} {
		if err := os.Symlink(link[0], link[1]); err != nil {
			t.Fatal(err)
		}
	}
	srv, err := NewService(10, "test", ConfineFilesTo(root))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(srv.Close)
	sep := string(filepath.Separator)
	missing := connect.CodeOf(srv.readNamedFileError(filepath.Join(outside, "absent.sysml")))
	for _, name := range []string{
		filepath.Join(outside, "alias.sysml"),
		filepath.Join(outside, "back", "inside.sysml"),
		".." + sep + filepath.Base(outside) + sep + "alias.sysml",
		"out" + sep + "back" + sep + "inside.sysml",
		"out" + sep + "alias.sysml",
	} {
		err := srv.readNamedFileError(name)
		if err == nil || connect.CodeOf(err) != missing {
			t.Errorf("readNamedFile(%q) = %v, want the refusal a missing file gets (%v)", name, err, missing)
		}
	}
}

// The root is the same directory by the name it was given and by the one that
// resolves to, and a link inside it may name its target by either.
func TestConfinedServiceReadsUnderEitherNameOfItsRoot(t *testing.T) {
	physical, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(physical, "inside.sysml"), []byte("package P;"), 0o600); err != nil {
		t.Fatal(err)
	}
	given := filepath.Join(t.TempDir(), "given")
	if err := os.Symlink(physical, given); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	for name, target := range map[string]string{
		"by-given.sysml":    filepath.Join(given, "inside.sysml"),
		"by-physical.sysml": filepath.Join(physical, "inside.sysml"),
		"relative.sysml":    "inside.sysml",
	} {
		if err := os.Symlink(target, filepath.Join(physical, name)); err != nil {
			t.Fatal(err)
		}
	}
	srv, err := NewService(10, "test", ConfineFilesTo(given))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(srv.Close)
	for _, name := range []string{
		filepath.Join(given, "inside.sysml"), filepath.Join(physical, "inside.sysml"),
		"by-given.sysml", "by-physical.sysml", "relative.sysml",
	} {
		if data, err := srv.readNamedFile(name); err != nil || string(data) != "package P;" {
			t.Errorf("readNamedFile(%q) = %q, %v", name, data, err)
		}
	}
}

// An absolute name is under the root however its separators are doubled, as it is
// for the system.
func TestConfinedServiceReadsAnAbsoluteNameWithRepeatedSeparators(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.sysml"), []byte("package P;"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv, err := NewService(10, "test", ConfineFilesTo(root))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(srv.Close)
	sep := string(filepath.Separator)
	parent, base := filepath.Split(root)
	for _, name := range []string{
		parent + sep + base + sep + "inside.sysml",
		root + sep + sep + "inside.sysml",
		root + sep + "." + sep + "inside.sysml",
	} {
		if data, err := srv.readNamedFile(name); err != nil || string(data) != "package P;" {
			t.Errorf("readNamedFile(%q) = %q, %v", name, data, err)
		}
	}
}

// A name that goes through a file as if it were a directory names nothing, as for the
// system, whatever follows the file.
func TestConfinedServiceRefusesANameThroughAFile(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"inside.sysml", "beside.sysml"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package P;"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	linked := os.Symlink("inside.sysml"+string(filepath.Separator), filepath.Join(root, "link.sysml")) == nil
	srv, err := NewService(10, "test", ConfineFilesTo(root))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(srv.Close)
	sep := string(filepath.Separator)
	names := []string{
		"inside.sysml" + sep,
		"inside.sysml" + sep + ".",
		"inside.sysml" + sep + ".." + sep + "beside.sysml",
		"inside.sysml" + sep + "beside.sysml",
	}
	if linked {
		names = append(names, "link.sysml")
	}
	for _, name := range names {
		if _, err := os.ReadFile(root + sep + name); err == nil {
			t.Fatalf("the system reads %q, so it is no case of a name through a file", name)
		}
		if err := srv.readNamedFileError(name); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("readNamedFile(%q) = %v, want it refused", name, err)
		}
	}
}

// When the name the root was configured by is made to lead elsewhere, a request by
// that name is refused rather than answered with a file the name no longer reaches.
func TestConfinedServiceRefusesTheGivenNameOnceItLeadsElsewhere(t *testing.T) {
	physical, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(physical, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"m.sysml": "package Old;", filepath.Join("sub", "m.sysml"): "package New;"} {
		if err := os.WriteFile(filepath.Join(physical, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	given := filepath.Join(t.TempDir(), "given")
	if err := os.Symlink(physical, given); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	srv, err := NewService(10, "test", ConfineFilesTo(given))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(srv.Close)
	name := filepath.Join(given, "m.sysml")
	if data, err := srv.readNamedFile(name); err != nil || string(data) != "package Old;" {
		t.Fatalf("readNamedFile(%q) = %q, %v", name, data, err)
	}
	if err := os.Remove(given); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(physical, "sub"), given); err != nil {
		t.Fatal(err)
	}
	if data, err := srv.readNamedFile(name); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("readNamedFile(%q) = %q, %v, want it refused", name, data, err)
	}
	if data, err := srv.readNamedFile(filepath.Join(physical, "m.sysml")); err != nil || string(data) != "package Old;" {
		t.Errorf("the root's own name reads %q, %v", data, err)
	}
}

func (s *Service) readNamedFileError(name string) error {
	_, err := s.readNamedFile(name)
	return err
}
