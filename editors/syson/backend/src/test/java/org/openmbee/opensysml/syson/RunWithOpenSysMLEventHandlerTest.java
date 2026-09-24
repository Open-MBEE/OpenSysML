package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;

import java.util.UUID;

import org.eclipse.sirius.components.core.api.IObjectSearchService;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.syson.run.RunOperation;
import org.openmbee.opensysml.syson.run.RunResultStore;
import org.openmbee.opensysml.syson.run.RunWithOpenSysMLEventHandler;
import org.openmbee.opensysml.syson.run.RunWithOpenSysMLInput;
import org.openmbee.opensysml.syson.run.RunWithOpenSysMLService;

class RunWithOpenSysMLEventHandlerTest {
    @Test
    void handlesOnlyRunInputs() {
        RunWithOpenSysMLEventHandler handler = new RunWithOpenSysMLEventHandler(mock(IObjectSearchService.class),
                mock(RunWithOpenSysMLService.class));
        assertThat(handler.canHandle(mock(org.eclipse.sirius.components.core.api.IEditingContext.class),
                new RunWithOpenSysMLInput(UUID.randomUUID(), "ctx", "obj", RunOperation.INSTANTIATE, null, null, null,
                        null, null))).isTrue();
    }
}
