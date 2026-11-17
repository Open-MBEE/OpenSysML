using Xunit;

namespace OpenMBEE.OpenSysML.Generate.Tests;

[Trait(Category.Name, Category.Unit)]
public sealed class IdentifiersTests
{
    [Theory]
    [InlineData("Vehicle", "Vehicle")]
    [InlineData("my car", "my_car")]
    [InlineData("2nd", "_2nd")]
    [InlineData("a-b.c", "a_b_c")]
    [InlineData("Größe", "Größe")]
    [InlineData("+", "_")]
    [InlineData("string", "string")]
    [InlineData("A\u200DB", "AB")]
    [InlineData("\u200Drecord", "record")]
    [InlineData("\u00AD\u200D", "_")]
    [InlineData("Cafe\u0301", "Cafe\u0301")]
    [InlineData("車", "車")]
    [InlineData("A\U0001F697B", "A_B")]
    [InlineData("\U00010400", "_")]
    [InlineData("A\U00010400B", "A_B")]
    public void ATypeNameKeepsTheNameAsWrittenWhereCSharpAllows(string name, string expected) =>
        Assert.Equal(expected, Identifiers.TypeName(name));

    [Theory]
    [InlineData("mass", "Mass")]
    [InlineData("maxSpeed", "MaxSpeed")]
    [InlineData("top speed", "TopSpeed")]
    [InlineData("wheel_count", "WheelCount")]
    [InlineData("_hidden", "Hidden")]
    [InlineData("2d", "_2d")]
    [InlineData("größe", "Größe")]
    [InlineData("+", "")]
    [InlineData("", "")]
    [InlineData("A\U00010400B", "AB")]
    public void AMemberNameIsPascalCase(string name, string expected) =>
        Assert.Equal(expected, Identifiers.MemberName(name));

    [Fact]
    public void KeywordsAreEscapedAndContextualKeywordsAreNot()
    {
        Assert.Equal("@class", Identifiers.Escaped("class"));
        Assert.Equal("@string", Identifiers.Escaped("string"));
        Assert.Equal("var", Identifiers.Escaped("var"));
        Assert.Equal("Class", Identifiers.Escaped("Class"));
    }

    [Theory]
    [InlineData("record")]
    [InlineData("file")]
    [InlineData("required")]
    [InlineData("scoped")]
    [InlineData("__arglist")]
    [InlineData("__makeref")]
    [InlineData("__reftype")]
    [InlineData("__refvalue")]
    public void RestrictedTypeNamesAreEscapedAfterCanonicalization(string name)
    {
        Assert.Equal("@" + name, Identifiers.Escaped(Identifiers.TypeName(name)));
        Assert.Equal("@" + name, Identifiers.Escaped(Identifiers.TypeName(name.Insert(1, "\u200D"))));
    }

    [Fact]
    public void InvalidSurrogatesCannotBecomeIdentifierCharacters()
    {
        Assert.Equal("A_B", Identifiers.TypeName("A\uD800B"));
        Assert.Equal("A_B", Identifiers.TypeName("A\uDC00B"));
        Assert.Equal("_", Identifiers.TypeName("\uD800"));
    }

    [Theory]
    [InlineData("Acme", true)]
    [InlineData("Model2", true)]
    [InlineData("class", false)]
    [InlineData("2Model", false)]
    [InlineData("a-b", false)]
    [InlineData("", false)]
    [InlineData("A\u200DB", false)]
    [InlineData("cl\u200Dass", false)]
    [InlineData("record", false)]
    [InlineData("__arglist", false)]
    [InlineData("\U00010400", false)]
    public void APlainIdentifierIsOneAsWritten(string text, bool expected) =>
        Assert.Equal(expected, Identifiers.IsPlainIdentifier(text));

    [Fact]
    public void ALiteralHoldsTheTextExactly()
    {
        Assert.Equal("\"a\\\"b\\\\c\"", Identifiers.Literal("a\"b\\c"));
        Assert.Equal("\"line\\u000abreak\\u2028\"", Identifiers.Literal("line\nbreak\u2028"));
        Assert.Equal("\"\\ud83d\\ude97\"", Identifiers.Literal("\U0001F697"));
        Assert.Equal("\"Größe\"", Identifiers.Literal("Größe"));
    }

    [Fact]
    public void DocumentationTextIsEscapedOntoOneLine() =>
        Assert.Equal("a &lt;b&gt; &amp; c d", Identifiers.DocText("a <b> & c\nd"));
}
