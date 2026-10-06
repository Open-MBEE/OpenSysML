package buildinfo

import (
	"runtime"
	"runtime/debug"
	"testing"
)

// stampedBuild is what the Makefile's -X flags write.
var stampedBuild = Stamps{
	Version:   "v9.9.9",
	Commit:    "abc1234",
	BuildTime: "2026-01-02_03:04:05",
	GoVersion: "go1.99.0",
}

// unstampedBuild is what a main package declares when no -X flag reached it.
var unstampedBuild = Stamps{Version: "dev", Commit: "unknown", BuildTime: "unknown", GoVersion: "unknown"}

// releaseInfo is the build information of `go install …@v0.9.2`: a module version
// and no VCS settings, since the module cache is not a repository.
func releaseInfo() *debug.BuildInfo {
	return &debug.BuildInfo{
		GoVersion: "go1.25.11",
		Main:      debug.Module{Path: "github.com/Open-MBEE/OpenSysML", Version: "v0.9.2"},
	}
}

// checkoutInfo is the build information of `go install ./cmd/sysml` in a clean
// checkout past the last tag: a pseudo-version and the VCS settings.
func checkoutInfo(modified bool) *debug.BuildInfo {
	return &debug.BuildInfo{
		GoVersion: "go1.25.11",
		Main:      debug.Module{Path: "github.com/Open-MBEE/OpenSysML", Version: "v0.9.3-0.20261006023628-7e5aaa9e8902"},
		Settings: []debug.BuildSetting{
			{Key: "vcs", Value: "git"},
			{Key: "vcs.revision", Value: "7e5aaa9e890203813596bb29a3ae3ea2d7340de5"},
			{Key: "vcs.time", Value: "2026-10-06T02:36:28Z"},
			{Key: "vcs.modified", Value: map[bool]string{false: "false", true: "true"}[modified]},
		},
	}
}

func TestStampsWinOverBuildInfo(t *testing.T) {
	got := ResolveFrom(stampedBuild, checkoutInfo(false))
	want := Info{Version: "v9.9.9", Commit: "abc1234", BuildTime: "2026-01-02_03:04:05", GoVersion: "go1.99.0"}
	if got != want {
		t.Errorf("ResolveFrom(stamped, checkout) = %+v, want %+v", got, want)
	}
}

func TestStampsWinOverNoBuildInfo(t *testing.T) {
	got := ResolveFrom(stampedBuild, nil)
	want := Info{Version: "v9.9.9", Commit: "abc1234", BuildTime: "2026-01-02_03:04:05", GoVersion: "go1.99.0"}
	if got != want {
		t.Errorf("ResolveFrom(stamped, nil) = %+v, want %+v", got, want)
	}
}

func TestEachStampIsResolvedOnItsOwn(t *testing.T) {
	stamps := Stamps{Version: "v9.9.9", Commit: "unknown", BuildTime: "", GoVersion: "unknown"}
	got := ResolveFrom(stamps, checkoutInfo(false))
	want := Info{Version: "v9.9.9", Commit: "7e5aaa9e8902", BuildTime: "2026-10-06_02:36:28", GoVersion: "go1.25.11"}
	if got != want {
		t.Errorf("ResolveFrom(version only, checkout) = %+v, want %+v", got, want)
	}
}

func TestReleasedModuleVersionIsTheVersion(t *testing.T) {
	got := ResolveFrom(unstampedBuild, releaseInfo())
	want := Info{Version: "v0.9.2", Commit: "unknown", BuildTime: "unknown", GoVersion: "go1.25.11"}
	if got != want {
		t.Errorf("ResolveFrom(unstamped, release) = %+v, want %+v", got, want)
	}
}

func TestPrereleaseAndDirtyTagsAreVersions(t *testing.T) {
	for _, version := range []string{"v1.0.0-rc.1", "v0.9.2+dirty", "v0.9.2-rc1+dirty"} {
		info := releaseInfo()
		info.Main.Version = version
		if got := ResolveFrom(unstampedBuild, info).Version; got != version {
			t.Errorf("ResolveFrom(unstamped, %q).Version = %q", version, got)
		}
	}
}

func TestCheckoutReportsTheCommitNotThePseudoVersion(t *testing.T) {
	got := ResolveFrom(unstampedBuild, checkoutInfo(false))
	want := Info{Version: "dev", Commit: "7e5aaa9e8902", BuildTime: "2026-10-06_02:36:28", GoVersion: "go1.25.11"}
	if got != want {
		t.Errorf("ResolveFrom(unstamped, checkout) = %+v, want %+v", got, want)
	}
}

func TestModifiedCheckoutMarksTheCommitDirty(t *testing.T) {
	if got, want := ResolveFrom(unstampedBuild, checkoutInfo(true)).Commit, "7e5aaa9e8902-dirty"; got != want {
		t.Errorf("Commit = %q, want %q", got, want)
	}
}

func TestDevelMainModuleIsUnversioned(t *testing.T) {
	info := releaseInfo()
	info.Main.Version = "(devel)"
	got := ResolveFrom(unstampedBuild, info)
	want := Info{Version: "dev", Commit: "unknown", BuildTime: "unknown", GoVersion: "go1.25.11"}
	if got != want {
		t.Errorf("ResolveFrom(unstamped, devel) = %+v, want %+v", got, want)
	}
}

func TestNoBuildInfoReportsTheDefaults(t *testing.T) {
	got := ResolveFrom(unstampedBuild, nil)
	want := Info{Version: "dev", Commit: "unknown", BuildTime: "unknown", GoVersion: runtime.Version()}
	if got != want {
		t.Errorf("ResolveFrom(unstamped, nil) = %+v, want %+v", got, want)
	}
}

func TestUnparseableVCSTimeIsReportedAsRecorded(t *testing.T) {
	info := checkoutInfo(false)
	info.Settings[2].Value = "yesterday"
	if got := ResolveFrom(unstampedBuild, info).BuildTime; got != "yesterday" {
		t.Errorf("BuildTime = %q, want the recorded value", got)
	}
}

func TestResolveReadsTheTestBinary(t *testing.T) {
	// The test binary's own information is whatever the toolchain gave it; only
	// its invariants can be checked: stamps win, and the Go version is reported.
	got := Resolve(stampedBuild)
	if got.Version != "v9.9.9" || got.Commit != "abc1234" {
		t.Errorf("Resolve(stamped) = %+v, want the stamps", got)
	}
	if got := Resolve(unstampedBuild).GoVersion; got == "unknown" || got == "" {
		t.Errorf("Resolve(unstamped).GoVersion = %q, want the toolchain", got)
	}
}

func TestReleased(t *testing.T) {
	cases := map[string]bool{
		"":                                       false,
		"(devel)":                                false,
		"v0.9.2":                                 true,
		"v10.20.30":                              true,
		"v1.0.0-rc.1":                            true,
		"v0.9.2+dirty":                           true,
		"v0.0.0-20261006023628-7e5aaa9e8902":     false,
		"v0.9.3-0.20261006023628-7e5aaa9e8902":   false,
		"v0.9.2-pre.0.20261006023628-7e5aaa9e89": false,
		"0.9.2":                                  false,
		"v0.9":                                   false,
		"nightly":                                false,
	}
	for version, want := range cases {
		if got := Released(version); got != want {
			t.Errorf("Released(%q) = %v, want %v", version, got, want)
		}
	}
}

func TestStamped(t *testing.T) {
	cases := map[string]bool{
		"":                                     false,
		"(devel)":                              false,
		"v0.9.2":                               true,
		"v0.9.3-0.20261006023628-7e5aaa9e8902": true,
	}
	for version, want := range cases {
		if got := Stamped(version); got != want {
			t.Errorf("Stamped(%q) = %v, want %v", version, got, want)
		}
	}
}

func TestShortRevision(t *testing.T) {
	if got := ShortRevision("7e5aaa9e890203813596bb29a3ae3ea2d7340de5"); got != "7e5aaa9e8902" {
		t.Errorf("ShortRevision(full) = %q", got)
	}
	if got := ShortRevision("abc1234"); got != "abc1234" {
		t.Errorf("ShortRevision(short) = %q, want it unchanged", got)
	}
}

func TestReport(t *testing.T) {
	info := Info{Version: "v0.9.2", Commit: "abc1234", BuildTime: "2026-01-02_03:04:05", GoVersion: "go1.25.11"}
	want := "sysml v0.9.2\n  Commit:     abc1234\n  Build time: 2026-01-02_03:04:05\n  Go version: go1.25.11\n"
	if got := info.Report("sysml"); got != want {
		t.Errorf("Report = %q, want %q", got, want)
	}
}
