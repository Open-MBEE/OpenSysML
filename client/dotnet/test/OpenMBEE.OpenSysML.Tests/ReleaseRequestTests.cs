using System;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.InteropServices;
using System.Xml.Linq;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Which release a private service is provided from when no binary is named: the asset built for the platform,
/// the repository and tag asked for and in what order, and when a release nothing pins may be trusted anyway.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ReleaseRequestTests
{
    [Theory]
    [InlineData("linux", "amd64", "sysml-grpc-linux-amd64")]
    [InlineData("linux", "arm64", "sysml-grpc-linux-arm64")]
    [InlineData("darwin", "amd64", "sysml-grpc-darwin-amd64")]
    [InlineData("darwin", "arm64", "sysml-grpc-darwin-arm64")]
    [InlineData("windows", "amd64", "sysml-grpc-windows-amd64.exe")]
    public void EachOfTheFivePublishedPlatformsHasItsAsset(string goos, string goarch, string asset)
    {
        Assert.Equal(asset, ReleasePlatform.Asset(goos, goarch));
    }

    [Fact]
    public void ThePublishedPlatformsAreTheFiveTheReleasePipelineBuildsAndThatThePinsName()
    {
        Assert.Equal(["darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64"], ReleasePlatform.Published.Order(StringComparer.Ordinal));

        var pinned = ReleaseDigests.Embedded.Pinned(ConnectionOptions.DefaultGithubRepo, "v0.10.1", "sysml-grpc-linux-amd64");
        Assert.NotNull(pinned);
        foreach (var pair in ReleasePlatform.Published)
        {
            var parts = pair.Split('-');
            Assert.NotNull(ReleaseDigests.Embedded.Pinned(ConnectionOptions.DefaultGithubRepo, "v0.10.1", ReleasePlatform.Asset(parts[0], parts[1])));
        }
    }

    [Theory]
    [InlineData("windows", "arm64")]
    [InlineData("linux", "386")]
    [InlineData("linux", "arm")]
    [InlineData("darwin", "riscv64")]
    [InlineData("freebsd", "amd64")]
    [InlineData("android", "arm64")]
    [InlineData("unknown", "amd64")]
    public void AnUnpublishedPairIsRefusedByNameRatherThanFetchedAsA404(string goos, string goarch)
    {
        var error = Assert.Throws<UnsupportedPlatformException>(() => ReleasePlatform.Asset(goos, goarch));

        Assert.Equal($"{goos}-{goarch}", error.Platform);
        Assert.StartsWith($"No sysml-grpc release is published for {goos}-{goarch}; releases are built for darwin-amd64, darwin-arm64, linux-amd64, linux-arm64, windows-amd64.", error.Message, StringComparison.Ordinal);
        Assert.Contains("ConnectionOptions.BinaryPath or $OPENSYSML_GRPC_BINARY", error.Message, StringComparison.Ordinal);
        Assert.IsAssignableFrom<ServiceStartException>(error);
    }

    [Theory]
    [InlineData(Architecture.X64, "amd64")]
    [InlineData(Architecture.Arm64, "arm64")]
    [InlineData(Architecture.X86, "386")]
    [InlineData(Architecture.Arm, "arm")]
    [InlineData(Architecture.Armv6, "arm")]
    [InlineData(Architecture.Wasm, "wasm")]
    [InlineData(Architecture.S390x, "s390x")]
    [InlineData(Architecture.LoongArch64, "loong64")]
    [InlineData(Architecture.Ppc64le, "ppc64le")]
    [InlineData(Architecture.RiscV64, "riscv64")]
    [InlineData((Architecture)99, "99")]
    public void ArchitecturesAreNamedAsGoNamesThem(Architecture architecture, string goarch)
    {
        Assert.Equal(goarch, ReleasePlatform.Goarch(architecture));
    }

    [Fact]
    public void ThisMachineIsNamedAsGoNamesIt()
    {
        var goos = OperatingSystem.IsLinux() ? "linux" : OperatingSystem.IsMacOS() ? "darwin" : OperatingSystem.IsWindows() ? "windows" : null;
        Assert.SkipWhen(goos is null, "The suite runs on Linux, macOS and Windows; this platform has no published release to name.");
        var goarch = RuntimeInformation.OSArchitecture switch { Architecture.X64 => "amd64", Architecture.Arm64 => "arm64", _ => null };
        Assert.SkipWhen(goarch is null, $"{RuntimeInformation.OSArchitecture} has no published release to name.");

        Assert.Equal((goos, goarch), (ReleasePlatform.HostGoos, ReleasePlatform.HostGoarch));
        if (goos == "windows" && goarch == "arm64")
        {
            Assert.Throws<UnsupportedPlatformException>(ReleasePlatform.HostAsset);
        }
        else
        {
            Assert.Equal(ReleasePlatform.Asset(goos!, goarch!), ReleasePlatform.HostAsset());
        }
    }

    [Fact]
    public void TheReleaseBuiltAgainstIsTheVersionOfThisCheckout()
    {
        var props = XDocument.Load(Repository.PathOf("client", "dotnet", "Directory.Build.props"));
        var version = props.Descendants("Version").Single().Value;

        Assert.Equal("v" + version, ReleaseRequest.BuiltAgainst);
        Assert.Equal(version, ReleaseRequest.ClientVersion);
    }

    [Fact]
    public void TheOptionComesBeforeTheVariableBeforeTheReleaseBuiltAgainst()
    {
        var environment = Environment(new() { [ConnectionOptions.VersionVariable] = " v2.0.0 ", [ConnectionOptions.GithubRepoVariable] = " env/repo " });

        Assert.Equal(
            new ReleaseRequest("option/repo", "v3.0.0", true, false),
            ReleaseRequest.From(new ConnectionOptions { DownloadVersion = "v3.0.0", GithubRepo = "option/repo" }, environment));
        Assert.Equal(new ReleaseRequest("env/repo", "v2.0.0", true, false), ReleaseRequest.From(new ConnectionOptions(), environment));
        Assert.Equal(
            new ReleaseRequest(ConnectionOptions.DefaultGithubRepo, ReleaseRequest.BuiltAgainst, false, false),
            ReleaseRequest.From(new ConnectionOptions(), Environment(new() { [ConnectionOptions.VersionVariable] = "  ", [ConnectionOptions.GithubRepoVariable] = string.Empty })));
    }

    [Fact]
    public void LatestIsAskedForExplicitlyAndResolvedLater()
    {
        var request = ReleaseRequest.From(new ConnectionOptions(), Environment(new() { [ConnectionOptions.VersionVariable] = "latest" }));

        Assert.True(request.IsLatest);
        Assert.True(request.Explicit);
        Assert.False(ReleaseRequest.From(new ConnectionOptions(), Environment([])).IsLatest);
        Assert.False(new ReleaseRequest(ConnectionOptions.DefaultGithubRepo, "Latest", true, false).IsLatest);
    }

    [Theory]
    [InlineData(ConnectionOptions.GithubRepoVariable, "Open-MBEE", "$OPENSYSML_GITHUB_REPO is \"Open-MBEE\", which is not an owner/repo")]
    [InlineData(ConnectionOptions.GithubRepoVariable, "a/b/c", "$OPENSYSML_GITHUB_REPO is \"a/b/c\"")]
    [InlineData(ConnectionOptions.GithubRepoVariable, "../b", "$OPENSYSML_GITHUB_REPO is \"../b\"")]
    [InlineData(ConnectionOptions.GithubRepoVariable, "a/b?c", "$OPENSYSML_GITHUB_REPO is \"a/b?c\"")]
    [InlineData(ConnectionOptions.VersionVariable, "v1/../../x", "$OPENSYSML_GRPC_VERSION is \"v1/../../x\", which is neither \"latest\" nor a release tag")]
    [InlineData(ConnectionOptions.VersionVariable, "v1#x", "$OPENSYSML_GRPC_VERSION is \"v1#x\"")]
    [InlineData(ConnectionOptions.VersionVariable, "-v1", "$OPENSYSML_GRPC_VERSION is \"-v1\"")]
    public void AVariableNamingNoRepositoryOrTagIsRefusedNamingIt(string variable, string value, string message)
    {
        var error = Assert.Throws<InvalidRequestException>(() => ReleaseRequest.From(new ConnectionOptions(), Environment(new() { [variable] = value })));

        Assert.StartsWith(message, error.Message, StringComparison.Ordinal);
    }

    [Theory]
    [InlineData("v0.10.1", true)]
    [InlineData("v0.11.0-rc.1", true)]
    [InlineData("nightly-20261026-abcdef0", true)]
    [InlineData("v1.0.0+build.5", true)]
    [InlineData("1.0", true)]
    [InlineData("latest", true)]
    [InlineData("", false)]
    [InlineData(".", false)]
    [InlineData("..", false)]
    [InlineData(".hidden", false)]
    [InlineData("v1..2", false)]
    [InlineData("-rf", false)]
    [InlineData("v1/2", false)]
    [InlineData("v1\\2", false)]
    [InlineData("v1 2", false)]
    [InlineData("v1%2F", false)]
    [InlineData("v1\n", false)]
    [InlineData("vé", false)]
    public void ATagMustStayOneUrlPathSegment(string tag, bool valid)
    {
        Assert.Equal(valid, ReleaseRequest.IsVersion(tag));
        Assert.False(ReleaseRequest.IsVersion(new string('v', 129)));
        Assert.True(ReleaseRequest.IsVersion(new string('v', 128)));
    }

    [Theory]
    [InlineData("Open-MBEE/OpenSysML", true)]
    [InlineData("a_b.c/d-e.f", true)]
    [InlineData("owner/.repo", true)]
    [InlineData("owner", false)]
    [InlineData("/repo", false)]
    [InlineData("owner/", false)]
    [InlineData("a/b/c", false)]
    [InlineData("./repo", false)]
    [InlineData("owner/..", false)]
    [InlineData("owner/...", false)]
    [InlineData("own er/repo", false)]
    [InlineData("owner/re%2Fpo", false)]
    [InlineData("owner/repo\n", false)]
    public void ARepositoryMustBeAnOwnerAndANameEachOneUrlPathSegment(string repo, bool valid)
    {
        Assert.Equal(valid, ReleaseRequest.IsRepo(repo));
    }

    [Theory]
    [InlineData(null, false)]
    [InlineData("", false)]
    [InlineData("  ", false)]
    [InlineData("0", false)]
    [InlineData("false", false)]
    [InlineData("No", false)]
    [InlineData("1", true)]
    [InlineData(" TRUE ", true)]
    [InlineData("yes", true)]
    [InlineData("Open-MBEE/OpenSysML", true)]
    [InlineData("someone/fork, Open-MBEE/OpenSysML", true)]
    [InlineData("someone/fork", false)]
    [InlineData("open-mbee/opensysml", false)]
    [InlineData("Open-MBEE/OpenSysML2", false)]
    [InlineData("Open-MBEE", false)]
    public void SameOriginTrustIsGrantedForAnyRepositoryOrForTheOnesNamedExactly(string? setting, bool allowed)
    {
        Assert.Equal(allowed, ReleaseRequest.AllowsUnpinned(setting, "Open-MBEE/OpenSysML"));
        Assert.Equal(allowed, ReleaseRequest.From(new ConnectionOptions(), Environment(new() { [ConnectionOptions.AllowUnpinnedDownloadVariable] = setting })).UnpinnedAllowed);
    }

    [Fact]
    public void SameOriginTrustFollowsTheRepositoryAskedFor()
    {
        var environment = Environment(new() { [ConnectionOptions.AllowUnpinnedDownloadVariable] = "someone/fork" });

        Assert.True(ReleaseRequest.From(new ConnectionOptions { GithubRepo = "someone/fork" }, environment).UnpinnedAllowed);
        Assert.False(ReleaseRequest.From(new ConnectionOptions(), environment).UnpinnedAllowed);
    }

    private static Func<string, string?> Environment(Dictionary<string, string?> variables) => name => variables.GetValueOrDefault(name);
}
