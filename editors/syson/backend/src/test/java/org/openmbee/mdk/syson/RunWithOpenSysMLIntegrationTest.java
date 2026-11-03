package org.openmbee.mdk.syson;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;
import java.util.UUID;

import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.ConnectionOptions;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.mdk.syson.export.ExportedProject;
import org.openmbee.mdk.syson.export.ProjectExporter;
import org.openmbee.mdk.syson.identity.ElementIndex;
import org.openmbee.mdk.syson.export.ServiceLibraries;
import org.assertj.core.api.InstanceOfAssertFactories;
import org.openmbee.mdk.syson.run.RunDocumentForm;
import org.openmbee.mdk.syson.run.RunOperation;
import org.openmbee.mdk.syson.run.RunTableRow;
import org.openmbee.mdk.syson.run.RunResult;
import org.openmbee.mdk.syson.run.RunResultStore;
import org.openmbee.mdk.syson.run.RunWithOpenSysMLInput;
import org.openmbee.mdk.syson.run.RunWithOpenSysMLService;
import org.openmbee.mdk.syson.export.ExportedProject.DocumentRange;
import org.eclipse.syson.sysml.Element;

class RunWithOpenSysMLIntegrationTest {
    private static Connection connection;
    private static Path repository;

    @BeforeAll
    static void startService() {
        repository = Path.of("").toAbsolutePath();
        while (repository != null && !Files.exists(repository.resolve("go.mod"))) repository = repository.getParent();
        connection = Connection.open(ConnectionOptions.builder().binaryPath(ServiceBinary.required()).build());
    }

    @AfterAll
    static void stopService() {
        if (connection != null) connection.close();
    }

    @Test
    void instantiatesVehicleAndStoresResult() throws Exception {
        String source = Files.readString(repository.resolve("editors/syson/backend/src/test/resources/models/vehicle.sysml"));
        Element target = target("Vehicle::Car");
        ExportedProject project = project("vehicle.sysml", source, target);
        RunResult result = service(project).run(context(), target, input(RunOperation.INSTANTIATE, Map.of()));
        assertThat(result.ok()).isTrue();
        assertThat(result.instances()).isNotEmpty();
        assertThat(result.instances().get(0).typeSiriusId()).isNotNull();
        assertThat(result.modelHash()).isEqualTo(connection.parseSources(List.of(SourceDocument.inline("vehicle.sysml", source))).hash());
    }

    @Test
    void executesCounterWithDeclaredSchedule() throws Exception {
        String source = "action def Counter { in x : Integer; out y : Integer; first start; "
                + "action compute { assign y := x * 2; } succession first start then compute; "
                + "succession first compute then done; }";
        Element target = target("Counter");
        RunResult result = service(project("counter.sysml", source, target)).run(context(), target,
                new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx", target.getElementId(), RunOperation.EXECUTE_ACTION,
                        Map.of("x", "21"), List.of(), List.of(), "declared", null, null));
        assertThat(result.ok()).isTrue();
        assertThat(result.schedule()).isEqualTo("declared");
        assertThat(result.outputs()).anyMatch(value -> value.name().equals("y") && value.value().contains("42"));
    }

    @Test
    void mapsBrokenModelDiagnosticsToTarget() throws Exception {
        String source = Files.readString(repository.resolve("editors/syson/backend/src/test/resources/models/broken.sysml"));
        Element target = target("Broken::A");
        RunResult result = service(project("broken.sysml", source, target)).run(context(), target,
                input(RunOperation.INSTANTIATE, Map.of()));
        assertThat(result.diagnostics()).isNotEmpty();
        assertThat(result.diagnostics()).anyMatch(diagnostic -> "Broken::A".equals(diagnostic.qualifiedName()));
    }

    @Test
    void mapsNestedSyntaxErrorToTheInnermostElement() throws Exception {
        String source = "package P {\n\taction def A {\n\t\tfirst start then action x;\n\t}\n}";
        Element root = target("P");
        Element action = target("P::A");
        ElementIndex index = new ElementIndex(Map.of(
                "P", new ElementIndex.IndexedElement("P", root.getElementId(), root.getElementId(), root),
                "P::A", new ElementIndex.IndexedElement("P::A", action.getElementId(), action.getElementId(),
                        action)));
        ExportedProject project = new ExportedProject(List.of(SourceDocument.inline("nested.sysml", source)),
                index, List.of(), List.of(
                        new DocumentRange("nested.sysml", 1, 5, root),
                        new DocumentRange("nested.sysml", 2, 4, action)));
        RunResult result = service(project).run(context(), action, input(RunOperation.INSTANTIATE, Map.of()));
        assertThat(result.diagnostics()).isNotEmpty();
        assertThat(result.diagnostics()).anyMatch(diagnostic -> "P::A".equals(diagnostic.qualifiedName()));
    }

    @Test
    void verifiesHoldingAndViolatedRequirements() throws Exception {
        String source = Files.readString(repository.resolve("editors/syson/backend/src/test/resources/models/req.sysml"));
        Element holding = target("Holding");
        Element violated = target("Violated");
        RunResult holdingResult = service(project("req.sysml", source, holding)).run(context(), holding,
                input(RunOperation.VERIFY_REQUIREMENT, Map.of()));
        RunResult violatedResult = service(project("req.sysml", source, violated)).run(context(), violated,
                input(RunOperation.VERIFY_REQUIREMENT, Map.of()));
        assertThat(holdingResult.verdict()).isEqualTo("holds");
        assertThat(violatedResult.verdict()).isEqualTo("violated");
    }

    @Test
    void exploresChoiceActionIntoMultipleOutcomes() throws Exception {
        String source = Files.readString(repository.resolve("editors/syson/backend/src/test/resources/models/choice.sysml"));
        Element target = target("test::tally");
        RunResult result = service(project("choice.sysml", source, target)).run(context(), target,
                input(RunOperation.EXPLORE_ACTION, Map.of()));
        assertThat(result.outcomes()).hasSizeGreaterThanOrEqualTo(2);
    }

    @Test
    void rendersDocumentAsMarkdownAndHtml() throws Exception {
        String source = Files.readString(repository.resolve("editors/syson/backend/src/test/resources/models/report.sysml"));
        Element target = target("Observatory::MassReport");
        RunResult markdown = service(project("report.sysml", source, target)).run(context(), target,
                new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx", "target", RunOperation.RENDER_DOCUMENT, Map.of(),
                        List.of(), List.of(), null, null, null));
        assertThat(markdown.ok()).isTrue();
        assertThat(markdown.document().form()).isEqualTo(RunDocumentForm.MARKDOWN);
        assertThat(markdown.document().fileName()).isEqualTo("Observatory-MassReport.md");
        assertThat(markdown.document().content()).startsWith("# Telescope Mass Report").contains("| mount | 15 |");
        RunResult html = service(project("report.sysml", source, target)).run(context(), target,
                new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx", "target", RunOperation.RENDER_DOCUMENT, Map.of(),
                        List.of(), List.of(), null, null, RunDocumentForm.HTML));
        assertThat(html.ok()).isTrue();
        assertThat(html.document().fileName()).isEqualTo("Observatory-MassReport.html");
        assertThat(html.document().content()).startsWith("<!DOCTYPE html>").contains("<title>Telescope Mass Report</title>");
    }

    @Test
    void refusesRenderingANonDocument() throws Exception {
        String source = Files.readString(repository.resolve("editors/syson/backend/src/test/resources/models/report.sysml"));
        Element target = target("Observatory::Subsystem");
        RunResult result = service(project("report.sysml", source, target)).run(context(), target,
                input(RunOperation.RENDER_DOCUMENT, Map.of()));
        assertThat(result.ok()).isFalse();
        assertThat(result.document()).isNull();
        assertThat(result.diagnostics()).anyMatch(diagnostic -> diagnostic.message().contains("is not a document"));
    }

    @Test
    void runsDocumentQueryWithElementAndLiteralBindings() throws Exception {
        String source = Files.readString(repository.resolve("editors/syson/backend/src/test/resources/models/report.sysml"));
        Element target = target("Observatory::HeavySubsystemNames");
        Map<String, String> bindings = new java.util.LinkedHashMap<>();
        bindings.put("root", "Observatory::telescope");
        bindings.put("threshold", "\"10\"");
        RunResult result = service(project("report.sysml", source, target)).run(context(), target,
                input(RunOperation.RUN_DOCUMENT_QUERY, bindings));
        assertThat(result.ok()).isTrue();
        assertThat(result.table().columns()).containsExactly("name");
        assertThat(result.table().rows()).extracting(row -> row.cells().get(0)).containsExactly("mount", "segmentControl");
        assertThat(result.table().rows()).extracting(RunTableRow::element)
                .containsExactly("Observatory::telescope::mount", "Observatory::telescope::segmentControl");
        assertThat(result.resultText()).isEqualTo("2 rows");
    }

    @Test
    void reportsAMistypedBindingAsADiagnostic() throws Exception {
        String source = Files.readString(repository.resolve("editors/syson/backend/src/test/resources/models/report.sysml"));
        Element target = target("Observatory::HeavySubsystemNames");
        Map<String, String> bindings = new java.util.LinkedHashMap<>();
        bindings.put("root", "Observatory::telescope");
        bindings.put("threshold", "10");
        RunResult result = service(project("report.sysml", source, target)).run(context(), target,
                input(RunOperation.RUN_DOCUMENT_QUERY, bindings));
        assertThat(result.ok()).isFalse();
        assertThat(result.table()).isNull();
        assertThat(result.diagnostics()).anyMatch(diagnostic -> diagnostic.message().contains("threshold"));
    }

    @Test
    void recognizesTheLibrariesTheServiceBundles() {
        ServiceLibraries libraries = new ServiceLibraries(connection);
        assertThat(libraries.declarations("DocumentQueries")).isPresent().get(InstanceOfAssertFactories.collection(String.class))
                .contains("Query", "Query::result", "Document", "Document::title", "Verdict::kind", "Paragraph", "Section", "Table");
        assertThat(libraries.declarations("ScalarValues")).isPresent();
        assertThat(libraries.declarations("Observatory")).isEmpty();
        assertThat(libraries.declarations("")).isEmpty();
    }

    @Test
    void namesDownloadsAsTheServiceLinksThem() throws Exception {
        String source = Files.readString(repository.resolve("editors/syson/backend/src/test/resources/models/linked.sysml"));
        Element report = target("Observatory::SystemReport");
        Element appendix = target("Observatory::'Mass Appendix'");
        RunResult html = service(project("linked.sysml", source, report)).run(context(), report,
                new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx", "target", RunOperation.RENDER_DOCUMENT, Map.of(),
                        List.of(), List.of(), null, null, RunDocumentForm.HTML));
        RunResult markdown = service(project("linked.sysml", source, appendix)).run(context(), appendix,
                input(RunOperation.RENDER_DOCUMENT, Map.of()));
        assertThat(html.ok()).isTrue();
        assertThat(markdown.ok()).isTrue();
        assertThat(html.document().fileName()).isEqualTo("Observatory-SystemReport.html");
        assertThat(markdown.document().fileName()).isEqualTo("Observatory-Mass.20Appendix.md");
        assertThat(html.document().content()).contains("href=\"" + markdown.document().fileName().replace(".md", ".html") + "\"");
        assertThat(markdown.document().content()).contains("](" + html.document().fileName().replace(".html", ".md") + ")");
    }

    private RunWithOpenSysMLService service(ExportedProject project) {
        ProjectExporter exporter = context -> project;
        return new RunWithOpenSysMLService(connection, exporter, new RunResultStore(), new OpenSysMLProperties());
    }

    private static ExportedProject project(String name, String source, Element target) {
        ElementIndex.IndexedElement indexed = new ElementIndex.IndexedElement(target.getQualifiedName(),
                target.getElementId(), target.getElementId(), target);
        return new ExportedProject(List.of(SourceDocument.inline(name, source)),
                new ElementIndex(Map.of(target.getQualifiedName(), indexed)), List.of(),
                List.of(new DocumentRange(name, 1, Integer.MAX_VALUE, target)));
    }

    private static Element target(String qualifiedName) {
        Element element = mock(Element.class);
        when(element.getQualifiedName()).thenReturn(qualifiedName);
        when(element.getElementId()).thenReturn("id-" + qualifiedName);
        when(element.getOwner()).thenReturn(null);
        when(element.isIsLibraryElement()).thenReturn(false);
        return element;
    }

    private static IEMFEditingContext context() {
        IEMFEditingContext context = mock(IEMFEditingContext.class);
        when(context.getId()).thenReturn("ctx");
        return context;
    }

    private static RunWithOpenSysMLInput input(RunOperation operation, Map<String, String> values) {
        return new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx", "target", operation, values, List.of(), List.of(),
                null, null, null);
    }
}
