using System;
using System.Collections.Concurrent;
using System.IO;
using System.Linq;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Editing models against a real sysml-grpc over every client wire: every arm written, its notation as the service
/// writes it and the edited model reading back without errors; nested action bodies built from an
/// <see cref="Editor.Body"/>; a model of several documents edited as one, a rename and a cascade delete followed into
/// the other document, a document named, and refusals naming their referrers; every refusal kind the typed API reaches;
/// a missing model; the edited notation saved and loaded again; and a service predating documents, or withholding the
/// authoring capabilities, answered or refused as it advertises.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceEditingTests(ServiceFixture services) : IClassFixture<ServiceFixture>, IDisposable
{
    private const string Withheld = "edit_documents,authoring";

    private static readonly string[] EngineDocuments = ["engine_library.sysml", "engine_user.sysml"];

    private readonly string directory = Directory.CreateTempSubdirectory("opensysml-edit-").FullName;

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    public void Dispose() => Directory.Delete(directory, recursive: true);

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task EveryArmIsWrittenAsTheServiceWritesItAndTheEditedModelReadsBack(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        (string Fixture, Edit[] Edits, string[] Written)[] cases =
        [
            ("editable.sysml", [new Edit.SetValue("Demo::SC::unitMass", "1050.0[SI::kg]"), new Edit.Rename("Demo::SC::margin", "slack")], ["= 1050.0[SI::kg];", "attribute slack : ISQ::MassValue;"]),
            ("editable.sysml", [new Edit.AddMember("Demo", "part def", "AbstractVehicle") { IsAbstract = true }], ["abstract part def AbstractVehicle;"]),
            ("editable.sysml", [new Edit.Move("Demo::SC::margin", "Demo")], ["        attribute unitMass : ISQ::MassValue = 1000.0[SI::kg];\n    }\n", "    part sc : SC;\n    attribute margin : ISQ::MassValue;\n}\n"]),
            ("apply_edits_connection.sysml", [new Edit.AddConnection("Demo::System", "allocation", "a", "b") { Name = "alloc1" }], ["allocation alloc1 allocate a to b;"]),
            (
                "requirement_authoring.sysml",
                [
                    new Edit.AddSatisfy("Demo::holder", "Demo::spec") { SatisfyingFeature = "owned", IsAsserted = true },
                    new Edit.AddRequirementConstraint("Demo::spec", "require", "true") { Name = "isValid" },
                ],
                ["assert satisfy Demo::spec by owned;", "require constraint isValid { true }"]),
            (
                "apply_edits_transition.sysml",
                [
                    Edit.AddTransition.Entry("Demo::S", "idle"),
                    new Edit.AddTransition("Demo::S", "idle", "toasting") { Name = "idle_to_toasting", Trigger = "CycleStart", Guard = "true", Effect = "action cool" },
                ],
                ["entry; then idle;", "transition idle_to_toasting first idle accept CycleStart if true do action cool then toasting;"]),
            (
                "apply_edits_documentation.sysml",
                [new Edit.AddDocumentation("Demo::Bread", "Sliced bread."), new Edit.AddMember("Demo", "item def", "Toast") { Doc = "Browned bread.\nReady to eat." }],
                ["    item def Bread {\n        doc /* Sliced bread.*/\n    }\n", "    item def Toast {\n        doc /* Browned bread.\n         * Ready to eat.*/\n    }\n"]),
            (
                "apply_edits_comment.sysml",
                [
                    new Edit.AddComment("Demo", "Toasting.\n  Indented, with a trailing space ") { Name = "Why", About = ["Bread", "Demo::Toaster"], Locale = "en" },
                    new Edit.AddComment(string.Empty, "File note."),
                    new Edit.AddNote("Demo::Toaster::power", "DimensionOneValue"),
                ],
                ["    comment Why about Bread, Demo::Toaster locale \"en\" /* Toasting.\n     *   Indented, with a trailing space */\n}\n", "}\ncomment /* File note.*/\n", "        // DimensionOneValue\n        attribute power;\n"]),
            (
                "apply_edits_import.sysml",
                [new Edit.AddImport("ToasterDemo", "ScalarValues::*"), new Edit.AddImport("ToasterDemo", "ISQ::*") { IsImportAll = true }],
                ["    private import ScalarValues::*;\n", "    private import all ISQ::*;\n"]),
            (
                "verification_metadata_authoring.sysml",
                [
                    new Edit.AddVerify("ToasterDemo::TimelyToastTest", "timely"),
                    new Edit.AddMetadata("ToasterDemo::TimelyToastTest", "VerificationMethod") { Values = [new MetadataFeatureValue("kind", "VerificationMethodKind::test")] },
                    new Edit.AddMember("ToasterDemo", "part def", "Heater") { MetadataPrefixes = ["ToasterDemo::Safety"] },
                    new Edit.AddMetadataPrefix("ToasterDemo::TimelyToast", "ToasterDemo::Safety"),
                ],
                ["verify timely;", "metadata VerificationMethod { kind = VerificationMethodKind::test; }", "#ToasterDemo::Safety part def Heater;", "#ToasterDemo::Safety requirement def TimelyToast {"]),
            (
                "apply_edits_constraint_body.sysml",
                [
                    new Edit.AddMember("Demo", "constraint def", "C") { BodyExpression = "true" },
                    new Edit.AddMember("Demo", "assert not constraint", "notPositive") { BodyExpression = "x <= 0" },
                    new Edit.AddMember("Demo::Host", "exhibit state", "shown") { Type = "Machine" },
                    new Edit.AddMember("Demo::Machine", "entry action", "entryWork") { Type = "Work" },
                ],
                ["constraint def C { true }", "assert not constraint notPositive { x <= 0 }", "exhibit state shown : Machine;", "entry action entryWork : Work;"]),
            ("apply_edits_assert_result_expression.sysml", [new Edit.AddMember("Demo", "assert", "c"), new Edit.AddMember("Demo", "calc def", "Double") { BodyExpression = "3 * 2" }], ["assert c;", "calc def Double { 3 * 2 }"]),
        ];

        foreach (var (fixture, edits, written) in cases)
        {
            var model = (await connection.LoadAsync(Fixture(fixture), cancellationToken: Token)).RequireNoErrors();

            var result = await model.ApplyEditsAsync(edits, cancellationToken: Token);

            Assert.False(result.HasSeveralDocuments);
            Assert.All(written, fragment => Assert.Contains(fragment, result.Content!, StringComparison.Ordinal));
            Assert.Equal([Fixture(fixture)], result.Documents.Select(document => document.Name));
            Assert.Equal(result.Content, result.Documents[0].Content);
            Assert.All(result.Applied, applied => Assert.Equal(Fixture(fixture), applied.Document));
            Assert.Equal(Enumerable.Range(0, edits.Length), result.Applied.Select(applied => applied.OperationIndex).Distinct().Order());
            Assert.Empty((await connection.LoadContentAsync(result.Content!, cancellationToken: Token)).Errors);
        }

        var removed = await (await connection.LoadAsync(Fixture("editable.sysml"), cancellationToken: Token)).Edit().Delete("Demo::SC::margin").ApplyAsync(cancellationToken: Token);
        Assert.DoesNotContain("margin", removed.Content!, StringComparison.Ordinal);
        Assert.Equal("\n        attribute margin : ISQ::MassValue;\n", Assert.Single(removed.Applied).OldText);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ANestedActionBodyIsWrittenFromTheItemsABodyCollects(ClientWire wire)
    {
        const string Nested = "private import ScalarValues::*;\n\naction def A {\n    attribute x : Integer = 0;\n    first start;\n    then if x == 0 {\n        if x == 0 {\n            assign x := 1;\n        } else {\n            assign x := 2;\n        }\n    } else {\n        assign x := 3;\n    }\n    [1] then done;\n}\n";
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadAsync(Fixture("apply_edits_action_body.sysml"), cancellationToken: Token)).RequireNoErrors();
        var inner = new Editor.Body().AddIf("x == 0", new Editor.Body().AddAssign("x", "1"), new Editor.Body().AddAssign("x", "2"));

        var nested = await model.Edit()
            .AddFirst("A", "start")
            .AddIf("A", "x == 0", inner, new Editor.Body().AddAssign("x", "3"))
            .Add(Edit.AddSequence.Then("A", "done") with { Multiplicity = "[1]" })
            .ApplyAsync(cancellationToken: Token);
        var loops = await model.Edit()
            .AddFirst("A", "start")
            .AddWhile("A", "x < 3", new Editor.Body().AddAssign("x", "x + 1"))
            .AddLoop("A", new Editor.Body().AddAssign("x", "x - 1"), "x <= 0")
            .AddThen("A", "done")
            .ApplyAsync(cancellationToken: Token);
        var sequenced = await (await connection.LoadAsync(Fixture("apply_edits_sequence.sysml"), cancellationToken: Token)).Edit()
            .AddFirst("A", "start")
            .Add(Edit.AddSequence.ThenMember("A", "action", "b") with { After = "a" })
            .AddThen("A", "done")
            .ApplyAsync(cancellationToken: Token);

        Assert.Equal(Nested, nested.Content);
        Assert.Contains("    then while x < 3 {\n        assign x := x + 1;\n    }\n", loops.Content!, StringComparison.Ordinal);
        Assert.Contains("    then loop {\n        assign x := x - 1;\n    } until x <= 0;\n", loops.Content!, StringComparison.Ordinal);
        Assert.All(["then action b;", "first start;", "then done;"], fragment => Assert.Contains(fragment, sequenced.Content!, StringComparison.Ordinal));
        Assert.Empty((await connection.LoadContentAsync(loops.Content!, cancellationToken: Token)).Errors);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AModelOfSeveralDocumentsIsEditedAsOneAndARefusalNamesItsReferrers(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await EnginesAsync(connection);

        var renamed = await model.Edit().Rename("EngineLibrary::Engine", "Motor").ApplyAsync(cancellationToken: Token);
        var named = await model.ApplyEditsAsync([new Edit.Rename("EngineUser::Car", "Automobile")], new EditOptions { InDocument = "engine_user.sysml" }, Token);
        var cascaded = await model.ApplyEditsAsync([new Edit.Delete("EngineLibrary::Engine", cascade: true)], cancellationToken: Token);
        var refused = await Assert.ThrowsAsync<EditException>(() => model.ApplyEditsAsync([new Edit.Delete("EngineLibrary::Engine")], cancellationToken: Token));
        var elsewhere = await Assert.ThrowsAsync<EditException>(() => model.ApplyEditsAsync([new Edit.Rename("EngineUser::Car", "Automobile")], cancellationToken: Token));
        var content = await Assert.ThrowsAsync<ServiceException>(() => model.ApplyEditsAsync([new Edit.Rename("EngineLibrary::Engine", "Motor")], new EditOptions { AcceptDocuments = false }, Token));
        var unknown = await Assert.ThrowsAsync<ServiceException>(() => model.ApplyEditsAsync([new Edit.Rename("EngineLibrary::Engine", "Motor")], new EditOptions { InDocument = "nope.sysml" }, Token));

        Assert.Equal((null, true), (renamed.Content, renamed.HasSeveralDocuments));
        Assert.Equal(["engine_library.sysml", "engine_user.sysml"], renamed.Documents.Select(document => document.Name));
        Assert.Contains("part def Motor {", renamed.Documents[0].Content, StringComparison.Ordinal);
        Assert.Contains("part motor : Motor;", renamed.Documents[1].Content, StringComparison.Ordinal);
        Assert.Equal(["engine_library.sysml", "engine_user.sysml"], renamed.Applied.Select(applied => applied.Document));
        Assert.Equal(
            [new EditedDocument("engine_user.sysml", "package EngineUser {\n\tprivate import EngineLibrary::*;\n\tpart def Automobile {\n\t\tpart motor : Engine;\n\t}\n}\n")],
            named.Documents);
        Assert.Equal(
            [
                new EditedDocument("engine_library.sysml", "package EngineLibrary {\n\tprivate import ScalarValues::*;\n}\n"),
                new EditedDocument("engine_user.sysml", "package EngineUser {\n\tprivate import EngineLibrary::*;\n\tpart def Car {\n\t}\n}\n"),
            ],
            cascaded.Documents);
        Assert.Equal(EditFailure.DeleteReferenced, refused.Failure);
        Assert.Equal(["EngineUser::Car::motor (engine_user.sysml)"], refused.ReferringElements);
        Assert.Equal([new Referrer("EngineUser::Car::motor", "engine_user.sysml")], refused.Referrers);
        Assert.Equal(EditFailure.UnknownTarget, elsewhere.Failure);
        Assert.Contains("engine_user.sysml", elsewhere.Message, StringComparison.Ordinal);
        Assert.Equal(StatusCode.FailedPrecondition, content.StatusCode);
        Assert.Contains("accept_documents", content.ServiceMessage, StringComparison.Ordinal);
        Assert.Equal(StatusCode.InvalidArgument, unknown.StatusCode);
        var written = await renamed.Documents[1].SaveAsync(Path.Combine(directory, "engine_user.sysml"), cancellationToken: Token);
        Assert.Equal(renamed.Documents[1].Content, await File.ReadAllTextAsync(written, Token));
        await Assert.ThrowsAsync<InvalidOperationException>(() => renamed.SaveAsync(Path.Combine(directory, "model.sysml"), cancellationToken: Token));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task EveryRefusalTheTypedApiReachesIsAnEditExceptionOfItsKind(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadAsync(Fixture("editable.sysml"), cancellationToken: Token)).RequireNoErrors();

        async Task<EditException> RefusedAsync(params Edit[] edits) => await Assert.ThrowsAsync<EditException>(() => model.ApplyEditsAsync(edits, cancellationToken: Token));

        var none = await RefusedAsync();
        var unknown = await RefusedAsync(new Edit.SetValue("Demo::Nope::x", "1"));
        var invalid = await RefusedAsync(new Edit.SetValue("Demo::SC::unitMass", "1050.0[SI::kg"));
        var name = await RefusedAsync(new Edit.Rename("Demo::SC::margin", "1bad"));
        var taken = await RefusedAsync(new Edit.AddMember("Demo::SC", "attribute", "margin"));
        var owner = await RefusedAsync(new Edit.AddMember("Demo::Nope", "part", "p"));
        var overlap = await RefusedAsync(new Edit.SetValue("Demo::SC::unitMass", "1.0[SI::kg]"), new Edit.SetValue("Demo::SC::unitMass", "2.0[SI::kg]"));
        var inside = await RefusedAsync(new Edit.Move("Demo::SC", "Demo::SC"));
        var referenced = await RefusedAsync(new Edit.Delete("Demo::SC"));

        Assert.Equal(
            [
                EditFailure.NoOperations, EditFailure.UnknownTarget, EditFailure.InvalidValue, EditFailure.InvalidName, EditFailure.MemberNameTaken,
                EditFailure.OwnerUnknown, EditFailure.OverlappingEdits, EditFailure.OwnerInsideTarget, EditFailure.DeleteReferenced,
            ],
            new[] { none, unknown, invalid, name, taken, owner, overlap, inside, referenced }.Select(refusal => refusal.Failure));
        Assert.NotEmpty(invalid.Diagnostics);
        Assert.Equal([new Referrer("Demo::sc", Fixture("editable.sysml"))], referenced.Referrers);
        Assert.All(new[] { none, unknown, invalid }, refusal => Assert.Equal(Model.ApplyEditsMethod, refusal.Method));
        await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("0123456789abcdef").ApplyEditsAsync([new Edit.Rename("A", "B")], cancellationToken: Token));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheEditedNotationIsSavedWholeAndLoadsAsTheEditedModel(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var path = Path.Combine(directory, "editable.sysml");
        File.Copy(Fixture("editable.sysml"), path);
        var model = (await connection.LoadAsync(path, cancellationToken: Token)).RequireNoErrors();
        var result = await model.Edit().SetValue("Demo::SC::unitMass", "1050.0[SI::kg]").ApplyAsync(cancellationToken: Token);

        await Assert.ThrowsAsync<UnwritableException>(() => result.SaveAsync(path, cancellationToken: Token));
        var written = await result.SaveAsync(path, new SaveOptions { Overwrite = true }, Token);
        var edited = (await connection.LoadAsync(written, cancellationToken: Token)).RequireNoErrors();
        var again = await edited.Edit().SetValue("Demo::SC::unitMass", "1000.0[SI::kg]").ApplyAsync(cancellationToken: Token);

        Assert.Equal(result.Content, await File.ReadAllTextAsync(path, Token));
        Assert.Equal(await File.ReadAllTextAsync(Fixture("editable.sysml"), Token), again.Content);
        Assert.Equal(["editable.sysml"], Directory.GetFileSystemEntries(directory).Select(Path.GetFileName));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AServiceWithoutDocumentsOrAuthoringAnswersAsItAdvertisesAndIsAskedNothingItCannotDo(ClientWire wire)
    {
        var withheld = await services.ServiceAsync("connect", Withheld);
        using var counting = new CountingHandler();
        await using var connection = await Connection.OpenAsync(
            new ConnectionOptions { Address = withheld.Address, Protocol = wire.Protocol(), Encoding = wire.Encoding(), HttpMessageHandler = counting },
            Token);
        var model = (await connection.LoadAsync(Fixture("editable.sysml"), cancellationToken: Token)).RequireNoErrors();
        var several = await EnginesAsync(connection);

        var single = await model.Edit().SetValue("Demo::SC::unitMass", "1050.0[SI::kg]").ApplyAsync(cancellationToken: Token);
        var hashed = await connection.ModelByHash(model.Hash).ApplyEditsAsync([new Edit.SetValue("Demo::SC::unitMass", "1050.0[SI::kg]")], cancellationToken: Token);
        var refused = await Assert.ThrowsAsync<ServiceException>(() => several.ApplyEditsAsync([new Edit.Rename("EngineLibrary::Engine", "Motor")], cancellationToken: Token));
        var authoring = await Assert.ThrowsAsync<CapabilityException>(() => model.Edit().AddMember("Demo", "part", "p").ApplyAsync(cancellationToken: Token));
        var document = await Assert.ThrowsAsync<CapabilityException>(
            () => several.ApplyEditsAsync([new Edit.Rename("EngineUser::Car", "Automobile")], new EditOptions { InDocument = "engine_user.sysml" }, Token));

        Assert.Contains("1050.0[SI::kg]", single.Content!, StringComparison.Ordinal);
        Assert.Empty(single.Documents);
        Assert.Null(Assert.Single(single.Applied).Document);
        Assert.Equal(single, hashed);
        Assert.Equal(StatusCode.FailedPrecondition, refused.StatusCode);
        Assert.Contains(Capabilities.EditDocuments, refused.ServiceMessage, StringComparison.Ordinal);
        Assert.Equal((Capabilities.Authoring, Capabilities.EditDocuments), (authoring.Capability, document.Capability));
        Assert.Equal(["GetServerInfo", "ParseFile", "ParseSources", "ApplyEdits", "ApplyEdits", "ApplyEdits"], counting.Methods);
    }

    private static string Fixture(string name) => Repository.PathOf("conformance", "fixtures", name);

    // The two engine documents as one model, each named by its file name as the conformance suite names them.
    private static async Task<Model> EnginesAsync(Connection connection)
    {
        var documents = await Task.WhenAll(EngineDocuments.Select(async name =>
            SourceDocument.Inline(await File.ReadAllTextAsync(Fixture(name), Token), name)));
        return (await connection.ParseSourcesAsync(documents, cancellationToken: Token)).RequireNoErrors();
    }

    private async Task<Connection> OpenAsync(ClientWire wire)
    {
        var service = await services.ServiceAsync("connect");
        return await Connection.OpenAsync(wire.To(service.Address), Token);
    }

    /// <summary>A borrowed handler that records the method of every request it carries to the service.</summary>
    private sealed class CountingHandler() : DelegatingHandler(new SocketsHttpHandler())
    {
        private readonly ConcurrentQueue<string> methods = new();

        internal string[] Methods => [.. methods];

        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            methods.Enqueue(request.RequestUri!.AbsolutePath.Split('/')[^1]);
            return base.SendAsync(request, cancellationToken);
        }
    }
}
