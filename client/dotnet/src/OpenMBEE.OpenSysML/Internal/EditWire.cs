using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// Converts <see cref="Edit"/>s to the service's <c>EditOperation</c>s, judges the capabilities a batch needs as the
/// service judges them, and reads an <c>ApplyEditsResponse</c>.
/// </summary>
internal static class EditWire
{
    /// <summary>The capabilities a batch's edits can need, in the order the service checks them, after <c>apply_edits</c>.</summary>
    internal static readonly IReadOnlyList<string> GateOrder =
    [
        Capabilities.Authoring,
        Capabilities.ConnectionAuthoring,
        Capabilities.SatisfyAuthoring,
        Capabilities.RequirementConstraintAuthoring,
        Capabilities.TransitionAuthoring,
        Capabilities.VerificationObjectiveAuthoring,
        Capabilities.MetadataAuthoring,
        Capabilities.MetadataPrefixAuthoring,
        Capabilities.SequenceAuthoring,
        Capabilities.ActionBodyStatementAuthoring,
        Capabilities.ImportAuthoring,
        Capabilities.MemberModifiers,
        Capabilities.ImplicitParameters,
        Capabilities.ConstraintBodyAuthoring,
        Capabilities.StateActionAuthoring,
        Capabilities.DocumentationAuthoring,
        Capabilities.CommentAuthoring,
    ];

    private static readonly HashSet<string> AssertedConstraintKinds = new(["assert", "assert not", "assert constraint", "assert not constraint"], StringComparer.Ordinal);

    private static readonly HashSet<string> StateActionKinds = new(["exhibit state", "exhibit", "entry action", "do action", "exit action"], StringComparer.Ordinal);

    private static readonly HashSet<string> StatementKinds = new(["accept", "send", "assign", "if", "while", "loop", "for", "terminate"], StringComparer.Ordinal);

    /// <summary>Refuses an optional part set to empty text, which the service would read as absent, naming where it is.</summary>
    /// <exception cref="InvalidRequestException">An optional part is empty.</exception>
    internal static void Validate(IReadOnlyList<Edit> edits)
    {
        for (var i = 0; i < edits.Count; i++)
        {
            var at = $"edits[{i}]";
            switch (edits[i])
            {
                case Edit.AddMember member:
                    Optional(at, member, nameof(member.Type), member.Type);
                    Optional(at, member, nameof(member.Multiplicity), member.Multiplicity);
                    Optional(at, member, nameof(member.Value), member.Value);
                    Optional(at, member, nameof(member.Direction), member.Direction);
                    Optional(at, member, nameof(member.Doc), member.Doc);
                    Optional(at, member, nameof(member.BodyExpression), member.BodyExpression);
                    break;
                case Edit.AddConnection connection:
                    Optional(at, connection, nameof(connection.Name), connection.Name);
                    Optional(at, connection, nameof(connection.Type), connection.Type);
                    break;
                case Edit.AddSatisfy satisfy:
                    Optional(at, satisfy, nameof(satisfy.SatisfyingFeature), satisfy.SatisfyingFeature);
                    break;
                case Edit.AddRequirementConstraint constraint:
                    Optional(at, constraint, nameof(constraint.Name), constraint.Name);
                    break;
                case Edit.AddTransition transition:
                    Optional(at, transition, nameof(transition.Name), transition.Name);
                    Optional(at, transition, nameof(transition.Trigger), transition.Trigger);
                    Optional(at, transition, nameof(transition.Guard), transition.Guard);
                    Optional(at, transition, nameof(transition.Effect), transition.Effect);
                    break;
                case Edit.AddImport import:
                    Optional(at, import, nameof(import.Visibility), import.Visibility);
                    break;
                case Edit.AddDocumentation documentation:
                    Optional(at, documentation, nameof(documentation.Name), documentation.Name);
                    Optional(at, documentation, nameof(documentation.Locale), documentation.Locale);
                    break;
                case Edit.AddMetadata metadata:
                    Optional(at, metadata, nameof(metadata.Name), metadata.Name);
                    break;
                case Edit.AddComment comment:
                    Optional(at, comment, nameof(comment.Name), comment.Name);
                    Optional(at, comment, nameof(comment.Locale), comment.Locale);
                    break;
                case Edit.AddSequence sequence:
                    Sequence(at, sequence);
                    break;
            }
        }
    }

    /// <summary>The capabilities a batch needs beyond <c>apply_edits</c>, in the order the service checks them.</summary>
    internal static IEnumerable<string> Needed(IReadOnlyList<Edit> edits) => GateOrder.Where(capability => edits.Any(edit => Needs(edit, capability)));

    /// <summary>Writes an edit for a request.</summary>
    internal static Wire.EditOperation ToWire(Edit edit) => edit switch
    {
        Edit.SetValue set => new() { SetValue = new Wire.SetValueEdit { Target = set.Target, Value = set.Value } },
        Edit.Rename rename => new() { Rename = new Wire.RenameEdit { Target = rename.Target, NewName = rename.NewName } },
        Edit.AddMember member => new()
        {
            AddMember = new Wire.AddMemberEdit
            {
                Owner = member.Owner,
                Kind = member.Kind,
                Name = member.Name,
                Type = member.Type ?? string.Empty,
                Multiplicity = member.Multiplicity ?? string.Empty,
                Value = member.Value ?? string.Empty,
                Specializes = { member.Specializes },
                IsAbstract = member.IsAbstract,
                Redefines = { member.Redefines },
                IsDefault = member.IsDefault,
                Direction = member.Direction ?? string.Empty,
                MetadataPrefixes = { member.MetadataPrefixes },
                Doc = member.Doc ?? string.Empty,
                BodyExpression = member.BodyExpression ?? string.Empty,
            },
        },
        Edit.Delete delete => new() { Delete = new Wire.DeleteEdit { Target = delete.Target, Cascade = delete.Cascade } },
        Edit.Move move => new() { Move = new Wire.MoveEdit { Target = move.Target, Owner = move.Owner } },
        Edit.AddConnection connection => new()
        {
            AddConnection = new Wire.AddConnectionEdit
            {
                Owner = connection.Owner,
                Kind = connection.Kind,
                FromEnd = connection.FromEnd,
                ToEnd = connection.ToEnd,
                Name = connection.Name ?? string.Empty,
                Type = connection.Type ?? string.Empty,
            },
        },
        Edit.AddSatisfy satisfy => new()
        {
            AddSatisfy = new Wire.AddSatisfyEdit
            {
                Owner = satisfy.Owner,
                Requirement = satisfy.Requirement,
                SatisfyingFeature = satisfy.SatisfyingFeature ?? string.Empty,
                IsAsserted = satisfy.IsAsserted,
                IsNegated = satisfy.IsNegated,
            },
        },
        Edit.AddRequirementConstraint constraint => new()
        {
            AddRequirementConstraint = new Wire.AddRequirementConstraintEdit
            {
                Owner = constraint.Owner,
                Kind = constraint.Kind,
                Expression = constraint.Expression,
                Name = constraint.Name ?? string.Empty,
            },
        },
        Edit.AddTransition transition => new()
        {
            AddTransition = new Wire.AddTransitionEdit
            {
                Owner = transition.Owner,
                Name = transition.Name ?? string.Empty,
                Source = transition.Source,
                Target = transition.Target,
                Trigger = transition.Trigger ?? string.Empty,
                Guard = transition.Guard ?? string.Empty,
                Effect = transition.Effect ?? string.Empty,
                Initial = transition.Initial,
            },
        },
        Edit.AddImport import => new()
        {
            AddImport = new Wire.AddImportEdit
            {
                Owner = import.Owner,
                Visibility = import.Visibility ?? string.Empty,
                Target = import.Target,
                IsRecursive = import.IsRecursive,
                IsImportAll = import.IsImportAll,
                Filters = { import.Filters },
            },
        },
        Edit.AddDocumentation documentation => new()
        {
            AddDocumentation = new Wire.AddDocumentationEdit
            {
                Target = documentation.Target,
                Body = documentation.Body,
                Name = documentation.Name ?? string.Empty,
                Locale = documentation.Locale ?? string.Empty,
                Replace = documentation.Replace,
            },
        },
        Edit.AddVerify verify => new() { AddVerify = new Wire.AddVerifyEdit { Owner = verify.Owner, Requirement = verify.Requirement } },
        Edit.AddMetadata metadata => new()
        {
            AddMetadata = new Wire.AddMetadataEdit
            {
                Owner = metadata.Owner,
                MetadataType = metadata.MetadataType,
                Name = metadata.Name ?? string.Empty,
                About = { metadata.About },
                Values = { metadata.Values.Select(value => new Wire.MetadataFeatureValue { Feature = value.Feature, Value = value.Value }) },
                Shorthand = metadata.Shorthand,
            },
        },
        Edit.AddSequence sequence => new() { AddSequence = ToWire(sequence) },
        Edit.AddMetadataPrefix prefix => new() { AddMetadataPrefix = new Wire.AddMetadataPrefixEdit { Target = prefix.Target, MetadataType = prefix.MetadataType } },
        Edit.AddComment comment => new()
        {
            AddComment = new Wire.AddCommentEdit
            {
                Owner = comment.Owner,
                Body = comment.Body,
                Name = comment.Name ?? string.Empty,
                About = { comment.About },
                Locale = comment.Locale ?? string.Empty,
            },
        },
        Edit.AddNote note => new() { AddNote = new Wire.AddNoteEdit { Target = note.Target, Text = note.Text } },
        _ => throw new InvalidOperationException($"No wire form is written for a {edit.GetType().FullName}."),
    };

    /// <summary>
    /// Reads an answer to a batch of <paramref name="operations"/> edits: the refusal it reports, or the edited notation.
    /// A model of one document answers its notation in <c>content</c>, one of several in <c>documents</c> alone;
    /// <paramref name="documentCount"/> says which when the model's documents are known, and otherwise the answer does.
    /// </summary>
    /// <param name="response">The answer.</param>
    /// <param name="method">The method answered.</param>
    /// <param name="operations">How many edits the batch sent.</param>
    /// <param name="documentCount">How many documents the model has; null when the client does not know.</param>
    /// <param name="editsDocuments">Whether the service advertises <see cref="Capabilities.EditDocuments"/>, without which it edits a model of one document alone.</param>
    /// <exception cref="EditException">The answer reports a refusal.</exception>
    /// <exception cref="TransportException">The answer is malformed.</exception>
    internal static EditResult Result(Wire.ApplyEditsResponse response, string method, int operations, int? documentCount, bool editsDocuments)
    {
        var diagnostics = ModelWire.Diagnostics(response.Diagnostics);
        if (response.Error.Length > 0)
        {
            if (response.Content.Length > 0 || response.Documents.Count > 0 || response.Applied.Count > 0)
            {
                throw Malformed(method, "a refusal beside edited notation, which a refusal never carries");
            }

            throw new EditException(
                response.Error,
                method,
                diagnostics,
                (EditFailure)(int)response.Failure,
                response.ReferringElements,
                response.Referrers.Select(referrer => new Referrer(referrer.Name, referrer.Document)));
        }

        if (response.Failure != Wire.EditFailure.Unspecified)
        {
            throw Malformed(method, $"the refusal kind {response.Failure} without the refusal it names");
        }

        if (response.ReferringElements.Count > 0 || response.Referrers.Count > 0)
        {
            throw Malformed(method, "the declarations referring to a refused edit's target, though it refused nothing");
        }

        var names = DocumentNames(response.Documents, method);
        var applied = response.Applied.Select((edit, i) => Applied(edit, i, method, operations, names)).ToList();
        var single = documentCount is { } count ? count == 1 : !editsDocuments || response.Content.Length > 0;
        if (!single && response.Content.Length > 0)
        {
            throw Malformed(method, "notation in content for a model of several documents, whose notation is in documents alone");
        }

        if (single && response.Documents.Count > 0 && (response.Documents.Count > 1 || response.Documents[0].Content != response.Content))
        {
            throw Malformed(method, "documents other than the one document of a single-document model, whose notation content repeats");
        }

        return new EditResult(
            single ? response.Content : null,
            applied,
            response.Documents.Select(document => new EditedDocument(document.Name, document.Content)),
            diagnostics);
    }

    /// <summary>The names of the edited documents an answer carries, each given and none repeated.</summary>
    /// <exception cref="TransportException">A document is unnamed, or named twice.</exception>
    internal static HashSet<string> DocumentNames(IReadOnlyList<Wire.EditedDocument> documents, string method)
    {
        var names = new HashSet<string>(StringComparer.Ordinal);
        for (var i = 0; i < documents.Count; i++)
        {
            var name = documents[i].Name;
            if (name.Length == 0)
            {
                throw Malformed(method, $"documents[{i}] without its name, which every edited document is given");
            }

            if (!names.Add(name))
            {
                throw Malformed(method, $"documents[{i}] repeating the document \"{name}\"");
            }
        }

        return names;
    }

    private static AppliedEdit Applied(Wire.AppliedEdit edit, int i, string method, int operations, HashSet<string> documents)
    {
        if (edit.OperationIndex < 0 || edit.OperationIndex >= operations)
        {
            throw Malformed(method, $"applied[{i}] for edit {edit.OperationIndex}, which a batch of {operations} has none of");
        }

        if (edit.Offset < 0 || edit.Length < 0)
        {
            throw Malformed(method, $"applied[{i}] at offset {edit.Offset} for {edit.Length} bytes, which no document has");
        }

        if (edit.Document.Length > 0 && documents.Count > 0 && !documents.Contains(edit.Document))
        {
            throw Malformed(method, $"applied[{i}] in the document \"{edit.Document}\", which documents does not list");
        }

        return new AppliedEdit(edit.OperationIndex, edit.Target, edit.Offset, edit.Length, edit.OldText, edit.NewText, edit.Document.Length > 0 ? edit.Document : null);
    }

    private static Wire.AddSequenceEdit ToWire(Edit.AddSequence sequence) => new()
    {
        Owner = sequence.Owner,
        Keyword = sequence.Keyword,
        Ref = sequence.Ref ?? string.Empty,
        MemberKind = sequence.MemberKind ?? string.Empty,
        MemberName = sequence.MemberName ?? string.Empty,
        Type = sequence.Type ?? string.Empty,
        After = sequence.After ?? string.Empty,
        Condition = sequence.Condition ?? string.Empty,
        Value = sequence.Value ?? string.Empty,
        Target = sequence.Target ?? string.Empty,
        Via = sequence.Via ?? string.Empty,
        Until = sequence.Until ?? string.Empty,
        Body = { sequence.Body.Select(ToWire) },
        ElseBody = { sequence.ElseBody.Select(ToWire) },
        Multiplicity = sequence.Multiplicity ?? string.Empty,
        Parameter = sequence.Parameter ?? string.Empty,
    };

    // The predicates of the service's own checks, one per capability.
    private static bool Needs(Edit edit, string capability) => capability switch
    {
        Capabilities.Authoring => edit is not (Edit.SetValue or Edit.Rename),
        Capabilities.ConnectionAuthoring => edit is Edit.AddConnection,
        Capabilities.SatisfyAuthoring => edit is Edit.AddSatisfy,
        Capabilities.RequirementConstraintAuthoring => edit is Edit.AddRequirementConstraint,
        Capabilities.TransitionAuthoring => edit is Edit.AddTransition,
        Capabilities.VerificationObjectiveAuthoring => edit is Edit.AddVerify or Edit.AddMember { Kind: "objective", Name: "" },
        Capabilities.MetadataAuthoring => edit is Edit.AddMetadata || (edit is Edit.AddMember member && member.MetadataPrefixes.Count > 0),
        Capabilities.MetadataPrefixAuthoring => edit is Edit.AddMetadataPrefix,
        Capabilities.SequenceAuthoring => edit is Edit.AddSequence,
        Capabilities.ActionBodyStatementAuthoring => edit is Edit.AddSequence sequence && NeedsStatementAuthoring(sequence),
        Capabilities.ImportAuthoring => edit is Edit.AddImport,
        Capabilities.MemberModifiers => edit is Edit.AddMember member
            && (member.IsAbstract || member.Redefines.Count > 0 || member.IsDefault || member.Direction is not null || member.Kind is "ref" or "return"),
        Capabilities.ImplicitParameters => edit is Edit.AddMember { Kind: "" },
        Capabilities.ConstraintBodyAuthoring => edit is Edit.AddMember member && (member.BodyExpression is not null || AssertedConstraintKinds.Contains(member.Kind)),
        Capabilities.StateActionAuthoring => edit is Edit.AddMember member && StateActionKinds.Contains(member.Kind),
        Capabilities.DocumentationAuthoring => edit is Edit.AddDocumentation || (edit is Edit.AddMember member && member.Doc is not null),
        Capabilities.CommentAuthoring => edit is Edit.AddComment or Edit.AddNote,
        _ => false,
    };

    // An item writes more than first, then or a declared member; one nesting any item already writes a statement body,
    // so the service's walk of nested items never decides more than this.
    private static bool NeedsStatementAuthoring(Edit.AddSequence item) =>
        item.Condition is not null || item.Value is not null || item.Target is not null || item.Via is not null || item.Until is not null
        || item.Multiplicity is not null || item.Parameter is not null || item.Body.Count > 0 || item.ElseBody.Count > 0
        || item.Keyword is "if" or "else" || (item.Keyword.Length == 0 && item.MemberKind is not null)
        || (item.MemberKind is { } kind && StatementKinds.Contains(kind));

    private static void Sequence(string at, Edit.AddSequence item)
    {
        Optional(at, item, nameof(item.Ref), item.Ref);
        Optional(at, item, nameof(item.MemberKind), item.MemberKind);
        Optional(at, item, nameof(item.MemberName), item.MemberName);
        Optional(at, item, nameof(item.Type), item.Type);
        Optional(at, item, nameof(item.After), item.After);
        Optional(at, item, nameof(item.Condition), item.Condition);
        Optional(at, item, nameof(item.Value), item.Value);
        Optional(at, item, nameof(item.Target), item.Target);
        Optional(at, item, nameof(item.Via), item.Via);
        Optional(at, item, nameof(item.Until), item.Until);
        Optional(at, item, nameof(item.Multiplicity), item.Multiplicity);
        Optional(at, item, nameof(item.Parameter), item.Parameter);
        for (var i = 0; i < item.Body.Count; i++)
        {
            Sequence($"{at}.Body[{i}]", item.Body[i]);
        }

        for (var i = 0; i < item.ElseBody.Count; i++)
        {
            Sequence($"{at}.ElseBody[{i}]", item.ElseBody[i]);
        }
    }

    private static void Optional(string at, Edit edit, string property, string? value)
    {
        if (value is { Length: 0 })
        {
            throw new InvalidRequestException(
                $"The {edit.GetType().Name} at {at} sets {property} to empty text, which the service reads as none; leave it null to write none.");
        }
    }

    private static TransportException Malformed(string method, string what) =>
        new(StatusCode.Internal, $"{method} answered {what}.", method);
}
