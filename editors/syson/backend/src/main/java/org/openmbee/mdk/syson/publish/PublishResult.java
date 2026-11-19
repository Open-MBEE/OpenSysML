package org.openmbee.mdk.syson.publish;

import java.util.List;

import org.openmbee.mdk.syson.run.RunDiagnostic;
import org.openmbee.opensysml.repository.Publisher;

/**
 * What a publication did; {@code ok} is false when nothing was written (model errors, or
 * {@code branchMoved}). {@code head} is the branch's commit afterwards, empty when it has none.
 */
public record PublishResult(boolean ok, boolean branchMoved, String summary, String server, String project,
        String projectName, boolean createdProject, String branch, String commit, String head, int created,
        int updated, int deleted, int leftInPlace, List<RunDiagnostic> diagnostics) {
    public PublishResult {
        diagnostics = List.copyOf(diagnostics);
    }

    static PublishResult published(String server, Publisher.Result result, List<RunDiagnostic> diagnostics) {
        return new PublishResult(true, false, result.describe(), server, result.project(), result.projectName(),
                result.createdProject(), result.branch(), result.commit().orElse(null), result.head(),
                result.created(), result.updated(), result.deleted(), result.leftInPlace(), diagnostics);
    }

    static PublishResult refused(String server, PublishWithOpenSysMLInput input, String summary,
            List<RunDiagnostic> diagnostics) {
        return new PublishResult(false, false, summary, server, input.project(), input.projectName(), false,
                input.branch(), null, "", 0, 0, 0, 0, diagnostics);
    }

    static PublishResult moved(String server, PublishWithOpenSysMLInput input, String summary,
            List<RunDiagnostic> diagnostics) {
        return new PublishResult(false, true, summary, server, input.project(), input.projectName(), false,
                input.branch(), null, "", 0, 0, 0, 0, diagnostics);
    }
}
