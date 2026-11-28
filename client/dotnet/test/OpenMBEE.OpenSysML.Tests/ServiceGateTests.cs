using System;
using System.Collections.Generic;
using System.IO;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

[Trait(Category.Name, Category.Unit)]
public sealed class ServiceGateTests : IDisposable
{
    private readonly string directory = Directory.CreateTempSubdirectory("opensysml-gate-").FullName;

    public void Dispose() => Directory.Delete(directory, recursive: true);

    [Theory]
    [InlineData("1")]
    [InlineData("true")]
    [InlineData("yes")]
    [InlineData(" 1 ")]
    public void PromisedServiceIsRequired(string value)
    {
        Assert.True(Gate((ServiceGate.RequireServiceVariable, value)).Required);
    }

    [Theory]
    [InlineData(null)]
    [InlineData("")]
    [InlineData(" ")]
    [InlineData("0")]
    [InlineData("false")]
    [InlineData("FALSE")]
    [InlineData("no")]
    public void UnpromisedServiceIsNotRequired(string? value)
    {
        Assert.False(Gate((ServiceGate.RequireServiceVariable, value)).Required);
    }

    [Fact]
    public void MissingBinarySkipsWhenNoServiceWasPromised()
    {
        var decision = Gate().Decide();

        Assert.Null(decision.Binary);
        Assert.False(decision.Fails);
        Assert.Equal($"${ServiceGate.BinaryVariable} names no sysml-grpc binary", decision.Reason);
    }

    [Fact]
    public void MissingBinaryFailsWhenAServiceWasPromised()
    {
        var decision = Gate((ServiceGate.RequireServiceVariable, "1")).Decide();

        Assert.Null(decision.Binary);
        Assert.True(decision.Fails);
        Assert.Contains(ServiceGate.RequireServiceVariable, decision.Reason, StringComparison.Ordinal);
        Assert.Contains($"${ServiceGate.BinaryVariable} names no sysml-grpc binary", decision.Reason, StringComparison.Ordinal);
    }

    [Fact]
    public void BinaryThatIsNotAFileFailsWhenAServiceWasPromised()
    {
        var absent = Path.Combine(directory, "sysml-grpc");
        var decision = Gate((ServiceGate.RequireServiceVariable, "1"), (ServiceGate.BinaryVariable, absent)).Decide();

        Assert.Null(decision.Binary);
        Assert.True(decision.Fails);
        Assert.Contains($"names {absent}, which is not a file", decision.Reason, StringComparison.Ordinal);
    }

    [Fact]
    public void BinaryThatIsNotAFileSkipsWhenNoServiceWasPromised()
    {
        var decision = Gate((ServiceGate.BinaryVariable, directory)).Decide();

        Assert.Null(decision.Binary);
        Assert.False(decision.Fails);
        Assert.Equal($"${ServiceGate.BinaryVariable} names {directory}, which is not a file", decision.Reason);
    }

    [Fact]
    public void ConfiguredBinaryIsReturnedAsAFullPath()
    {
        var binary = Path.Combine(directory, "sysml-grpc");
        File.WriteAllBytes(binary, []);
        var relative = Path.GetRelativePath(Environment.CurrentDirectory, binary);
        var gate = Gate((ServiceGate.RequireServiceVariable, "1"), (ServiceGate.BinaryVariable, relative));

        Assert.Equal(new ServiceGate.Decision(binary, null, Fails: false), gate.Decide());
        Assert.Equal(binary, gate.RequireBinary());
    }

    private static ServiceGate Gate(params (string Name, string? Value)[] variables)
    {
        var environment = new Dictionary<string, string?>(StringComparer.Ordinal);
        foreach (var (name, value) in variables)
        {
            environment[name] = value;
        }

        return new ServiceGate(name => environment.GetValueOrDefault(name));
    }
}
