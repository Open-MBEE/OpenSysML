package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.util.List;
import java.util.Map;
import java.util.Optional;

import org.eclipse.syson.sysml.Element;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.DocumentValue;
import org.openmbee.opensysml.Model;
import org.openmbee.opensysml.SourceDocument;
import org.openmbee.opensysml.Symbol;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.syson.export.ExportedProject;
import org.openmbee.opensysml.syson.identity.ElementIndex;
import org.openmbee.opensysml.syson.run.DocumentBindings;

class DocumentBindingsTest {
    private static final String TELESCOPE = "Observatory::telescope";

    private static ExportedProject project() {
        Element element = mock(Element.class);
        return new ExportedProject(List.of(SourceDocument.inline("report.sysml", "")),
                new ElementIndex(Map.of(TELESCOPE, new ElementIndex.IndexedElement(TELESCOPE, "eid", "sid", element))),
                List.of(), List.of());
    }

    @Test
    void bindsAnExportedQualifiedNameWithoutAskingTheService() {
        Model model = mock(Model.class);

        Map<String, List<DocumentValue>> bindings = DocumentBindings.bind(model, project(), Map.of("root", " " + TELESCOPE));

        assertThat(bindings.get("root")).containsExactly(new DocumentValue.ElementRef(TELESCOPE, ""));
        verify(model, never()).find(TELESCOPE);
        verify(model, never()).eval(TELESCOPE);
    }

    @Test
    void bindsALibraryNameTheServiceResolves() {
        Model model = mock(Model.class);
        Symbol symbol = mock(Symbol.class);
        when(symbol.id()).thenReturn("ScalarValues::Real");
        when(model.find("ScalarValues::Real")).thenReturn(Optional.of(symbol));

        Map<String, List<DocumentValue>> bindings = DocumentBindings.bind(model, project(), Map.of("type", "ScalarValues::Real"));

        assertThat(bindings.get("type")).containsExactly(new DocumentValue.ElementRef("ScalarValues::Real", ""));
        verify(model, never()).eval("ScalarValues::Real");
    }

    @Test
    void evaluatesLiteralsAndFlattensSequences() {
        Model model = mock(Model.class);
        when(model.find("threshold")).thenReturn(Optional.empty());
        when(model.eval("\"10\"")).thenReturn(new Value.StringValue("10"));
        when(model.eval("(1, 2.5, true)")).thenReturn(new Value.Sequence(List.of(new Value.IntegerValue(1),
                new Value.RealValue(2.5), new Value.BooleanValue(true))));

        assertThat(DocumentBindings.bind(model, project(), Map.of("t", "\"10\"")).get("t"))
                .containsExactly(new DocumentValue.StringValue("10"));
        assertThat(DocumentBindings.bind(model, project(), Map.of("s", "(1, 2.5, true)")).get("s"))
                .containsExactly(new DocumentValue.IntegerValue(1), new DocumentValue.RealValue(2.5),
                        new DocumentValue.BooleanValue(true));
    }

    @Test
    void fallsBackToEvaluatingANameTheServiceDoesNotKnow() {
        Model model = mock(Model.class);
        when(model.find("true")).thenReturn(Optional.empty());
        when(model.eval("true")).thenReturn(new Value.BooleanValue(true));

        assertThat(DocumentBindings.bind(model, project(), Map.of("flag", "true")).get("flag"))
                .containsExactly(new DocumentValue.BooleanValue(true));
    }

    @Test
    void refusesAValueADocumentQueryCannotTake() {
        Model model = mock(Model.class);
        when(model.eval("#1")).thenReturn(new Value.InstanceReference(1));

        assertThatThrownBy(() -> DocumentBindings.bind(model, project(), Map.of("object", "#1")))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("binding object = #1");
    }

    @Test
    void bindsQuotedQualifiedNames() {
        Model model = mock(Model.class);
        Element element = mock(Element.class);
        String report = "Reports::'Mass Report'";
        ExportedProject project = new ExportedProject(List.of(SourceDocument.inline("report.sysml", "")),
                new ElementIndex(Map.of(report, new ElementIndex.IndexedElement(report, "eid", "sid", element))),
                List.of(), List.of());
        Symbol symbol = mock(Symbol.class);
        when(symbol.id()).thenReturn("Library::'Odd Name'::item");
        when(model.find("Library::'Odd Name'::item")).thenReturn(Optional.of(symbol));

        assertThat(DocumentBindings.bind(model, project, Map.of("doc", report)).get("doc"))
                .containsExactly(new DocumentValue.ElementRef(report, ""));
        assertThat(DocumentBindings.bind(model, project, Map.of("item", "Library::'Odd Name'::item")).get("item"))
                .containsExactly(new DocumentValue.ElementRef("Library::'Odd Name'::item", ""));
        verify(model, never()).eval(org.mockito.ArgumentMatchers.anyString());
    }
}
