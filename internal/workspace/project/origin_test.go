package project

import (
	"errors"
	"strings"
	"testing"
)

func TestParseOrigin(t *testing.T) {
	cases := []struct {
		name string
		u    Usage
		want Origin
	}{
		{"git purl", Usage{Resource: "pkg:git/github.com/acme/widgets"}, Origin{IRI: "pkg:git/github.com/acme/widgets", Kind: OriginGit, Host: "github.com", Path: "acme/widgets", Remote: "https://github.com/acme/widgets.git"}},
		{"git purl versioned", Usage{Resource: "pkg:git/github.com/acme/widgets@v1.2.3"}, Origin{IRI: "pkg:git/github.com/acme/widgets", Kind: OriginGit, Host: "github.com", Path: "acme/widgets", Remote: "https://github.com/acme/widgets.git", Constraint: "v1.2.3"}},
		{"git purl subpath", Usage{Resource: "pkg:git/github.com/acme/mono@1.0.0#libraries/widgets"}, Origin{IRI: "pkg:git/github.com/acme/mono#libraries/widgets", Kind: OriginGit, Host: "github.com", Path: "acme/mono", Remote: "https://github.com/acme/mono.git", Subpath: "libraries/widgets", Constraint: "1.0.0"}},
		{"git purl group", Usage{Resource: "pkg:git/gitlab.example.org/acme/tools/widgets", VersionConstraint: "^1.0"}, Origin{IRI: "pkg:git/gitlab.example.org/acme/tools/widgets", Kind: OriginGit, Host: "gitlab.example.org", Path: "acme/tools/widgets", Remote: "https://gitlab.example.org/acme/tools/widgets.git", Constraint: "^1.0"}},
		{"git purl encoded version", Usage{Resource: "pkg:Git/GitHub.com/acme/widgets@1.0.0%2Bbuild"}, Origin{IRI: "pkg:git/github.com/acme/widgets", Kind: OriginGit, Host: "github.com", Path: "acme/widgets", Remote: "https://github.com/acme/widgets.git", Constraint: "1.0.0+build"}},
		{"git purl local", Usage{Resource: "pkg:git//srv/git/widgets@1.0.0"}, Origin{IRI: "pkg:git//srv/git/widgets", Kind: OriginGit, Path: "/srv/git/widgets", Remote: "file:///srv/git/widgets", Constraint: "1.0.0"}},
		{"https git", Usage{Resource: "https://git.example.org/acme/widgets.git", VersionConstraint: "1.0.0"}, Origin{IRI: "pkg:git/git.example.org/acme/widgets", Kind: OriginGit, Host: "git.example.org", Path: "acme/widgets", Remote: "https://git.example.org/acme/widgets.git", Constraint: "1.0.0"}},
		{"https git versioned", Usage{Resource: "https://git.example.org/acme/widgets.git@v2.0.0#sub/dir"}, Origin{IRI: "pkg:git/git.example.org/acme/widgets#sub/dir", Kind: OriginGit, Host: "git.example.org", Path: "acme/widgets", Remote: "https://git.example.org/acme/widgets.git", Subpath: "sub/dir", Constraint: "v2.0.0"}},
		{"ssh git", Usage{Resource: "ssh://git@git.example.org/acme/widgets@1.0.0"}, Origin{IRI: "pkg:git/git.example.org/acme/widgets", Kind: OriginGit, Host: "git.example.org", Path: "acme/widgets", Remote: "ssh://git@git.example.org/acme/widgets", Constraint: "1.0.0"}},
		{"file git", Usage{Resource: "file:///srv/git/widgets@1.0.0"}, Origin{IRI: "pkg:git//srv/git/widgets", Kind: OriginGit, Path: "/srv/git/widgets", Remote: "file:///srv/git/widgets", Constraint: "1.0.0"}},
		{"scp git", Usage{Resource: "git@github.com:acme/widgets.git@1.0.0"}, Origin{IRI: "pkg:git/github.com/acme/widgets", Kind: OriginGit, Host: "github.com", Path: "acme/widgets", Remote: "git@github.com:acme/widgets.git", Constraint: "1.0.0"}},
		{"archive url", Usage{Resource: "https://downloads.example.org/widgets-1.0.0.kpar"}, Origin{IRI: "https://downloads.example.org/widgets-1.0.0.kpar", Kind: OriginArchive, Host: "downloads.example.org", Path: "https://downloads.example.org/widgets-1.0.0.kpar"}},
		{"local dir", Usage{Resource: "../widgets/"}, Origin{IRI: "../widgets/", Kind: OriginLocal, Path: "../widgets"}},
		{"local archive", Usage{Resource: "/opt/libs/widgets.kpar"}, Origin{IRI: "/opt/libs/widgets.kpar", Kind: OriginLocal, Path: "/opt/libs/widgets.kpar"}},
		{"standard library", Usage{Resource: "https://www.omg.org/spec/KerML/20250201/Kernel-Semantic-Library.kpar", VersionConstraint: "1.0.0"}, Origin{IRI: "https://www.omg.org/spec/KerML/20250201/Kernel-Semantic-Library.kpar", Kind: OriginStandard, Constraint: "1.0.0"}},
		{"agreeing constraint", Usage{Resource: "pkg:git/github.com/acme/widgets@1.0.0", VersionConstraint: "=v1.0.0"}, Origin{IRI: "pkg:git/github.com/acme/widgets", Kind: OriginGit, Host: "github.com", Path: "acme/widgets", Remote: "https://github.com/acme/widgets.git", Constraint: "1.0.0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseOrigin(tc.u)
			if err != nil {
				t.Fatalf("ParseOrigin: %v", err)
			}
			if *got != tc.want {
				t.Fatalf("ParseOrigin(%+v)\n got %+v\nwant %+v", tc.u, *got, tc.want)
			}
		})
	}
}

func TestParseOriginRefuses(t *testing.T) {
	cases := []struct {
		name    string
		u       Usage
		wantErr error
		wantMsg string
	}{
		{"empty", Usage{}, ErrNotAnOrigin, "names no resource"},
		{"unknown purl type", Usage{Resource: "pkg:npm/acme/widgets"}, ErrNotAnOrigin, `type "npm" is not git`},
		{"host purl type", Usage{Resource: "pkg:github/acme/widgets"}, ErrNotAnOrigin, `type "github" is not git`},
		{"purl without repository", Usage{Resource: "pkg:git/github.com/acme"}, nil, "at least two segments"},
		{"purl empty version", Usage{Resource: "pkg:git/github.com/acme/widgets@"}, nil, "names no version"},
		{"subpath escape", Usage{Resource: "pkg:git/github.com/acme/widgets#../etc"}, nil, "subpath"},
		{"disagreeing versions", Usage{Resource: "pkg:git/github.com/acme/widgets@1.0.0", VersionConstraint: "2.0.0"}, nil, "state the version once"},
		{"https without .git or .kpar", Usage{Resource: "https://example.org/acme/widgets"}, ErrNotAnOrigin, "ends in .git"},
		{"ftp", Usage{Resource: "ftp://example.org/widgets.kpar"}, ErrNotAnOrigin, `scheme "ftp"`},
		{"archive subpath", Usage{Resource: "https://example.org/widgets.kpar#sub"}, nil, "takes no #subpath"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseOrigin(tc.u)
			if err == nil {
				t.Fatalf("ParseOrigin(%+v) accepted", tc.u)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("error %v does not wrap %v", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error %q lacks %q", err, tc.wantMsg)
			}
		})
	}
}

func TestInstallName(t *testing.T) {
	cases := []struct {
		p    Interchange
		want string
	}{
		{Interchange{Publisher: "ODE4HERA", Name: "Requirements Management", Version: "2.0.1"}, "ode4hera-requirements-management_2.0.1"},
		{Interchange{Name: "Widgets", Version: "v1.0.0"}, "widgets_1.0.0"},
	}
	for _, tc := range cases {
		if got := InstallName(&tc.p); got != tc.want {
			t.Errorf("InstallName(%+v) = %q, want %q", tc.p, got, tc.want)
		}
	}
}

func TestOriginKindString(t *testing.T) {
	for k, want := range map[OriginKind]string{OriginGit: "git", OriginArchive: "archive", OriginLocal: "local", OriginStandard: "standard", OriginKind(9): "OriginKind(9)"} {
		if got := k.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(k), got, want)
		}
	}
}
