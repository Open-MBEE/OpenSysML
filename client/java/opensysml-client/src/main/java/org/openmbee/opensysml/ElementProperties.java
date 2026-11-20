package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * What one {@code GetElementProperties} read answered: the element, and its metamodel properties
 * by their specification names ({@code inheritedFeature}, {@code connectorEnd}, …) as the model's
 * API element form spells them, in the order asked for, or the element form's order when none was
 * named.
 *
 * @param element the element read, whose {@link ElementRef#id()} names it again
 * @param properties one entry per property, answered or not
 * @param excludedDocuments the documents the model was read without; empty when the whole model
 *     was read
 */
public record ElementProperties(
    ElementRef element,
    List<ElementProperties.Property> properties,
    List<ElementProperties.ExcludedDocument> excludedDocuments) {
  /**
   * Creates a read, copying the properties and the excluded documents.
   *
   * @param element the element read
   * @param properties the properties answered
   * @param excludedDocuments the documents the model was read without
   */
  public ElementProperties {
    Objects.requireNonNull(element, "element");
    properties = List.copyOf(properties);
    excludedDocuments = List.copyOf(excludedDocuments);
  }

  /**
   * Creates a read of a whole model, with no excluded document.
   *
   * @param element the element read
   * @param properties the properties answered
   */
  public ElementProperties(ElementRef element, List<ElementProperties.Property> properties) {
    this(element, properties, List.of());
  }

  /**
   * A document the read left out of the model: one the parser could not read whole, or one
   * declaring an element another document already declares.
   *
   * @param document the document's name as the parse request gave it
   * @param error why it was left out
   */
  public record ExcludedDocument(String document, String error) {
    /** Creates an excluded document. */
    public ExcludedDocument {
      Objects.requireNonNull(document, "document");
      Objects.requireNonNull(error, "error");
    }
  }

  /**
   * The entry for one property.
   *
   * @param name the specification name
   * @return the entry, absent when the read did not ask for it
   */
  public Optional<Property> get(String name) {
    Objects.requireNonNull(name, "name");
    return properties.stream().filter(property -> property.name().equals(name)).findFirst();
  }

  /**
   * Whether the read answered the named property.
   *
   * @param name the specification name
   * @return whether an entry carries it
   */
  public boolean has(String name) {
    return get(name).isPresent();
  }

  /**
   * One metamodel property of an element.
   *
   * @param name the specification name ({@code "connectorEnd"})
   * @param definingClass the metaclass declaring the property ({@code "Connector"}); empty when the
   *     element's metaclass declares no such property
   * @param many whether the property holds a collection
   * @param derived whether the specification derives the property
   * @param values the values in the collection's order; empty for an empty or null property
   * @param unsupported why there are no values when the property cannot be answered: the metaclass
   *     declares no such property, or OpenSysML does not derive it yet; absent when answered
   */
  public record Property(
      String name,
      String definingClass,
      boolean many,
      boolean derived,
      List<PropertyValue> values,
      Optional<String> unsupported) {
    /**
     * Creates a property, copying its values.
     *
     * @param name the specification name, never {@code null}
     * @param definingClass the declaring metaclass
     * @param many whether it is a collection
     * @param derived whether it is derived
     * @param values the values
     * @param unsupported why it has none, when unanswered
     */
    public Property {
      Objects.requireNonNull(name, "name");
      Objects.requireNonNull(definingClass, "definingClass");
      values = List.copyOf(values);
      Objects.requireNonNull(unsupported, "unsupported");
    }

    /**
     * Whether the property was answered, with or without values.
     *
     * @return {@code true} unless {@link #unsupported()} says why not
     */
    public boolean supported() {
      return unsupported.isEmpty();
    }

    /**
     * The one value of a single-valued property.
     *
     * @return the first value, absent when there is none
     */
    public Optional<PropertyValue> value() {
      return values.isEmpty() ? Optional.empty() : Optional.of(values.get(0));
    }

    /**
     * The element references among the values, in order.
     *
     * @return the referenced elements
     */
    public List<ElementRef> elements() {
      return values.stream()
          .filter(PropertyValue.Element.class::isInstance)
          .map(value -> ((PropertyValue.Element) value).element())
          .toList();
    }
  }

  /** One value of a metamodel property, as the element form spells it. */
  public sealed interface PropertyValue {
    /**
     * A reference to an element, which {@link Model#elementProperties} accepts back.
     *
     * @param element the referenced element
     */
    record Element(ElementRef element) implements PropertyValue {
      /**
       * Creates an element value.
       *
       * @param element the referenced element, never {@code null}
       */
      public Element {
        Objects.requireNonNull(element, "element");
      }
    }

    /**
     * A string, an enumeration literal included.
     *
     * @param value the text
     */
    record Text(String value) implements PropertyValue {
      /**
       * Creates a text value.
       *
       * @param value the text, never {@code null}
       */
      public Text {
        Objects.requireNonNull(value, "value");
      }
    }

    /**
     * A boolean.
     *
     * @param value the truth value
     */
    record Bool(boolean value) implements PropertyValue {}

    /**
     * An integer {@code int64} holds.
     *
     * @param value the integer
     */
    record Integer(long value) implements PropertyValue {}

    /**
     * A real {@code float64} holds.
     *
     * @param value the real
     */
    record Real(double value) implements PropertyValue {}

    /**
     * A number beyond {@code int64} and {@code float64}, as its decimal text.
     *
     * @param digits the decimal text
     */
    record Number(String digits) implements PropertyValue {
      /**
       * Creates a number value.
       *
       * @param digits the decimal text, never {@code null}
       */
      public Number {
        Objects.requireNonNull(digits, "digits");
      }
    }
  }
}
