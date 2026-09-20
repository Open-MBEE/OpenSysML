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
    private final Map<String, IndexedElement> byName;
    private final Map<Element, IndexedElement> byElement;

    public ElementIndex(Map<String, IndexedElement> entries) {
        this.byName = Map.copyOf(entries);
        this.byElement = new LinkedHashMap<>();
        entries.values().forEach(value -> byElement.put(value.element(), value));
    }

    public Optional<IndexedElement> byQualifiedName(String name) {
        return Optional.ofNullable(byName.get(name));
    }

    public Optional<IndexedElement> byElement(Element element) {
        return Optional.ofNullable(byElement.get(element));
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
}
