using System;
using System.Linq;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Internal.Mapping;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Document values, the instants of a state run's trace and the rows of document queries: every arm read
/// as the service sent it, a value of no arm read as an element named by no id unless it carries an arm
/// this client does not know, what the schema rules out refused as malformed, and equality by content.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class DocumentValueTests
{
    private const string Method = Model.ExecuteStateMethod;

    [Fact]
    public void EachArmIsReadAsTheServiceSentIt()
    {
        Assert.Equal(new DocumentValue.ElementValue("Garage::car", "PartUsage"), Read(CompleteExecutionWire.Element));
        Assert.Equal(new DocumentValue.ElementValue(null, "AssertConstraintUsage"), Read(new Wire.DocumentValue { ElementType = "AssertConstraintUsage" }));
        Assert.Equal(new DocumentValue.ElementValue(string.Empty, string.Empty), Read(new Wire.DocumentValue { ElementId = string.Empty }));
        Assert.Equal(new DocumentValue.TextValue("t"), Read(new Wire.DocumentValue { StringValue = "t" }));
        Assert.Equal(new DocumentValue.NumberValue(new Value.IntegerValue(-3)), Read(new Wire.DocumentValue { IntValue = -3 }));
        Assert.Equal(
            new DocumentValue.NumberValue(new Value.BigIntegerValue(System.Numerics.BigInteger.Parse("9223372036854775808", System.Globalization.CultureInfo.InvariantCulture))),
            Read(new Wire.DocumentValue { BigIntValue = "9223372036854775808" }));
        Assert.Equal(new DocumentValue.NumberValue(new Value.RationalValue(new Rational(1, 3))), Read(new Wire.DocumentValue { RationalValue = new Wire.Rational { Numerator = "1", Denominator = "3" } }));
        Assert.Equal(new DocumentValue.NumberValue(new Value.RealValue(double.NaN)), Read(new Wire.DocumentValue { RealValue = double.NaN }));
        Assert.Equal(new DocumentValue.BooleanValue(false), Read(new Wire.DocumentValue { BoolValue = false }));
        Assert.Equal(new DocumentValue.InfinityValue(), Read(new Wire.DocumentValue { Infinity = true }));
        Assert.Equal(new DocumentValue.QuantityValue(new Quantity(new Value.IntegerValue(2), "kg", null)), Read(new Wire.DocumentValue { Quantity = new Wire.Quantity { IntMagnitude = 2, Unit = "kg" } }));

        var verdict = Assert.IsType<DocumentValue.VerdictValue>(Read(new Wire.DocumentValue { Verdict = CompleteExecutionWire.Verdict })).Verdict;
        Assert.Equal(new DocumentValue.ElementValue("Garage::massKnown", "ConstraintUsage"), verdict.Assertion);
        Assert.Equal(("constraint", "assert constraint massKnown", "Garage::car", "violated", "mass > 0", "mass is 0"), (verdict.Kind, verdict.Text, verdict.Path, verdict.Verdict, verdict.Condition, verdict.Reason));
        Assert.Equal(["fail", "pass"], verdict.Verification);

        var state = Assert.IsType<DocumentValue.StateValue>(Read(new Wire.DocumentValue { State = CompleteExecutionWire.State })).State;
        Assert.Equal(new DocumentObject(2, "Garage::car", new DocumentValue.ElementValue("Garage::car", "PartUsage")), state.Subject);
        Assert.Equal(("lp", "dim", "on.dim", "light"), (state.Machine, state.Name, state.StatePath, state.Region));
        Assert.Equal(new DocumentValue.ElementValue("Lamp::on::dim", "StateUsage"), state.State);
        Assert.Equal(["on"], state.Enclosing);

        var nested = Assert.IsType<DocumentValue.EventValue>(Read(new Wire.DocumentValue { Event = CompleteExecutionWire.Event })).Event;
        Assert.Equal("send Switch to #3", nested.Text);
        Assert.Equal(new DocumentObject(3, "#3", null), Assert.IsType<DocumentValue.ObjectValue>(Read(new Wire.DocumentValue { Object = CompleteExecutionWire.Event.Target })).Value);
    }

    [Fact]
    public void AnAnonymousAssertionOrUndeclaredStateIsAnElementOfNoIdAndAnAbsentOneIsNull()
    {
        var verdict = Assert.IsType<DocumentValue.VerdictValue>(Read(new Wire.DocumentValue { Verdict = new Wire.DocumentVerdict { Assertion = new Wire.DocumentValue { ElementType = "SatisfyRequirementUsage" } } })).Verdict;
        var state = Assert.IsType<DocumentValue.StateValue>(Read(new Wire.DocumentValue { State = new Wire.DocumentState { State = new Wire.DocumentValue() } })).State;
        var bare = Assert.IsType<DocumentValue.StateValue>(Read(new Wire.DocumentValue { State = new Wire.DocumentState() })).State;

        Assert.Equal(new DocumentValue.ElementValue(null, "SatisfyRequirementUsage"), verdict.Assertion);
        Assert.Equal(new DocumentValue.ElementValue(null, string.Empty), state.State);
        Assert.Null(bare.State);
        Assert.Null(bare.Subject);
        Assert.Equal("(no object). in ", bare.ToString());
    }

    [Fact]
    public void AnArmThisClientDoesNotKnowIsUnsupportedNeverAnAnonymousElement()
    {
        var future = new Wire.DocumentValue { ElementType = "PartUsage" };
        future.MergeFrom(new byte[] { 0xFA, 0x03, 0x01, 0x00 }); // field 63, an arm no client knows yet

        var unsupported = Assert.Throws<UnsupportedValueException>(() => DocumentWire.Value(future, Method, "trace[2].time"));

        Assert.Equal("trace[2].time", unsupported.Path);
        Assert.Equal(Method, unsupported.Method);
        Assert.Equal(future.ToByteArray(), unsupported.ValueBytes.ToArray());
    }

    [Fact]
    public void WhatTheSchemaRulesOutIsAMalformedAnswerNamingWhereItIs()
    {
        var typed = Malformed(new Wire.DocumentValue { IntValue = 1, ElementType = "PartUsage" });
        var element = Malformed(new Wire.DocumentValue { Object = new Wire.DocumentObject { Element = new Wire.DocumentValue { StringValue = "car" } } });
        var assertion = Malformed(new Wire.DocumentValue { Verdict = new Wire.DocumentVerdict { Assertion = new Wire.DocumentValue { BoolValue = true } } });
        var infinity = Malformed(new Wire.DocumentValue { Infinity = false });
        var digits = Malformed(new Wire.DocumentValue { BigIntValue = "12" });
        var rational = Malformed(new Wire.DocumentValue { RationalValue = new Wire.Rational { Numerator = "1", Denominator = "2" } });
        var magnitude = Malformed(new Wire.DocumentValue { Event = new Wire.DocumentEvent { Time = new Wire.DocumentValue { Quantity = new Wire.Quantity { Unit = "s" } } } });

        Assert.All([typed, element, assertion, infinity, digits, rational, magnitude], error => Assert.Equal(StatusCode.Internal, error.StatusCode));
        Assert.Contains("element_type", typed.Message, StringComparison.Ordinal);
        Assert.Contains("t.object.element", element.Message, StringComparison.Ordinal);
        Assert.Contains("t.verdict.assertion", assertion.Message, StringComparison.Ordinal);
        Assert.Contains("infinity", infinity.Message, StringComparison.Ordinal);
        Assert.Contains("within int64", digits.Message, StringComparison.Ordinal);
        Assert.Contains("double", rational.Message, StringComparison.Ordinal);
        Assert.Contains("t.event.time.quantity", magnitude.Message, StringComparison.Ordinal);
    }

    [Fact]
    public void EveryArmIsEqualToItselfReadAgainAndToNoOtherSample()
    {
        var samples = CompleteExecutionWire.DocumentValues.Concat(CompleteExecutionWire.SparseDocumentValues).ToList();
        for (var i = 0; i < samples.Count; i++)
        {
            var once = Read(samples[i]);
            var again = Read(samples[i]);

            Assert.Equal(once, again);
            Assert.Equal(once.GetHashCode(), again.GetHashCode());
            Assert.True(once == again);
            for (var j = 0; j < samples.Count; j++)
            {
                if (i != j)
                {
                    Assert.NotEqual(once, Read(samples[j]));
                }
            }
        }
    }

    [Fact]
    public void ARecordNestedPastTheBoundIsMalformedRatherThanExhaustingTheStack()
    {
        var value = new Wire.DocumentValue { IntValue = 1 };
        for (var i = 0; i < ValueWire.MaxDepth + 1; i++)
        {
            value = new Wire.DocumentValue { Event = new Wire.DocumentEvent { Time = value } };
        }

        var deep = Malformed(value);

        Assert.Contains($"deeper than {ValueWire.MaxDepth}", deep.Message, StringComparison.Ordinal);
    }

    [Fact]
    public void ValuesCompareByContentAndReadAsWritten()
    {
        var element = new DocumentValue.ElementValue("Garage::car", "PartUsage");

        Assert.Equal(element, new DocumentValue.ElementValue("Garage::car", "PartUsage"));
        Assert.Equal(element.GetHashCode(), new DocumentValue.ElementValue("Garage::car", "PartUsage").GetHashCode());
        Assert.NotEqual<DocumentValue>(element, new DocumentValue.ElementValue(null, "PartUsage"));
        Assert.NotEqual<DocumentValue>(new DocumentValue.NumberValue(new Value.IntegerValue(1)), new DocumentValue.NumberValue(new Value.RealValue(1)));
        Assert.NotEqual<DocumentValue>(new DocumentValue.TextValue("1"), new DocumentValue.NumberValue(new Value.IntegerValue(1)));
        Assert.True(new DocumentValue.InfinityValue() == new DocumentValue.InfinityValue());
        Assert.True(new DocumentValue.BooleanValue(true) != new DocumentValue.BooleanValue(false));
        Assert.False(element.Equals((object)"Garage::car"));
        Assert.Equal("Garage::car (PartUsage)", element.ToString());
        Assert.Equal("(PartUsage)", new DocumentValue.ElementValue(null, "PartUsage").ToString());
        Assert.Equal("(no element)", new DocumentValue.ElementValue(null, string.Empty).ToString());
        Assert.Equal("Garage::car", new DocumentValue.ElementValue("Garage::car", string.Empty).ToString());
        Assert.Equal("#4", new DocumentValue.ObjectValue(new DocumentObject(4, string.Empty, null)).ToString());
        Assert.Equal("*", new DocumentValue.InfinityValue().ToString());
        Assert.Equal("false", new DocumentValue.BooleanValue(false).ToString());
        Assert.Equal("t", new DocumentValue.TextValue("t").ToString());
        Assert.Equal("4.0", new DocumentValue.NumberValue(new Value.RealValue(4)).ToString());
        Assert.Equal("assert constraint massKnown on Garage::car: violated", Read(new Wire.DocumentValue { Verdict = CompleteExecutionWire.Verdict }).ToString());
        Assert.Equal("Garage::car.lp in on.dim", Read(new Wire.DocumentValue { State = CompleteExecutionWire.State }).ToString());
        Assert.Equal("send Switch to #3", Read(new Wire.DocumentValue { Event = CompleteExecutionWire.Event }).ToString());
        Assert.Throws<ArgumentNullException>(() => new DocumentValue.ElementValue("x", null!));
        Assert.Throws<ArgumentNullException>(() => new DocumentValue.NumberValue(null!));
        Assert.Throws<ArgumentNullException>(() => new DocumentObject(1, null!, null));
        Assert.Equal("assign count := 3", Read(new Wire.DocumentValue { Event = CompleteExecutionWire.Assign }).ToString());
        Assert.Throws<ArgumentNullException>(() => new DocumentEvent("assign", null, null, string.Empty, string.Empty, string.Empty, string.Empty, null, string.Empty, [], [], string.Empty, "t", 1, 1, null!, null));
    }

    private static DocumentValue Read(Wire.DocumentValue value) => DocumentWire.Value(value, Method, "t");

    private static TransportException Malformed(Wire.DocumentValue value) => Assert.Throws<TransportException>(() => DocumentWire.Value(value, Method, "t"));
}
