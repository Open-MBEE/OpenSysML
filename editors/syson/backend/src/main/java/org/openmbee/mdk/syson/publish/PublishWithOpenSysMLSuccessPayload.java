package org.openmbee.mdk.syson.publish;

import java.util.List;
import java.util.UUID;

import org.eclipse.sirius.components.core.api.IPayload;
import org.eclipse.sirius.components.representations.Message;

public record PublishWithOpenSysMLSuccessPayload(UUID id, List<Message> messages, PublishResult result)
        implements IPayload {
}
