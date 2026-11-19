package org.openmbee.mdk.syson.publish;

import java.util.UUID;

import org.eclipse.sirius.components.core.api.IInput;

/**
 * One publication of an editing context's model to the configured SysML v2 API server.
 *
 * @param project the project's id; blank creates a project named {@code projectName}
 * @param branch the branch's id; blank is the project's default branch
 * @param lastSeenCommit the commit this browser last saw the branch at, {@code null} to publish wherever it is
 * @param token a bearer token for this publication, blank for the configured one
 */
public record PublishWithOpenSysMLInput(UUID id, String editingContextId, String project, String projectName,
        String branch, String message, boolean deleteMissing, String lastSeenCommit, String token) implements IInput {
    public PublishWithOpenSysMLInput {
        project = project == null ? "" : project.strip();
        projectName = projectName == null ? "" : projectName.strip();
        branch = branch == null ? "" : branch.strip();
        message = message == null ? "" : message;
        token = token == null ? "" : token.strip();
    }

    // Inputs are logged, so the token never appears in the text form.
    @Override
    public String toString() {
        return "PublishWithOpenSysMLInput[id=" + id + ", editingContextId=" + editingContextId + ", project=" + project
                + ", projectName=" + projectName + ", branch=" + branch + ", deleteMissing=" + deleteMissing
                + ", lastSeenCommit=" + lastSeenCommit + ", token=" + (token.isEmpty() ? "" : "***") + "]";
    }
}
