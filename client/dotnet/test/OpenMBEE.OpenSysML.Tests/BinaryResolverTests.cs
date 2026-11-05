using System;
using System.Collections.Generic;
using System.IO;
using System.Runtime.Versioning;
using System.Security.Cryptography;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Where a private service's binary is looked for, in what order, and what is refused.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class BinaryResolverTests : IDisposable
{
    private readonly string root = Path.Combine(Path.GetTempPath(), "opensysml-resolve-" + Guid.NewGuid().ToString("N"));

    public BinaryResolverTests()
    {
        Directory.CreateDirectory(root);
    }

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    private static string Name => BinaryResolver.ExecutableName;

    [Fact]
    public void TheNamesFollowThePlatform()
    {
        Assert.Equal(OperatingSystem.IsWindows() ? "sysml-grpc.exe" : "sysml-grpc", BinaryResolver.ExecutableName);
        Assert.Equal(OperatingSystem.IsWindows() ? "USERPROFILE" : "HOME", BinaryResolver.HomeVariable);
        Assert.Equal("OPENSYSML_GRPC_BINARY", ConnectionOptions.BinaryVariable);
        Assert.Null(BinaryResolver.ProcessLessPlatform);
    }

    [Fact]
    public void TheOptionIsNamedBeforeTheVariableAndNeitherIsNothing()
    {
        var option = Executable("option");
        var variable = Executable("variable");
        var environment = Environment(variable, null, null);

        Assert.Equal(option, BinaryResolver.FindNamed(new ConnectionOptions { BinaryPath = option }, environment));
        Assert.Equal(variable, BinaryResolver.FindNamed(new ConnectionOptions(), environment));
        Assert.Null(BinaryResolver.FindNamed(new ConnectionOptions(), Environment("  ", null, null)));
    }

    [Fact]
    public void TheFirstExecutableOnPathWinsAndANonExecutableOneIsPassedOver()
    {
        var first = Executable("first");
        var second = Executable("second");
        var blank = Path.Combine(root, "blank");
        Directory.CreateDirectory(Path.Combine(blank, Name));
        var path = string.Join(Path.PathSeparator, Path.Combine(root, "missing"), blank, " ", Path.Combine(root, "first"), Path.Combine(root, "second"));

        Assert.Equal(first, BinaryResolver.FindOnPath(Environment(null, null, path), out _));
        Assert.Equal(second, BinaryResolver.FindOnPath(Environment(null, null, Path.Combine(root, "second")), out _));
        Assert.Null(BinaryResolver.FindOnPath(Environment(null, null, Path.Combine(root, "missing") + Path.PathSeparator + blank), out var searched));
        Assert.Equal($"$PATH: no executable {Name} in its 2 directories", searched);
    }

    [Theory]
    [InlineData(true)]
    [InlineData(false)]
    public void ABinaryNamedExplicitlyThatIsNotThereFailsWithoutLookingElsewhere(bool byOption)
    {
        var missing = Path.Combine(root, "missing", Name);
        var home = Directory.CreateDirectory(Path.Combine(root, "home")).FullName;
        Executable(Path.Combine("home", ".opensysml", "bin"));
        var options = byOption ? new ConnectionOptions { BinaryPath = missing } : new ConnectionOptions();
        var environment = Environment(byOption ? null : missing, home, Path.Combine(root, "home", ".opensysml", "bin"));

        var error = Assert.Throws<BinaryNotFoundException>(() => BinaryResolver.FindNamed(options, environment));

        var source = byOption ? "ConnectionOptions.BinaryPath" : "$OPENSYSML_GRPC_BINARY";
        Assert.StartsWith($"{source} names {missing}, which is absent, not an executable sysml-grpc; it is used as named or not at all.", error.Message, StringComparison.Ordinal);
        Assert.Equal([$"{source}: {missing}: absent"], error.SearchedLocations);
        Assert.IsAssignableFrom<ServiceStartException>(error);
    }

    [Fact]
    public void ADirectoryOrALinkToNothingIsNoBinary()
    {
        var directory = Directory.CreateDirectory(Path.Combine(root, "dir")).FullName;
        Assert.False(BinaryResolver.IsExecutable(directory, out var why));
        Assert.Equal("a directory", why);

        if (!OperatingSystem.IsWindows())
        {
            var dangling = Path.Combine(root, "dangling");
            File.CreateSymbolicLink(dangling, Path.Combine(root, "nowhere"));
            Assert.False(BinaryResolver.IsExecutable(dangling, out why));
            Assert.Equal("a link to nothing", why);
        }
    }

    [Fact]
    [UnsupportedOSPlatform("windows")]
    public void OnUnixAFileWithoutAnExecuteBitIsNoBinary()
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "Windows has no execute bit; any file can be named.");
        var file = Executable("plain");
        File.SetUnixFileMode(file, UnixFileMode.UserRead | UnixFileMode.UserWrite);

        Assert.False(BinaryResolver.IsExecutable(file, out var why));
        Assert.Equal("not executable", why);
        var error = Assert.Throws<BinaryNotFoundException>(() => BinaryResolver.FindNamed(new ConnectionOptions { BinaryPath = file }, Environment(null, null, null)));
        Assert.Contains("which is not executable", error.Message, StringComparison.Ordinal);

        File.SetUnixFileMode(file, UnixFileMode.UserRead | UnixFileMode.OtherExecute);
        Assert.True(BinaryResolver.IsExecutable(file, out _));
    }

    [Fact]
    public async Task FindingNothingListsEveryPlaceLookedAtAndWhyNoReleaseCouldBeHad()
    {
        var home = Directory.CreateDirectory(Path.Combine(root, "home")).FullName;
        var path = Path.Combine(root, "a") + Path.PathSeparator + Path.Combine(root, "b");
        var releases = ReleaseServer.Offline(ReleaseServer.Pins((ReleaseServer.Repo, ReleaseRequest.BuiltAgainst, ReleaseServer.Asset, new string('a', 64))));

        var error = await Assert.ThrowsAsync<BinaryNotFoundException>(() => BinaryResolver.ResolveAsync(new ConnectionOptions(), Environment(null, home, path), releases, Token));

        var cached = Path.Combine(home, ".opensysml", "bin", Name);
        Assert.Equal(4, error.SearchedLocations.Count);
        Assert.Equal("$OPENSYSML_GRPC_BINARY: unset", error.SearchedLocations[0]);
        Assert.Equal($"{cached}: absent", error.SearchedLocations[1]);
        Assert.StartsWith($"release {ReleaseRequest.BuiltAgainst} of {ReleaseServer.Repo}: {releases.Downloads}{ReleaseServer.Repo}/releases/download/", error.SearchedLocations[2], StringComparison.Ordinal);
        Assert.Equal($"$PATH: no executable {Name} in its 2 directories", error.SearchedLocations[3]);
        Assert.IsType<ServiceStartException>(error.InnerException);
        Assert.IsType<System.Net.Http.HttpRequestException>(error.InnerException.InnerException);
        Assert.Contains("make build-grpc", error.Message, StringComparison.Ordinal);
        Assert.Contains("ConnectionOptions.DownloadVersion or $OPENSYSML_GRPC_VERSION", error.Message, StringComparison.Ordinal);
        Assert.Contains(string.Join("; ", error.SearchedLocations), error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task WithoutAHomeOrAPathThereIsNothingToSearchAndItSaysSo()
    {
        var error = await Assert.ThrowsAsync<BinaryNotFoundException>(() => Resolve(new ConnectionOptions(), Environment(null, null, null)));

        Assert.Equal(
            [
                "$OPENSYSML_GRPC_BINARY: unset",
                $"~/.opensysml/bin/{Name}: no home directory, since ${BinaryResolver.HomeVariable} is unset, so no release can be kept there",
                "$PATH: unset or empty",
            ],
            error.SearchedLocations);
        Assert.Null(error.InnerException);
    }

    [Fact]
    public async Task WithoutAHomeAReleaseAskedForFailsRatherThanFallingBackToPath()
    {
        Executable("path");

        var error = await Assert.ThrowsAsync<ServiceStartException>(() =>
            Resolve(new ConnectionOptions { DownloadVersion = "v1.0.0" }, Environment(null, null, Path.Combine(root, "path"))));

        Assert.StartsWith($"sysml-grpc v1.0.0 of {ReleaseServer.Repo} was asked for, but ${BinaryResolver.HomeVariable} is unset", error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task TheBinaryIsHashedAndCheckedAgainstTheDigestRequiredInEitherCase()
    {
        var binary = Executable("bin", "contents of a binary");
        var digest = Convert.ToHexStringLower(SHA256.HashData("contents of a binary"u8));

        var resolved = await Resolve(new ConnectionOptions { BinaryPath = binary, ExpectedBinarySha256 = digest.ToUpperInvariant() }, Environment(null, null, null));

        Assert.Equal(new ResolvedBinary(BinaryResolver.Canonical(binary), digest), resolved);
    }

    [Fact]
    public async Task ABinaryWithAnotherDigestIsRefusedNamingBoth()
    {
        var binary = Executable("bin", "contents of a binary");
        var digest = Convert.ToHexStringLower(SHA256.HashData("contents of a binary"u8));
        var expected = new string('A', 64);

        var error = await Assert.ThrowsAsync<ChecksumMismatchException>(() =>
            Resolve(new ConnectionOptions { BinaryPath = binary, ExpectedBinarySha256 = expected }, Environment(null, null, null)));

        Assert.Equal(BinaryResolver.Canonical(binary), error.BinaryPath);
        Assert.Equal(new string('a', 64), error.ExpectedSha256);
        Assert.Equal(digest, error.ActualSha256);
        Assert.StartsWith($"sysml-grpc at {error.BinaryPath} has SHA-256 {digest}, but {error.ExpectedSha256} was required; it was not started.", error.Message, StringComparison.Ordinal);
        Assert.IsAssignableFrom<ServiceStartException>(error);
    }

    [Fact]
    [UnsupportedOSPlatform("windows")]
    public async Task ABinaryThatCannotBeReadIsAStartFailureNamingIt()
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "Windows has no execute-only mode.");
        var binary = Executable("sealed");
        File.SetUnixFileMode(binary, UnixFileMode.UserExecute);

        var error = await Assert.ThrowsAsync<ServiceStartException>(() =>
            Resolve(new ConnectionOptions { BinaryPath = binary }, Environment(null, null, null)));

        Assert.StartsWith($"sysml-grpc at {binary} could not be read: ", error.Message, StringComparison.Ordinal);
        Assert.IsType<UnauthorizedAccessException>(error.InnerException);
        File.SetUnixFileMode(binary, UnixFileMode.UserRead | UnixFileMode.UserWrite);
    }

    [Fact]
    public void TheCanonicalPathFollowsEveryLinkInIt()
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "Creating links needs privileges Windows runners may lack.");
        var binary = Executable(Path.Combine("real", "bin"));
        var realRoot = BinaryResolver.Canonical(root);
        File.CreateSymbolicLink(Path.Combine(root, "link"), binary);
        Directory.CreateSymbolicLink(Path.Combine(root, "dirlink"), Path.Combine(root, "real"));
        Directory.CreateSymbolicLink(Path.Combine(root, "relative"), "real");

        var expected = Path.Combine(realRoot, "real", "bin", Name);
        Assert.Equal(expected, BinaryResolver.Canonical(Path.Combine(root, "link")));
        Assert.Equal(expected, BinaryResolver.Canonical(Path.Combine(root, "dirlink", "bin", Name)));
        Assert.Equal(expected, BinaryResolver.Canonical(Path.Combine(root, "relative", "bin", ".", Name)));
        Assert.Equal(expected, BinaryResolver.Canonical(Path.Combine(root, "real", "..", "dirlink", "bin", Name)));
    }

    [Theory]
    [InlineData(false, false)]
    [InlineData(false, true)]
    [InlineData(true, false)]
    [InlineData(true, true)]
    public async Task ParentComponentsFollowLinksBeforeTheBinaryIsSelectedAndHashed(bool throughFileLink, bool relative)
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "Creating links needs privileges Windows runners may lack.");
        var binary = Executable("target", "the selected binary");
        Executable("parent", "a different binary");
        Directory.CreateDirectory(Path.Combine(root, "target", "child"));
        Directory.CreateSymbolicLink(Path.Combine(root, "parent", "link"), Path.Combine("..", "target", "child"));
        var basis = relative ? Path.GetRelativePath(throughFileLink ? root : System.Environment.CurrentDirectory, root) : root;
        var selected = Path.Combine(basis, "parent", "link", "..", Name);

        if (throughFileLink)
        {
            var link = Path.Combine(root, "selected");
            File.CreateSymbolicLink(link, selected);
            selected = link;
        }

        var digest = Convert.ToHexStringLower(SHA256.HashData("the selected binary"u8));
        var resolved = await Resolve(new ConnectionOptions { BinaryPath = selected, ExpectedBinarySha256 = digest }, Environment(null, null, null));

        Assert.Equal(new ResolvedBinary(BinaryResolver.Canonical(binary), digest), resolved);
    }

    [Theory]
    [InlineData(false)]
    [InlineData(true)]
    public void AParentComponentCannotSkipAMissingDirectoryOrARegularFile(bool regularFile)
    {
        var binary = Executable("bin");
        var parent = Path.Combine(root, "not-a-directory");
        if (regularFile)
        {
            File.WriteAllText(parent, "not a directory");
        }

        var selected = Path.Combine(parent, "..", "bin", Name);

        Assert.Throws<BinaryNotFoundException>(() =>
            BinaryResolver.FindNamed(new ConnectionOptions { BinaryPath = selected }, Environment(null, null, null)));
        Assert.True(File.Exists(binary));
    }

    [Fact]
    public void ALinkLoopIsNoBinaryAndHasNoCanonicalPath()
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "Creating links needs privileges Windows runners may lack.");
        var a = Path.Combine(root, "a");
        var b = Path.Combine(root, "b");
        File.CreateSymbolicLink(a, b);
        File.CreateSymbolicLink(b, a);

        Assert.Throws<IOException>(() => BinaryResolver.Canonical(a));
        var error = Assert.Throws<BinaryNotFoundException>(() => BinaryResolver.FindNamed(new ConnectionOptions { BinaryPath = a }, Environment(null, null, null)));
        Assert.Contains($"{a}, which is a loop of links", error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public void APlatformThatCannotStartProcessesIsRefusedNamingIt()
    {
        BinaryResolver.RequireProcesses(null);

        var error = Assert.Throws<UnsupportedPlatformException>(() => BinaryResolver.RequireProcesses("browser"));

        Assert.Equal("browser", error.Platform);
        Assert.Contains("cannot be started on browser", error.Message, StringComparison.Ordinal);
        Assert.Contains("ConnectionOptions.Address or $OPENSYSML_SERVICE", error.Message, StringComparison.Ordinal);
        Assert.IsAssignableFrom<ServiceStartException>(error);
    }

    public void Dispose()
    {
        try
        {
            Directory.Delete(root, recursive: true);
        }
        catch (IOException)
        {
        }
    }

    // Nothing here downloads: a source refusing every connection proves it.
    private static Task<ResolvedBinary> Resolve(ConnectionOptions options, Func<string, string?> environment) =>
        BinaryResolver.ResolveAsync(options, environment, ReleaseServer.Offline(), Token);

    private static Func<string, string?> Environment(string? binary, string? home, string? path)
    {
        var variables = new Dictionary<string, string?>
        {
            [ConnectionOptions.BinaryVariable] = binary,
            ["HOME"] = home,
            ["USERPROFILE"] = home,
            ["PATH"] = path,
        };
        return name => variables.GetValueOrDefault(name);
    }

    private string Executable(string directory, string contents = "binary")
    {
        var full = Path.Combine(root, directory);
        Directory.CreateDirectory(full);
        var file = Path.Combine(full, Name);
        File.WriteAllText(file, contents);
        if (!OperatingSystem.IsWindows())
        {
            File.SetUnixFileMode(file, UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute);
        }

        return file;
    }
}
