package org.openmbee.opensysml.conformance;

import com.google.protobuf.Descriptors.FieldDescriptor;
import com.google.protobuf.Message;
import java.util.List;
import java.util.Map;
import java.util.function.UnaryOperator;

/**
 * Deliberate corruptions of an answer, so a run can show the comparison is not vacuous: with one of
 * these applied the scenarios that assert the corrupted thing must fail.
 */
enum Mutations implements UnaryOperator<Message> {

  /** Leaves the answer alone. */
  NONE {
    @Override
    public Message apply(Message message) {
      return message;
    }
  },

  /** Moves every real by a millionth, far above the 1e-9 relative tolerance. */
  PERTURB_REALS {
    @Override
    Object mutate(FieldDescriptor field, Object value) {
      return switch (field.getJavaType()) {
        case DOUBLE -> (Double) value * 1.000001 + 1e-9;
        case FLOAT -> (Float) value * 1.000001f + 1e-9f;
        default -> value;
      };
    }
  },

  /** Drops the last element of every repeated field, which exact list lengths must catch. */
  TRUNCATE_LISTS {
    @Override
    boolean keep(FieldDescriptor field, int index, int size) {
      return index < size - 1;
    }
  },

  /** Rewrites every string, which every text and substring assertion must catch. */
  REWRITE_STRINGS {
    @Override
    Object mutate(FieldDescriptor field, Object value) {
      return field.getJavaType() == FieldDescriptor.JavaType.STRING ? "mutated" : value;
    }
  };

  /**
   * The mutation of a name.
   *
   * @param name the name as a flag spells it, such as {@code perturb-reals}
   * @return the mutation
   * @throws IllegalArgumentException if there is no such mutation
   */
  static Mutations of(String name) {
    return valueOf(name.replace('-', '_').toUpperCase(java.util.Locale.ROOT));
  }

  /** Applies the mutation to every field of a message, recursively. */
  @Override
  public Message apply(Message message) {
    Message.Builder builder = message.toBuilder();
    for (Map.Entry<FieldDescriptor, Object> entry : message.getAllFields().entrySet()) {
      FieldDescriptor field = entry.getKey();
      if (field.isRepeated()) {
        List<?> items = (List<?>) entry.getValue();
        builder.clearField(field);
        for (int index = 0; index < items.size(); index++) {
          if (keep(field, index, items.size())) {
            builder.addRepeatedField(field, value(field, items.get(index)));
          }
        }
      } else {
        builder.setField(field, value(field, entry.getValue()));
      }
    }
    return builder.build();
  }

  private Object value(FieldDescriptor field, Object value) {
    return value instanceof Message nested ? apply(nested) : mutate(field, value);
  }

  /** What a scalar becomes; unchanged by default. */
  Object mutate(FieldDescriptor field, Object value) {
    return value;
  }

  /** Whether a repeated element survives; all of them do by default. */
  boolean keep(FieldDescriptor field, int index, int size) {
    return true;
  }
}
