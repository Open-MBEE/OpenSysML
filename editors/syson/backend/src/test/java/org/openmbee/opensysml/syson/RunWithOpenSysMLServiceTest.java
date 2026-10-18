package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;
import java.util.Map;
import java.util.UUID;

import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.eclipse.sirius.components.representations.MessageLevel;
import org.eclipse.syson.sysml.Element;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Instantiation;
import org.openmbee.opensysml.Instance;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.Satisfaction;
import org.openmbee.opensysml.syson.export.ExportedProject;
import org.openmbee.opensysml.syson.export.ProjectTextExporter;
import org.openmbee.opensysml.syson.identity.ElementIndex;
import org.openmbee.opensysml.syson.run.RunOperation;
import org.openmbee.opensysml.syson.run.RunResult;
import org.openmbee.opensysml.syson.run.RunResultStore;
import org.openmbee.opensysml.syson.run.RunWithOpenSysMLInput;
import org.openmbee.opensysml.syson.run.RunWithOpenSysMLService;

class RunWithOpenSysMLServiceTest {
    private static final String VEHICLE = "Vehicle";
    private static final String VEHICLE_ID = "vehicle-id";
    @Test
    void doesNotEvaluateStaleArgumentsWhenInstantiating() {
        Connection connection = org.mockito.Mockito.mock(Connection.class);
        Model model = org.mockito.Mockito.mock(Model.class);
        Element target = org.mockito.Mockito.mock(Element.class);
        org.mockito.Mockito.when(target.getQualifiedName()).thenReturn(VEHICLE);
        org.mockito.Mockito.when(target.getElementId()).thenReturn(VEHICLE_ID);
        org.mockito.Mockito.when(connection.parseSources(org.mockito.ArgumentMatchers.any()))
                .thenReturn(model);
        org.mockito.Mockito.when(model.diagnostics()).thenReturn(List.of());
        org.mockito.Mockito.when(model.hash()).thenReturn("hash");
        Instance instance = org.mockito.Mockito.mock(Instance.class);
        org.mockito.Mockito.when(instance.id()).thenReturn(1L);
        org.mockito.Mockito.when(instance.typeSymbolId()).thenReturn(VEHICLE);
        org.mockito.Mockito.when(instance.featureValues()).thenReturn(Map.of());
        org.mockito.Mockito.when(model.instantiate(VEHICLE))
                .thenReturn(new Instantiation(instance, List.of(instance), List.of()));
        ExportedProject project = new ExportedProject(List.of(SourceDocument.inline("vehicle.sysml", "part def Vehicle;")),
                new ElementIndex(Map.of(VEHICLE, new ElementIndex.IndexedElement(VEHICLE, VEHICLE_ID,
                        "sirius-id", target))),
                List.of(), List.of());
        IEMFEditingContext context = org.mockito.Mockito.mock(IEMFEditingContext.class);
        org.mockito.Mockito.when(context.getId()).thenReturn("ctx");
        RunWithOpenSysMLService service = new RunWithOpenSysMLService(connection, ignored -> project,
                new RunResultStore(), new OpenSysMLProperties());

        service.run(context, target, new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx", VEHICLE_ID,
                RunOperation.INSTANTIATE, Map.of("stale", "1"), List.of(), List.of("2"), null, null, null));

        org.mockito.Mockito.verify(model, org.mockito.Mockito.never()).eval(org.mockito.ArgumentMatchers.anyString());
    }

    @Test
    void scopesSatisfactionToTargetWhenSubjectIsAbsent() {
        Connection connection = org.mockito.Mockito.mock(Connection.class);
        Model model = org.mockito.Mockito.mock(Model.class);
        Element target = org.mockito.Mockito.mock(Element.class);
        org.mockito.Mockito.when(target.getQualifiedName()).thenReturn(VEHICLE);
        org.mockito.Mockito.when(target.getElementId()).thenReturn(VEHICLE_ID);
        org.mockito.Mockito.when(connection.parseSources(org.mockito.ArgumentMatchers.any()))
                .thenReturn(model);
        org.mockito.Mockito.when(model.diagnostics()).thenReturn(List.of());
        org.mockito.Mockito.when(model.hash()).thenReturn("hash");
        org.mockito.Mockito.when(model.verifySatisfaction(VEHICLE))
                .thenReturn(new Satisfaction(List.of(), List.of(), List.of(), List.of()));
        ExportedProject project = new ExportedProject(List.of(SourceDocument.inline("vehicle.sysml", "part def Vehicle;")),
                new ElementIndex(Map.of(VEHICLE, new ElementIndex.IndexedElement(VEHICLE, VEHICLE_ID,
                        "sirius-id", target))),
                List.of(), List.of());
        IEMFEditingContext context = org.mockito.Mockito.mock(IEMFEditingContext.class);
        org.mockito.Mockito.when(context.getId()).thenReturn("ctx");
        RunWithOpenSysMLService service = new RunWithOpenSysMLService(connection, ignored -> project,
                new RunResultStore(), new OpenSysMLProperties());

        service.run(context, target, new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx", VEHICLE_ID,
                RunOperation.VERIFY_SATISFACTION, Map.of(), List.of(), List.of(), null, null, null));

        org.mockito.Mockito.verify(model).verifySatisfaction(VEHICLE);
        org.mockito.Mockito.verify(model, org.mockito.Mockito.never()).verifySatisfaction();
    }

    @Test
    void reportsMissingQualifiedNameWithoutCallingService() {
        var connection = org.mockito.Mockito.mock(org.openmbee.opensysml.Connection.class);
        var element = new FakeElement(null);
        var exporter = org.mockito.Mockito.mock(ProjectTextExporter.class);
        org.mockito.Mockito.when(exporter.export(org.mockito.Mockito.any())).thenReturn(
                new ExportedProject(List.of(), new ElementIndex(Map.of()), List.of(), List.of()));
        var context = org.mockito.Mockito.mock(IEMFEditingContext.class);
        org.mockito.Mockito.when(context.getId()).thenReturn("ctx");
        var service = new RunWithOpenSysMLService(connection, exporter, new RunResultStore(), new OpenSysMLProperties());
        var result = service.run(context, element, new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx", "id",
                RunOperation.INSTANTIATE, null, null, null, null, null, null));
        assertThat(result.diagnostics().get(0).message())
                .isEqualTo("selected element has no qualified name in the export");
        org.mockito.Mockito.verifyNoInteractions(connection);
    }

    @Test
    void handledFailureKeepsModelHashNonNull() {
        Connection connection = org.mockito.Mockito.mock(Connection.class);
        Element target = org.mockito.Mockito.mock(Element.class);
        org.mockito.Mockito.when(target.getQualifiedName()).thenReturn(VEHICLE);
        org.mockito.Mockito.when(target.getElementId()).thenReturn(VEHICLE_ID);
        org.mockito.Mockito.when(connection.parseSources(org.mockito.ArgumentMatchers.any()))
                .thenThrow(new IllegalArgumentException("unparsable"));
        ExportedProject project = new ExportedProject(List.of(SourceDocument.inline("vehicle.sysml", "part def ;")),
                new ElementIndex(Map.of(VEHICLE, new ElementIndex.IndexedElement(VEHICLE, VEHICLE_ID,
                        "sirius-id", target))),
                List.of(), List.of());
        IEMFEditingContext context = org.mockito.Mockito.mock(IEMFEditingContext.class);
        org.mockito.Mockito.when(context.getId()).thenReturn("ctx");
        RunWithOpenSysMLService service = new RunWithOpenSysMLService(connection, ignored -> project,
                new RunResultStore(), new OpenSysMLProperties());

        RunResult result = service.run(context, target, new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx",
                VEHICLE_ID, RunOperation.INSTANTIATE, Map.of(), List.of(), List.of(), null, null, null));

        assertThat(result.ok()).isFalse();
        assertThat(result.modelHash()).isEqualTo("");
        assertThat(result.diagnostics()).isNotEmpty();
    }

    @Test
    void attributesExportMessagesToTheElementTheyName() {
        String anonymousId = "8d21bfea-68e1-40f5-8207-065154a67033";
        FakeElement part = new FakeElement("Pkg::Part");
        ElementIndex.IndexedElement partIndexed = new ElementIndex.IndexedElement("Pkg::Part", "part-id",
                "sirius-part", part);
        Connection connection = org.mockito.Mockito.mock(Connection.class);
        Model model = org.mockito.Mockito.mock(Model.class);
        Element target = org.mockito.Mockito.mock(Element.class);
        org.mockito.Mockito.when(target.getQualifiedName()).thenReturn(VEHICLE);
        org.mockito.Mockito.when(target.getElementId()).thenReturn(VEHICLE_ID);
        org.mockito.Mockito.when(connection.parseSources(org.mockito.ArgumentMatchers.any())).thenReturn(model);
        org.mockito.Mockito.when(model.diagnostics()).thenReturn(List.of());
        org.mockito.Mockito.when(model.hash()).thenReturn("hash");
        Instance instance = org.mockito.Mockito.mock(Instance.class);
        org.mockito.Mockito.when(instance.id()).thenReturn(1L);
        org.mockito.Mockito.when(instance.typeSymbolId()).thenReturn(VEHICLE);
        org.mockito.Mockito.when(instance.featureValues()).thenReturn(Map.of());
        org.mockito.Mockito.when(model.instantiate(VEHICLE))
                .thenReturn(new Instantiation(instance, List.of(instance), List.of()));
        ExportedProject project = new ExportedProject(
                List.of(SourceDocument.inline("doc.sysml", "part def Other;")),
                new ElementIndex(Map.of(VEHICLE,
                        new ElementIndex.IndexedElement(VEHICLE, VEHICLE_ID, "sirius-id", target),
                        "Pkg::Part", partIndexed),
                        Map.of(), Map.of(anonymousId, partIndexed)),
                List.of(new ExportedProject.ExportMessage(MessageLevel.WARNING,
                        "Unable to export a SuccessionAsUsage (" + anonymousId
                                + ") with an implicit target and no following action")),
                List.of());
        IEMFEditingContext context = org.mockito.Mockito.mock(IEMFEditingContext.class);
        org.mockito.Mockito.when(context.getId()).thenReturn("ctx");
        RunWithOpenSysMLService service = new RunWithOpenSysMLService(connection, ignored -> project,
                new RunResultStore(), new OpenSysMLProperties());

        RunResult result = service.run(context, target, new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx",
                VEHICLE_ID, RunOperation.INSTANTIATE, Map.of(), List.of(), List.of(), null, null, null));

        assertThat(result.diagnostics()).anySatisfy(diagnostic -> {
            assertThat(diagnostic.qualifiedName()).isEqualTo("Pkg::Part");
            assertThat(diagnostic.elementId()).isEqualTo("part-id");
            assertThat(diagnostic.siriusId()).isEqualTo("sirius-part");
        });
    }

    @Test
    void runsSubjectScopedOperationsOnTheEnclosingElement() {
        FakeElement part = new FakeElement("Pkg::Part");
        ElementIndex.IndexedElement partIndexed = new ElementIndex.IndexedElement("Pkg::Part", "part-id",
                "sirius-part", part);
        FakeElement anonymous = new FakeElement(null).elementId("anonymous-id");
        Connection connection = org.mockito.Mockito.mock(Connection.class);
        Model model = org.mockito.Mockito.mock(Model.class);
        org.mockito.Mockito.when(connection.parseSources(org.mockito.ArgumentMatchers.any())).thenReturn(model);
        org.mockito.Mockito.when(model.diagnostics()).thenReturn(List.of());
        org.mockito.Mockito.when(model.hash()).thenReturn("hash");
        org.mockito.Mockito.when(model.verifySatisfaction("Pkg::Part"))
                .thenReturn(new Satisfaction(List.of(), List.of(), List.of(), List.of()));
        ExportedProject project = new ExportedProject(List.of(SourceDocument.inline("doc.sysml", "")),
                new ElementIndex(Map.of("Pkg::Part", partIndexed), Map.of(anonymous, partIndexed),
                        Map.of("anonymous-id", partIndexed)),
                List.of(), List.of());
        IEMFEditingContext context = org.mockito.Mockito.mock(IEMFEditingContext.class);
        org.mockito.Mockito.when(context.getId()).thenReturn("ctx");
        RunWithOpenSysMLService service = new RunWithOpenSysMLService(connection, ignored -> project,
                new RunResultStore(), new OpenSysMLProperties());

        RunResult result = service.run(context, anonymous, new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx",
                "anonymous-id", RunOperation.VERIFY_SATISFACTION, Map.of(), List.of(), List.of(), null, null, null));

        org.mockito.Mockito.verify(model).verifySatisfaction("Pkg::Part");
        assertThat(result.target()).isEqualTo("Pkg::Part");
    }

    @Test
    void namesTheEnclosingElementWhenAnAnonymousTargetCannotRun() {
        FakeElement part = new FakeElement("Pkg::Part");
        ElementIndex.IndexedElement partIndexed = new ElementIndex.IndexedElement("Pkg::Part", "part-id",
                "sirius-part", part);
        FakeElement anonymous = new FakeElement(null).elementId("anonymous-id");
        Connection connection = org.mockito.Mockito.mock(Connection.class);
        ExportedProject project = new ExportedProject(List.of(SourceDocument.inline("doc.sysml", "")),
                new ElementIndex(Map.of("Pkg::Part", partIndexed), Map.of(anonymous, partIndexed), Map.of()),
                List.of(), List.of());
        IEMFEditingContext context = org.mockito.Mockito.mock(IEMFEditingContext.class);
        org.mockito.Mockito.when(context.getId()).thenReturn("ctx");
        RunWithOpenSysMLService service = new RunWithOpenSysMLService(connection, ignored -> project,
                new RunResultStore(), new OpenSysMLProperties());

        RunResult result = service.run(context, anonymous, new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx",
                "anonymous-id", RunOperation.EXECUTE_ACTION, Map.of(), List.of(), List.of(), null, null, null));

        assertThat(result.ok()).isFalse();
        assertThat(result.diagnostics().get(0).message()).isEqualTo(
                "selected element has no qualified name in the export; select its enclosing element Pkg::Part");
        org.mockito.Mockito.verifyNoInteractions(connection);
    }

    @Test
    void rendersTheSelectedDocumentInTheRequestedForm() {
        Connection connection = org.mockito.Mockito.mock(Connection.class);
        Model model = org.mockito.Mockito.mock(Model.class);
        Element target = org.mockito.Mockito.mock(Element.class);
        org.mockito.Mockito.when(target.getQualifiedName()).thenReturn("Reports::'Mass Report'");
        org.mockito.Mockito.when(target.getElementId()).thenReturn("report-id");
        org.mockito.Mockito.when(connection.parseSources(org.mockito.ArgumentMatchers.any())).thenReturn(model);
        org.mockito.Mockito.when(model.diagnostics()).thenReturn(List.of());
        org.mockito.Mockito.when(model.hash()).thenReturn("hash");
        org.mockito.Mockito.when(model.renderDocument("Reports::'Mass Report'", org.openmbee.opensysml.DocumentForm.HTML))
                .thenReturn(new org.openmbee.opensysml.RenderedDocument(org.openmbee.opensysml.DocumentForm.HTML,
                        "<!DOCTYPE html><html></html>", "Reports-Mass.20Report.html"));
        ExportedProject project = new ExportedProject(List.of(SourceDocument.inline("report.sysml", "")),
                new ElementIndex(Map.of("Reports::'Mass Report'", new ElementIndex.IndexedElement(
                        "Reports::'Mass Report'", "report-id", "sirius-id", target))), List.of(), List.of());
        IEMFEditingContext context = org.mockito.Mockito.mock(IEMFEditingContext.class);
        org.mockito.Mockito.when(context.getId()).thenReturn("ctx");
        RunResultStore store = new RunResultStore();
        RunWithOpenSysMLService service = new RunWithOpenSysMLService(connection, ignored -> project, store,
                new OpenSysMLProperties());

        RunResult result = service.run(context, target, new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx",
                "report-id", RunOperation.RENDER_DOCUMENT, Map.of(), List.of(), List.of(), null, null,
                org.openmbee.opensysml.syson.run.RunDocumentForm.HTML));

        assertThat(result.ok()).isTrue();
        assertThat(result.document().form()).isEqualTo(org.openmbee.opensysml.syson.run.RunDocumentForm.HTML);
        assertThat(result.document().content()).startsWith("<!DOCTYPE html>");
        assertThat(result.document().fileName()).isEqualTo("Reports-Mass.20Report.html");
        assertThat(result.table()).isNull();
        assertThat(store.latest("ctx")).contains(result);
        org.mockito.Mockito.verify(model, org.mockito.Mockito.never()).eval(org.mockito.ArgumentMatchers.anyString());
    }

    @Test
    void mapsDocumentQueryRowsBackToExportedElements() {
        Connection connection = org.mockito.Mockito.mock(Connection.class);
        Model model = org.mockito.Mockito.mock(Model.class);
        Element target = org.mockito.Mockito.mock(Element.class);
        Element mount = org.mockito.Mockito.mock(Element.class);
        org.mockito.Mockito.when(target.getQualifiedName()).thenReturn("Observatory::Subsystems");
        org.mockito.Mockito.when(target.getElementId()).thenReturn("query-id");
        org.mockito.Mockito.when(connection.parseSources(org.mockito.ArgumentMatchers.any())).thenReturn(model);
        org.mockito.Mockito.when(model.diagnostics()).thenReturn(List.of());
        org.mockito.Mockito.when(model.hash()).thenReturn("hash");
        org.mockito.Mockito.when(model.eval("\"10\"")).thenReturn(new org.openmbee.opensysml.Value.StringValue("10"));
        org.openmbee.opensysml.DocumentValue.ElementRef mountRef = new org.openmbee.opensysml.DocumentValue.ElementRef(
                "Observatory::telescope::mount", "PartUsage");
        org.openmbee.opensysml.DocumentValue.ElementRef opticsRef = new org.openmbee.opensysml.DocumentValue.ElementRef(
                "Observatory::telescope::optics", "PartUsage");
        org.mockito.Mockito.when(model.runDocumentQuery(org.mockito.ArgumentMatchers.eq("Observatory::Subsystems"),
                org.mockito.ArgumentMatchers.any())).thenReturn(new org.openmbee.opensysml.DocumentQueryResult(
                        List.of("name", "mass"), List.of(
                                new org.openmbee.opensysml.DocumentRow(mountRef, List.of(
                                        List.of(new org.openmbee.opensysml.DocumentValue.StringValue("mount")),
                                        List.of(new org.openmbee.opensysml.DocumentValue.RealValue(15))),
                                        java.util.Optional.empty(), java.util.Optional.empty(),
                                        java.util.Optional.empty(), java.util.Optional.empty()),
                                new org.openmbee.opensysml.DocumentRow(opticsRef, List.of(
                                        List.of(new org.openmbee.opensysml.DocumentValue.StringValue("optics")),
                                        List.of()),
                                        java.util.Optional.empty(), java.util.Optional.empty(),
                                        java.util.Optional.empty(), java.util.Optional.empty()))));
        ExportedProject project = new ExportedProject(List.of(SourceDocument.inline("report.sysml", "")),
                new ElementIndex(Map.of(
                        "Observatory::Subsystems", new ElementIndex.IndexedElement("Observatory::Subsystems",
                                "query-id", "query-sirius", target),
                        "Observatory::telescope", new ElementIndex.IndexedElement("Observatory::telescope",
                                "telescope-id", "telescope-sirius", org.mockito.Mockito.mock(Element.class)),
                        "Observatory::telescope::mount", new ElementIndex.IndexedElement(
                                "Observatory::telescope::mount", "mount-id", "mount-sirius", mount))),
                List.of(), List.of());
        IEMFEditingContext context = org.mockito.Mockito.mock(IEMFEditingContext.class);
        org.mockito.Mockito.when(context.getId()).thenReturn("ctx");
        RunWithOpenSysMLService service = new RunWithOpenSysMLService(connection, ignored -> project,
                new RunResultStore(), new OpenSysMLProperties());
        Map<String, String> bindings = new java.util.LinkedHashMap<>();
        bindings.put("root", "Observatory::telescope");
        bindings.put("threshold", "\"10\"");

        RunResult result = service.run(context, target, new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx",
                "query-id", RunOperation.RUN_DOCUMENT_QUERY, bindings, List.of(), List.of(), null, null, null));

        assertThat(result.ok()).isTrue();
        assertThat(result.resultText()).isEqualTo("2 rows");
        assertThat(result.table().columns()).containsExactly("name", "mass");
        assertThat(result.table().rows()).hasSize(2);
        assertThat(result.table().rows().get(0).element()).isEqualTo("Observatory::telescope::mount");
        assertThat(result.table().rows().get(0).siriusId()).isEqualTo("mount-sirius");
        assertThat(result.table().rows().get(0).cells()).containsExactly("mount", "15.0");
        assertThat(result.table().rows().get(1).siriusId()).isNull();
        assertThat(result.table().rows().get(1).cells()).containsExactly("optics", "");
        @SuppressWarnings("unchecked")
        org.mockito.ArgumentCaptor<Map<String, List<org.openmbee.opensysml.DocumentValue>>> captor =
                org.mockito.ArgumentCaptor.forClass(Map.class);
        org.mockito.Mockito.verify(model).runDocumentQuery(org.mockito.ArgumentMatchers.eq("Observatory::Subsystems"),
                captor.capture());
        assertThat(captor.getValue()).containsOnly(
                Map.entry("root", List.of(new org.openmbee.opensysml.DocumentValue.ElementRef("Observatory::telescope", ""))),
                Map.entry("threshold", List.of(new org.openmbee.opensysml.DocumentValue.StringValue("10"))));
    }
}
