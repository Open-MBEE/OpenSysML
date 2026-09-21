package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;
import java.util.Map;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.syson.export.ExportedProject;
import org.openmbee.opensysml.syson.identity.ElementIndex;
import org.openmbee.opensysml.syson.run.DiagnosticMapper;

class DiagnosticMapperTest {
    private static final String PKG_A = "Pkg::A";
    @Test
    void mapsSpanAndNamedElement() {
        FakeElement element = new FakeElement(PKG_A);
        ElementIndex index = new ElementIndex(Map.of(PKG_A,
                new ElementIndex.IndexedElement(PKG_A, "id-Pkg::A", "sirius://a", element)));
        ExportedProject project = new ExportedProject(List.of(), index, List.of(),
                List.of(new ExportedProject.DocumentRange("doc.sysml", 1, 3, element)));
        Diagnostic diagnostic = new Diagnostic(Diagnostic.Severity.ERROR, "failed in Pkg::A", "syntax",
                java.util.Optional.of(new Diagnostic.Span("doc.sysml", 2, 1, 2, 2)));
        var mapped = DiagnosticMapper.map(diagnostic, project);
        assertThat(mapped.diagnostic().qualifiedName()).isEqualTo(PKG_A);
        assertThat(mapped.element()).isSameAs(element);
    }
}
