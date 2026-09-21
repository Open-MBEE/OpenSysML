package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;

import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.eclipse.syson.sysml.Element;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.openmbee.opensysml.Analysis;
import org.openmbee.opensysml.AnalysisOptions;
import org.openmbee.opensysml.Connection;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.Standing;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.syson.export.ExportedProject;
import org.openmbee.opensysml.syson.export.ProjectExporter;
import org.openmbee.opensysml.syson.identity.ElementIndex;
import org.openmbee.opensysml.syson.run.RunOperation;
import org.openmbee.opensysml.syson.run.RunResultStore;
import org.openmbee.opensysml.syson.run.RunWithOpenSysMLInput;
import org.openmbee.opensysml.syson.run.RunWithOpenSysMLService;

class RunWithOpenSysMLServiceAnalysisTest {
    @Test
    void passesAnalysisOptionsToModel() {
        Connection connection = mock(Connection.class);
        Model model = mock(Model.class);
        Element target = mock(Element.class);
        when(target.getQualifiedName()).thenReturn("Analysis");
        when(target.getElementId()).thenReturn("analysis-id");
        when(connection.parseSources(any())).thenReturn(model);
        when(model.diagnostics()).thenReturn(List.of());
        when(model.hash()).thenReturn("hash");
        when(model.eval("21")).thenReturn(new Value.IntegerValue(21));
        when(model.runAnalysis(eq("Analysis"), any(AnalysisOptions.class)))
                .thenReturn(new Analysis(Map.of(), List.of(), List.of(), List.of(), List.of(), List.of(), Standing.none()));
        ExportedProject project = new ExportedProject(List.of(),
                new ElementIndex(Map.of("Analysis", new ElementIndex.IndexedElement("Analysis", "analysis-id",
                        "sirius-id", target))),
                List.of(), List.of());
        ProjectExporter exporter = context -> project;
        IEMFEditingContext context = mock(IEMFEditingContext.class);
        when(context.getId()).thenReturn("ctx");
        RunWithOpenSysMLService service = new RunWithOpenSysMLService(connection, exporter, new RunResultStore(),
                new OpenSysMLProperties());

        service.run(context, target, new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx", "analysis-id",
                RunOperation.RUN_ANALYSIS, Map.of("x", "21"), List.of(), List.of("21"), "declared", "subject"));

        ArgumentCaptor<AnalysisOptions> options = ArgumentCaptor.forClass(AnalysisOptions.class);
        verify(model).runAnalysis(eq("Analysis"), options.capture());
        assertThat(options.getValue().subject()).isEqualTo(Optional.of("subject"));
        assertThat(options.getValue().arguments()).containsExactly(new Value.IntegerValue(21));
        assertThat(options.getValue().namedArguments()).containsEntry("x", new Value.IntegerValue(21));
        assertThat(options.getValue().schedule()).isEqualTo(Optional.of("declared"));
    }
}
