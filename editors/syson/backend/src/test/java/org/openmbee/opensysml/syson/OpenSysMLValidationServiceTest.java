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
    private static final String PKG_A = "Pkg::A";
    @Test
    void emptyValidationHasNoDiagnostics() {
        assertThat(new OpenSysMLValidationService(new RunResultStore()).validate(new Object(), null)).isEmpty();
    }

    @Test
    void storesDiagnosticsByEditingContext() {
        RunResultStore store = new RunResultStore();
        store.put("ctx", RunResult.builder().operation(RunOperation.INSTANTIATE).target(PKG_A)
                .mappedDiagnostics(List.of(new MappedDiagnostic(
                        new RunDiagnostic("error", "bad", "x", null, null, null, null, null), null))).build());
        assertThat(new OpenSysMLValidationService(store)
                .validate((org.eclipse.sirius.components.core.api.IEditingContext) () -> "ctx")).hasSize(1);
    }

    @Test
    void preservesMappedAndUnmappedDiagnostics() {
        RunResultStore store = new RunResultStore();
        FakeElement element = new FakeElement(PKG_A);
        RunDiagnostic mapped = new RunDiagnostic("error", "mapped", "x", null, null, PKG_A, "id-Pkg::A",
                "sirius://a");
        RunDiagnostic unmapped = new RunDiagnostic("warning", "unmapped", "y", null, null, null, null, null);
        RunResult result = RunResult.builder().operation(RunOperation.INSTANTIATE).target(PKG_A)
                .mappedDiagnostics(List.of(new MappedDiagnostic(mapped, element), new MappedDiagnostic(unmapped, null)))
                .build();
        store.put("ctx", result);

        List<Object> diagnostics = new OpenSysMLValidationService(store).validate(
                (org.eclipse.sirius.components.core.api.IEditingContext) () -> "ctx");

        assertThat(diagnostics).hasSize(2);
        assertThat(((org.eclipse.emf.common.util.BasicDiagnostic) diagnostics.get(0)).getData()).hasSize(1);
        assertThat(((org.eclipse.emf.common.util.BasicDiagnostic) diagnostics.get(0)).getData().get(0))
                .isSameAs(element);
        assertThat(((org.eclipse.emf.common.util.BasicDiagnostic) diagnostics.get(1)).getData()).isEmpty();
    }
}
