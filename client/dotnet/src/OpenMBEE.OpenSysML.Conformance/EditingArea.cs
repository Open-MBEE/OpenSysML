using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Editing a model through <see cref="Model.ApplyEditsAsync"/>, each operation read as the <see cref="Edit"/> of its
/// arm. A batch the service answered but refused reaches the caller as an <see cref="EditException"/> keeping its kind,
/// diagnostics and referrers, which are rendered back in band.
/// </summary>
internal static class EditingArea
{
    /// <summary>The area's adapters.</summary>
    internal static IEnumerable<RpcAdapter> Adapters { get; } =
    [
        RpcAdapter.Of<Wire.ApplyEditsRequest>("ApplyEdits", ["model_hash", "operations", "document", "accept_documents"], ApplyEditsAsync),
    ];

    private static async Task<ScenarioAnswer> ApplyEditsAsync(Connection connection, Wire.ApplyEditsRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var edits = request.Operations.Select((operation, i) => Edit(operation, $"operations[{i}]")).ToList();
        var options = new EditOptions { InDocument = Named(request.Document), AcceptDocuments = request.AcceptDocuments };
        try
        {
            return new ScenarioAnswer.Answered(Rendering.EditResult(await model.ApplyEditsAsync(edits, options, cancellationToken).ConfigureAwait(false)));
        }
        catch (EditException failure)
        {
            return new ScenarioAnswer.Answered(Rendering.EditResult(failure));
        }
    }

    private static Edit Edit(Wire.EditOperation operation, string path)
    {
        try
        {
            return operation.OperationCase switch
            {
                Wire.EditOperation.OperationOneofCase.SetValue => new Edit.SetValue(operation.SetValue.Target, operation.SetValue.Value),
                Wire.EditOperation.OperationOneofCase.Rename => new Edit.Rename(operation.Rename.Target, operation.Rename.NewName),
                Wire.EditOperation.OperationOneofCase.AddMember => Member(operation.AddMember),
                Wire.EditOperation.OperationOneofCase.Delete => new Edit.Delete(operation.Delete.Target, operation.Delete.Cascade),
                Wire.EditOperation.OperationOneofCase.Move => new Edit.Move(operation.Move.Target, operation.Move.Owner),
                Wire.EditOperation.OperationOneofCase.AddConnection => new Edit.AddConnection(
                    operation.AddConnection.Owner, operation.AddConnection.Kind, operation.AddConnection.FromEnd, operation.AddConnection.ToEnd)
                {
                    Name = Named(operation.AddConnection.Name),
                    Type = Named(operation.AddConnection.Type),
                },
                Wire.EditOperation.OperationOneofCase.AddSatisfy => new Edit.AddSatisfy(operation.AddSatisfy.Owner, operation.AddSatisfy.Requirement)
                {
                    SatisfyingFeature = Named(operation.AddSatisfy.SatisfyingFeature),
                    IsAsserted = operation.AddSatisfy.IsAsserted,
                    IsNegated = operation.AddSatisfy.IsNegated,
                },
                Wire.EditOperation.OperationOneofCase.AddRequirementConstraint => new Edit.AddRequirementConstraint(
                    operation.AddRequirementConstraint.Owner, operation.AddRequirementConstraint.Kind, operation.AddRequirementConstraint.Expression)
                {
                    Name = Named(operation.AddRequirementConstraint.Name),
                },
                Wire.EditOperation.OperationOneofCase.AddTransition => Transition(operation.AddTransition),
                Wire.EditOperation.OperationOneofCase.AddImport => new Edit.AddImport(operation.AddImport.Owner, operation.AddImport.Target)
                {
                    Visibility = Named(operation.AddImport.Visibility),
                    IsRecursive = operation.AddImport.IsRecursive,
                    IsImportAll = operation.AddImport.IsImportAll,
                    Filters = operation.AddImport.Filters,
                },
                Wire.EditOperation.OperationOneofCase.AddDocumentation => new Edit.AddDocumentation(operation.AddDocumentation.Target, operation.AddDocumentation.Body)
                {
                    Name = Named(operation.AddDocumentation.Name),
                    Locale = Named(operation.AddDocumentation.Locale),
                    Replace = operation.AddDocumentation.Replace,
                },
                Wire.EditOperation.OperationOneofCase.AddVerify => new Edit.AddVerify(operation.AddVerify.Owner, operation.AddVerify.Requirement),
                Wire.EditOperation.OperationOneofCase.AddMetadata => new Edit.AddMetadata(operation.AddMetadata.Owner, operation.AddMetadata.MetadataType)
                {
                    Name = Named(operation.AddMetadata.Name),
                    About = operation.AddMetadata.About,
                    Values = [.. operation.AddMetadata.Values.Select(value => new MetadataFeatureValue(value.Feature, value.Value))],
                    Shorthand = operation.AddMetadata.Shorthand,
                },
                Wire.EditOperation.OperationOneofCase.AddSequence => Sequence(operation.AddSequence),
                Wire.EditOperation.OperationOneofCase.AddMetadataPrefix => new Edit.AddMetadataPrefix(operation.AddMetadataPrefix.Target, operation.AddMetadataPrefix.MetadataType),
                Wire.EditOperation.OperationOneofCase.AddComment => new Edit.AddComment(operation.AddComment.Owner, operation.AddComment.Body)
                {
                    Name = Named(operation.AddComment.Name),
                    About = operation.AddComment.About,
                    Locale = Named(operation.AddComment.Locale),
                },
                Wire.EditOperation.OperationOneofCase.AddNote => new Edit.AddNote(operation.AddNote.Target, operation.AddNote.Text),
                _ => throw new UnexpressibleRequestException($"the public API names an edit's form, which {path} does not"),
            };
        }
        catch (ArgumentException invalid)
        {
            var reason = invalid.ParamName is { } name ? invalid.Message.Replace($" (Parameter '{name}')", string.Empty, StringComparison.Ordinal) : invalid.Message;
            throw new UnexpressibleRequestException($"the public API cannot hold {path} as spelled: {reason}");
        }
    }

    private static Edit.AddMember Member(Wire.AddMemberEdit add) => new(add.Owner, add.Kind, add.Name)
    {
        Type = Named(add.Type),
        Multiplicity = Named(add.Multiplicity),
        Value = Named(add.Value),
        Specializes = add.Specializes,
        IsAbstract = add.IsAbstract,
        Redefines = add.Redefines,
        IsDefault = add.IsDefault,
        Direction = Named(add.Direction),
        MetadataPrefixes = add.MetadataPrefixes,
        Doc = Named(add.Doc),
        BodyExpression = Named(add.BodyExpression),
    };

    private static Edit.AddTransition Transition(Wire.AddTransitionEdit add) => new(add.Owner, add.Source, add.Target)
    {
        Name = Named(add.Name),
        Trigger = Named(add.Trigger),
        Guard = Named(add.Guard),
        Effect = Named(add.Effect),
        Initial = add.Initial,
    };

    private static Edit.AddSequence Sequence(Wire.AddSequenceEdit add) => new(add.Owner, add.Keyword)
    {
        Ref = Named(add.Ref),
        MemberKind = Named(add.MemberKind),
        MemberName = Named(add.MemberName),
        Type = Named(add.Type),
        After = Named(add.After),
        Condition = Named(add.Condition),
        Value = Named(add.Value),
        Target = Named(add.Target),
        Via = Named(add.Via),
        Until = Named(add.Until),
        Body = [.. add.Body.Select(Sequence)],
        ElseBody = [.. add.ElseBody.Select(Sequence)],
        Multiplicity = Named(add.Multiplicity),
        Parameter = Named(add.Parameter),
    };

    // A field the request leaves empty is an optional part the API leaves null.
    private static string? Named(string value) => value.Length > 0 ? value : null;
}
