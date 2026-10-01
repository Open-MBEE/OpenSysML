package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.Map;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.syson.identity.ElementIndex;

class ElementIndexTest {
    private static final String PKG_A = "Pkg::A";
    @Test
    void findsQuotedAndBareQualifiedNames() {
        FakeElement element = new FakeElement(PKG_A);
        ElementIndex index = new ElementIndex(Map.of(PKG_A,
                new ElementIndex.IndexedElement(PKG_A, "id-Pkg::A", "sirius://a", element)));
        assertThat(index.firstNamedIn("bad in 'Pkg::A'").orElseThrow().element()).isSameAs(element);
        assertThat(index.firstNamedIn("bad at Pkg::A").orElseThrow().element()).isSameAs(element);
    }

    @Test
    void findsElementIdsInMessages() {
        FakeElement element = new FakeElement(PKG_A);
        String uuid = "8d21bfea-68e1-40f5-8207-065154a67033";
        ElementIndex.IndexedElement indexed = new ElementIndex.IndexedElement(PKG_A, "id-Pkg::A", "sirius://a",
                element);
        ElementIndex index = new ElementIndex(Map.of(PKG_A, indexed), Map.of(), Map.of(uuid, indexed));
        assertThat(index.firstElementIdIn("Unable to export a SuccessionAsUsage (" + uuid + ") with an implicit target")
                .orElseThrow().element()).isSameAs(element);
        assertThat(index.firstElementIdIn("no identifier here")).isEmpty();
        assertThat(index.firstElementIdIn("unknown 11111111-2222-3333-4444-555555555555 id")).isEmpty();
    }
}
