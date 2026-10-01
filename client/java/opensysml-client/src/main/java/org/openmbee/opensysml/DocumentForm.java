package org.openmbee.opensysml;

/** The form {@link Model#renderDocument(String, DocumentForm)} renders a document in. */
public enum DocumentForm {
  /** Markdown, byte-for-byte what the command line writes. */
  MARKDOWN("markdown"),
  /** A standalone HTML page; needs {@link Capabilities#RENDER_DOCUMENT_HTML}. */
  HTML("html");

  private final String wireName;

  DocumentForm(String wireName) {
    this.wireName = wireName;
  }

  /**
   * The name a {@code RenderDocumentRequest.form} carries.
   *
   * @return {@code "markdown"} or {@code "html"}
   */
  public String wireName() {
    return wireName;
  }

  /**
   * The form a wire name names; empty is Markdown.
   *
   * @param wireName {@code ""}, {@code "markdown"} or {@code "html"}
   * @return the form
   * @throws IllegalArgumentException for any other name
   */
  public static DocumentForm fromWireName(String wireName) {
    if (wireName == null || wireName.isEmpty() || wireName.equals(MARKDOWN.wireName)) {
      return MARKDOWN;
    }
    if (wireName.equals(HTML.wireName)) {
      return HTML;
    }
    throw new IllegalArgumentException("form must be 'markdown' or 'html', not '" + wireName + "'");
  }
}
