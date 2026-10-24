package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;
import org.openmbee.opensysml.proto.ImportDataResponse;

/**
 * What {@link Model#importData} answers: the values each import set and the edited model.
 *
 * @param files one per import, in order
 * @param documents the edited documents of a multi-document model; empty on a dry run
 * @param content the edited content of a one-document model; {@code ""} on a dry run
 * @param modelHash the hash of the imported model, parsed and cached; empty on a dry run
 * @param diagnostics the diagnostics the import raised
 * @param wire the response as the service answered it
 */
public record ImportResult(
    List<ImportedFile> files,
    List<EditedDocument> documents,
    String content,
    Optional<String> modelHash,
    List<Diagnostic> diagnostics,
    ImportDataResponse wire) {
  /** Validates the components and makes the lists immutable. */
  public ImportResult {
    files = List.copyOf(Objects.requireNonNull(files, "files"));
    documents = List.copyOf(Objects.requireNonNull(documents, "documents"));
    Objects.requireNonNull(content, "content");
    Objects.requireNonNull(modelHash, "modelHash");
    diagnostics = List.copyOf(Objects.requireNonNull(diagnostics, "diagnostics"));
    Objects.requireNonNull(wire, "wire");
  }

  /** Whether the import changed the model (false on a dry run). */
  public boolean applied() {
    return modelHash.isPresent();
  }
}
