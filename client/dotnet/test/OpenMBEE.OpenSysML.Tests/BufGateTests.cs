using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Where buf is looked for, how the BUF command line is read, and whether its absence skips or fails.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class BufGateTests : IDisposable
{
    private readonly string directory = Directory.CreateTempSubdirectory("opensysml-buf-gate-").FullName;

    public void Dispose() => Directory.Delete(directory, recursive: true);

    [Theory]
    [InlineData("1", true)]
    [InlineData(" yes ", true)]
    [InlineData(null, false)]
    [InlineData("", false)]
    [InlineData("0", false)]
    [InlineData("FALSE", false)]
    [InlineData("no", false)]
    public void OnlyAPromiseOfBufRequiresIt(string? value, bool required)
    {
        Assert.Equal(required, Gate((BufGate.RequireVariable, value)).Required);
    }

    [Fact]
    public void AMissingBufSkipsUnlessItWasPromisedAndThenFails()
    {
        var skipped = Gate(("PATH", directory)).Decide();
        var failed = Gate(("PATH", directory), (BufGate.RequireVariable, "1")).Decide();

        Assert.Equal(new BufGate.Decision(null, "no buf: neither $BUF nor $PATH names one", Fails: false), skipped);
        Assert.Equal((null, true), (failed.Command, failed.Fails));
        Assert.Equal($"${BufGate.RequireVariable} is set, so this test must build the schema with buf, but {skipped.Reason}.", failed.Reason);
    }

    [Fact]
    public void BufIsTheFirstOnPathWhenBufIsUnset()
    {
        var first = Tool("first", "buf");
        Tool("second", "buf");
        var searchPath = string.Join(Path.PathSeparator, Path.GetDirectoryName(first), Path.Combine(directory, "second"));

        Assert.Equal([first], Gate(("PATH", searchPath)).Decide().Command);
        Assert.Equal([first], Gate(("PATH", searchPath), (BufGate.CommandVariable, " ")).Decide().Command);
    }

    [Fact]
    public void BufIsReadAsTheMakefileReadsItAProgramAndItsArguments()
    {
        var go = Tool("go", "go");
        var gate = Gate(("PATH", Path.GetDirectoryName(go)), (BufGate.CommandVariable, " go  run github.com/bufbuild/buf/cmd/buf@v1.57.2 "));

        Assert.Equal([go, "run", "github.com/bufbuild/buf/cmd/buf@v1.57.2"], gate.Decide().Command);
        Assert.Equal([go, "run", "github.com/bufbuild/buf/cmd/buf@v1.57.2"], gate.RequireCommand());
    }

    [Fact]
    public void BufNamingAPathIsThatFileAndOverridesPath()
    {
        var onPath = Tool("path", "buf");
        var named = Tool("pinned", "buf");
        var relative = Path.GetRelativePath(Environment.CurrentDirectory, named);

        Assert.Equal([named], Gate(("PATH", Path.GetDirectoryName(onPath)), (BufGate.CommandVariable, relative)).Decide().Command);
    }

    [Fact]
    public void BufNamingNoProgramIsNotReplacedByOneOnPath()
    {
        var onPath = Tool("path", "buf");
        var absent = Path.Combine(directory, "absent", "buf");
        var skipped = Gate(("PATH", Path.GetDirectoryName(onPath)), (BufGate.CommandVariable, absent + " --debug")).Decide();
        var failed = Gate(("PATH", Path.GetDirectoryName(onPath)), (BufGate.CommandVariable, absent), (BufGate.RequireVariable, "1")).Decide();

        Assert.Equal(new BufGate.Decision(null, $"$BUF is \"{absent} --debug\", and {absent} is neither a file nor on $PATH", Fails: false), skipped);
        Assert.Equal((null, true), (failed.Command, failed.Fails));
        Assert.Contains($"but $BUF is \"{absent}\", and {absent} is neither a file nor on $PATH.", failed.Reason, StringComparison.Ordinal);
    }

    private static BufGate Gate(params (string Name, string? Value)[] variables)
    {
        var values = variables.ToDictionary(variable => variable.Name, variable => variable.Value);
        return new BufGate(name => values.GetValueOrDefault(name));
    }

    private string Tool(string subdirectory, string name)
    {
        var path = Path.Combine(directory, subdirectory, OperatingSystem.IsWindows() ? name + ".exe" : name);
        Directory.CreateDirectory(Path.GetDirectoryName(path)!);
        File.WriteAllText(path, string.Empty);
        return path;
    }
}
