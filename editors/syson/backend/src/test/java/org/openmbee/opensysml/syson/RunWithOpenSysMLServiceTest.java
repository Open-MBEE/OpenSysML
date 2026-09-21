package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;
import java.util.Map;
import java.util.UUID;

import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
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
                RunOperation.INSTANTIATE, Map.of("stale", "1"), List.of(), List.of("2"), null, null));

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
                RunOperation.VERIFY_SATISFACTION, Map.of(), List.of(), List.of(), null, null));

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
                RunOperation.INSTANTIATE, null, null, null, null, null));
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
                VEHICLE_ID, RunOperation.INSTANTIATE, Map.of(), List.of(), List.of(), null, null));

        assertThat(result.ok()).isFalse();
        assertThat(result.modelHash()).isEqualTo("");
        assertThat(result.diagnostics()).isNotEmpty();
    }
}
