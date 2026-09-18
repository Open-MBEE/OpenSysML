package org.openmbee.opensysml.internal;

import com.google.protobuf.InvalidProtocolBufferException;
import com.google.protobuf.Message;
import com.google.protobuf.util.JsonFormat;
import org.openmbee.opensysml.OpenSysMLException;

/**
 * Protobuf-JSON bodies, for {@link org.openmbee.opensysml.Encoding#JSON}.
 *
 * <p>Loaded only when a connection asks for JSON, so {@code protobuf-java-util} stays an optional
 * dependency of the client.
 */
final class JsonBodies {

  private JsonBodies() {}

  /** Fails with a clear message when the optional dependency is missing. */
  static void requireOnClasspath() {
    try {
      Class.forName("com.google.protobuf.util.JsonFormat");
    } catch (ClassNotFoundException e) {
      throw new OpenSysMLException(
          "Encoding.JSON needs com.google.protobuf:protobuf-java-util on the classpath; "
              + "the client declares it as an optional dependency because protobuf bodies are "
              + "the default",
          e);
    }
  }

  static String serialize(Message request) {
    try {
      return JsonFormat.printer().omittingInsignificantWhitespace().print(request);
    } catch (InvalidProtocolBufferException e) {
      throw new OpenSysMLException("the request could not be written as JSON", e);
    }
  }

  static <T extends Message> T parse(String json, T responseDefault)
      throws InvalidProtocolBufferException {
    Message.Builder builder = responseDefault.newBuilderForType();
    JsonFormat.parser().ignoringUnknownFields().merge(json, builder);
    // A builder of a message's own default instance builds that message type.
    @SuppressWarnings("unchecked")
    T response = (T) builder.build();
    return response;
  }
}
