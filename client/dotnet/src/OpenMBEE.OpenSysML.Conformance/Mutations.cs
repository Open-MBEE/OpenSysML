using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using Google.Protobuf.Reflection;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// A deliberate corruption of every answer, so a run shows the comparison is not vacuous: with one
/// applied, the scenarios asserting what it corrupts must fail.
/// </summary>
internal enum Mutation
{
    /// <summary>Leaves every answer alone.</summary>
    None,

    /// <summary>Moves every real by a millionth, far beyond the 1e-9 relative tolerance.</summary>
    PerturbReals,

    /// <summary>Drops the last element of every list, which exact list lengths must catch.</summary>
    TruncateLists,

    /// <summary>Rewrites every string, which every text and substring assertion must catch.</summary>
    RewriteStrings,
}

/// <summary>Applies a <see cref="Mutation"/> to a response, and names mutations as the command line does.</summary>
internal static class Mutations
{
    /// <summary>What <see cref="Mutation.RewriteStrings"/> makes of every string.</summary>
    internal const string Rewritten = "mutated";

    private static readonly (string Name, Mutation Mutation)[] Named =
    [
        ("perturb-reals", Mutation.PerturbReals),
        ("truncate-lists", Mutation.TruncateLists),
        ("rewrite-strings", Mutation.RewriteStrings),
    ];

    /// <summary>The names <c>-mutate</c> accepts, in the order the usage lists them.</summary>
    internal static IEnumerable<string> Names => Named.Select(named => named.Name);

    /// <summary>The mutation a name selects.</summary>
    /// <param name="name">The name, as <c>perturb-reals</c>.</param>
    /// <param name="mutation">The mutation.</param>
    /// <returns>Whether there is one of that name.</returns>
    internal static bool TryParse(string name, out Mutation mutation)
    {
        foreach (var named in Named)
        {
            if (named.Name == name)
            {
                mutation = named.Mutation;
                return true;
            }
        }

        mutation = Mutation.None;
        return false;
    }

    /// <summary>The name of a mutation, as <c>-mutate</c> takes it.</summary>
    /// <param name="mutation">A mutation other than <see cref="Mutation.None"/>.</param>
    /// <returns>Its name.</returns>
    internal static string Name(Mutation mutation) => Named.Single(named => named.Mutation == mutation).Name;

    /// <summary>A mutated copy of a message; the message itself is left alone.</summary>
    /// <param name="message">The message.</param>
    /// <param name="mutation">What to do to it.</param>
    /// <returns>The copy.</returns>
    internal static IMessage Apply(IMessage message, Mutation mutation)
    {
        ArgumentNullException.ThrowIfNull(message);
        var copy = message.Descriptor.Parser.ParseFrom(message.ToByteString());
        if (mutation != Mutation.None)
        {
            Mutate(copy, mutation);
        }

        return copy;
    }

    // Only fields the wire carries are touched, so no default becomes a value and no oneof changes arm.
    // A map's keys name its entries rather than state facts, so only its values are mutated.
    private static void Mutate(IMessage message, Mutation mutation)
    {
        foreach (var field in message.Descriptor.Fields.InFieldNumberOrder())
        {
            var value = field.Accessor.GetValue(message);
            if (!Normalizer.IsSet(field, message, value))
            {
                continue;
            }

            if (field.IsMap)
            {
                var map = (IDictionary)value;
                var valueField = field.MessageType.FindFieldByNumber(2);
                foreach (var key in new ArrayList(map.Keys))
                {
                    map[key] = Element(valueField, map[key]!, mutation);
                }
            }
            else if (field.IsRepeated)
            {
                var list = (IList)value;
                if (mutation == Mutation.TruncateLists)
                {
                    list.RemoveAt(list.Count - 1);
                }

                for (var i = 0; i < list.Count; i++)
                {
                    list[i] = Element(field, list[i]!, mutation);
                }
            }
            else if (field.FieldType == FieldType.Message)
            {
                Mutate((IMessage)value, mutation);
            }
            else
            {
                field.Accessor.SetValue(message, Scalar(field, value, mutation));
            }
        }
    }

    private static object Element(FieldDescriptor field, object value, Mutation mutation)
    {
        if (field.FieldType == FieldType.Message)
        {
            Mutate((IMessage)value, mutation);
            return value;
        }

        return Scalar(field, value, mutation);
    }

    private static object Scalar(FieldDescriptor field, object value, Mutation mutation) => (mutation, field.FieldType) switch
    {
        (Mutation.PerturbReals, FieldType.Double) => (double)value * 1.000001 + 1e-9,
        (Mutation.PerturbReals, FieldType.Float) => (float)value * 1.000001f + 1e-9f,
        (Mutation.RewriteStrings, FieldType.String) => Rewritten,
        _ => value,
    };
}

/// <summary>Answers through another client and corrupts every response it answers with by a <see cref="Mutation"/>.</summary>
/// <param name="inner">The client whose answers are corrupted.</param>
/// <param name="mutation">The corruption.</param>
internal sealed class MutatingScenarioClient(IScenarioClient inner, Mutation mutation) : IScenarioClient
{
    /// <inheritdoc/>
    public string Protocol => inner.Protocol;

    /// <inheritdoc/>
    public string Service => inner.Service;

    /// <inheritdoc/>
    public IReadOnlyCollection<string> Capabilities => inner.Capabilities;

    /// <inheritdoc/>
    public Task<ModelParse> ParseAsync(ScenarioModel model, IReadOnlyList<FixtureSource> documents, CancellationToken cancellationToken) =>
        inner.ParseAsync(model, documents, cancellationToken);

    /// <inheritdoc/>
    public async Task<ScenarioAnswer> CallAsync(MethodDescriptor method, IMessage request, CancellationToken cancellationToken)
    {
        var answer = await inner.CallAsync(method, request, cancellationToken).ConfigureAwait(false);
        return answer is ScenarioAnswer.Answered answered ? new ScenarioAnswer.Answered(Mutations.Apply(answered.Response, mutation)) : answer;
    }
}
