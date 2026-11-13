using System.Linq;
using Xunit;

namespace OpenMBEE.OpenSysML.Conformance.Tests;

/// <summary>
/// Spellings and orders read as the reference runner's: each expected string below is what Go
/// writes for the same value.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class SpellingTests
{
    public static TheoryData<double, string> Reals => new()
    {
        { 1500, "1500" },
        { 1500.1, "1500.1" },
        { -1500.5, "-1500.5" },
        { 100000, "100000" },
        { 123456, "123456" },
        { 1234567, "1.234567e+06" },
        { 1e6, "1e+06" },
        { 123456789, "1.23456789e+08" },
        { 1e20, "1e+20" },
        { 1e21, "1e+21" },
        { 1e100, "1e+100" },
        { 1e-4, "0.0001" },
        { 0.000123, "0.000123" },
        { 0.012, "0.012" },
        { 1e-5, "1e-05" },
        { 2e-7, "2e-07" },
        { 5e-324, "5e-324" },
        { double.MaxValue, "1.7976931348623157e+308" },
        { 1.0 / 3, "0.3333333333333333" },
        { 0.0, "0" },
        { -0.0, "-0" },
        { double.NaN, "NaN" },
        { double.PositiveInfinity, "+Inf" },
        { double.NegativeInfinity, "-Inf" },
    };

    [Theory]
    [MemberData(nameof(Reals))]
    public void ARealIsSpelledAsGoPrintsIt(double value, string expected)
    {
        Assert.Equal(expected, Spelling.Real(value));
    }

    [Fact]
    public void ARealIsSpelledTheSameUnderAnyCulture()
    {
        var previous = System.Globalization.CultureInfo.CurrentCulture;
        try
        {
            System.Globalization.CultureInfo.CurrentCulture = new System.Globalization.CultureInfo("sv-SE");
            Assert.Equal("1500.1", Spelling.Real(1500.1));
            Assert.Equal("-1.5e+07", Spelling.Real(-15e6));
        }
        finally
        {
            System.Globalization.CultureInfo.CurrentCulture = previous;
        }
    }

    [Fact]
    public void TextIsQuotedAsGoQuotesIt()
    {
        Assert.Equal("\"a\\\"b\\\\c\\nd\\te\\x01é\\u2028\"", Spelling.Quote("a\"b\\c\nd\te\x01é\u2028"));
        Assert.Equal("\"\\u00a0x\\a\\b\\f\\v\\x7f\U0001F600\u0301\"", Spelling.Quote("\u00a0x\a\b\f\v\x7f\U0001F600\u0301"));
        Assert.Equal("\"\\ud800\"", Spelling.Quote("\ud800"));
        Assert.Equal("\"\\r\\U000e0001\"", Spelling.Quote("\r\U000E0001"));
    }

    [Fact]
    public void StringsOrderByCodePoint()
    {
        string[] words = ["b", "\U0001F600", "\uE000", "a", "B", "ab"];

        Assert.Equal(["B", "a", "ab", "b", "\uE000", "\U0001F600"], words.Order(Spelling.CodePointOrder));
        Assert.True(Spelling.CodePointOrder.Compare(null, "") < 0);
        Assert.True(Spelling.CodePointOrder.Compare("", null) > 0);
        Assert.Equal(0, Spelling.CodePointOrder.Compare(null, null));
        // UTF-16 code units alone would put the astral character first of the two.
        Assert.True(string.CompareOrdinal("\U0001F600", "\uE000") < 0);
    }
}
