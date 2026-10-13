package org.openmbee.opensysml.syson.run;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.regex.Pattern;

import org.openmbee.opensysml.DocumentValue;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.syson.export.ExportedProject;

// Turns the dialog's name/expression inputs into document-query bindings: a qualified name the
// export or the service knows binds that element (quoted segments included), anything else is
// evaluated and bound as a literal.
public final class DocumentBindings {
    private static final String NAME = "(?:[\\p{L}_][\\p{L}\\p{N}_]*|'[^']+')";
    private static final Pattern QUALIFIED_NAME = Pattern.compile(NAME + "(?:::" + NAME + ")*");

    private DocumentBindings() {}

    public static Map<String, List<DocumentValue>> bind(Model model, ExportedProject project,
            Map<String, String> inputs) {
        Map<String, List<DocumentValue>> bindings = new LinkedHashMap<>();
        inputs.forEach((name, expression) -> bindings.put(name, bind(model, project, name, expression)));
        return bindings;
    }

    static List<DocumentValue> bind(Model model, ExportedProject project, String name, String expression) {
        String text = expression == null ? "" : expression.strip();
        if (project.index().byQualifiedName(text).isPresent()) {
            return List.of(new DocumentValue.ElementRef(text, ""));
        }
        if (QUALIFIED_NAME.matcher(text).matches()) {
            var symbol = model.find(text);
            if (symbol.isPresent()) {
                return List.of(new DocumentValue.ElementRef(symbol.get().id(), ""));
            }
        }
        List<DocumentValue> values = new ArrayList<>();
        collect(model.eval(text), name, text, values);
        return values;
    }

    private static void collect(Value value, String name, String text, List<DocumentValue> into) {
        if (value instanceof Value.Sequence sequence) {
            sequence.elements().forEach(element -> collect(element, name, text, into));
        } else if (value instanceof Value.SetValue set) {
            set.elements().forEach(element -> collect(element, name, text, into));
        } else if (value instanceof Value.ArrayValue array) {
            array.elements().forEach(element -> collect(element, name, text, into));
        } else {
            into.add(scalar(value, name, text));
        }
    }

    private static DocumentValue scalar(Value value, String name, String text) {
        if (value instanceof Value.IntegerValue integer) return new DocumentValue.IntegerValue(integer.value());
        if (value instanceof Value.BigIntegerValue integer) return new DocumentValue.BigIntegerValue(integer.value());
        if (value instanceof Value.RealValue real) return new DocumentValue.RealValue(real.value());
        if (value instanceof Value.RationalValue rational) return new DocumentValue.RationalValue(rational.value());
        if (value instanceof Value.BooleanValue bool) return new DocumentValue.BooleanValue(bool.value());
        if (value instanceof Value.StringValue string) return new DocumentValue.StringValue(string.value());
        if (value instanceof Value.InfinityValue) return new DocumentValue.InfinityValue();
        if (value instanceof Value.QuantityValue quantity) return new DocumentValue.QuantityValue(quantity.quantity());
        if (value instanceof Value.MetaobjectValue metaobject) {
            return new DocumentValue.ElementRef(metaobject.elementId(), metaobject.metaclassId());
        }
        if (value instanceof Value.EnumerationValue enumeration) {
            return new DocumentValue.ElementRef(enumeration.literal().literalId(), "");
        }
        throw new IllegalArgumentException("binding " + name + " = " + text + " evaluates to "
                + ValueText.render(value) + ", which a document query cannot take");
    }
}
