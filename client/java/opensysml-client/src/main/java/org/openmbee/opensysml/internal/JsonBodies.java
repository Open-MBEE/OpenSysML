package org.openmbee.opensysml.internal;

import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.google.gson.JsonPrimitive;
import com.google.protobuf.Descriptors.FieldDescriptor;
import com.google.protobuf.InvalidProtocolBufferException;
import com.google.protobuf.Message;
import com.google.protobuf.util.JsonFormat;
import java.util.List;
import java.util.Map;
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
    T response = (T) withNegativeZeros(builder.build(), json);
    return response;
  }

  /**
   * Restores the sign of each {@code -0} the body carries, which {@link JsonFormat} reads through
   * {@link java.math.BigDecimal} and so as {@code 0}.
   */
  private static Message withNegativeZeros(Message message, String json) {
    if (!json.contains("-0")) {
      return message;
    }
    return withNegativeZeros(message, JsonParser.parseString(json));
  }

  private static Message withNegativeZeros(Message message, JsonElement json) {
    if (!json.isJsonObject()) {
      return message;
    }
    Message.Builder builder = null;
    for (Map.Entry<String, JsonElement> member : json.getAsJsonObject().entrySet()) {
      FieldDescriptor field = fieldNamed(message, member.getKey());
      if (field != null && !member.getValue().isJsonNull()) {
        builder = restoredField(message, builder, field, member.getValue());
      }
    }
    return builder == null ? message : builder.build();
  }

  /**
   * Puts one field's restored values into {@code builder}, opened from {@code message} the first
   * time one needs restoring; answers the builder, still {@code null} while nothing has.
   */
  private static Message.Builder restoredField(
      Message message, Message.Builder builder, FieldDescriptor field, JsonElement json) {
    if (field.isMapField()) {
      return json.isJsonObject()
          ? restoredEntries(message, builder, field, json.getAsJsonObject())
          : builder;
    }
    if (field.isRepeated()) {
      return json.isJsonArray() ? restoredItems(message, builder, field, json.getAsJsonArray()) : builder;
    }
    Object restored = restored(field, message.getField(field), json);
    if (restored == null) {
      return builder;
    }
    Message.Builder opened = builder != null ? builder : message.toBuilder();
    opened.setField(field, restored);
    return opened;
  }

  private static Message.Builder restoredEntries(
      Message message, Message.Builder builder, FieldDescriptor field, JsonObject entries) {
    Message.Builder opened = builder;
    List<?> pairs = (List<?>) message.getField(field);
    for (int i = 0; i < pairs.size(); i++) {
      Message pair = (Message) pairs.get(i);
      FieldDescriptor key = pair.getDescriptorForType().findFieldByNumber(1);
      FieldDescriptor value = pair.getDescriptorForType().findFieldByNumber(2);
      JsonElement entry = entries.get(String.valueOf(pair.getField(key)));
      Object restored = entry == null ? null : restored(value, pair.getField(value), entry);
      if (restored != null) {
        opened = opened != null ? opened : message.toBuilder();
        opened.setRepeatedField(field, i, pair.toBuilder().setField(value, restored).build());
      }
    }
    return opened;
  }

  private static Message.Builder restoredItems(
      Message message, Message.Builder builder, FieldDescriptor field, JsonArray items) {
    Message.Builder opened = builder;
    List<?> parsed = (List<?>) message.getField(field);
    for (int i = 0; i < parsed.size() && i < items.size(); i++) {
      Object restored = restored(field, parsed.get(i), items.get(i));
      if (restored != null) {
        opened = opened != null ? opened : message.toBuilder();
        opened.setRepeatedField(field, i, restored);
      }
    }
    return opened;
  }

  /** The value to put back for one field's JSON, or {@code null} when it is already right. */
  private static Object restored(FieldDescriptor field, Object parsed, JsonElement json) {
    switch (field.getJavaType()) {
      case DOUBLE, FLOAT -> {
        if (!isNegativeZero(json)) {
          return null;
        }
        return field.getJavaType() == FieldDescriptor.JavaType.DOUBLE ? -0.0d : -0.0f;
      }
      case MESSAGE -> {
        Message restored = withNegativeZeros((Message) parsed, json);
        return restored == parsed ? null : restored;
      }
      default -> {
        return null;
      }
    }
  }

  private static boolean isNegativeZero(JsonElement json) {
    if (!(json instanceof JsonPrimitive primitive) || primitive.isBoolean()) {
      return false;
    }
    try {
      return Double.doubleToRawLongBits(Double.parseDouble(primitive.getAsString()))
          == Double.doubleToRawLongBits(-0.0d);
    } catch (NumberFormatException notANumber) {
      return false;
    }
  }

  private static FieldDescriptor fieldNamed(Message message, String name) {
    for (FieldDescriptor field : message.getDescriptorForType().getFields()) {
      if (field.getJsonName().equals(name) || field.getName().equals(name)) {
        return field;
      }
    }
    return null;
  }
}
