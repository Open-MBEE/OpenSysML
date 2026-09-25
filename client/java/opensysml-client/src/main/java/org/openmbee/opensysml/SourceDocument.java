package org.openmbee.opensysml;

import java.nio.file.Path;
import java.util.Objects;
import java.util.Optional;

/**
 * One document of a model {@link Connection#parseSources(java.util.List)} parses: a file the
 * service reads, or content carried inline.
 *
 * <p>An inline document is reported under its {@link #name()} in diagnostics and is indexed under
 * it, so two documents of a model need distinct names; a file is named by its path. A document's
 * own {@link #language()} is sent when present; {@link ParseOptions#language()} does not apply per
 * document.
 *
 * @param file the source the service reads, present when the document is a file
 * @param content the notation carried inline, present when the document is inline
 * @param name the name an inline document is reported and indexed under, absent to be named by
 *     position
 * @param language the notation inline content is written in, absent for the service's default;
 *     ignored for a file, whose extension says which
 */
public record SourceDocument(
    Optional<Path> file, Optional<String> content, Optional<String> name,
    Optional<Language> language) {

  /**
   * Validates the document: exactly one of file and content.
   *
   * @param file the file, when the document is one
   * @param content the inline content, when the document is one
   * @param name the inline document's name, when given
   * @param language the inline content's notation, when given
   */
  public SourceDocument {
    Objects.requireNonNull(file, "file");
    Objects.requireNonNull(content, "content");
    Objects.requireNonNull(name, "name");
    Objects.requireNonNull(language, "language");
    if (file.isPresent() == content.isPresent()) {
      throw new IllegalArgumentException(
          "a source document is a file or inline content, not both and not neither");
    }
    if (file.isPresent() && (name.isPresent() || language.isPresent())) {
      throw new IllegalArgumentException("a file's name and language come from the file itself");
    }
  }

  /**
   * A document the service reads from a file.
   *
   * @param file the source path, as the service resolves it
   * @return the document
   */
  public static SourceDocument file(Path file) {
    return new SourceDocument(
        Optional.of(Objects.requireNonNull(file, "file")),
        Optional.empty(),
        Optional.empty(),
        Optional.empty());
  }

  /**
   * A document of inline content, named for its diagnostics and its index entry.
   *
   * @param name the name the document is reported and indexed under
   * @param content the notation
   * @return the document
   */
  public static SourceDocument inline(String name, String content) {
    return new SourceDocument(
        Optional.empty(), Optional.of(content), Optional.of(name), Optional.empty());
  }

  /**
   * The same document under another name.
   *
   * @param name the name the document is reported and indexed under
   * @return the document named so
   */
  public SourceDocument withName(String name) {
    return new SourceDocument(file, content, Optional.of(name), language);
  }

  /**
   * The same document read in another notation.
   *
   * @param language the notation the content is written in
   * @return the document read as that notation
   */
  public SourceDocument withLanguage(Language language) {
    return new SourceDocument(file, content, name, Optional.of(language));
  }
}
