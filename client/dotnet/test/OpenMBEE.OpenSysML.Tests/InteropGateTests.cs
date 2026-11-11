using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Where Python, which the foreign lock holder runs on, is looked for, and whether its absence skips or fails.</summary>
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
    public void OnlyAPromiseOfPythonRequiresIt(string? value, bool required)
    {
        Assert.Equal(required, Gate((InteropGate.RequireVariable, value)).Required);
    }

    [Fact]
    public void AMissingPythonSkipsUnlessItWasPromisedAndThenFails()
    {
        var skipped = Gate(("PATH", directory)).Decide();
        var failed = Gate(("PATH", directory), (InteropGate.RequireVariable, "1")).Decide();

        Assert.Equal((null, false), (skipped.Tool, skipped.Fails));
        Assert.Equal((null, true), (failed.Tool, failed.Fails));
        Assert.Contains($"${InteropGate.RequireVariable} is set, so this test must run against the Python client's lock, but there is {skipped.Reason}.", failed.Reason, StringComparison.Ordinal);
    }

    [Fact]
    public void PythonIsWhatPythonNamesElseTheFirstOnPath()
    {
        var onPath = Tool("path", OperatingSystem.IsWindows() ? "python" : "python3");
        var named = Tool("named", "my-python");

        Assert.Equal(onPath, Gate(("PATH", Path.GetDirectoryName(onPath))).Decide().Tool);
        Assert.Equal(named, Gate(("PATH", Path.GetDirectoryName(onPath)), ("PYTHON", named)).Decide().Tool);
        Assert.Equal(onPath, Gate(("PATH", Path.GetDirectoryName(onPath)), ("PYTHON", "missing-python")).Decide().Tool);
    }

    [Fact]
    public void APythonThatDoesNotRunIsNoneAndSaysWhy()
    {
        var python = Tool("path", OperatingSystem.IsWindows() ? "python" : "python3");
        var values = new Dictionary<string, string?> { ["PATH"] = Path.GetDirectoryName(python) };
        static string? Broken(string tool) => "exits with code 9009 when asked its version: Python was not found";

        var skipped = new InteropGate(values.GetValueOrDefault, Broken).Decide();
        values[InteropGate.RequireVariable] = "1";
        var failed = new InteropGate(values.GetValueOrDefault, Broken).Decide();

        Assert.Equal((null, false), (skipped.Tool, skipped.Fails));
        Assert.Equal($"no working Python: {python} exits with code 9009 when asked its version: Python was not found", skipped.Reason);
        Assert.Equal((null, true), (failed.Tool, failed.Fails));
        Assert.Contains(skipped.Reason!, failed.Reason, StringComparison.Ordinal);
    }

    [Fact]
    [UnsupportedOSPlatform("windows")]
    public void APlaceholderThatCannotRunIsTakenForNoPythonAndAWorkingOneIsUsed()
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "The stand-ins are shell scripts.");
        var placeholder = Script("placeholder", "python3", "echo 'python3: no Python is installed here' >&2; exit 1");
        var working = Script("working", "python3", "[ \"$1\" = --version ] || exit 2; echo 'Python 3.12.3'");

        var none = new InteropGate(name => name == "PATH" ? Path.GetDirectoryName(placeholder) : null).Decide();
        var found = new InteropGate(name => name == "PATH" ? Path.GetDirectoryName(working) : null).Decide();

        Assert.Null(none.Tool);
        Assert.Equal($"no working Python: {placeholder} exits with code 1 when asked its version: python3: no Python is installed here", none.Reason);
        Assert.Equal(working, found.Tool);
    }

    // Every Python found runs, unless a test says otherwise.
    private static InteropGate Gate(params (string Name, string? Value)[] variables)
    {
        var values = variables.ToDictionary(variable => variable.Name, variable => variable.Value);
        return new InteropGate(name => values.GetValueOrDefault(name), _ => null);
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
