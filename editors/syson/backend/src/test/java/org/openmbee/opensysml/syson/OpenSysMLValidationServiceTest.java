package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;
import java.util.Map;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.syson.run.RunDiagnostic;
import org.openmbee.opensysml.syson.run.RunOperation;
import org.openmbee.opensysml.syson.run.RunResult;
import org.openmbee.opensysml.syson.run.RunResultStore;
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
                List.of(new RunDiagnostic("error", "bad", "x", null, null, null, null, null)),
                List.of(), List.of(), Map.of()));
        assertThat(new OpenSysMLValidationService(store).validate(new org.eclipse.sirius.components.core.api.IEditingContext() {
            public String getId() { return "ctx"; }
        })).hasSize(1);
    }
}
