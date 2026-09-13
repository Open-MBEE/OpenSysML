package project

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestExpandDirectoryIsWalkedInSortedOrder(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "b.sysml"), "package B { }\n")
	write(t, filepath.Join(dir, "a.sysml"), "package A { }\n")
	write(t, filepath.Join(dir, "sub", "c.kerml"), "package C { }\n")
	write(t, filepath.Join(dir, "notes.md"), "ignored\n")
	write(t, filepath.Join(dir, ".hidden", "d.sysml"), "package D { }\n")

	got, err := Expand([]string{dir})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{
		filepath.Join(dir, "a.sysml"),
		filepath.Join(dir, "b.sysml"),
		filepath.Join(dir, "sub", "c.kerml"),
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Expand(%s) = %v, want %v", dir, got, want)
	}
}

func TestExpandGlob(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.sysml"), "package A { }\n")
	write(t, filepath.Join(dir, "b.sysml"), "package B { }\n")
	write(t, filepath.Join(dir, "c.kerml"), "package C { }\n")

	got, err := Expand([]string{filepath.Join(dir, "*.sysml")})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{filepath.Join(dir, "a.sysml"), filepath.Join(dir, "b.sysml")}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("glob = %v, want %v", got, want)
	}
}

func TestExpandGlobMatchingNothingIsReported(t *testing.T) {
	dir := t.TempDir()
	_, err := Expand([]string{filepath.Join(dir, "*.sysml")})
	if err == nil {
		t.Fatal("expected an error for a pattern matching nothing")
	}
	if !strings.Contains(err.Error(), "no model files match") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExpandEmptyDirectoryIsReported(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "README.md"), "no models here\n")
	_, err := Expand([]string{dir})
	if err == nil {
		t.Fatal("expected an error for a directory holding no model files")
	}
	if !strings.Contains(err.Error(), "no .sysml or .kerml files") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExpandFileNamedTwiceLoadsOnce(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.sysml")
	write(t, file, "package A { }\n")

	got, err := Expand([]string{file, dir, file})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(got) != 1 || got[0] != file {
		t.Fatalf("Expand = %v, want [%s]", got, file)
	}
}

func TestExpandMissingFileIsLeftToTheLoader(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.sysml")
	got, err := Expand([]string{missing})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(got) != 1 || got[0] != missing {
		t.Fatalf("Expand = %v, want [%s]", got, missing)
	}
}

func TestExpandMultipleInputsKeepTheOrderGiven(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "z.sysml"), "package Z { }\n")
	write(t, filepath.Join(dir, "sub", "a.sysml"), "package A { }\n")
	write(t, filepath.Join(dir, "sub", "b.sysml"), "package B { }\n")

	got, err := Expand([]string{filepath.Join(dir, "z.sysml"), filepath.Join(dir, "sub")})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{
		filepath.Join(dir, "z.sysml"),
		filepath.Join(dir, "sub", "a.sysml"),
		filepath.Join(dir, "sub", "b.sysml"),
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Expand = %v, want %v", got, want)
	}
}

func TestIsModelFile(t *testing.T) {
	for path, want := range map[string]bool{
		"a.sysml": true, "a.SysML": true, "a.kerml": true,
		"a.md": false, "sysml": false, "a.sysml.bak": false,
	} {
		if got := IsModelFile(path); got != want {
			t.Errorf("IsModelFile(%q) = %v, want %v", path, got, want)
		}
	}
}

// A project may keep a shared directory as a symlink; loading it should load the
// files it points at.
func TestExpandFollowsASymlinkedDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(target, "a.sysml"), "package X { }\n")
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	got, err := Expand([]string{link})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{filepath.Join(link, "a.sysml")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand = %v, want %v", got, want)
	}
}

// A symlinked subdirectory contributes its files too, and a link back up the
// tree does not make the walk run forever.
func TestExpandFollowsASymlinkedSubdirectoryOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "a.sysml"), "package X { }\n")
	write(t, filepath.Join(sub, "b.sysml"), "package X { }\n")
	if err := os.Symlink(dir, filepath.Join(sub, "up")); err != nil {
		t.Fatal(err)
	}

	got, err := Expand([]string{dir})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{
		filepath.Join(dir, "a.sysml"),
		filepath.Join(sub, "b.sysml"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand = %v, want %v", got, want)
	}
}

// A link pointing nowhere is skipped rather than failing the walk.
func TestExpandSkipsADanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.sysml"), "package X { }\n")
	if err := os.Symlink(filepath.Join(dir, "gone.sysml"), filepath.Join(dir, "b.sysml")); err != nil {
		t.Fatal(err)
	}

	got, err := Expand([]string{dir})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{filepath.Join(dir, "a.sysml")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand = %v, want %v", got, want)
	}
}

// A pattern names its matches collectively: a model-free directory among them
// contributes nothing instead of failing the load.
func TestExpandGlobMatchingAModelFreeDirectory(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "models", "a.sysml"), "package A { }\n")
	write(t, filepath.Join(dir, "docs", "notes.md"), "ignored\n")

	got, err := Expand([]string{filepath.Join(dir, "*")})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{filepath.Join(dir, "models", "a.sysml")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand = %v, want %v", got, want)
	}
}

// Nor does a pattern submit a non-model file it happens to catch.
func TestExpandGlobSkipsNonModelMatches(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a.sysml"), "package A { }\n")
	write(t, filepath.Join(dir, "README.md"), "ignored\n")

	got, err := Expand([]string{filepath.Join(dir, "*")})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{filepath.Join(dir, "a.sysml")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand = %v, want %v", got, want)
	}
}

// A directory the user named is another matter: it names exactly that path, so
// finding no model in it is worth reporting.
func TestExpandNamedEmptyDirectoryStillErrors(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "notes.md"), "ignored\n")

	if _, err := Expand([]string{dir}); err == nil {
		t.Fatal("want an error naming the model-free directory")
	}
}
