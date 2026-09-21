package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.*;

import java.util.List;

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
        ElementSerializer serializer = (element, report) -> "one";

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
        ElementSerializer serializer = (element, report) -> element == first ? "first" : "second";

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
        ElementSerializer serializer = (element, report) -> element == first ? "part def A;\n"
                : element == second ? "part def B;" : "";

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
            return "line 1\nline 2\nline 3";
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
        ElementSerializer serializer = (element, report) -> "part def X;";

        var result = new ProjectTextExporter(serializer, mock(IIdentityService.class)).export(context);

        assertThat(result.documents()).extracting(document -> document.name().orElseThrow())
                .containsExactly("foo.sysml", "foo-1.sysml");
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
