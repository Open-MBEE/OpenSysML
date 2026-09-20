package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.syson.run.RunDiagnostic;
import org.openmbee.opensysml.syson.run.RunOperation;
import org.openmbee.opensysml.syson.run.RunResult;
import org.openmbee.opensysml.syson.run.RunResultStore;
import org.openmbee.opensysml.syson.run.RunResult.MappedDiagnostic;
import org.openmbee.opensysml.syson.validation.OpenSysMLValidationService;

class OpenSysMLValidationServiceTest {
    @Test
    void emptyValidationHasNoDiagnostics() {
        assertThat(new OpenSysMLValidationService(new RunResultStore()).validate(new Object(), null)).isEmpty();
    }

    @Test
    void storesDiagnosticsByEditingContext() {
        RunResultStore store = new RunResultStore();
        store.put("ctx", new RunResult("", RunOperation.INSTANTIATE, "Pkg::A", false, null, null, null,
                List.of(), List.of(), null,
                List.of(), List.of(), List.of(new MappedDiagnostic(
                        new RunDiagnostic("error", "bad", "x", null, null, null, null, null), null))));
        assertThat(new OpenSysMLValidationService(store).validate(new org.eclipse.sirius.components.core.api.IEditingContext() {
            public String getId() { return "ctx"; }
        })).hasSize(1);
    }

    @Test
    void preservesMappedAndUnmappedDiagnostics() {
        RunResultStore store = new RunResultStore();
        FakeElement element = new FakeElement("Pkg::A");
        RunDiagnostic mapped = new RunDiagnostic("error", "mapped", "x", null, null, "Pkg::A", "id-Pkg::A",
                "sirius://a");
        RunDiagnostic unmapped = new RunDiagnostic("warning", "unmapped", "y", null, null, null, null, null);
        RunResult result = new RunResult("", RunOperation.INSTANTIATE, "Pkg::A", false, null, null, null,
                List.of(), List.of(), null, List.of(), List.of(),
                List.of(new MappedDiagnostic(mapped, element), new MappedDiagnostic(unmapped, null)));
        store.put("ctx", result);

        List<Object> diagnostics = new OpenSysMLValidationService(store).validate(
                new org.eclipse.sirius.components.core.api.IEditingContext() {
                    public String getId() { return "ctx"; }
                });

        assertThat(diagnostics).hasSize(2);
        assertThat(((org.eclipse.emf.common.util.BasicDiagnostic) diagnostics.get(0)).getData()).hasSize(1);
        assertThat(((org.eclipse.emf.common.util.BasicDiagnostic) diagnostics.get(0)).getData().get(0))
                .isSameAs(element);
        assertThat(((org.eclipse.emf.common.util.BasicDiagnostic) diagnostics.get(1)).getData()).isEmpty();
    }
}
