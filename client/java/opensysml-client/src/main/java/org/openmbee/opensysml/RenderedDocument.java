package org.openmbee.opensysml;

import java.util.Objects;

/**
 * A rendered document: what {@link Model#renderDocument(String, DocumentForm)} answers, byte-for-byte
 * what the service's command line writes in that form.
 *
 * @param form the form it was rendered in
 * @param content the rendered document
 * @param fileName the file a rendered set ({@code -render-documents}) writes the document to in
 *     this form, which the other documents' links to it name; empty when the service predates it
 */
public record RenderedDocument(DocumentForm form, String content, String fileName) {

  /**
   * Creates a rendered document.
   *
   * @param form the form, never {@code null}
   * @param content the rendered text, never {@code null}
   * @param fileName the planned file name, never {@code null}
   */
  public RenderedDocument {
    Objects.requireNonNull(form, "form");
    Objects.requireNonNull(content, "content");
    Objects.requireNonNull(fileName, "fileName");
  }

  /**
   * Creates a rendered document whose planned file name is not known.
   *
   * @param form the form, never {@code null}
   * @param content the rendered text, never {@code null}
   */
  public RenderedDocument(DocumentForm form, String content) {
    this(form, content, "");
  }

  /**
   * Creates a document rendered to Markdown.
   *
   * @param markdown the Markdown, never {@code null}
   */
  public RenderedDocument(String markdown) {
    this(DocumentForm.MARKDOWN, markdown);
  }

  /**
   * The Markdown rendering.
   *
   * @return the content when rendered to Markdown, otherwise empty
   */
  public String markdown() {
    return form == DocumentForm.MARKDOWN ? content : "";
  }

  /**
   * The HTML rendering.
   *
   * @return the content when rendered to HTML, otherwise empty
   */
  public String html() {
    return form == DocumentForm.HTML ? content : "";
  }

  /**
   * The AsciiDoc rendering.
   *
   * @return the content when rendered to AsciiDoc, otherwise empty
   */
  public String asciidoc() {
    return form == DocumentForm.ASCIIDOC ? content : "";
  }

  @Override
  public String toString() {
    return content;
  }
}
