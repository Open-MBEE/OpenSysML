package org.openmbee.opensysml.syson.run;

import java.util.List;
import java.util.UUID;

import org.eclipse.sirius.components.core.api.IPayload;
import org.eclipse.sirius.components.representations.Message;

public record RunWithOpenSysMLSuccessPayload(UUID id, List<Message> messages, RunResult result) implements IPayload {
}
