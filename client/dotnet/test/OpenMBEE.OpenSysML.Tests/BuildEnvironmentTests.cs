using System;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Runtime.Versioning;
using System.Text.Json;
using System.Xml.Linq;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

[Trait(Category.Name, Category.Unit)]
public sealed class BuildEnvironmentTests
{
    private static readonly Assembly Client = typeof(ReleaseDigests).Assembly;

    [Fact]
    public void TestsRunOnTheNet10Runtime()
    {
        Assert.Equal(10, Environment.Version.Major);
        Assert.StartsWith(".NET 10.", RuntimeInformation.FrameworkDescription, StringComparison.Ordinal);
    }

    [Fact]
    public void ClientAndTestsTargetNet10Only()
    {
        Assert.Equal(".NETCoreApp,Version=v10.0", Client.GetCustomAttribute<TargetFrameworkAttribute>()?.FrameworkName);
        Assert.Equal(".NETCoreApp,Version=v10.0", typeof(BuildEnvironmentTests).Assembly.GetCustomAttribute<TargetFrameworkAttribute>()?.FrameworkName);
        Assert.Equal("net10.0", BuildProperty("TargetFramework"));
    }

    [Fact]
    public void RootGlobalJsonPinsAnExactSdkAndTheTestingPlatform()
    {
        using var globalJson = JsonDocument.Parse(File.ReadAllBytes(Repository.PathOf("global.json")));
        var sdk = globalJson.RootElement.GetProperty("sdk");

        Assert.Matches(@"^10\.0\.[1-9][0-9]{2}$", sdk.GetProperty("version").GetString());
        Assert.Equal("latestPatch", sdk.GetProperty("rollForward").GetString());
        Assert.False(sdk.GetProperty("allowPrerelease").GetBoolean());
        Assert.Equal("Microsoft.Testing.Platform", globalJson.RootElement.GetProperty("test").GetProperty("runner").GetString());
    }

    [Fact]
    public void NoNestedGlobalJsonOverridesTheRootPin()
    {
        var clientDirectory = Repository.PathOf("client", "dotnet");
        var nested = Directory.EnumerateFiles(clientDirectory, "global.json", SearchOption.AllDirectories)
            .Select(path => Path.GetRelativePath(Repository.Root, path))
            .ToList();

        Assert.Empty(nested);
    }

    [Fact]
    public void ClientVersionIsTheDeclaredVersion()
    {
        var version = BuildProperty("Version");

        Assert.Equal(version, Client.GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion);
        Assert.Equal(version, typeof(BuildEnvironmentTests).Assembly.GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion);
    }

    [Fact]
    public void NothingIsPackable()
    {
        Assert.Equal("false", BuildProperty("IsPackable"));
    }

    private static string BuildProperty(string name)
    {
        var props = XDocument.Load(Repository.PathOf("client", "dotnet", "Directory.Build.props"));
        var values = props.Root!.Elements("PropertyGroup")
            .Where(group => group.Attribute("Condition") is null)
            .Elements(name)
            .Select(element => element.Value)
            .ToList();
        return Assert.Single(values);
    }
}
