package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * A value the service evaluated: an immutable variant of {@code sysml.Value}.
 *
 * <p>The arms are exhaustive and sealed, so a {@code switch} over them needs no default that
 * silently swallows a value kind added later — a new arm becomes a compile error instead.
 */
public sealed interface Value {

  /** An {@code Integer} value. */
  record IntegerValue(long value) implements Value {}

  /** A {@code Real} value. */
  record RealValue(double value) implements Value {}

  /**
   * A {@code Complex} value in rectangular form: one number, never a sequence of two reals.
   *
   * <p>Only a service advertising the {@code complex_values} capability reports one as itself
   * rather than as an unsupported {@link NullValue}. It is not numeric to {@link #asDouble()}: a
   * complex number has no single real magnitude.
   *
   * @param real the real part
   * @param imaginary the imaginary part
   */
  record ComplexValue(double real, double imaginary) implements Value {
    /**
     * This number as SysML writes it, {@code 1.5 - 2.0i}, each part in {@link Double#toString}
     * form; the sign between them is the imaginary part's.
     *
     * @return the rectangular rendering
     */
    public String format() {
      boolean negative = Double.doubleToRawLongBits(imaginary) < 0;
      return real + (negative ? " - " : " + ") + Math.abs(imaginary) + "i";
    }
  }

  /** A {@code Boolean} value. */
  record BooleanValue(boolean value) implements Value {}

  /** A {@code String} value. */
  record StringValue(String value) implements Value {
    /**
     * Creates a string value.
     *
     * @param value the string, never {@code null}
     */
    public StringValue {
      Objects.requireNonNull(value, "value");
    }
  }

  /** The null value: a feature that resolved to nothing. */
  record NullValue() implements Value {}

  /**
   * A materialized feature of a value type that holds no value.
   *
   * <p>Only a service advertising the {@code unset_value} capability distinguishes it from an
   * empty object.
   */
  record UnsetValue() implements Value {}

  /**
   * A reference to a runtime instance, by the id the service assigned it.
   *
   * @param instanceId id of the referenced instance
   */
  record InstanceReference(long instanceId) implements Value {}

  /**
   * A sequence of values.
   *
   * @param elements the elements, in order
   */
  record Sequence(List<Value> elements) implements Value {
    /**
     * Creates a sequence, copying the elements.
     *
     * @param elements the elements, never {@code null}
     */
    public Sequence {
      elements = List.copyOf(elements);
    }
  }

  /**
   * A magnitude with the unit it is expressed in.
   *
   * @param quantity the quantity
   */
  record QuantityValue(Quantity quantity) implements Value {
    /**
     * Creates a quantity value.
     *
     * @param quantity the quantity, never {@code null}
     */
    public QuantityValue {
      Objects.requireNonNull(quantity, "quantity");
    }
  }

  /**
   * A measurement unit held as a value by itself, with no magnitude: {@code SI::m}, or {@code m /
   * s} as an operation composed it.
   *
   * <p>Only a service advertising the {@code measurement_refs} capability reports one as itself
   * rather than as an unsupported {@link NullValue}.
   *
   * @param unit the unit as written ({@code "km"}), or empty for one never written down
   * @param reduction what the unit reduces to, which the service always sends
   * @param unitId FQN of the one declaration the unit names ({@code "SI::kilometre"}), absent for a
   *     unit an operation composed
   */
  record MeasurementRefValue(String unit, Quantity.UnitTerm reduction, Optional<String> unitId)
      implements Value {
    /**
     * Creates a measurement reference.
     *
     * @param unit the unit as written, never {@code null}
     * @param reduction the unit's reduction to base units, never {@code null}
     * @param unitId the declaration named, never {@code null}
     */
    public MeasurementRefValue {
      Objects.requireNonNull(unit, "unit");
      Objects.requireNonNull(reduction, "reduction");
      Objects.requireNonNull(unitId, "unitId");
    }
  }

  /**
   * A calc held as a value: a calc definition, or a calc usage with an input no read could
   * supply, as {@code Sq} in {@code Fn(Sq, 3.0)} or the {@code f} of {@code in calc f {...}}.
   *
   * <p>It is the declaration it is a value of, which is its identity: two functions are equal
   * exactly when both components are. A function closing over the bindings of the behavior body
   * it is declared in has no wire form; the service sends it as an unsupported {@link NullValue},
   * as does a service without the {@code function_values} capability for every function.
   *
   * @param calcId FQN of the calc declaration ({@code "Analysis::Sq"}), never empty
   * @param selfId id of the object the calc's feature names resolve against, for a calc usage read
   *     off a part ({@code holder.scale}); absent for a function closing over no object
   */
  record FunctionValue(String calcId, Optional<Long> selfId) implements Value {
    /**
     * Creates a function.
     *
     * @param calcId the calc declaration, never {@code null} or empty
     * @param selfId the object read against, never {@code null}
     * @throws IllegalArgumentException if {@code calcId} is empty
     */
    public FunctionValue {
      Objects.requireNonNull(calcId, "calcId");
      Objects.requireNonNull(selfId, "selfId");
      if (calcId.isEmpty()) {
        throw new IllegalArgumentException("a function names no calc");
      }
    }
  }

  /**
   * One literal of an enumeration definition.
   *
   * <p>Only a service advertising the {@code enum_values} capability reports a literal as itself
   * rather than as {@link NullValue}.
   *
   * @param literal the literal
   */
  record EnumerationValue(EnumLiteral literal) implements Value {
    /**
     * Creates an enumeration value.
     *
     * @param literal the literal, never {@code null}
     */
    public EnumerationValue {
      Objects.requireNonNull(literal, "literal");
    }
  }

  /**
   * A multidimensional array: its shape and its elements flattened in row-major order, the last
   * dimension varying fastest. A rank-0 array holds exactly one element; an element is any value,
   * a nested array or a quantity included.
   *
   * <p>Only a service advertising the {@code structured_values} capability reports one as itself
   * rather than as an unsupported {@link NullValue}.
   *
   * @param dimensions the extent of each dimension, all positive
   * @param elements the elements, row-major, exactly as many as the dimensions multiply to
   */
  record ArrayValue(List<Long> dimensions, List<Value> elements) implements Value {
    /**
     * Creates an array, copying the dimensions and the elements.
     *
     * @param dimensions the extent of each dimension, never {@code null}
     * @param elements the elements, never {@code null}
     * @throws IllegalArgumentException if a dimension is not positive, or the elements do not fill
     *     the dimensions exactly
     */
    public ArrayValue {
      dimensions = List.copyOf(dimensions);
      elements = List.copyOf(elements);
      long size = 1;
      for (long extent : dimensions) {
        if (extent <= 0) {
          throw new IllegalArgumentException("array dimension is not positive: " + extent);
        }
        size = Math.multiplyExact(size, extent);
      }
      if (size != elements.size()) {
        throw new IllegalArgumentException(
            "array of dimensions " + dimensions + " holds " + elements.size()
                + " element(s), want " + size);
      }
    }

    /**
     * Number of dimensions.
     *
     * @return the rank
     */
    public int rank() {
      return dimensions.size();
    }

    /**
     * The element at a multi-index, one coordinate per dimension.
     *
     * @param index the coordinates, each within its dimension
     * @return the element there
     * @throws IndexOutOfBoundsException if the index has the wrong rank or a coordinate is outside
     *     its dimension
     */
    public Value get(long... index) {
      if (index.length != dimensions.size()) {
        throw new IndexOutOfBoundsException(
            "index has " + index.length + " coordinate(s), array has rank " + dimensions.size());
      }
      long flat = 0;
      for (int i = 0; i < index.length; i++) {
        long extent = dimensions.get(i);
        if (index[i] < 0 || index[i] >= extent) {
          throw new IndexOutOfBoundsException(
              "coordinate " + index[i] + " is outside dimension " + i + " of extent " + extent);
        }
        flat = flat * extent + index[i];
      }
      return elements.get((int) flat);
    }
  }

  /**
   * A vector of numbers, each an {@link IntegerValue} or a {@link RealValue} as the model computed
   * it: one value, never a sequence of numbers.
   *
   * <p>Only a service advertising the {@code structured_values} capability reports one as itself
   * rather than as an unsupported {@link NullValue}.
   *
   * @param components the components, in order
   */
  record VectorValue(List<Value> components) implements Value {
    /**
     * Creates a vector, copying the components.
     *
     * @param components the components, each an {@link IntegerValue} or a {@link RealValue}
     * @throws IllegalArgumentException if a component is not a number
     */
    public VectorValue {
      components = List.copyOf(components);
      for (Value component : components) {
        if (!(component instanceof IntegerValue) && !(component instanceof RealValue)) {
          throw new IllegalArgumentException(
              "vector component is not a number: " + component.getClass().getSimpleName());
        }
      }
    }

    /**
     * Number of components.
     *
     * @return the dimension
     */
    public int dimension() {
      return components.size();
    }
  }

  /**
   * A vector whose components are quantities, each with its own unit: {@code VectorOf((3.0, 4.0))
   * [m]} holds two metres. The units usually agree but need not.
   *
   * <p>Only a service advertising the {@code structured_values} capability reports one as itself
   * rather than as an unsupported {@link NullValue}.
   *
   * @param components the components, at least one
   */
  record VectorQuantityValue(List<Quantity> components) implements Value {
    /**
     * Creates a vector quantity, copying the components.
     *
     * @param components the components, never empty
     * @throws IllegalArgumentException if there are no components
     */
    public VectorQuantityValue {
      components = List.copyOf(components);
      if (components.isEmpty()) {
        throw new IllegalArgumentException("vector quantity has no components");
      }
    }

    /**
     * Number of components.
     *
     * @return the dimension
     */
    public int dimension() {
      return components.size();
    }

    /**
     * The one unit every component is written in, or empty when they differ.
     *
     * @return the shared unit as written, when there is one
     */
    public Optional<String> unit() {
      Optional<String> first = components.get(0).unit();
      for (Quantity component : components) {
        if (!component.unit().equals(first)) {
          return Optional.empty();
        }
      }
      return first;
    }
  }

  /**
   * This value as a {@code double}, for the numeric arms.
   *
   * @return the magnitude of an integer, real or quantity value
   * @throws IllegalStateException if this value is not numeric
   */
  default double asDouble() {
    if (this instanceof IntegerValue integer) {
      return integer.value();
    }
    if (this instanceof RealValue real) {
      return real.value();
    }
    if (this instanceof QuantityValue quantity) {
      return quantity.quantity().magnitude().doubleValue();
    }
    throw new IllegalStateException(getClass().getSimpleName() + " is not a numeric value");
  }

  /**
   * This value as a {@code long}, for the integer arm alone.
   *
   * @return the integer value
   * @throws IllegalStateException if this value is not an {@link IntegerValue}
   */
  default long asLong() {
    if (this instanceof IntegerValue integer) {
      return integer.value();
    }
    throw new IllegalStateException(getClass().getSimpleName() + " is not an integer value");
  }

  /**
   * This value as a {@code boolean}.
   *
   * @return the boolean value
   * @throws IllegalStateException if this value is not a {@link BooleanValue}
   */
  default boolean asBoolean() {
    if (this instanceof BooleanValue bool) {
      return bool.value();
    }
    throw new IllegalStateException(getClass().getSimpleName() + " is not a boolean value");
  }

  /**
   * This value as a {@code String}.
   *
   * @return the string value
   * @throws IllegalStateException if this value is not a {@link StringValue}
   */
  default String asString() {
    if (this instanceof StringValue string) {
      return string.value();
    }
    throw new IllegalStateException(getClass().getSimpleName() + " is not a string value");
  }
}
