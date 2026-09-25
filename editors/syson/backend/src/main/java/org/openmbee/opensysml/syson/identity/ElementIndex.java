package org.openmbee.opensysml.syson.identity;

import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Optional;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import org.eclipse.syson.sysml.Element;

public final class ElementIndex {
    public record IndexedElement(String qualifiedName, String elementId, String siriusId, Element element) {}

    private static final Pattern QUOTED = Pattern.compile("['`]([^'`]+)['`]");
    private static final Pattern QUALIFIED = Pattern.compile("\\b[A-Za-z_]\\w*(::[A-Za-z_]\\w*)+\\b");
    private static final Pattern UUID = Pattern
            .compile("[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}");
    private final Map<String, IndexedElement> byName;
    private final Map<Element, IndexedElement> byElement;
    private final Map<Element, IndexedElement> enclosing;
    private final Map<String, IndexedElement> byElementId;

    public ElementIndex(Map<String, IndexedElement> entries) {
        this(entries, Map.of(), Map.of());
    }

    public ElementIndex(Map<String, IndexedElement> entries, Map<Element, IndexedElement> enclosing,
            Map<String, IndexedElement> byElementId) {
        this.byName = Map.copyOf(entries);
        this.byElement = new LinkedHashMap<>();
        entries.values().forEach(value -> this.byElement.put(value.element(), value));
        this.enclosing = Map.copyOf(enclosing);
        this.byElementId = Map.copyOf(byElementId);
    }

    public Optional<IndexedElement> byQualifiedName(String name) {
        return Optional.ofNullable(byName.get(name));
    }

    public Optional<IndexedElement> byElement(Element element) {
        return Optional.ofNullable(byElement.get(element));
    }

    // The nearest named element at or above this one; an element's own entry when it has one.
    public Optional<IndexedElement> enclosing(Element element) {
        IndexedElement own = byElement.get(element);
        return Optional.ofNullable(own != null ? own : enclosing.get(element));
    }

    public Optional<IndexedElement> byElementId(String elementId) {
        return Optional.ofNullable(byElementId.get(elementId));
    }

    public Optional<IndexedElement> firstNamedIn(String message) {
        Matcher quoted = QUOTED.matcher(message);
        while (quoted.find()) {
            Optional<IndexedElement> match = byQualifiedName(quoted.group(1));
            if (match.isPresent()) return match;
        }
        Matcher bare = QUALIFIED.matcher(message);
        while (bare.find()) {
            Optional<IndexedElement> match = byQualifiedName(bare.group());
            if (match.isPresent()) return match;
        }
        return Optional.empty();
    }

    public Optional<IndexedElement> firstElementIdIn(String message) {
        Matcher matcher = UUID.matcher(message);
        while (matcher.find()) {
            Optional<IndexedElement> match = byElementId(matcher.group());
            if (match.isPresent()) return match;
        }
        return Optional.empty();
    }
}
