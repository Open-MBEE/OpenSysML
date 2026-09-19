package org.openmbee.opensysml;

/**
 * The body encoding a connection sends and accepts over the Connect protocol.
 *
 * <p>{@link #PROTOBUF} is the default and the one to use: a 468 KB {@code Query} answer takes
 * ~6.5 ms with a protobuf body against ~42 ms with JSON, which is JSON parsing cost rather than
 * bytes on the wire (see {@code docs/internals/design/transport-evaluation.md}).
 */
public enum Encoding {
  /** {@code application/proto} bodies. */
  PROTOBUF("application/proto"),
  /**
   * {@code application/json} bodies, for debugging and for comparing against {@code curl}.
   *
   * <p>Needs {@code com.google.protobuf:protobuf-java-util} on the classpath, which the client
   * declares as an optional dependency.
   */
  JSON("application/json");

  private final String contentType;

  Encoding(String contentType) {
    this.contentType = contentType;
  }

  /**
   * The content type this encoding sends.
   *
   * @return the media type
   */
  public String contentType() {
    return contentType;
  }
}
