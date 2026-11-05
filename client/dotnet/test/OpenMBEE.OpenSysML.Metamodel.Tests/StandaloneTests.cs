using System;
using System.IO;
using System.Linq;
using System.Text.Json;
using OpenMBEE.OpenSysML.Tests;
using Xunit;

namespace OpenMBEE.OpenSysML.Metamodel.Tests;

/// <summary>The metamodel assembly stands alone and records which metamodel it was generated from.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class StandaloneTests
{
    private static readonly string[] Forbidden = ["OpenMBEE.OpenSysML", "Google.Protobuf", "Grpc.", "Microsoft.Extensions.Logging", "Sigstore"];

    /// <summary>The provenance constants are the metamodel table's own.</summary>
    [Fact]
    public void ProvenanceIsTheTables()
    {
        Assert.Equal(Metaclasses.MetamodelVersion, CommittedOntology.TableConstant("Version"));
        Assert.Equal(Metaclasses.SourceTag, CommittedOntology.TableConstant("SourceTag"));
        Assert.Equal(Metaclasses.SourceCommit, CommittedOntology.TableConstant("SourceCommit"));
        var header = File.ReadLines(Repository.PathOf("client", "dotnet", "src", "OpenMBEE.OpenSysML.Metamodel", "Generated.cs")).Take(9).ToList();
        Assert.Contains($"//     Metamodel: {Metaclasses.MetamodelVersion}", header);
        Assert.Contains($"//     Pilot commit: {Metaclasses.SourceCommit}", header);
    }

    /// <summary>The assembly references the framework alone: no client library, protobuf, gRPC, logging or Sigstore assembly.</summary>
    [Fact]
    public void AssemblyReferencesOnlyTheFramework()
    {
        var assembly = typeof(MetamodelElement).Assembly;
        Assert.Equal("OpenMBEE.OpenSysML.Metamodel", assembly.GetName().Name);
        var references = assembly.GetReferencedAssemblies().Select(reference => reference.Name!).ToList();
        Assert.NotEmpty(references);
        Assert.All(references, name => Assert.True(name.StartsWith("System.", StringComparison.Ordinal) || name == "netstandard", $"references {name}"));
        Assert.DoesNotContain(AppDomain.CurrentDomain.GetAssemblies(), loaded => loaded.GetName().Name == "OpenMBEE.OpenSysML");
    }

    /// <summary>
    /// The project's locked dependency graph holds no project and no package but the build-time API analyzer,
    /// and this consumer's graph holds the metamodel project and none of the client's dependencies.
    /// </summary>
    [Fact]
    public void LockedDependencyGraphsExcludeTheClient()
    {
        var metamodel = LockedDependencies(Repository.PathOf("client", "dotnet", "src", "OpenMBEE.OpenSysML.Metamodel", "packages.lock.json"));
        Assert.Equal(["Microsoft.CodeAnalysis.PublicApiAnalyzers"], metamodel.Select(dependency => dependency.Name));
        var project = File.ReadAllText(Repository.PathOf("client", "dotnet", "src", "OpenMBEE.OpenSysML.Metamodel", "OpenMBEE.OpenSysML.Metamodel.csproj"));
        Assert.DoesNotContain("ProjectReference", project, StringComparison.Ordinal);
        Assert.Contains("PrivateAssets=\"all\"", project, StringComparison.Ordinal);

        var consumer = LockedDependencies(Repository.PathOf("client", "dotnet", "test", "OpenMBEE.OpenSysML.Metamodel.Tests", "packages.lock.json"));
        Assert.Equal(["openmbee.opensysml.metamodel"], consumer.Where(dependency => dependency.Type == "Project").Select(dependency => dependency.Name));
        Assert.All(consumer, dependency => Assert.DoesNotContain(
            Forbidden.Where(prefix => prefix != "OpenMBEE.OpenSysML"),
            prefix => dependency.Name.StartsWith(prefix, StringComparison.OrdinalIgnoreCase)));
        Assert.DoesNotContain(consumer, dependency => dependency.Name.Equals("OpenMBEE.OpenSysML", StringComparison.OrdinalIgnoreCase));
    }

    private static (string Name, string Type)[] LockedDependencies(string path)
    {
        using var document = JsonDocument.Parse(File.ReadAllText(path));
        var frameworks = document.RootElement.GetProperty("dependencies").EnumerateObject().ToList();
        Assert.Equal(["net10.0"], frameworks.Select(framework => framework.Name));
        return [.. frameworks[0].Value.EnumerateObject().Select(dependency => (dependency.Name, dependency.Value.GetProperty("type").GetString()!))];
    }
}
