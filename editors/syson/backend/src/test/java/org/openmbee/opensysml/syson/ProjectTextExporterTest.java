package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.*;

import java.util.List;
import java.util.Map;

import org.eclipse.emf.common.command.BasicCommandStack;
import org.eclipse.emf.common.util.URI;
import org.eclipse.emf.edit.provider.ComposedAdapterFactory;
import org.eclipse.emf.edit.domain.AdapterFactoryEditingDomain;
import org.eclipse.emf.ecore.resource.impl.ResourceImpl;
import org.eclipse.emf.ecore.resource.Resource;
import org.eclipse.emf.ecore.resource.ResourceSet;
import org.eclipse.emf.ecore.resource.impl.ResourceSetImpl;
import org.eclipse.sirius.components.core.api.IIdentityService;
import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.syson.export.ElementSerializer;
import org.openmbee.opensysml.syson.export.ProjectTextExporter;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Status;

class ProjectTextExporterTest {
    @Test
    void mapsOneLineRootAtFirstLine() {
        ResourceSet set = new ResourceSetImpl();
        FakeElement root = new FakeElement("Pkg");
        set.getResources().add(new ResourceImpl(URI.createURI("sirius:///one")));
        set.getResources().get(0).getContents().add(root);
        IEMFEditingContext context = context(set);
        ElementSerializer serializer = (element, report) -> ElementSerializer.Serialization.of("one");

        var result = new ProjectTextExporter(serializer, mock(IIdentityService.class)).export(context);

        assertThat(result.ranges()).singleElement().satisfies(range -> {
            assertThat(range.startLine()).isEqualTo(1);
            assertThat(range.endLine()).isEqualTo(1);
        });
        assertThat(result.elementAt("one.sysml", 1)).isPresent();
    }

    @Test
    void separatesRootsWithoutTrailingNewlines() {
        ResourceSet set = new ResourceSetImpl();
        FakeElement first = new FakeElement("First");
        FakeElement second = new FakeElement("Second");
        Resource resource = new ResourceImpl(URI.createURI("sirius:///two"));
        resource.getContents().add(first);
        resource.getContents().add(second);
        set.getResources().add(resource);
        IEMFEditingContext context = context(set);
        ElementSerializer serializer = (element, report) -> ElementSerializer.Serialization
                .of(element == first ? "first" : "second");

        var result = new ProjectTextExporter(serializer, mock(IIdentityService.class)).export(context);

        assertThat(result.ranges()).extracting(ProjectTextExporterTest::startLine).containsExactly(1, 2);
        assertThat(result.elementAt("two.sysml", 1)).isPresent();
        assertThat(result.elementAt("two.sysml", 2)).isPresent();
    }

    @Test
    void excludesTrailingNewlineFromRootRange() {
        ResourceSet set = new ResourceSetImpl();
        FakeElement first = new FakeElement("First");
        FakeElement second = new FakeElement("Second");
        FakeElement empty = new FakeElement("Empty");
        Resource resource = new ResourceImpl(URI.createURI("sirius:///three"));
        resource.getContents().add(first);
        resource.getContents().add(empty);
        resource.getContents().add(second);
        set.getResources().add(resource);
        IEMFEditingContext context = context(set);
        Map<Object, String> texts = Map.of(first, "part def A;\n", second, "part def B;");
        ElementSerializer serializer = (element, report) -> ElementSerializer.Serialization
                .of(texts.getOrDefault(element, ""));

        var result = new ProjectTextExporter(serializer, mock(IIdentityService.class)).export(context);

        assertThat(result.ranges()).extracting(ProjectTextExporterTest::startLine).containsExactly(1, 2, 2);
        assertThat(result.ranges()).extracting(range -> range.endLine()).containsExactly(1, 2, 2);
        assertThat(result.elementAt("three.sysml", 2)).hasValueSatisfying(element ->
                assertThat(element.element()).isSameAs(second));
    }

    @Test
    void exportsStableDocumentNames() {
        ResourceSet set = new ResourceSetImpl();
        FakeElement root = new FakeElement("Pkg");
        root.addChild(new FakeElement("Pkg::A").addChild(new FakeElement("Pkg::A::x")));
        set.getResources().add(new ResourceImpl(URI.createURI("sirius:///doc-a")));
        set.getResources().get(0).getContents().add(root);
        set.getResources().add(new ResourceImpl(URI.createURI("sysmllibrary:///ScalarValues.sysml")));
        AdapterFactoryEditingDomain domain = new AdapterFactoryEditingDomain(new ComposedAdapterFactory(),
                new BasicCommandStack(), set);
        IEMFEditingContext context = mock(IEMFEditingContext.class);
        when(context.getDomain()).thenReturn(domain);
        var serializer = (org.openmbee.opensysml.syson.export.ElementSerializer) (element, report) -> {
            report.accept(Status.warning("warning"));
            return ElementSerializer.Serialization.of("line 1\nline 2\nline 3");
        };
        IIdentityService identities = mock(IIdentityService.class);
        when(identities.getId(any())).thenAnswer(invocation -> "id-" + ((org.eclipse.syson.sysml.Element) invocation.getArgument(0)).getQualifiedName());
        var result = new ProjectTextExporter(serializer, identities).export(context);
        assertThat(result.documents()).singleElement().isNotNull();
        assertThat(result.documents().get(0).name()).contains("doc-a.sysml");
        assertThat(result.index().byQualifiedName("Pkg::A::x")).isPresent();
        assertThat(result.elementAt("doc-a.sysml", 3)).isPresent();
        assertThat(result.messages()).singleElement().isNotNull();
        assertThat(result.messages().get(0).message()).isEqualTo("warning");
    }

    @Test
    void disambiguatesDuplicateDocumentNames() {
        ResourceSet set = new ResourceSetImpl();
        set.getResources().add(new ResourceImpl(URI.createURI("sirius:///a/foo")));
        set.getResources().get(0).getContents().add(new FakeElement("First"));
        set.getResources().add(new ResourceImpl(URI.createURI("sirius:///b/foo")));
        set.getResources().get(1).getContents().add(new FakeElement("Second"));
        IEMFEditingContext context = context(set);
        ElementSerializer serializer = (element, report) -> ElementSerializer.Serialization.of("part def X;");

        var result = new ProjectTextExporter(serializer, mock(IIdentityService.class)).export(context);

        assertThat(result.documents()).extracting(document -> document.name().orElseThrow())
                .containsExactly("foo.sysml", "foo-1.sysml");
    }

    @Test
    void locatesNestedElementsWithinTheRootRange() {
        ResourceSet set = new ResourceSetImpl();
        FakeElement root = new FakeElement("Pkg");
        FakeElement a = new FakeElement("Pkg::A");
        FakeElement b = new FakeElement("Pkg::A::b");
        root.addChild(a.addChild(b));
        set.getResources().add(new ResourceImpl(URI.createURI("sirius:///nested")));
        set.getResources().get(0).getContents().add(root);
        IEMFEditingContext context = context(set);
        Map<org.eclipse.emf.ecore.EObject, String> fragments = Map.of(root,
                "package Pkg {\n\tpart def A {\n\t\tpart b;\n\t}\n}",
                a, "part def A {\n\tpart b;\n}", b, "part b;");
        ElementSerializer serializer = (element, report) -> new ElementSerializer.Serialization(
                fragments.getOrDefault(element, ""), Map.copyOf(fragments));

        var result = new ProjectTextExporter(serializer, mock(IIdentityService.class)).export(context);

        assertThat(result.ranges()).filteredOn(range -> range.element() == root)
                .singleElement().satisfies(range -> {
                    assertThat(range.startLine()).isEqualTo(1);
                    assertThat(range.endLine()).isEqualTo(5);
                });
        assertThat(result.ranges()).filteredOn(range -> range.element() == a)
                .singleElement().satisfies(range -> {
                    assertThat(range.startLine()).isEqualTo(2);
                    assertThat(range.endLine()).isEqualTo(4);
                });
        assertThat(result.ranges()).filteredOn(range -> range.element() == b)
                .singleElement().satisfies(range -> {
                    assertThat(range.startLine()).isEqualTo(3);
                    assertThat(range.endLine()).isEqualTo(3);
                });
        assertThat(result.elementAt("nested.sysml", 3)).hasValueSatisfying(element ->
                assertThat(element.qualifiedName()).isEqualTo("Pkg::A::b"));
        assertThat(result.elementAt("nested.sysml", 2)).hasValueSatisfying(element ->
                assertThat(element.qualifiedName()).isEqualTo("Pkg::A"));
        assertThat(result.elementAt("nested.sysml", 5)).hasValueSatisfying(element ->
                assertThat(element.qualifiedName()).isEqualTo("Pkg"));
    }

    @Test
    void resolvesAnonymousElementsToTheirEnclosingElement() {
        ResourceSet set = new ResourceSetImpl();
        FakeElement root = new FakeElement("Pkg");
        FakeElement a = new FakeElement("Pkg::A");
        FakeElement anonymous = new FakeElement(null).elementId("anonymous-id");
        root.addChild(a.addChild(anonymous));
        set.getResources().add(new ResourceImpl(URI.createURI("sirius:///anon")));
        set.getResources().get(0).getContents().add(root);
        IEMFEditingContext context = context(set);
        Map<org.eclipse.emf.ecore.EObject, String> fragments = Map.of(root,
                "package Pkg {\n\tpart def A {\n\t\tpart b;\n\t}\n}",
                a, "part def A {\n\tpart b;\n}", anonymous, "part b;");
        ElementSerializer serializer = (element, report) -> new ElementSerializer.Serialization(
                fragments.getOrDefault(element, ""), Map.copyOf(fragments));

        var result = new ProjectTextExporter(serializer, mock(IIdentityService.class)).export(context);

        assertThat(result.elementAt("anon.sysml", 3)).hasValueSatisfying(element ->
                assertThat(element.qualifiedName()).isEqualTo("Pkg::A"));
        assertThat(result.index().byElementId("anonymous-id")).hasValueSatisfying(element ->
                assertThat(element.qualifiedName()).isEqualTo("Pkg::A"));
        assertThat(result.index().enclosing(anonymous)).hasValueSatisfying(element ->
                assertThat(element.qualifiedName()).isEqualTo("Pkg::A"));
    }

    @Test
    void mapsIdenticalSiblingFragmentsInOrder() {
        ResourceSet set = new ResourceSetImpl();
        FakeElement root = new FakeElement("Pkg");
        FakeElement first = new FakeElement("Pkg::first");
        FakeElement second = new FakeElement("Pkg::second");
        root.addChild(first).addChild(second);
        set.getResources().add(new ResourceImpl(URI.createURI("sirius:///twins")));
        set.getResources().get(0).getContents().add(root);
        IEMFEditingContext context = context(set);
        Map<org.eclipse.emf.ecore.EObject, String> fragments = Map.of(root,
                "package Pkg {\n\tpart x;\n\tpart x;\n}", first, "part x;", second, "part x;");
        ElementSerializer serializer = (element, report) -> new ElementSerializer.Serialization(
                fragments.getOrDefault(element, ""), Map.copyOf(fragments));

        var result = new ProjectTextExporter(serializer, mock(IIdentityService.class)).export(context);

        assertThat(result.ranges()).filteredOn(range -> range.element() == first)
                .singleElement().satisfies(range -> assertThat(range.startLine()).isEqualTo(2));
        assertThat(result.ranges()).filteredOn(range -> range.element() == second)
                .singleElement().satisfies(range -> assertThat(range.startLine()).isEqualTo(3));
    }

    @Test
    void locatesDescendantsWhenAChildFragmentIsMissing() {
        ResourceSet set = new ResourceSetImpl();
        FakeElement root = new FakeElement("Pkg");
        FakeElement a = new FakeElement("Pkg::A");
        FakeElement ghost = new FakeElement("Pkg::A::ghost");
        FakeElement real = new FakeElement("Pkg::A::real");
        root.addChild(a.addChild(ghost.addChild(real)));
        set.getResources().add(new ResourceImpl(URI.createURI("sirius:///ghost")));
        set.getResources().get(0).getContents().add(root);
        IEMFEditingContext context = context(set);
        Map<org.eclipse.emf.ecore.EObject, String> fragments = Map.of(root,
                "package Pkg {\n\tpart def A {\n\t\tpart real;\n\t}\n}",
                a, "part def A {\n\tpart real;\n}", ghost, "not present;", real, "part real;");
        ElementSerializer serializer = (element, report) -> new ElementSerializer.Serialization(
                fragments.getOrDefault(element, ""), Map.copyOf(fragments));

        var result = new ProjectTextExporter(serializer, mock(IIdentityService.class)).export(context);

        assertThat(result.ranges()).noneMatch(range -> range.element() == ghost);
        assertThat(result.ranges()).filteredOn(range -> range.element() == real)
                .singleElement().satisfies(range -> {
                    assertThat(range.startLine()).isEqualTo(3);
                    assertThat(range.endLine()).isEqualTo(3);
                });
    }

    private static IEMFEditingContext context(ResourceSet set) {
        AdapterFactoryEditingDomain domain = new AdapterFactoryEditingDomain(new ComposedAdapterFactory(),
                new BasicCommandStack(), set);
        IEMFEditingContext context = mock(IEMFEditingContext.class);
        when(context.getDomain()).thenReturn(domain);
        return context;
    }

    private static int startLine(org.openmbee.opensysml.syson.export.ExportedProject.DocumentRange range) {
        return range.startLine();
    }
}
