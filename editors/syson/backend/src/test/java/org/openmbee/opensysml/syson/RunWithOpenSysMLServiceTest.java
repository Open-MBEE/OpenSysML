package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;
import java.util.Map;
import java.util.UUID;

import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.syson.export.ExportedProject;
import org.openmbee.opensysml.syson.export.ProjectTextExporter;
import org.openmbee.opensysml.syson.identity.ElementIndex;
import org.openmbee.opensysml.syson.run.RunOperation;
import org.openmbee.opensysml.syson.run.RunResultStore;
import org.openmbee.opensysml.syson.run.RunWithOpenSysMLInput;
import org.openmbee.opensysml.syson.run.RunWithOpenSysMLService;

class RunWithOpenSysMLServiceTest {
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
}
