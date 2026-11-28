using System.Linq;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Tests;
using Xunit;

namespace OpenMBEE.OpenSysML.Generate.Tests;

[Trait(Category.Name, Category.Integration)]
public sealed class ModelClosureTests(LiveServices services) : IClassFixture<LiveServices>
{
    public static TheoryData<ClientWire> Wires => ClientWires.All;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ReopenedPackagesExposeEveryDefinitionAndCrossDocumentType(ClientWire wire)
    {
        var token = TestContext.Current.CancellationToken;
        await using var connection = await services.OpenAsync(wire, token);
        var sources = new[]
        {
            Document("a.sysml", "package P { part def A { part b : Q::Engine; } package Nested { part def X; } }"),
            Document("b.sysml", "package P { part def B; package Nested { part def Y; } } package Q { part def Engine; }"),
        };

        var generated = await GenerateCommand.GenerateAsync(connection, sources, "Test.Model", token);
        var reversed = await GenerateCommand.GenerateAsync(connection, [.. sources.Reverse()], "Test.Model", token);

        Assert.Equal(generated, reversed);
        Assert.Contains("public class A :", generated);
        Assert.Contains("public class B :", generated);
        Assert.Contains("public class X :", generated);
        Assert.Contains("public class Y :", generated);
        Assert.Contains("global::Test.Model.Q.IEngine B { get; }", generated);
        Assert.Empty(Compiler.Diagnostics(generated));
    }

    private static SourceFile Document(string name, string text) =>
        new(name, name, System.Text.Encoding.UTF8.GetBytes(text), text);
}
