package org.openmbee.opensysml;

/** The notation inline content is written in. */
public enum Language {
  /** SysML v2 textual notation, the default. */
  SYSML("sysml"),
  /** KerML textual notation. */
  KERML("kerml");

  private final String wireName;

  Language(String wireName) {
    this.wireName = wireName;
  }

  /**
   * The name the service knows this language by.
   *
   * @return the wire name ({@code "sysml"} or {@code "kerml"})
   */
  public String wireName() {
    return wireName;
  }

  /**
   * The language a wire name names, defaulting to SysML for an empty one as the service does.
   *
   * @param wireName {@code "sysml"}, {@code "kerml"} or empty
   * @return the language
   * @throws IllegalArgumentException if the name is neither
   */
  public static Language fromWireName(String wireName) {
    if (wireName == null || wireName.isEmpty() || wireName.equals(SYSML.wireName)) {
      return SYSML;
    }
    if (wireName.equals(KERML.wireName)) {
      return KERML;
    }
    throw new IllegalArgumentException("no such language: " + wireName);
  }
}
