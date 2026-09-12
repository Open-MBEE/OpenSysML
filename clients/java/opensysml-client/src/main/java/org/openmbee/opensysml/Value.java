package org.openmbee.opensysml;

import java.math.BigInteger;
import java.util.HashMap;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
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
   * The unbounded value {@code *}: no number, ordered above every finite magnitude.
   *
   * <p>Only a service advertising the {@code infinity_value} capability reports it as itself rather
   * than as an unsupported {@link NullValue}.
   */
  record InfinityValue() implements Value {}

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
   * An element of the model held as an instance of its reflective metaclass: what {@code x meta
   * KerML::Feature}, or the last element of {@code x.metadata}, evaluates to.
   *
   * <p>It is the element it reflects on, which is its identity: two metaobjects are equal exactly
   * when {@code elementId} is, whatever type each was cast to. Its features ({@code declaredName},
   * {@code ownedFeature}, ...) are read in the model, not carried. Only a service advertising the
   * {@code metaobject_values} capability reports one as itself rather than as an unsupported
   * {@link NullValue}.
   *
   * @param elementId FQN of the element reflected on ({@code "Vehicle::seatBelt"}), never empty
   * @param metaclassId FQN of the element's own reflective metaclass ({@code
   *     "SysML::Systems::PartUsage"}), not the type it was cast to; the service always sends it
   */
  record MetaobjectValue(String elementId, String metaclassId) implements Value {
    /**
     * Creates a metaobject.
     *
     * @param elementId the element reflected on, never {@code null} or empty
     * @param metaclassId the element's own metaclass, never {@code null}
     * @throws IllegalArgumentException if {@code elementId} is empty
     */
    public MetaobjectValue {
      Objects.requireNonNull(elementId, "elementId");
      Objects.requireNonNull(metaclassId, "metaclassId");
      if (elementId.isEmpty()) {
        throw new IllegalArgumentException("a metaobject names no element");
      }
    }

    /** The element is the identity, whatever type each side was cast to. */
    @Override
    public boolean equals(Object other) {
      return other instanceof MetaobjectValue that && elementId.equals(that.elementId);
    }

    @Override
    public int hashCode() {
      return elementId.hashCode();
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
   * A unique, unordered collection: a {@code Collections::Set}'s elements.
   *
   * <p>The service sends the members in its canonical order (numbers ascending, then strings, and
   * so on), each exactly once; two sets are equal when they hold the same members in any order.
   * Only a service advertising the {@code set_values} capability reports one as itself rather
   * than as an unsupported {@link NullValue}.
   *
   * <p>Membership, and so equality and the refusal of a member listed twice, are judged by {@link
   * Value#sameValue}, as the service judges them: {@code 1} and {@code 1.0} are one member.
   *
   * @param elements the members, each once, in the order the service sent them
   */
  record SetValue(List<Value> elements) implements Value {
    /**
     * Creates a set, copying the members.
     *
     * @param elements the members, never {@code null}
     * @throws IllegalArgumentException if a member is listed twice
     */
    public SetValue {
      elements = List.copyOf(elements);
      for (int i = 0; i < elements.size(); i++) {
        if (holds(elements.subList(0, i), elements.get(i))) {
          throw new IllegalArgumentException("set lists a member twice: " + elements.get(i));
        }
      }
    }

    private static boolean holds(List<Value> members, Value value) {
      for (Value member : members) {
        if (member.sameValue(value)) {
          return true;
        }
      }
      return false;
    }

    /**
     * Number of members.
     *
     * @return the size
     */
    public int size() {
      return elements.size();
    }

    /**
     * Whether the set has no members.
     *
     * @return {@code true} for the empty set
     */
    public boolean isEmpty() {
      return elements.isEmpty();
    }

    /**
     * Whether a value is a member.
     *
     * @param value the value to look for
     * @return {@code true} when the set holds it
     */
    public boolean contains(Value value) {
      return holds(elements, value);
    }

    /** Order-insensitive: the same members in any order are the same set. */
    @Override
    public boolean equals(Object other) {
      if (!(other instanceof SetValue that) || elements.size() != that.elements.size()) {
        return false;
      }
      for (Value element : elements) {
        if (!that.contains(element)) {
          return false;
        }
      }
      return true;
    }

    @Override
    public int hashCode() {
      int hash = 0;
      for (Value element : elements) {
        hash += valueHash(element);
      }
      return hash;
    }

    /** A hash consistent with {@link Value#sameValue}: values the model equates hash alike. */
    private static int valueHash(Value value) {
      if (Value.isEmpty(value)) {
        return 0;
      }
      Number magnitude = onRealAxis(value);
      if (magnitude != null) {
        return Double.hashCode(magnitude.doubleValue() + 0.0);
      }
      if (value instanceof QuantityValue quantity) {
        Optional<Quantity.UnitTerm> reduction = quantity.quantity().reduction();
        if (reduction.isPresent()) {
          return exponents(reduction.get()).hashCode();
        }
        return Double.hashCode(quantity.quantity().magnitude().doubleValue() + 0.0)
            ^ quantity.quantity().unit().hashCode();
      }
      if (value instanceof MeasurementRefValue ref) {
        return exponents(ref.reduction()).hashCode();
      }
      if (value instanceof EnumerationValue literal) {
        return literal.literal().literalId().hashCode();
      }
      if (value instanceof SetValue
          || value instanceof Sequence
          || value instanceof ArrayValue
          || value instanceof VectorValue
          || value instanceof VectorQuantityValue
          || value instanceof TensorQuantityValue) {
        return value.getClass().hashCode();
      }
      return value.hashCode();
    }
  }

  /**
   * A tensor of quantities of any rank: its shape and its components flattened in row-major order,
   * each a {@link Quantity} with its own unit. A rank-one tensor stays a tensor, distinct from a
   * {@link VectorQuantityValue}.
   *
   * <p>Only a service advertising the {@code tensor_values} capability reports one as itself
   * rather than as an unsupported {@link NullValue}.
   *
   * @param dimensions the extent of each dimension, all positive
   * @param components the components, row-major, exactly as many as the dimensions multiply to
   */
  record TensorQuantityValue(List<Long> dimensions, List<Quantity> components) implements Value {
    /**
     * Creates a tensor, copying the dimensions and the components.
     *
     * @param dimensions the extent of each dimension, never {@code null}
     * @param components the components, never {@code null}
     * @throws IllegalArgumentException if a dimension is not positive, the dimensions overflow a
     *     {@code long}, or the components do not fill the dimensions exactly
     */
    public TensorQuantityValue {
      dimensions = List.copyOf(dimensions);
      components = List.copyOf(components);
      long size = 1;
      for (long extent : dimensions) {
        if (extent <= 0) {
          throw new IllegalArgumentException("tensor dimension is not positive: " + extent);
        }
        try {
          size = Math.multiplyExact(size, extent);
        } catch (ArithmeticException overflow) {
          throw new IllegalArgumentException("tensor dimensions overflow: " + dimensions, overflow);
        }
      }
      if (size != components.size()) {
        throw new IllegalArgumentException(
            "tensor of dimensions " + dimensions + " holds " + components.size()
                + " component(s), want " + size);
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
     * The component at a multi-index, one coordinate per dimension.
     *
     * @param index the coordinates, each within its dimension
     * @return the component there
     * @throws IndexOutOfBoundsException if the index has the wrong rank or a coordinate is outside
     *     its dimension
     */
    public Quantity get(long... index) {
      if (index.length != dimensions.size()) {
        throw new IndexOutOfBoundsException(
            "index has " + index.length + " coordinate(s), tensor has rank " + dimensions.size());
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
      return components.get((int) flat);
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
   * Whether this is the same value as another to the model, as the service judges a set's
   * membership: numbers by value, so a whole {@link RealValue} is the {@link IntegerValue} of its
   * value and a {@link ComplexValue} on the real axis is its real part, exactly across the whole
   * {@code long} range; a sequence's order counts and a set's does not; a quantity is compared
   * over its base units, so {@code 1 [m]} is {@code 100 [cm]} — exactly while integer magnitudes
   * scale by whole factors — and one lacking a reduction is compared in its unit as written; a
   * {@link MeasurementRefValue} is one reduction at one scale however spelt, except that a named
   * unit of dimension one is only its own declaration ({@code rad} is not {@code sr}); an {@link
   * EnumerationValue} is its {@link EnumLiteral#literalId()}, whatever else describes it; a {@link
   * NullValue}, an empty sequence and an empty set are one value, the model's absent value however
   * spelt. Every other arm compares as {@link Object#equals} does, which stays structural: {@code
   * new IntegerValue(1).equals(new RealValue(1.0))} is {@code false}.
   *
   * @param other the value to compare with
   * @return {@code true} when the model would not tell the two apart
   */
  default boolean sameValue(Value other) {
    Objects.requireNonNull(other, "other");
    if (isEmpty(this) || isEmpty(other)) {
      return isEmpty(this) && isEmpty(other);
    }
    if (this instanceof IntegerValue || this instanceof RealValue || this instanceof ComplexValue) {
      return numbersEqual(this, other);
    }
    if (this instanceof Sequence a && other instanceof Sequence b) {
      return sameValues(a.elements(), b.elements());
    }
    if (this instanceof QuantityValue a && other instanceof QuantityValue b) {
      return quantitiesEqual(a.quantity(), b.quantity());
    }
    if (this instanceof ArrayValue a && other instanceof ArrayValue b) {
      return a.dimensions().equals(b.dimensions()) && sameValues(a.elements(), b.elements());
    }
    if (this instanceof VectorValue a && other instanceof VectorValue b) {
      return sameValues(a.components(), b.components());
    }
    if (this instanceof VectorQuantityValue a && other instanceof VectorQuantityValue b) {
      return sameQuantities(a.components(), b.components());
    }
    if (this instanceof TensorQuantityValue a && other instanceof TensorQuantityValue b) {
      return a.dimensions().equals(b.dimensions())
          && sameQuantities(a.components(), b.components());
    }
    if (this instanceof MeasurementRefValue a && other instanceof MeasurementRefValue b) {
      return measurementRefsEqual(a, b);
    }
    if (this instanceof EnumerationValue a && other instanceof EnumerationValue b) {
      return a.literal().literalId().equals(b.literal().literalId());
    }
    return equals(other);
  }

  /** The model's absent value: a null, or a collection with no members. */
  private static boolean isEmpty(Value value) {
    return value instanceof NullValue
        || (value instanceof Sequence sequence && sequence.elements().isEmpty())
        || (value instanceof SetValue set && set.elements().isEmpty());
  }

  // One reduction at one scale (SI::'m/s' is m/s, km/m is m/mm); a named unit
  // reducing to nothing is only the declaration it names.
  private static boolean measurementRefsEqual(MeasurementRefValue a, MeasurementRefValue b) {
    if (!sameReduction(a.reduction(), b.reduction())) {
      return false;
    }
    if (exponents(a.reduction()).isEmpty() && (a.unitId().isPresent() || b.unitId().isPresent())) {
      return a.unitId().equals(b.unitId());
    }
    return true;
  }

  /** One reduction: commensurable at one scale, however the ratio is written. */
  private static boolean sameReduction(Quantity.UnitTerm x, Quantity.UnitTerm y) {
    return exponents(x).equals(exponents(y))
        && !zeroScale(x)
        && !zeroScale(y)
        && x.scaleNumerator() * y.scaleDenominator() == y.scaleNumerator() * x.scaleDenominator();
  }

  private static boolean numbersEqual(Value a, Value b) {
    Number x = onRealAxis(a);
    Number y = onRealAxis(b);
    return x != null && y != null ? magnitudesEqual(x, y) : a.equals(b);
  }

  /** A number's magnitude as a {@link Long} or {@link Double}; {@code null} off the real axis. */
  private static Number onRealAxis(Value value) {
    if (value instanceof IntegerValue integer) {
      return integer.value();
    }
    if (value instanceof RealValue real) {
      return real.value();
    }
    if (value instanceof ComplexValue complex && complex.imaginary() == 0.0) {
      return complex.real();
    }
    return null;
  }

  private static boolean magnitudesEqual(Number a, Number b) {
    if (a instanceof Long x) {
      return b instanceof Long y ? x.longValue() == y : realIsLong(b.doubleValue(), x);
    }
    return b instanceof Long y ? realIsLong(a.doubleValue(), y) : a.doubleValue() == b.doubleValue();
  }

  // Whether r is exactly the integer n, never rounding n.
  private static boolean realIsLong(double r, long n) {
    return r == Math.rint(r) && r >= -0x1p63 && r < 0x1p63 && (long) r == n;
  }

  private static boolean sameValues(List<Value> a, List<Value> b) {
    if (a.size() != b.size()) {
      return false;
    }
    Iterator<Value> others = b.iterator();
    for (Value value : a) {
      if (!value.sameValue(others.next())) {
        return false;
      }
    }
    return true;
  }

  private static boolean quantitiesEqual(Quantity a, Quantity b) {
    if (a.reduction().isEmpty() || b.reduction().isEmpty()) {
      return magnitudesEqual(a.magnitude(), b.magnitude())
          && a.unit().equals(b.unit())
          && a.reduction().equals(b.reduction());
    }
    Quantity.UnitTerm x = a.reduction().get();
    Quantity.UnitTerm y = b.reduction().get();
    if (!exponents(x).equals(exponents(y)) || zeroScale(x) || zeroScale(y)) {
      return false;
    }
    BigInteger[] m = exactBaseMagnitude(a);
    BigInteger[] n = exactBaseMagnitude(b);
    if (m.length != 0 && n.length != 0) {
      return m[0].multiply(n[1]).equals(n[0].multiply(m[1]));
    }
    return baseMagnitude(a) == baseMagnitude(b);
  }

  /** The base-unit exponents, repeated units summed and cancelled ones dropped. */
  private static Map<String, Double> exponents(Quantity.UnitTerm term) {
    Map<String, Double> totals = new HashMap<>();
    for (Quantity.UnitFactor factor : term.factors()) {
      totals.merge(factor.unitId(), factor.exponent(), Double::sum);
    }
    totals.values().removeIf(exponent -> exponent == 0.0);
    return totals;
  }

  private static boolean zeroScale(Quantity.UnitTerm term) {
    return term.scaleNumerator() == 0.0 || term.scaleDenominator() == 0.0;
  }

  private static double baseMagnitude(Quantity quantity) {
    Quantity.UnitTerm term = quantity.reduction().get();
    return quantity.magnitude().doubleValue() * term.scaleNumerator() / term.scaleDenominator();
  }

  /**
   * The base magnitude as an exact numerator/denominator while an integer scales by whole factors;
   * empty otherwise.
   */
  private static BigInteger[] exactBaseMagnitude(Quantity quantity) {
    Quantity.UnitTerm term = quantity.reduction().get();
    if (!(quantity.magnitude() instanceof Long magnitude)
        || !isWhole(term.scaleNumerator())
        || !isWhole(term.scaleDenominator())) {
      return new BigInteger[0];
    }
    return new BigInteger[] {
      BigInteger.valueOf(magnitude).multiply(wholeOf(term.scaleNumerator())),
      wholeOf(term.scaleDenominator())
    };
  }

  private static boolean isWhole(double scale) {
    return scale == Math.rint(scale) && !Double.isInfinite(scale);
  }

  // The integer a whole double is, exactly as the service reads it: beyond a long, its
  // significand shifted by its exponent rather than its shortest decimal rendering.
  private static BigInteger wholeOf(double scale) {
    if (Math.abs(scale) < 0x1p63) {
      return BigInteger.valueOf((long) scale);
    }
    long significand = (Double.doubleToLongBits(scale) & ((1L << 52) - 1)) | (1L << 52);
    BigInteger whole = BigInteger.valueOf(significand).shiftLeft(Math.getExponent(scale) - 52);
    return scale < 0 ? whole.negate() : whole;
  }

  private static boolean sameQuantities(List<Quantity> a, List<Quantity> b) {
    if (a.size() != b.size()) {
      return false;
    }
    Iterator<Quantity> others = b.iterator();
    for (Quantity quantity : a) {
      if (!quantitiesEqual(quantity, others.next())) {
        return false;
      }
    }
    return true;
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
