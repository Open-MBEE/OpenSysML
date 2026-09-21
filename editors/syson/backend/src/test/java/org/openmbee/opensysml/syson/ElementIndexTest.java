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
}
