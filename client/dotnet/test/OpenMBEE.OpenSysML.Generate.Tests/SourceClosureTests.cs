using System.IO;
using System.Linq;
using System.Text;
using Xunit;

namespace OpenMBEE.OpenSysML.Generate.Tests;

/// <summary>
/// The documents a model is generated from, and the stamp of exactly what they held: their logical names and
/// exact bytes, nothing about the machine, the working directory or the order they were named in.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class SourceClosureTests
{
    [Fact]
    public void AFileIsNamedByItsFileNameAndADirectoryByItsOwnNameAndPathWithin()
    {
        using var temp = new TempDirectory();
        var top = temp.Write("top.sysml", "package Top;");
        temp.Write("models/b/inner.kerml", "package Inner;");
        temp.Write("models/a.sysml", "package A;");
        temp.Write("models/.hidden/skipped.sysml", "package Skipped;");
        temp.Write("models/notes.txt", "not a model");

        var documents = SourceClosure.Read([top, Path.Combine(temp.Path, "models"), top]);

        Assert.Equal(["top.sysml", "models/a.sysml", "models/b/inner.kerml"], documents.Select(document => document.LogicalName));
        Assert.Equal([null, null, SourceLanguage.KerML], documents.Select(document => document.InlineLanguage));
    }

    [Fact]
    public void RefusesWhatItCannotRead()
    {
        using var temp = new TempDirectory();
        var a = temp.Write("a/model.sysml", "package A;");
        var b = temp.Write("b/model.sysml", "package B;");
        var empty = Directory.CreateDirectory(Path.Combine(temp.Path, "empty")).FullName;
        var latin1 = temp.Write("latin1.sysml", [0x70, 0xE9, 0x0A]);
        var missing = Path.Combine(temp.Path, "missing.sysml");

        Assert.Equal($"{a} and {b} would both be the document model.sysml; name the directory that holds them instead", Assert.Throws<UsageException>(() => SourceClosure.Read([a, b])).Message);
        Assert.Equal($"{empty} holds no .sysml or .kerml file", Assert.Throws<UsageException>(() => SourceClosure.Read([empty])).Message);
        Assert.Equal($"{latin1} is not UTF-8 text", Assert.Throws<UsageException>(() => SourceClosure.Read([latin1])).Message);
        Assert.Equal($"{missing} does not exist", Assert.Throws<UsageException>(() => SourceClosure.Read([missing])).Message);
    }

    [Fact]
    public void AByteOrderMarkIsHashedButNotParsed()
    {
        using var temp = new TempDirectory();
        var marked = temp.Write("marked.sysml", [0xEF, 0xBB, 0xBF, .. Encoding.UTF8.GetBytes("package A;")]);

        var document = Assert.Single(SourceClosure.Read([marked]));

        Assert.Equal("package A;", document.Text);
        Assert.Equal(13, document.Bytes.Length);
    }

    [Fact]
    public void TheStampIsOfNamesAndExactBytesWhateverTheOrderOrPlace()
    {
        var lib = Document("lib.sysml", "package Lib;\n");
        var top = Document("top.sysml", "package Top;\n");
        var stamp = SourceClosure.Stamp([lib, top]);

        Assert.Matches("^sha256:[0-9a-f]{64}$", stamp);
        Assert.Equal(stamp, SourceClosure.Stamp([top, lib]));
        Assert.Equal(stamp, SourceClosure.Stamp([lib with { Path = "/elsewhere/lib.sysml" }, top]));
        Assert.NotEqual(stamp, SourceClosure.Stamp([lib, Document("top.sysml", "package Top;\r\n")]));
        Assert.NotEqual(stamp, SourceClosure.Stamp([lib, Document("top2.sysml", "package Top;\n")]));
        Assert.NotEqual(stamp, SourceClosure.Stamp([lib]));
    }

    [Fact]
    public void TheStampDelimitsEveryNameAndContent()
    {
        Assert.NotEqual(SourceClosure.Stamp([Document("ab", "c")]), SourceClosure.Stamp([Document("a", "bc")]));
        Assert.NotEqual(SourceClosure.Stamp([Document("a", "b"), Document("c", "d")]), SourceClosure.Stamp([Document("a", "bcd")]));
    }

    [Fact]
    public void TheStampIsPinned()
    {
        // The documented algorithm, computed independently, so the stamp cannot drift without this test.
        Assert.Equal(
            "sha256:" + System.Convert.ToHexStringLower(System.Security.Cryptography.SHA256.HashData(
            [
                0, 0, 0, 0, 0, 0, 0, 1,
                0, 0, 0, 0, 0, 0, 0, 1, (byte)'a',
                0, 0, 0, 0, 0, 0, 0, 2, (byte)'b', (byte)'c',
            ])),
            SourceClosure.Stamp([Document("a", "bc")]));
    }

    private static SourceFile Document(string name, string text) => new(name, name, Encoding.UTF8.GetBytes(text), text);
}
