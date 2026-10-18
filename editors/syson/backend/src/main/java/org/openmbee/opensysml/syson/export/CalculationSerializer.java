package org.openmbee.opensysml.syson.export;

import java.util.List;
import java.util.function.Consumer;
import java.util.regex.Pattern;

import org.eclipse.syson.sysml.CalculationDefinition;
import org.eclipse.syson.sysml.CalculationUsage;
import org.eclipse.syson.sysml.Element;
import org.eclipse.syson.sysml.Expression;
import org.eclipse.syson.sysml.Feature;
import org.eclipse.syson.sysml.FeatureReferenceExpression;
import org.eclipse.syson.sysml.FeatureValue;
import org.eclipse.syson.sysml.InvocationExpression;
import org.eclipse.syson.sysml.Membership;
import org.eclipse.syson.sysml.ParameterMembership;
import org.eclipse.syson.sysml.Redefinition;
import org.eclipse.syson.sysml.Relationship;
import org.eclipse.syson.sysml.ResultExpressionMembership;
import org.eclipse.syson.sysml.Specialization;
import org.eclipse.syson.sysml.VisibilityKind;
import org.eclipse.syson.sysml.metamodel.services.textual.SysMLElementSerializer;
import org.eclipse.syson.sysml.metamodel.services.textual.SysMLSerializingOptions;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Appender;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.INameDeresolver;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Status;

/**
 * SysON's serializer (2026.9) emits nothing for a {@code calc def}, prints a {@code calc} usage as an
 * {@code action}, drops result expressions and flattens named arguments into a positional list. This
 * subclass serializes those constructs; everything else stays with SysON.
 */
class CalculationSerializer extends SysMLElementSerializer {
    // Prefixes before a definition or usage keyword come from a closed keyword set that never
    // contains "action", so the first whole word "action" is the keyword itself.
    private static final Pattern ACTION_KEYWORD = Pattern.compile("\\baction\\b");

    private final INameDeresolver names;
    private final String lineSeparator;
    private final String indentation;
    private final Consumer<Status> report;

    CalculationSerializer(SysMLSerializingOptions options, Consumer<Status> report) {
        super(options, report);
        this.names = options.nameDeresolver();
        this.lineSeparator = options.lineSeparator();
        this.indentation = options.indentation();
        this.report = report;
    }

    @Override
    public String caseCalculationDefinition(CalculationDefinition definition) {
        // ActionDefinition is the closest supertype SysON serializes; its body filter keeps the
        // parameters but not the result expression, which is appended as the last body item.
        String text = withCalcKeyword(super.caseActionDefinition(definition));
        ResultExpressionMembership result = definition.getOwnedRelationship().stream()
                .filter(ResultExpressionMembership.class::isInstance).map(ResultExpressionMembership.class::cast)
                .findFirst().orElse(null);
        if (result == null) return text;
        String item = doSwitch(result);
        if (item == null || item.isBlank()) return text;
        return withLastBodyItem(text, item, definition);
    }

    @Override
    public String caseCalculationUsage(CalculationUsage usage) {
        // A usage body is serialized from every owned membership, so its result expression reaches
        // caseResultExpressionMembership on its own.
        return withCalcKeyword(super.caseActionUsage(usage));
    }

    @Override
    public String caseResultExpressionMembership(ResultExpressionMembership membership) {
        Expression expression = membership.getOwnedResultExpression();
        String value = expression == null ? null : doSwitch(expression);
        if (value == null || value.isBlank()) return "";
        VisibilityKind visibility = membership.getVisibility();
        String prefix = visibility == null || visibility == VisibilityKind.PUBLIC ? "" : visibility.getLiteral() + " ";
        return prefix + value;
    }

    // SysON's importer drops a reference it cannot resolve (one to the enclosing calc def's own
    // parameter, for instance), leaving a membership with neither member nor name; say so instead
    // of exporting a hole the service can only report as a syntax error.
    @Override
    public String caseFeatureReferenceExpression(FeatureReferenceExpression expression) {
        boolean unresolved = expression.getOwnedRelationship().stream()
                .filter(Membership.class::isInstance).map(Membership.class::cast)
                .anyMatch(membership -> membership.getMemberElement() == null && membership.getMemberName() == null);
        if (unresolved) {
            report.accept(Status.error("Feature reference in {0} has no referent in the SysON model; the reference was lost when the model was imported",
                    describe(owningNamedElement(expression))));
        }
        return super.caseFeatureReferenceExpression(expression);
    }

    @Override
    public String caseInvocationExpression(InvocationExpression expression) {
        List<ParameterMembership> arguments = expression.getOwnedRelationship().stream()
                .filter(ParameterMembership.class::isInstance).map(ParameterMembership.class::cast).toList();
        Specialization typing = expression.getOwnedRelationship().stream()
                .filter(Specialization.class::isInstance).map(Specialization.class::cast).findFirst().orElse(null);
        long named = arguments.stream().filter(CalculationSerializer::isNamed).count();
        if (typing == null || named == 0) return super.caseInvocationExpression(expression);
        String callee = typing.getGeneral() == null ? null : names.getDeresolvedName(typing.getGeneral(), typing);
        if (callee == null || callee.isBlank()) return super.caseInvocationExpression(expression);
        // SysML allows either all-named or all-positional arguments; a mix only arises when the
        // import could not resolve one argument's parameter, so the names are not silently dropped.
        if (named < arguments.size()) {
            report.accept(Status.error("Call of {0} in {1} names only some of its arguments; the parameter of an unnamed one was lost when the model was imported",
                    callee, describe(owningNamedElement(expression))));
            return super.caseInvocationExpression(expression);
        }
        Appender builder = new Appender(lineSeparator, indentation);
        builder.append(callee).append("(");
        for (int i = 0; i < arguments.size(); i++) {
            if (i > 0) builder.append(", ");
            Feature parameter = arguments.get(i).getOwnedMemberParameter();
            builder.appendPrintableName(redefinedName(parameter)).append(" = ");
            String value = doSwitch(featureValue(parameter).getValue());
            builder.append(value == null ? "" : value);
        }
        return builder.append(")").toString();
    }

    // A named argument is a parameter that explicitly redefines the callee's parameter and holds a value.
    private static boolean isNamed(ParameterMembership membership) {
        Feature parameter = membership.getOwnedMemberParameter();
        return parameter != null && redefinedName(parameter) != null && featureValue(parameter) != null
                && featureValue(parameter).getValue() != null;
    }

    private static String redefinedName(Feature parameter) {
        for (Redefinition redefinition : parameter.getOwnedRedefinition()) {
            if (redefinition.isIsImplied() || redefinition.getRedefinedFeature() == null) continue;
            String name = redefinition.getRedefinedFeature().getName();
            if (name != null && !name.isBlank()) return name;
        }
        return null;
    }

    private static FeatureValue featureValue(Feature parameter) {
        for (Relationship relationship : parameter.getOwnedRelationship()) {
            if (relationship instanceof FeatureValue value) return value;
        }
        return null;
    }

    // The nearest named element outside the expression tree: the calculation or the feature it values.
    private static Element owningNamedElement(Element element) {
        Element owner = element;
        while (owner != null && (owner.getName() == null || owner instanceof Expression
                || owner.getOwner() instanceof Expression)) {
            owner = owner.getOwner();
        }
        return owner == null ? element : owner;
    }

    private static String describe(Element element) {
        String name = element.getQualifiedName() != null ? element.getQualifiedName() : element.getName();
        return name != null ? name : element.eClass().getName() + " " + element.getElementId();
    }

    private static String withCalcKeyword(String text) {
        return text == null ? null : ACTION_KEYWORD.matcher(text).replaceFirst("calc");
    }

    // SysON closes a body as ";" when empty and as "<newline>}" otherwise; the item goes inside either.
    private String withLastBodyItem(String text, String item, CalculationDefinition definition) {
        String indented = indentation + item.replace(lineSeparator, lineSeparator + indentation);
        if (text.endsWith(";")) {
            return text.substring(0, text.length() - 1) + " {" + lineSeparator + indented + lineSeparator + "}";
        }
        if (text.endsWith("}")) {
            return text.substring(0, text.length() - 1) + indented + lineSeparator + "}";
        }
        report.accept(Status.warning("Result expression of {0} could not be placed in its body", definition.getElementId()));
        return text;
    }
}
