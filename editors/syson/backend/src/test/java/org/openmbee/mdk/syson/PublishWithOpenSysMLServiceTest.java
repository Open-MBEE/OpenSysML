package org.openmbee.mdk.syson;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;

import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.eclipse.sirius.components.representations.MessageLevel;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.openmbee.mdk.syson.export.ExportedProject;
import org.openmbee.mdk.syson.identity.ElementIndex;
import org.openmbee.mdk.syson.publish.PublishResult;
import org.openmbee.mdk.syson.publish.PublishWithOpenSysMLInput;
import org.openmbee.mdk.syson.publish.PublishWithOpenSysMLService;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Conversion;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.repository.ApiException;
import org.openmbee.opensysml.repository.FakeSysmlApi;
import org.openmbee.opensysml.repository.Publisher;
import org.openmbee.opensysml.repository.SysmlApiClient;

/** Publications over real HTTP to an in-process API server, from a model the service is mocked to convert. */
class PublishWithOpenSysMLServiceTest {
    private static final String API_JSON = """
            [
              {"@type": "Package", "@id": "P", "declaredName": "P", "sysx:statedDefaults": true},
              {"@type": "PartDefinition", "@id": "A", "declaredName": "A", "owner": {"@id": "P"}}
            ]
            """;

    private FakeSysmlApi api;
    private Connection connection;
    private Model model;
    private IEMFEditingContext context;
    private ExportedProject exported;

    @BeforeEach
    void start() throws Exception {
        api = new FakeSysmlApi();
        api.project("p1", "Mission", "main");
        connection = mock(Connection.class);
        model = mock(Model.class);
        context = mock(IEMFEditingContext.class);
        when(connection.parseSources(any())).thenReturn(model);
        when(model.diagnostics()).thenReturn(List.of());
        when(model.convert("api-json"))
                .thenReturn(new Conversion(API_JSON, "sysml", "api-json", List.of(), false, "", List.of()));
        exported = new ExportedProject(List.of(SourceDocument.inline("p.sysml", "package P { part def A; }")),
                new ElementIndex(Map.of()), List.of(), List.of());
    }

    @AfterEach
    void stop() {
        api.close();
    }

    private PublishWithOpenSysMLService service(Map<String, String> environment) {
        OpenSysMLProperties properties = new OpenSysMLProperties();
        properties.getPublish().setUrl(api.url());
        return new PublishWithOpenSysMLService(connection, ignored -> exported, properties, environment,
                config -> new Publisher(new SysmlApiClient(config)));
    }

    private static PublishWithOpenSysMLInput input(String project, String projectName, boolean deleteMissing,
            String lastSeen, String token) {
        return new PublishWithOpenSysMLInput(UUID.randomUUID(), "ctx", project, projectName, "", "first", deleteMissing,
                lastSeen, token);
    }

    @Test
    void commitsTheModelToTheDefaultBranchWithoutExtensionProperties() {
        PublishResult result = service(Map.of()).publish(context, input("p1", "", false, null, ""));

        assertThat(result.ok()).isTrue();
        assertThat(result.server()).isEqualTo(api.url());
        assertThat(result.branch()).isEqualTo("main");
        assertThat(result.created()).isEqualTo(2);
        assertThat(result.commit()).isEqualTo(api.head("p1", "main"));
        assertThat(result.head()).isEqualTo(api.head("p1", "main"));
        assertThat(api.elements("p1", "main").keySet()).containsExactlyInAnyOrder("P", "A");
        assertThat(api.elements("p1", "main").get("P").has("sysx:statedDefaults")).isFalse();
        assertThat(result.summary()).contains("2 created, 0 updated, 0 deleted");
    }

    @Test
    void publishingAgainWritesNothing() {
        PublishWithOpenSysMLService service = service(Map.of());
        PublishResult first = service.publish(context, input("p1", "", false, null, ""));
        PublishResult second = service.publish(context, input("p1", "", false, first.head(), ""));

        assertThat(second.ok()).isTrue();
        assertThat(second.commit()).isNull();
        assertThat(second.head()).isEqualTo(first.head());
        assertThat(api.posted).hasSize(1);
    }

    @Test
    void createsAProjectWhenNoneIsNamed() {
        PublishResult result = service(Map.of()).publish(context, input("", "Rover", false, null, ""));

        assertThat(result.ok()).isTrue();
        assertThat(result.createdProject()).isTrue();
        assertThat(result.projectName()).isEqualTo("Rover");
        assertThat(api.elements(result.project(), result.branch())).hasSize(2);
    }

    @Test
    void refusesABranchThatMovedSinceItWasLastSeenAndPublishesOnceTheMoveIsAccepted() {
        PublishWithOpenSysMLService service = service(Map.of());
        PublishResult moved = service.publish(context, input("p1", "", false, "an-older-commit", ""));

        assertThat(moved.ok()).isFalse();
        assertThat(moved.branchMoved()).isTrue();
        assertThat(moved.summary()).contains("moved from commit an-older-commit");
        assertThat(api.posted).isEmpty();

        assertThat(service.publish(context, input("p1", "", false, null, "")).ok()).isTrue();
    }

    @Test
    void deletesWhatTheBranchHoldsBeyondTheModelOnlyWhenAsked() {
        PublishWithOpenSysMLService service = service(Map.of());
        service.publish(context, input("p1", "", false, null, ""));
        when(model.convert("api-json")).thenReturn(new Conversion(
                "[{\"@type\": \"Package\", \"@id\": \"P\", \"declaredName\": \"P\"}]", "sysml", "api-json", List.of(),
                false, "", List.of()));

        PublishResult kept = service.publish(context, input("p1", "", false, null, ""));
        assertThat(kept.leftInPlace()).isEqualTo(1);
        assertThat(api.elements("p1", "main")).containsKey("A");

        PublishResult deleted = service.publish(context, input("p1", "", true, null, ""));
        assertThat(deleted.deleted()).isEqualTo(1);
        assertThat(api.elements("p1", "main")).doesNotContainKey("A");
    }

    @Test
    void refusesAModelWithErrorsBeforeAnythingIsSent() {
        when(model.diagnostics()).thenReturn(List.of(
                new Diagnostic(Diagnostic.Severity.WARNING, "unused", "W1", Optional.empty()),
                new Diagnostic(Diagnostic.Severity.ERROR, "cannot resolve B", "E1", Optional.empty())));

        PublishResult result = service(Map.of()).publish(context, input("p1", "", false, null, ""));

        assertThat(result.ok()).isFalse();
        assertThat(result.branchMoved()).isFalse();
        assertThat(result.summary()).isEqualTo(
                "The model has 1 error(s) and was not published; the first: cannot resolve B");
        assertThat(result.diagnostics()).hasSize(2);
        verify(model, never()).convert(any());
        assertThat(api.requests).isEmpty();
    }

    @Test
    void refusesAnExportThatReportedAnError() {
        exported = new ExportedProject(exported.documents(), exported.index(),
                List.of(new ExportedProject.ExportMessage(MessageLevel.ERROR, "element x has no text")), List.of());

        PublishResult result = service(Map.of()).publish(context, input("p1", "", false, null, ""));

        assertThat(result.ok()).isFalse();
        assertThat(result.summary()).contains("element x has no text");
        assertThat(api.requests).isEmpty();
    }

    @Test
    void sendsTheConfiguredTokenUnlessThePublicationCarriesItsOwn() {
        api.requiredToken = "user-token";
        PublishWithOpenSysMLService service = service(Map.of("FLEXO_INTEROP_TOKEN", "server-token"));

        assertThatThrownBy(() -> service.publish(context, input("p1", "", false, null, "")))
                .isInstanceOf(ApiException.class);
        assertThat(api.authorizations).contains("Bearer server-token");

        assertThat(service.publish(context, input("p1", "", false, null, "user-token")).ok()).isTrue();
    }

    @Test
    void refusesPlainHttpToAnotherHostBeforeExporting() {
        OpenSysMLProperties properties = new OpenSysMLProperties();
        properties.getPublish().setUrl("http://flexo.example.org");
        PublishWithOpenSysMLService service = new PublishWithOpenSysMLService(connection, ignored -> exported,
                properties, Map.of(), config -> new Publisher(new SysmlApiClient(config)));

        assertThatThrownBy(() -> service.publish(context, input("p1", "", false, null, "secret")))
                .isInstanceOf(ApiException.class);
        verify(connection, never()).parseSources(any());
    }

    @Test
    void refusesAPublicationThatNamesNoProject() {
        assertThatThrownBy(() -> service(Map.of()).publish(context, input("", " ", false, null, "")))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("names a project id");
    }

    @Test
    void theTokenStaysOutOfTheInputsTextForm() {
        assertThat(input("p1", "", false, null, "secret").toString()).doesNotContain("secret").contains("p1");
    }
}
