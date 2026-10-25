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
		{"github purl", Usage{Resource: "pkg:github/acme/widgets"}, Origin{IRI: "pkg:github/acme/widgets", Kind: OriginHost, Host: "github.com", Repo: "acme/widgets"}},
		{"github purl versioned", Usage{Resource: "pkg:github/acme/widgets@v1.2.3"}, Origin{IRI: "pkg:github/acme/widgets", Kind: OriginHost, Host: "github.com", Repo: "acme/widgets", Constraint: "v1.2.3"}},
		{"github purl subpath", Usage{Resource: "pkg:github/acme/mono@1.0.0#libraries/widgets"}, Origin{IRI: "pkg:github/acme/mono", Kind: OriginHost, Host: "github.com", Repo: "acme/mono", Subpath: "libraries/widgets", Constraint: "1.0.0"}},
		{"gitlab purl group", Usage{Resource: "pkg:gitlab/acme/tools/widgets", VersionConstraint: "^1.0"}, Origin{IRI: "pkg:gitlab/acme/tools/widgets", Kind: OriginHost, Host: "gitlab.com", Repo: "acme/tools/widgets", Constraint: "^1.0"}},
		{"purl encoded version", Usage{Resource: "pkg:GitHub/acme/widgets@1.0.0%2Bbuild"}, Origin{IRI: "pkg:github/acme/widgets", Kind: OriginHost, Host: "github.com", Repo: "acme/widgets", Constraint: "1.0.0+build"}},
		{"https git", Usage{Resource: "https://git.example.org/acme/widgets.git", VersionConstraint: "1.0.0"}, Origin{IRI: "https://git.example.org/acme/widgets.git", Kind: OriginGit, Host: "git.example.org", Repo: "https://git.example.org/acme/widgets.git", Constraint: "1.0.0"}},
		{"https git versioned", Usage{Resource: "https://git.example.org/acme/widgets.git@v2.0.0#sub/dir"}, Origin{IRI: "https://git.example.org/acme/widgets.git", Kind: OriginGit, Host: "git.example.org", Repo: "https://git.example.org/acme/widgets.git", Subpath: "sub/dir", Constraint: "v2.0.0"}},
		{"ssh git", Usage{Resource: "ssh://git@git.example.org/acme/widgets"}, Origin{IRI: "ssh://git@git.example.org/acme/widgets", Kind: OriginGit, Host: "git.example.org", Repo: "ssh://git@git.example.org/acme/widgets"}},
		{"scp git", Usage{Resource: "git@github.com:acme/widgets.git@1.0.0"}, Origin{IRI: "git@github.com:acme/widgets.git", Kind: OriginGit, Host: "github.com", Repo: "git@github.com:acme/widgets.git", Constraint: "1.0.0"}},
		{"archive url", Usage{Resource: "https://downloads.example.org/widgets-1.0.0.kpar"}, Origin{IRI: "https://downloads.example.org/widgets-1.0.0.kpar", Kind: OriginArchive, Host: "downloads.example.org", Repo: "https://downloads.example.org/widgets-1.0.0.kpar"}},
		{"local dir", Usage{Resource: "../widgets/"}, Origin{IRI: "../widgets/", Kind: OriginLocal, Repo: "../widgets"}},
		{"local archive", Usage{Resource: "/opt/libs/widgets.kpar"}, Origin{IRI: "/opt/libs/widgets.kpar", Kind: OriginLocal, Repo: "/opt/libs/widgets.kpar"}},
		{"standard library", Usage{Resource: "https://www.omg.org/spec/KerML/20250201/Kernel-Semantic-Library.kpar", VersionConstraint: "1.0.0"}, Origin{IRI: "https://www.omg.org/spec/KerML/20250201/Kernel-Semantic-Library.kpar", Kind: OriginStandard, Constraint: "1.0.0"}},
		{"agreeing constraint", Usage{Resource: "pkg:github/acme/widgets@1.0.0", VersionConstraint: "=v1.0.0"}, Origin{IRI: "pkg:github/acme/widgets", Kind: OriginHost, Host: "github.com", Repo: "acme/widgets", Constraint: "1.0.0"}},
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
		{"unknown purl type", Usage{Resource: "pkg:npm/acme/widgets"}, ErrNotAnOrigin, `type "npm" is not a git host`},
		{"purl without name", Usage{Resource: "pkg:github/acme"}, nil, "needs both a namespace and a name"},
		{"purl empty version", Usage{Resource: "pkg:github/acme/widgets@"}, nil, "names no version"},
		{"subpath escape", Usage{Resource: "pkg:github/acme/widgets#../etc"}, nil, "subpath"},
		{"disagreeing versions", Usage{Resource: "pkg:github/acme/widgets@1.0.0", VersionConstraint: "2.0.0"}, nil, "state the version once"},
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
	for k, want := range map[OriginKind]string{OriginHost: "host", OriginGit: "git", OriginArchive: "archive", OriginLocal: "local", OriginStandard: "standard", OriginKind(9): "OriginKind(9)"} {
		if got := k.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(k), got, want)
		}
	}
}
