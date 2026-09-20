package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.*;

import java.util.List;

import org.eclipse.emf.common.command.BasicCommandStack;
import org.eclipse.emf.common.util.URI;
import org.eclipse.emf.edit.provider.ComposedAdapterFactory;
import org.eclipse.emf.edit.domain.AdapterFactoryEditingDomain;
import org.eclipse.emf.ecore.resource.impl.ResourceImpl;
import org.eclipse.emf.ecore.resource.ResourceSet;
import org.eclipse.emf.ecore.resource.impl.ResourceSetImpl;
import org.eclipse.sirius.components.core.api.IIdentityService;
import org.eclipse.sirius.components.emf.services.api.IEMFEditingContext;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.syson.export.ProjectTextExporter;
import org.eclipse.syson.sysml.metamodel.services.textual.utils.Status;

class ProjectTextExporterTest {
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
}
