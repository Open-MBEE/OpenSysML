using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Where the foreign lock holders' toolchains are looked for, and whether their absence skips or fails.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class InteropGateTests : IDisposable
{
    private readonly string directory = Directory.CreateTempSubdirectory("opensysml-interop-gate-").FullName;

    public void Dispose() => Directory.Delete(directory, recursive: true);

    [Theory]
    [InlineData("1", true)]
    [InlineData(" yes ", true)]
    [InlineData(null, false)]
    [InlineData("", false)]
    [InlineData("0", false)]
    [InlineData("FALSE", false)]
    [InlineData("no", false)]
    public void OnlyAPromiseOfTheToolchainsRequiresThem(string? value, bool required)
    {
        Assert.Equal(required, Gate((InteropGate.RequireVariable, value)).Required);
    }

    [Theory]
    [MemberData(nameof(InteropGate.All), MemberType = typeof(InteropGate))]
    public void AMissingToolchainSkipsUnlessItWasPromisedAndThenFails(ForeignClient client)
    {
        var skipped = Gate(("PATH", directory)).Decide(client);
        var failed = Gate(("PATH", directory), (InteropGate.RequireVariable, "1")).Decide(client);

        Assert.Equal((null, false), (skipped.Tool, skipped.Fails));
        Assert.Equal((null, true), (failed.Tool, failed.Fails));
        Assert.Contains($"${InteropGate.RequireVariable} is set, so this test must run against the {client} client's lock, but there is {skipped.Reason}.", failed.Reason, StringComparison.Ordinal);
    }

    [Fact]
    public void PythonIsWhatPythonNamesElseTheFirstOnPath()
    {
        var onPath = Tool("path", OperatingSystem.IsWindows() ? "python" : "python3");
        var named = Tool("named", "my-python");

        Assert.Equal(onPath, Gate(("PATH", Path.GetDirectoryName(onPath))).Decide(ForeignClient.Python).Tool);
        Assert.Equal(named, Gate(("PATH", Path.GetDirectoryName(onPath)), ("PYTHON", named)).Decide(ForeignClient.Python).Tool);
        Assert.Equal(onPath, Gate(("PATH", Path.GetDirectoryName(onPath)), ("PYTHON", "missing-python")).Decide(ForeignClient.Python).Tool);
    }

    [Fact]
    public void JavaIsAJdkFromJavaHomeElseFromPathAndAJreAloneIsNone()
    {
        var home = Path.Combine(directory, "jdk");
        var java = Tool(Path.Combine("jdk", "bin"), "java");
        var jre = Tool("jre", "java");

        Assert.Null(Gate(("JAVA_HOME", home)).Decide(ForeignClient.Java).Tool);
        Assert.Null(Gate(("PATH", Path.GetDirectoryName(jre))).Decide(ForeignClient.Java).Tool);
        Tool(Path.Combine("jdk", "bin"), "javac");
        Assert.Equal(java, Gate(("JAVA_HOME", home), ("PATH", Path.GetDirectoryName(jre))).Decide(ForeignClient.Java).Tool);
        Assert.Equal(java, Gate(("PATH", Path.GetDirectoryName(java))).Decide(ForeignClient.Java).Tool);
    }

    [Fact]
    public void CargoIsTheFirstOnPathElseTheOneRustupInstalls()
    {
        var home = Path.Combine(directory, "home");
        var installed = Tool(Path.Combine("home", ".cargo", "bin"), "cargo");
        var onPath = Tool("path", "cargo");
        var homeVariable = OperatingSystem.IsWindows() ? "USERPROFILE" : "HOME";

        Assert.Equal(onPath, Gate(("PATH", Path.GetDirectoryName(onPath)), (homeVariable, home)).Decide(ForeignClient.Rust).Tool);
        Assert.Equal(installed, Gate((homeVariable, home)).Decide(ForeignClient.Rust).Tool);
    }

    [Theory]
    [MemberData(nameof(InteropGate.All), MemberType = typeof(InteropGate))]
    public void AToolThatDoesNotRunIsNoneAndSaysWhy(ForeignClient client)
    {
        Tool("path", OperatingSystem.IsWindows() ? "python" : "python3");
        Tool("path", "java");
        Tool("path", "javac");
        var cargo = Tool("path", "cargo");
        var values = new Dictionary<string, string?> { ["PATH"] = Path.GetDirectoryName(cargo) };
        string? Broken(ForeignClient _, string tool) => "exits with code 1 when asked its version: Unable to locate a Java Runtime.";

        var skipped = new InteropGate(values.GetValueOrDefault, Broken).Decide(client);
        values[InteropGate.RequireVariable] = "1";
        var failed = new InteropGate(values.GetValueOrDefault, Broken).Decide(client);

        Assert.Equal((null, false), (skipped.Tool, skipped.Fails));
        Assert.Matches($"^no working {client} toolchain: .+ exits with code 1 when asked its version: Unable to locate a Java Runtime\\.$", skipped.Reason);
        Assert.Equal((null, true), (failed.Tool, failed.Fails));
        Assert.Contains(skipped.Reason!, failed.Reason, StringComparison.Ordinal);
    }

    [Fact]
    [UnsupportedOSPlatform("windows")]
    public void APlaceholderThatCannotRunIsTakenForNoJdkAndAWorkingOneIsUsed()
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "The stand-ins are shell scripts.");
        var placeholder = Script("placeholder", "java", "echo 'The operation could not be completed. Unable to locate a Java Runtime.' >&2; exit 1");
        Script("placeholder", "javac", "exit 1");
        var working = Script("working", "java", "[ \"$1\" = -version ] || exit 2; echo 'openjdk version \"21\"' >&2");
        Script("working", "javac", "exit 0");

        var none = new InteropGate(name => name == "PATH" ? Path.GetDirectoryName(placeholder) : null).Decide(ForeignClient.Java);
        var found = new InteropGate(name => name == "PATH" ? Path.GetDirectoryName(working) : null).Decide(ForeignClient.Java);

        Assert.Null(none.Tool);
        Assert.Equal($"no working Java toolchain: {placeholder} exits with code 1 when asked its version: The operation could not be completed. Unable to locate a Java Runtime.", none.Reason);
        Assert.Equal(working, found.Tool);
    }

    // Every tool found runs, unless a test says otherwise.
    private static InteropGate Gate(params (string Name, string? Value)[] variables)
    {
        var values = variables.ToDictionary(variable => variable.Name, variable => variable.Value);
        return new InteropGate(name => values.GetValueOrDefault(name), (_, _) => null);
    }

    [UnsupportedOSPlatform("windows")]
    private string Script(string subdirectory, string name, string body)
    {
        var path = Tool(subdirectory, name);
        File.WriteAllText(path, "#!/bin/sh\n" + body + "\n");
        File.SetUnixFileMode(path, UnixFileMode.UserRead | UnixFileMode.UserExecute);
        return path;
    }

    private string Tool(string subdirectory, string name)
    {
        var path = Path.Combine(directory, subdirectory, OperatingSystem.IsWindows() ? name + ".exe" : name);
        Directory.CreateDirectory(Path.GetDirectoryName(path)!);
        File.WriteAllText(path, string.Empty);
        return path;
    }
}
