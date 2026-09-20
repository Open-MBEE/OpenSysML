package org.openmbee.opensysml.syson;

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
import org.openmbee.opensysml.syson.export.ExportedProject;
import org.openmbee.opensysml.syson.export.ProjectExporter;
import org.openmbee.opensysml.syson.identity.ElementIndex;
import org.openmbee.opensysml.syson.run.RunOperation;
import org.openmbee.opensysml.syson.run.RunResult;
import org.openmbee.opensysml.syson.run.RunResultStore;
import org.openmbee.opensysml.syson.run.RunWithOpenSysMLInput;
import org.openmbee.opensysml.syson.run.RunWithOpenSysMLService;
import org.openmbee.opensysml.syson.export.ExportedProject.DocumentRange;
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
                        Map.of("x", "21"), List.of(), List.of(), "declared", null));
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
                null, null);
    }
}
