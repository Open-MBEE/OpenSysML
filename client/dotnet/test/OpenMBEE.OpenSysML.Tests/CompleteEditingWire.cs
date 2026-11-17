using System.Collections.Generic;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// A batch of edits of every arm with every option set, the request it is sent as, and answers that set every field an
/// ApplyEdits response can reach: of a single-document model, of a model of several, of a service predating documents,
/// and a refusal.
/// </summary>
internal static class CompleteEditingWire
{
    /// <summary>The model the samples edit.</summary>
    internal const string Hash = "h";

    /// <summary>The options of the batch, naming a document and not accepting documents, the opposite of the defaults.</summary>
    internal static EditOptions Options => new() { InDocument = "b.sysml", AcceptDocuments = false };

    /// <summary>One edit of every arm, every optional part written, and a statement nesting two levels of bodies.</summary>
    internal static IReadOnlyList<Edit> Edits =>
    [
        new Edit.SetValue("Demo::sc::unitMass", "1050.0[SI::kg]"),
        new Edit.Rename("Demo::Car", "Automobile"),
        new Edit.AddMember("Demo", "part def", "Engine")
        {
            Type = "Motor",
            Multiplicity = "[1]",
            Value = "new()",
            Specializes = ["Base", "Other"],
            IsAbstract = true,
            Redefines = ["engine"],
            IsDefault = true,
            Direction = "in",
            MetadataPrefixes = ["Safety"],
            Doc = "The engine.",
            BodyExpression = "true",
        },
        new Edit.Delete("Demo::Old", cascade: true),
        new Edit.Move("Demo::Wheel", "Demo::Parts"),
        new Edit.AddConnection("Demo::System", "connection", "a", "b") { Name = "c1", Type = "Link" },
        new Edit.AddSatisfy("Demo::holder", "Demo::spec") { SatisfyingFeature = "owned", IsAsserted = true, IsNegated = true },
        new Edit.AddRequirementConstraint("Demo::spec", "require", "mass < 10") { Name = "light" },
        new Edit.AddTransition("Demo::S", "idle", "busy") { Name = "go", Trigger = "Start", Guard = "ready", Effect = "action warm", Initial = true },
        new Edit.AddImport("Demo", "Lib::*") { Visibility = "public", IsRecursive = true, IsImportAll = true, Filters = ["@Safety", "true"] },
        new Edit.AddDocumentation("Demo::Car", "A car.") { Name = "carDoc", Locale = "en_US", Replace = true },
        new Edit.AddVerify("Demo::test", "Demo::spec"),
        new Edit.AddMetadata("Demo", "Safety")
        {
            Name = "s1",
            About = ["Demo::Car", "Demo::Wheel"],
            Values = [new MetadataFeatureValue("isMandatory", "true"), new MetadataFeatureValue("level", "2")],
            Shorthand = true,
        },
        new Edit.AddSequence("Demo::A", "then")
        {
            Ref = "done",
            MemberKind = "if",
            MemberName = "branch",
            Type = "Check",
            After = "start",
            Condition = "x == 0",
            Value = "1",
            Target = "x",
            Via = "port",
            Until = "x > 3",
            Body =
            [
                new Edit.AddSequence(string.Empty, string.Empty)
                {
                    MemberKind = "if",
                    Condition = "y == 0",
                    Body = [Edit.AddSequence.Assign(string.Empty, "y", "1").WithoutThen()],
                    ElseBody = [Edit.AddSequence.Assign(string.Empty, "y", "2").WithoutThen()],
                },
            ],
            ElseBody = [Edit.AddSequence.Accept(string.Empty, "signal").WithoutThen() with { Type = "Signal", Via = "inbox" }],
            Multiplicity = "[1]",
            Parameter = "item",
        },
        new Edit.AddMetadataPrefix("Demo::Car", "Safety"),
        new Edit.AddComment("Demo", "Reviewed.") { Name = "review", About = ["Demo::Car"], Locale = "en_US" },
        new Edit.AddNote("Demo::Car", "checked by hand"),
    ];

    /// <summary>The request <see cref="Edits"/> under <see cref="Options"/> is sent as.</summary>
    internal static Wire.ApplyEditsRequest Request => new()
    {
        ModelHash = Hash,
        Document = "b.sysml",
        AcceptDocuments = false,
        Operations =
        {
            new Wire.EditOperation { SetValue = new Wire.SetValueEdit { Target = "Demo::sc::unitMass", Value = "1050.0[SI::kg]" } },
            new Wire.EditOperation { Rename = new Wire.RenameEdit { Target = "Demo::Car", NewName = "Automobile" } },
            new Wire.EditOperation
            {
                AddMember = new Wire.AddMemberEdit
                {
                    Owner = "Demo",
                    Kind = "part def",
                    Name = "Engine",
                    Type = "Motor",
                    Multiplicity = "[1]",
                    Value = "new()",
                    Specializes = { "Base", "Other" },
                    IsAbstract = true,
                    Redefines = { "engine" },
                    IsDefault = true,
                    Direction = "in",
                    MetadataPrefixes = { "Safety" },
                    Doc = "The engine.",
                    BodyExpression = "true",
                },
            },
            new Wire.EditOperation { Delete = new Wire.DeleteEdit { Target = "Demo::Old", Cascade = true } },
            new Wire.EditOperation { Move = new Wire.MoveEdit { Target = "Demo::Wheel", Owner = "Demo::Parts" } },
            new Wire.EditOperation
            {
                AddConnection = new Wire.AddConnectionEdit { Owner = "Demo::System", Kind = "connection", FromEnd = "a", ToEnd = "b", Name = "c1", Type = "Link" },
            },
            new Wire.EditOperation
            {
                AddSatisfy = new Wire.AddSatisfyEdit { Owner = "Demo::holder", Requirement = "Demo::spec", SatisfyingFeature = "owned", IsAsserted = true, IsNegated = true },
            },
            new Wire.EditOperation
            {
                AddRequirementConstraint = new Wire.AddRequirementConstraintEdit { Owner = "Demo::spec", Kind = "require", Expression = "mass < 10", Name = "light" },
            },
            new Wire.EditOperation
            {
                AddTransition = new Wire.AddTransitionEdit
                {
                    Owner = "Demo::S",
                    Name = "go",
                    Source = "idle",
                    Target = "busy",
                    Trigger = "Start",
                    Guard = "ready",
                    Effect = "action warm",
                    Initial = true,
                },
            },
            new Wire.EditOperation
            {
                AddImport = new Wire.AddImportEdit
                {
                    Owner = "Demo",
                    Visibility = "public",
                    Target = "Lib::*",
                    IsRecursive = true,
                    IsImportAll = true,
                    Filters = { "@Safety", "true" },
                },
            },
            new Wire.EditOperation
            {
                AddDocumentation = new Wire.AddDocumentationEdit { Target = "Demo::Car", Body = "A car.", Name = "carDoc", Locale = "en_US", Replace = true },
            },
            new Wire.EditOperation { AddVerify = new Wire.AddVerifyEdit { Owner = "Demo::test", Requirement = "Demo::spec" } },
            new Wire.EditOperation
            {
                AddMetadata = new Wire.AddMetadataEdit
                {
                    Owner = "Demo",
                    MetadataType = "Safety",
                    Name = "s1",
                    About = { "Demo::Car", "Demo::Wheel" },
                    Values =
                    {
                        new Wire.MetadataFeatureValue { Feature = "isMandatory", Value = "true" },
                        new Wire.MetadataFeatureValue { Feature = "level", Value = "2" },
                    },
                    Shorthand = true,
                },
            },
            new Wire.EditOperation
            {
                AddSequence = new Wire.AddSequenceEdit
                {
                    Owner = "Demo::A",
                    Keyword = "then",
                    Ref = "done",
                    MemberKind = "if",
                    MemberName = "branch",
                    Type = "Check",
                    After = "start",
                    Condition = "x == 0",
                    Value = "1",
                    Target = "x",
                    Via = "port",
                    Until = "x > 3",
                    Body =
                    {
                        new Wire.AddSequenceEdit
                        {
                            MemberKind = "if",
                            Condition = "y == 0",
                            Body = { new Wire.AddSequenceEdit { MemberKind = "assign", Target = "y", Value = "1" } },
                            ElseBody = { new Wire.AddSequenceEdit { MemberKind = "assign", Target = "y", Value = "2" } },
                        },
                    },
                    ElseBody = { new Wire.AddSequenceEdit { MemberKind = "accept", Parameter = "signal", Type = "Signal", Via = "inbox" } },
                    Multiplicity = "[1]",
                    Parameter = "item",
                },
            },
            new Wire.EditOperation { AddMetadataPrefix = new Wire.AddMetadataPrefixEdit { Target = "Demo::Car", MetadataType = "Safety" } },
            new Wire.EditOperation
            {
                AddComment = new Wire.AddCommentEdit { Owner = "Demo", Body = "Reviewed.", Name = "review", About = { "Demo::Car" }, Locale = "en_US" },
            },
            new Wire.EditOperation { AddNote = new Wire.AddNoteEdit { Target = "Demo::Car", Text = "checked by hand" } },
        },
    };

    /// <summary>The notation of a single-document model after one edit.</summary>
    internal const string Edited = "package Demo {\n    attribute unitMass = 1050.0[SI::kg];\n}\n";

    /// <summary>An edit of a single-document model, its notation in content and in its one document, with a diagnostic.</summary>
    internal static Wire.ApplyEditsResponse Single => new()
    {
        Content = Edited,
        Applied =
        {
            new Wire.AppliedEdit
            {
                OperationIndex = 0,
                Target = "Demo::unitMass",
                Offset = 41,
                Length = 14,
                OldText = "1000.0[SI::kg]",
                NewText = "1050.0[SI::kg]",
                Document = "a.sysml",
            },
            new Wire.AppliedEdit { OperationIndex = 1, Target = "Demo", Offset = 60, NewText = "// note\n", Document = "a.sysml" },
        },
        Diagnostics = { CompleteWire.Diagnostic },
        Documents = { new Wire.EditedDocument { Name = "a.sysml", Content = Edited } },
    };

    /// <summary>An edit of a model of several documents: no content, and each rewritten document with what changed in it.</summary>
    internal static Wire.ApplyEditsResponse Several => new()
    {
        Applied =
        {
            new Wire.AppliedEdit { OperationIndex = 0, Target = "EngineLibrary::Engine", Offset = 30, Length = 6, OldText = "Engine", NewText = "Motor", Document = "engine_library.sysml" },
            new Wire.AppliedEdit { OperationIndex = 0, Target = "EngineLibrary::Engine", Offset = 80, Length = 6, OldText = "Engine", NewText = "Motor", Document = "engine_user.sysml" },
        },
        Documents =
        {
            new Wire.EditedDocument { Name = "engine_library.sysml", Content = "package EngineLibrary { part def Motor; }\n" },
            new Wire.EditedDocument { Name = "engine_user.sysml", Content = "package EngineUser { part motor : EngineLibrary::Motor; }\n" },
        },
    };

    /// <summary>The same single-document edit from a service predating documents: content alone, no document named.</summary>
    internal static Wire.ApplyEditsResponse Bare => new()
    {
        Content = Edited,
        Applied = { new Wire.AppliedEdit { OperationIndex = 0, Target = "Demo::unitMass", Offset = 41, Length = 14, OldText = "1000.0[SI::kg]", NewText = "1050.0[SI::kg]" } },
    };

    /// <summary>A refused delete, naming its kind, a diagnostic and the declarations referring to its target.</summary>
    internal static Wire.ApplyEditsResponse Refusal => new()
    {
        Error = "EngineLibrary::Engine is referred to by EngineUser::Car::motor",
        Failure = Wire.EditFailure.DeleteReferenced,
        Diagnostics = { CompleteWire.Diagnostic },
        ReferringElements = { "EngineUser::Car::motor (engine_user.sysml)", "EngineLibrary::spare" },
        Referrers =
        {
            new Wire.Referrer { Name = "EngineLibrary::spare", Document = "engine_library.sysml" },
            new Wire.Referrer { Name = "EngineUser::Car::motor", Document = "engine_user.sysml" },
        },
    };

    /// <summary>Every answer, for coverage of the fields an ApplyEdits response reaches.</summary>
    internal static IEnumerable<Wire.ApplyEditsResponse> All => [Single, Several, Bare, Refusal];
}
