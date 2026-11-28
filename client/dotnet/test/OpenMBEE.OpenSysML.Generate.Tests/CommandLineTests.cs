using Xunit;

namespace OpenMBEE.OpenSysML.Generate.Tests;

[Trait(Category.Name, Category.Unit)]
public sealed class CommandLineTests
{
    [Fact]
    public void ReadsSourcesAndOptionsInEitherSpelling()
    {
        var command = CommandLine.Parse(["lib.sysml", "-o", "Out.cs", "--namespace=Acme.Model", "--check", "--address", "localhost:50051", "--", "-odd.sysml"]);

        Assert.Equal(["lib.sysml", "-odd.sysml"], command.Sources);
        Assert.Equal(("Out.cs", "Acme.Model", true, "localhost:50051", (string?)null), (command.Output, command.Namespace, command.Check, command.Address, command.Binary));
        Assert.Equal(
            "dotnet run --project client/dotnet/src/OpenMBEE.OpenSysML.Generate -- lib.sysml -odd.sysml -o Out.cs --namespace Acme.Model --address localhost:50051",
            command.RegenerateCommand());
    }

    [Fact]
    public void HelpNeedsNothingElse() => Assert.True(CommandLine.Parse(["--help"]).Help);

    [Theory]
    [InlineData(new[] { "--namespace", "A" }, "name at least one SOURCE")]
    [InlineData(new[] { "m.sysml" }, "--namespace is required")]
    [InlineData(new[] { "m.sysml", "--namespace" }, "--namespace is missing its value")]
    [InlineData(new[] { "m.sysml", "--namespace=" }, "--namespace is missing its value")]
    [InlineData(new[] { "m.sysml", "--namespace", "A", "--check" }, "--check needs -o/--output, the file to check")]
    [InlineData(new[] { "m.sysml", "--namespace", "A", "--frobnicate" }, "unknown option --frobnicate")]
    [InlineData(new[] { "m.sysml", "--namespace", "A", "--check=yes", "-o", "x" }, "unknown option --check=yes")]
    [InlineData(new[] { "m.sysml", "--namespace", "A", "--address", "h:1", "--binary", "b" }, "--address names a running service and --binary one to start; give one")]
    [InlineData(new[] { "m.sysml", "--namespace", "Acme.class" }, "--namespace Acme.class is not a C# namespace: 'class' is not an identifier it can hold")]
    [InlineData(new[] { "m.sysml", "--namespace", "Acme..Model" }, "--namespace Acme..Model is not a C# namespace: '' is not an identifier it can hold")]
    [InlineData(new[] { "m.sysml", "--namespace", "global.Model" }, "--namespace global.Model is not a C# namespace: 'global' is not an identifier it can hold")]
    [InlineData(new[] { "m.sysml", "--namespace", "Acme.My-Model" }, "--namespace Acme.My-Model is not a C# namespace: 'My-Model' is not an identifier it can hold")]
    public void RefusesWhatItCannotDoAsAsked(string[] arguments, string message) =>
        Assert.Equal(message, Assert.Throws<UsageException>(() => CommandLine.Parse(arguments)).Message);

    [Theory]
    [InlineData("Acme.A\u200DB")]
    [InlineData("Acme.cl\u200Dass")]
    [InlineData("Acme.\u200Dclass")]
    [InlineData("Acme.rec\u200Dord")]
    [InlineData("glo\u200Dbal.Model")]
    [InlineData("Acme.__arglist")]
    [InlineData("Acme.__arg\u200Dlist")]
    [InlineData("Acme.\U0001F697")]
    [InlineData("Acme.\U00010400")]
    public void NamespaceSegmentsMustBeCanonicalUnescapedIdentifiers(string name) =>
        Assert.Contains("is not a C# namespace", Assert.Throws<UsageException>(() =>
            CommandLine.Parse(["m.sysml", "--namespace", name])).Message, System.StringComparison.Ordinal);
}
