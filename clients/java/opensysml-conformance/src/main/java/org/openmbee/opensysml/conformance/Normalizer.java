package org.openmbee.opensysml.conformance;

import com.google.protobuf.Descriptors.EnumValueDescriptor;
import com.google.protobuf.Descriptors.FieldDescriptor;
import com.google.protobuf.Message;
import java.math.BigInteger;
import java.nio.file.InvalidPathException;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;

/**
 * Turns a response into a tree with the values that are not the same twice replaced. See
 * conformance/README.md; the placeholder spellings are the contract, not this class's choice.
 */
final class Normalizer {

  static final String MODEL_HASH = "${model_hash}";
  static final String VERSION = "${version}";
  static final String PATH = "${path}";

  /** The int64 fields carrying a runtime instance id, which is assigned per call. */
  private static final Set<String> NORMALIZED_IDS =
      Set.of(
          "sysml.Instance.id",
          "sysml.Value.instance_id",
          "sysml.Verdict.instance_id",
          "sysml.Function.self_id");

  private final String modelHash;
  private final Map<Long, String> labels = new HashMap<>();

  Normalizer(String modelHash) {
    this.modelHash = modelHash;
  }

  /**
   * Renders a message as maps, lists, strings, numbers and booleans. Only set fields appear, so a
   * scalar left at its default is absent, which is what it is on the wire.
   *
   * @param message the response
   * @return the normalized tree
   */
  Map<String, Object> normalize(Message message) {
    List<Map.Entry<FieldDescriptor, Object>> fields =
        new ArrayList<>(message.getAllFields().entrySet());
    // getAllFields has no defined order; field number order makes id labels reproducible.
    fields.sort(Comparator.comparingInt(entry -> entry.getKey().getNumber()));

    Map<String, Object> out = new LinkedHashMap<>();
    for (Map.Entry<FieldDescriptor, Object> entry : fields) {
      FieldDescriptor field = entry.getKey();
      if (field.isMapField()) {
        out.put(field.getName(), entries(field, entry.getValue()));
      } else if (field.isRepeated()) {
        List<Object> items = new ArrayList<>();
        for (Object item : (List<?>) entry.getValue()) {
          items.add(value(field, item));
        }
        out.put(field.getName(), items);
      } else {
        out.put(field.getName(), value(field, entry.getValue()));
      }
    }
    return out;
  }

  private Map<String, Object> entries(FieldDescriptor field, Object raw) {
    FieldDescriptor key = field.getMessageType().findFieldByName("key");
    FieldDescriptor value = field.getMessageType().findFieldByName("value");
    Map<String, Object> entries = new TreeMap<>();
    for (Object item : (List<?>) raw) {
      Message entry = (Message) item;
      entries.put(String.valueOf(entry.getField(key)), value(value, entry.getField(value)));
    }
    return entries;
  }

  private Object value(FieldDescriptor field, Object raw) {
    return switch (field.getType()) {
      case MESSAGE, GROUP -> normalize((Message) raw);
      case ENUM -> ((EnumValueDescriptor) raw).getName();
      case BOOL -> raw;
      case STRING -> string(field.getFullName(), (String) raw);
      case DOUBLE, FLOAT -> ((Number) raw).doubleValue();
      case INT64, SINT64, SFIXED64 ->
          NORMALIZED_IDS.contains(field.getFullName()) ? label((Long) raw) : (Long) raw;
      case INT32, SINT32, SFIXED32 -> ((Number) raw).longValue();
      case UINT32, FIXED32 -> BigInteger.valueOf(Integer.toUnsignedLong((Integer) raw));
      case UINT64, FIXED64 -> new BigInteger(Long.toUnsignedString((Long) raw));
      default -> String.valueOf(raw);
    };
  }

  /** Replaces the strings a call cannot repeat: the model hash, the version, an absolute path. */
  private String string(String field, String text) {
    if (field.equals("sysml.ServerInfoResponse.version")) {
      return VERSION;
    }
    if (!modelHash.isEmpty() && text.equals(modelHash)) {
      return MODEL_HASH;
    }
    return absolute(text) ? PATH : text;
  }

  private static boolean absolute(String text) {
    try {
      return !text.isEmpty() && Path.of(text).isAbsolute();
    } catch (InvalidPathException e) {
      return false;
    }
  }

  /** The symbolic name of a runtime instance id, in order of first appearance. */
  private String label(long id) {
    return labels.computeIfAbsent(id, key -> "@" + (labels.size() + 1));
  }
}
