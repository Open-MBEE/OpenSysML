package org.openmbee.mdk.syson.publish;

import java.util.UUID;

import org.eclipse.sirius.components.core.api.IInput;

/**
 * One publication of a model: a blank {@code project} creates one named {@code projectName}, a blank
 * {@code branch} is the default, a null {@code lastSeenCommit} publishes wherever the branch is.
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
