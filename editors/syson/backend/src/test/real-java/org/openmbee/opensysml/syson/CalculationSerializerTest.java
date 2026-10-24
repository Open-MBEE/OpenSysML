package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.ArrayList;
import java.util.List;

import org.eclipse.emf.common.util.URI;
import org.eclipse.emf.ecore.resource.Resource;
import org.eclipse.emf.ecore.resource.impl.ResourceImpl;
import org.eclipse.syson.sysml.CalculationDefinition;
import org.eclipse.syson.sysml.CalculationUsage;
import org.eclipse.syson.sysml.Element;
import org.eclipse.syson.sysml.Expression;
import org.eclipse.syson.sysml.Feature;
import org.eclipse.syson.sysml.FeatureDirectionKind;
import org.eclipse.syson.sysml.FeatureMembership;
import org.eclipse.syson.sysml.FeatureReferenceExpression;
import org.eclipse.syson.sysml.FeatureTyping;
import org.eclipse.syson.sysml.FeatureValue;
import org.eclipse.syson.sysml.InvocationExpression;
import org.eclipse.syson.sysml.LiteralString;
import org.eclipse.syson.sysml.Membership;
import org.eclipse.syson.sysml.Namespace;
import org.eclipse.syson.sysml.OwningMembership;
import org.eclipse.syson.sysml.ParameterMembership;
import org.eclipse.syson.sysml.PartDefinition;
import org.eclipse.syson.sysml.PartUsage;
import org.eclipse.syson.sysml.Redefinition;
import org.eclipse.syson.sysml.ReferenceUsage;
import org.eclipse.syson.sysml.ResultExpressionMembership;
import org.eclipse.syson.sysml.Subclassification;
import org.eclipse.syson.sysml.SysmlFactory;
import org.eclipse.syson.sysml.Type;
import org.eclipse.syson.sysml.VisibilityKind;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Severity;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Status;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.syson.export.ElementSerializer;
import org.openmbee.opensysml.syson.export.SysONElementSerializer;

// Serializes calculation constructs SysON's own serializer leaves out, over real metamodel objects.
class CalculationSerializerTest {
    private static final SysmlFactory F = SysmlFactory.eINSTANCE;

    private final org.eclipse.syson.sysml.Package library = pack("DocumentQueries");
    private final org.eclipse.syson.sysml.Package model = pack("Observatory");
    private final CalculationDefinition query = named(own(library, F.createCalculationDefinition()), "Query");
    private final CalculationDefinition project = named(own(library, F.createCalculationDefinition()), "Project");
    private final ReferenceUsage source = parameter(project, "source", null);
    private final ReferenceUsage properties = parameter(project, "properties", null);
    private final ReferenceUsage feature = parameter(project, "feature", null);
    private final org.eclipse.syson.sysml.Class element = named(own(library, F.createClass()), "Element");

    @Test
    void calcDefinitionKeepsParametersAndResultExpression() {
        CalculationDefinition subsystems = named(own(model, F.createCalculationDefinition()), "Subsystems");
        specialize(subsystems, query);
        ReferenceUsage root = parameter(subsystems, "root", element);
        result(subsystems, call(project, source, ref(root), properties, string("name"), feature, string("mass")));

        assertThat(serialize(model)).isEqualTo("""
                package Observatory {
                \tcalc def Subsystems :> DocumentQueries::Query {
                \t\tin root : DocumentQueries::Element;
                \t\tDocumentQueries::Project(source = Subsystems::root, properties = "name", feature = "mass")
                \t}
                }""");
    }

    @Test
    void calcDefinitionWithOnlyAResultOpensABody() {
        CalculationDefinition constant = named(own(model, F.createCalculationDefinition()), "Constant");
        result(constant, string("x"));

        assertThat(serialize(model)).isEqualTo("""
                package Observatory {
                \tcalc def Constant {
                \t\t"x"
                \t}
                }""");
    }

    @Test
    void calcDefinitionWithoutResultStaysCompact() {
        CalculationDefinition abstractQuery = named(own(model, F.createCalculationDefinition()), "Shape");
        parameter(abstractQuery, "root", element);

        assertThat(serialize(model)).isEqualTo("""
                package Observatory {
                \tcalc def Shape {
                \t\tin root : DocumentQueries::Element;
                \t}
                }""");
    }

    @Test
    void privateResultExpressionKeepsItsVisibility() {
        CalculationDefinition constant = named(own(model, F.createCalculationDefinition()), "Constant");
        result(constant, string("x")).setVisibility(VisibilityKind.PRIVATE);

        assertThat(serialize(model)).contains("\t\tprivate \"x\"\n");
    }

    @Test
    void calcUsageIsNotExportedAsAnAction() {
        CalculationDefinition table = named(own(model, F.createCalculationDefinition()), "SubsystemTable");
        parameter(table, "root", element);
        PartDefinition report = named(own(model, F.createPartDefinition()), "MassReport");
        PartUsage telescope = named(own(model, F.createPartUsage()), "telescope");
        CalculationUsage rows = named(feature(report, F.createCalculationUsage()), "rows");
        rows.setIsComposite(true);
        type(rows, table);
        ReferenceUsage root = parameter(rows, "root", null);
        FeatureValue value = F.createFeatureValue();
        value.getOwnedRelatedElement().add(ref(telescope));
        root.getOwnedRelationship().add(value);

        assertThat(serialize(model)).contains("""
                \tpart def MassReport {
                \t\tcalc rows : SubsystemTable {
                \t\t\tin root = telescope;
                \t\t}
                \t}""");
    }

    @Test
    void calcUsageResultExpressionIsKept() {
        PartDefinition report = named(own(model, F.createPartDefinition()), "MassReport");
        CalculationUsage total = named(feature(report, F.createCalculationUsage()), "total");
        total.setIsComposite(true);
        result(total, string("x"));

        assertThat(serialize(model)).contains("\t\tcalc total {\n\t\t\t\"x\"\n\t\t}\n");
    }

    @Test
    void positionalArgumentsStayWithSysON() {
        CalculationDefinition constant = named(own(model, F.createCalculationDefinition()), "Constant");
        InvocationExpression invocation = call(project);
        ParameterMembership membership = F.createParameterMembership();
        Feature argument = F.createFeature();
        FeatureValue value = F.createFeatureValue();
        value.getOwnedRelatedElement().add(string("name"));
        argument.getOwnedRelationship().add(value);
        membership.getOwnedRelatedElement().add(argument);
        invocation.getOwnedRelationship().add(membership);
        result(constant, invocation);

        assertThat(serialize(model)).contains("DocumentQueries::Project(\"name\")");
    }

    @Test
    void argumentWithoutAParameterIsReportedNotSilentlyUnnamed() {
        CalculationDefinition constant = named(own(model, F.createCalculationDefinition()), "Constant");
        InvocationExpression invocation = call(project, source, string("a"));
        ParameterMembership membership = F.createParameterMembership();
        Feature argument = F.createFeature();
        FeatureValue value = F.createFeatureValue();
        value.getOwnedRelatedElement().add(string("b"));
        argument.getOwnedRelationship().add(value);
        membership.getOwnedRelatedElement().add(argument);
        invocation.getOwnedRelationship().add(membership);
        result(constant, invocation);
        List<Status> statuses = new ArrayList<>();
        String text = new SysONElementSerializer().serialize(model, statuses::add).text();

        assertThat(text).contains("DocumentQueries::Project(\"a\", \"b\")");
        assertThat(statuses).filteredOn(status -> status.severity() == Severity.ERROR).singleElement()
                .extracting(Status::message).asString().contains("DocumentQueries::Project").contains("Observatory::Constant");
    }

    @Test
    void argumentNamesNeedingQuotesAreQuoted() {
        ReferenceUsage spaced = parameter(project, "max depth", null);
        CalculationDefinition constant = named(own(model, F.createCalculationDefinition()), "Constant");
        result(constant, call(project, spaced, string("3")));

        assertThat(serialize(model)).contains("DocumentQueries::Project('max depth' = \"3\")");
    }

    @Test
    void referenceLostByTheImportIsReported() {
        CalculationDefinition subsystems = named(own(model, F.createCalculationDefinition()), "Subsystems");
        FeatureReferenceExpression lost = F.createFeatureReferenceExpression();
        lost.getOwnedRelationship().add(F.createMembership());
        result(subsystems, call(project, source, lost));
        List<Status> statuses = new ArrayList<>();
        new SysONElementSerializer().serialize(model, statuses::add);

        assertThat(statuses).filteredOn(status -> status.severity() == Severity.ERROR).singleElement()
                .extracting(Status::message).asString().contains("Observatory::Subsystems").contains("lost");
    }

    @Test
    void nothingIsReportedForSupportedCalculations() {
        CalculationDefinition subsystems = named(own(model, F.createCalculationDefinition()), "Subsystems");
        ReferenceUsage root = parameter(subsystems, "root", element);
        result(subsystems, call(project, source, ref(root)));
        List<Status> statuses = new ArrayList<>();
        new SysONElementSerializer().serialize(model, statuses::add);

        assertThat(statuses).isEmpty();
    }

    private static String serialize(Element root) {
        List<Status> statuses = new ArrayList<>();
        ElementSerializer.Serialization serialized = new SysONElementSerializer().serialize(root, statuses::add);
        return serialized.text();
    }

    private static org.eclipse.syson.sysml.Package pack(String name) {
        Resource resource = new ResourceImpl(URI.createURI("mem:" + name + ".sysml"));
        org.eclipse.syson.sysml.Package pack = named(F.createPackage(), name);
        resource.getContents().add(pack);
        return pack;
    }

    private static <T extends Element> T named(T element, String name) {
        element.setDeclaredName(name);
        return element;
    }

    private static <T extends Element> T own(Namespace owner, T element) {
        OwningMembership membership = F.createOwningMembership();
        membership.getOwnedRelatedElement().add(element);
        owner.getOwnedRelationship().add(membership);
        return element;
    }

    private static <T extends Feature> T feature(Type owner, T feature) {
        FeatureMembership membership = F.createFeatureMembership();
        membership.getOwnedRelatedElement().add(feature);
        owner.getOwnedRelationship().add(membership);
        return feature;
    }

    private static ReferenceUsage parameter(Type owner, String name, Type type) {
        ReferenceUsage parameter = named(F.createReferenceUsage(), name);
        parameter.setDirection(FeatureDirectionKind.IN);
        if (type != null) type(parameter, type);
        return feature(owner, parameter);
    }

    private static void type(Feature feature, Type type) {
        FeatureTyping typing = F.createFeatureTyping();
        typing.setType(type);
        typing.setTypedFeature(feature);
        feature.getOwnedRelationship().add(typing);
    }

    private static void specialize(CalculationDefinition definition, CalculationDefinition general) {
        Subclassification subclassification = F.createSubclassification();
        subclassification.setSuperclassifier(general);
        subclassification.setSubclassifier(definition);
        definition.getOwnedRelationship().add(subclassification);
    }

    private static ResultExpressionMembership result(Type owner, Expression expression) {
        ResultExpressionMembership membership = F.createResultExpressionMembership();
        membership.getOwnedRelatedElement().add(expression);
        owner.getOwnedRelationship().add(membership);
        return membership;
    }

    private static LiteralString string(String value) {
        LiteralString literal = F.createLiteralString();
        literal.setValue(value);
        return literal;
    }

    private static FeatureReferenceExpression ref(Feature feature) {
        FeatureReferenceExpression expression = F.createFeatureReferenceExpression();
        Membership membership = F.createMembership();
        membership.setMemberElement(feature);
        expression.getOwnedRelationship().add(membership);
        return expression;
    }

    // Named arguments the way SysON files them: a parameter redefining the callee's, holding the value.
    private static InvocationExpression call(Type callee, Object... namedArguments) {
        InvocationExpression expression = F.createInvocationExpression();
        type(expression, callee);
        for (int i = 0; i < namedArguments.length; i += 2) {
            ParameterMembership membership = F.createParameterMembership();
            Feature argument = F.createFeature();
            argument.setDirection(FeatureDirectionKind.IN);
            Redefinition redefinition = F.createRedefinition();
            redefinition.setRedefinedFeature((Feature) namedArguments[i]);
            redefinition.setRedefiningFeature(argument);
            argument.getOwnedRelationship().add(redefinition);
            FeatureValue value = F.createFeatureValue();
            value.getOwnedRelatedElement().add((Expression) namedArguments[i + 1]);
            argument.getOwnedRelationship().add(value);
            membership.getOwnedRelatedElement().add(argument);
            expression.getOwnedRelationship().add(membership);
        }
        return expression;
    }
}
